package tracer

import (
	"fmt"
	"go/ast"
	"go/types"
	"slices"
	"sort"
)

type typeFieldCandidate struct {
	field *types.Var
	path  []int
	order int
}

type embeddedStructLevel struct {
	structure *types.Struct
	path      []int
	ancestors map[types.Type]bool
}

func semanticStructType(typ types.Type) (*types.Struct, types.Type) {
	if typ == nil {
		return nil, nil
	}
	typ = types.Unalias(typ)
	if pointer, ok := typ.(*types.Pointer); ok {
		typ = types.Unalias(pointer.Elem())
	}
	switch typ := typ.(type) {
	case *types.Named:
		structure, _ := typ.Underlying().(*types.Struct)
		return structure, typ
	case *types.Struct:
		return typ, typ
	default:
		return nil, typ
	}
}

func cloneTypeAncestors(ancestors map[types.Type]bool) map[types.Type]bool {
	result := make(map[types.Type]bool, len(ancestors)+1)
	for typ := range ancestors {
		result[typ] = true
	}
	return result
}

func appendFieldPath(prefix []int, index int) []int {
	result := make([]int, len(prefix)+1)
	copy(result, prefix)
	result[len(prefix)] = index
	return result
}

func collectTypeFieldCandidates(root types.Type) ([]typeFieldCandidate, error) {
	structure, key := semanticStructType(root)
	if structure == nil {
		return nil, fmt.Errorf("underlying type is not a struct")
	}
	ancestors := make(map[types.Type]bool)
	if key != nil {
		ancestors[key] = true
	}
	queue := []embeddedStructLevel{{structure: structure, ancestors: ancestors}}
	candidates := make([]typeFieldCandidate, 0, structure.NumFields())

	for len(queue) > 0 {
		level := queue[0]
		queue = queue[1:]
		for index := 0; index < level.structure.NumFields(); index++ {
			field := level.structure.Field(index)
			path := appendFieldPath(level.path, index)
			candidates = append(candidates, typeFieldCandidate{
				field: field, path: path, order: len(candidates),
			})
			if !field.Embedded() {
				continue
			}

			embedded, embeddedKey := semanticStructType(field.Type())
			if embedded == nil || embeddedKey == nil || level.ancestors[embeddedKey] {
				continue
			}
			childAncestors := cloneTypeAncestors(level.ancestors)
			childAncestors[embeddedKey] = true
			queue = append(queue, embeddedStructLevel{
				structure: embedded,
				path:      path,
				ancestors: childAncestors,
			})
		}
	}
	return candidates, nil
}

func selectedTypeFields(root types.Type, pkg *types.Package) ([]typeFieldCandidate, error) {
	candidates, err := collectTypeFieldCandidates(root)
	if err != nil {
		return nil, err
	}
	byName := make(map[string][]typeFieldCandidate)
	for _, candidate := range candidates {
		name := candidate.field.Name()
		byName[name] = append(byName[name], candidate)
	}

	selected := make([]typeFieldCandidate, 0, len(byName))
	for name, namedCandidates := range byName {
		object, index, _ := types.LookupFieldOrMethod(types.Unalias(root), true, pkg, name)
		if _, ok := object.(*types.Var); !ok {
			continue
		}
		for _, candidate := range namedCandidates {
			if slices.Equal(candidate.path, index) {
				selected = append(selected, candidate)
				break
			}
		}
	}
	sort.Slice(selected, func(left, right int) bool {
		return selected[left].order < selected[right].order
	})
	return selected, nil
}

func (r *typeResolver) sourceForField(field *types.Var) (*typeFieldSource, error) {
	if field == nil {
		return nil, fmt.Errorf("nil struct field: %w", ErrMissingTypeInfo)
	}
	origin := field.Origin()
	pkg, ok := r.tracer.packageForObject(origin)
	if !ok {
		return nil, fmt.Errorf("field %s: %w", field.Name(), ErrMissingTypeInfo)
	}
	index, err := r.packageIndex(pkg)
	if err != nil {
		return nil, err
	}
	source, ok := index.fields[origin]
	if !ok {
		return nil, fmt.Errorf("field %s: %w", field.Name(), ErrMissingTypeInfo)
	}
	return source, nil
}

func explicitASTField(source *typeFieldSource) *ast.Field {
	field := source.field
	return &ast.Field{
		Doc:     field.Doc,
		Names:   []*ast.Ident{source.name},
		Type:    field.Type,
		Tag:     field.Tag,
		Comment: field.Comment,
	}
}

func typeFieldError(st *ast.StructType, node *TypeSpecNode, cause error) error {
	if cause == nil {
		return nil
	}
	position := node.Fset.Position(node.Spec.Pos())
	expr := ast.Expr(node.Spec.Type)
	if st != nil {
		position = node.Fset.Position(st.Pos())
		expr = st
	}
	return &ErrExprProcessFailed{Pos: position, Expr: expr, Msg: cause.Error()}
}

// OrderedStructFields returns the visible fields in stable breadth-first source
// order after applying Go's promotion, shadowing, and ambiguity rules.
func OrderedStructFields(st *ast.StructType, node *TypeSpecNode) ([]*ast.Field, error) {
	if node == nil || node.Spec == nil || node.resolver == nil || node.typ == nil || node.Fset == nil {
		return nil, &ErrExprProcessFailed{Expr: st, Msg: ErrMissingTypeInfo.Error()}
	}
	selected, err := selectedTypeFields(node.typ, node.Pkg.Types)
	if err != nil {
		return nil, typeFieldError(st, node, err)
	}
	fields := make([]*ast.Field, 0, len(selected))
	for _, candidate := range selected {
		source, err := node.resolver.sourceForField(candidate.field)
		if err != nil {
			return nil, typeFieldError(st, node, err)
		}
		fields = append(fields, explicitASTField(source))
	}
	return fields, nil
}
