package tracer

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"

	"golang.org/x/tools/go/packages"
)

type typeTraceState uint8

const (
	typeTraceResolving typeTraceState = iota + 1
	typeTraceDone
	typeTraceFailed
)

type typeDeclaration struct {
	pkg  *packages.Package
	spec *ast.TypeSpec
}

type typeFieldSource struct {
	field *ast.Field
	name  *ast.Ident
}

type typePackageIndex struct {
	declarations map[*types.TypeName]*typeDeclaration
	fields       map[*types.Var]*typeFieldSource
}

type typeTraceEntry struct {
	node  *TypeSpecNode
	state typeTraceState
	err   error
}

type typeResolver struct {
	tracer      *ASTTracer
	packages    map[*packages.Package]*typePackageIndex
	packageErrs map[*packages.Package]error
	entries     map[*types.TypeName]*typeTraceEntry
}

func newTypeResolver(tracer *ASTTracer) *typeResolver {
	return &typeResolver{
		tracer:      tracer,
		packages:    make(map[*packages.Package]*typePackageIndex),
		packageErrs: make(map[*packages.Package]error),
		entries:     make(map[*types.TypeName]*typeTraceEntry),
	}
}

func hasTypeTraceInfo(pkg *packages.Package) bool {
	return pkg != nil && pkg.Types != nil && pkg.TypesInfo != nil &&
		pkg.TypesInfo.Defs != nil && pkg.TypesInfo.Uses != nil && len(pkg.Syntax) > 0
}

func (r *typeResolver) packageIndex(pkg *packages.Package) (*typePackageIndex, error) {
	if index, ok := r.packages[pkg]; ok {
		return index, nil
	}
	if err, ok := r.packageErrs[pkg]; ok {
		return nil, err
	}
	if !hasTypeTraceInfo(pkg) {
		err := fmt.Errorf("%w for type package", ErrMissingTypeInfo)
		r.packageErrs[pkg] = err
		return nil, err
	}

	index := &typePackageIndex{
		declarations: make(map[*types.TypeName]*typeDeclaration),
		fields:       make(map[*types.Var]*typeFieldSource),
	}
	for _, file := range pkg.Syntax {
		for _, declaration := range file.Decls {
			general, ok := declaration.(*ast.GenDecl)
			if !ok || general.Tok != token.TYPE {
				continue
			}
			for _, specification := range general.Specs {
				typeSpec := specification.(*ast.TypeSpec)
				object, ok := pkg.TypesInfo.Defs[typeSpec.Name].(*types.TypeName)
				if !ok {
					continue
				}
				index.declarations[object] = &typeDeclaration{pkg: pkg, spec: typeSpec}
				r.indexFieldSources(index, pkg, typeSpec)
			}
		}
	}
	r.packages[pkg] = index
	return index, nil
}

func (r *typeResolver) indexFieldSources(index *typePackageIndex, pkg *packages.Package, typeSpec *ast.TypeSpec) {
	ast.Inspect(typeSpec.Type, func(node ast.Node) bool {
		field, ok := node.(*ast.Field)
		if !ok {
			return true
		}
		if len(field.Names) > 0 {
			for _, name := range field.Names {
				if variable, ok := pkg.TypesInfo.Defs[name].(*types.Var); ok && variable.IsField() {
					index.fields[variable.Origin()] = &typeFieldSource{field: field, name: name}
				}
			}
			return true
		}

		name := exprToImplicitName(field.Type, make(map[ast.Expr]bool))
		if name == nil {
			return true
		}
		if variable, ok := pkg.TypesInfo.Defs[name].(*types.Var); ok && variable.IsField() {
			index.fields[variable.Origin()] = &typeFieldSource{field: field, name: name}
		}
		return true
	})
}

func (r *typeResolver) declarationFor(object *types.TypeName) (*typeDeclaration, error) {
	if object == nil || object.Pkg() == nil {
		return nil, nil
	}
	if _, isTypeParameter := object.Type().(*types.TypeParam); isTypeParameter {
		return nil, nil
	}
	pkg, ok := r.tracer.packageForObject(object)
	if !ok {
		return nil, fmt.Errorf("type %s: %w", object.Name(), ErrMissingTypeInfo)
	}
	index, err := r.packageIndex(pkg)
	if err != nil {
		return nil, err
	}
	declaration, ok := index.declarations[object]
	if !ok {
		return nil, fmt.Errorf("type %s: %w", object.Name(), ErrMissingTypeInfo)
	}
	return declaration, nil
}

func (r *typeResolver) referencedTypeName(pkg *packages.Package, ident *ast.Ident) *types.TypeName {
	if pkg == nil || pkg.TypesInfo == nil || ident == nil {
		return nil
	}
	object, _ := pkg.TypesInfo.Uses[ident].(*types.TypeName)
	return object
}

func (r *typeResolver) traceReference(pkg *packages.Package, ident *ast.Ident) (*TypeSpecNode, error) {
	object := r.referencedTypeName(pkg, ident)
	if object == nil || object.Pkg() == nil {
		return nil, nil
	}
	if _, isTypeParameter := object.Type().(*types.TypeParam); isTypeParameter {
		return nil, nil
	}
	return r.traceObject(object)
}

func (r *typeResolver) traceObject(object *types.TypeName) (*TypeSpecNode, error) {
	if entry, ok := r.entries[object]; ok {
		switch entry.state {
		case typeTraceResolving, typeTraceDone:
			return entry.node, nil
		case typeTraceFailed:
			return nil, entry.err
		}
	}

	declaration, err := r.declarationFor(object)
	if err != nil {
		return nil, err
	}
	if declaration == nil {
		return nil, nil
	}

	node := &TypeSpecNode{
		Pkg:      declaration.pkg,
		Spec:     declaration.spec,
		Children: make(map[*ast.Ident]*TypeSpecNode),
		Fset:     declaration.pkg.Fset,
		typ:      object.Type(),
		resolver: r,
	}
	entry := &typeTraceEntry{node: node, state: typeTraceResolving}
	r.entries[object] = entry

	failed := func(cause error) (*TypeSpecNode, error) {
		entry.state = typeTraceFailed
		entry.err = cause
		return nil, cause
	}
	inspect := func(root ast.Node) error {
		if root == nil {
			return nil
		}
		var inspectErr error
		ast.Inspect(root, func(current ast.Node) bool {
			if inspectErr != nil {
				return false
			}
			ident, ok := current.(*ast.Ident)
			if !ok {
				return true
			}
			object := r.referencedTypeName(declaration.pkg, ident)
			if object == nil {
				return true
			}
			child, err := r.traceReference(declaration.pkg, ident)
			if err != nil {
				inspectErr = err
				return false
			}
			node.Children[ident] = child
			return true
		})
		return inspectErr
	}
	if err := inspect(declaration.spec.Type); err != nil {
		return failed(err)
	}
	if declaration.spec.TypeParams != nil {
		if err := inspect(declaration.spec.TypeParams); err != nil {
			return failed(err)
		}
	}

	if ident := directTypeReference(declaration.spec.Type); ident != nil {
		child, err := r.traceReference(declaration.pkg, ident)
		if err != nil {
			return failed(err)
		}
		node.underlying = child
	}
	entry.state = typeTraceDone
	return node, nil
}

func directTypeReference(expr ast.Expr) *ast.Ident {
	switch expr := expr.(type) {
	case *ast.Ident:
		return expr
	case *ast.SelectorExpr:
		return expr.Sel
	case *ast.IndexExpr:
		return directTypeReference(expr.X)
	case *ast.IndexListExpr:
		return directTypeReference(expr.X)
	case *ast.ParenExpr:
		return directTypeReference(expr.X)
	default:
		return nil
	}
}

func exprToImplicitName(expr ast.Expr, visited map[ast.Expr]bool) *ast.Ident {
	if expr == nil || visited[expr] {
		return nil
	}
	visited[expr] = true

	switch expr := expr.(type) {
	case *ast.Ident:
		return expr
	case *ast.SelectorExpr:
		return expr.Sel
	case *ast.StarExpr:
		return exprToImplicitName(expr.X, visited)
	case *ast.IndexExpr:
		return exprToImplicitName(expr.X, visited)
	case *ast.IndexListExpr:
		return exprToImplicitName(expr.X, visited)
	case *ast.ParenExpr:
		return exprToImplicitName(expr.X, visited)
	default:
		return nil
	}
}

func (r *typeResolver) traceTypeSpec(pkg *packages.Package, typeSpec *ast.TypeSpec) (*TypeSpecNode, error) {
	if typeSpec == nil || !hasTypeTraceInfo(pkg) {
		return nil, ErrMissingTypeInfo
	}
	object, ok := pkg.TypesInfo.Defs[typeSpec.Name].(*types.TypeName)
	if !ok {
		return nil, fmt.Errorf("type %s: %w", typeSpec.Name.Name, ErrMissingTypeInfo)
	}
	return r.traceObject(object)
}

type TypeSpecNode struct {
	Pkg      *packages.Package
	Spec     *ast.TypeSpec
	Children map[*ast.Ident]*TypeSpecNode
	Fset     *token.FileSet

	typ        types.Type
	underlying *TypeSpecNode
	resolver   *typeResolver
}

type TypeSpecTree struct {
	Root *TypeSpecNode
	Fset *token.FileSet
}

// Underlying returns the directly referenced declaration for aliases and named
// wrappers. It is nil when the declaration has no source-level type reference.
func (n *TypeSpecNode) Underlying() *TypeSpecNode {
	if n == nil {
		return nil
	}
	return n.underlying
}

func (t *ASTTracer) TraceTypeSpec(typeSpec *ast.TypeSpec) *TypeSpecTree {
	if t == nil {
		return nil
	}
	t.mu.Lock()
	defer t.mu.Unlock()

	root, err := t.types.traceTypeSpec(t.pkg, typeSpec)
	if err != nil || root == nil {
		return nil
	}
	return &TypeSpecTree{Root: root, Fset: root.Fset}
}
