package frontend

import (
	"fmt"
	"go/ast"
	"go/constant"
	"go/token"
	"go/types"
	"strings"

	"golang.org/x/tools/go/packages"

	"github.com/WindowsSov8forUs/sonolus-core-go/core"
	"github.com/WindowsSov8forUs/sonolus-core-go/core/resource"
	"github.com/WindowsSov8forUs/sonolus-go/internal/newcompiler/catalog"
	"github.com/WindowsSov8forUs/sonolus-go/internal/newcompiler/directive"
	"github.com/WindowsSov8forUs/sonolus-go/internal/newcompiler/source"
)

type resourceDirectiveSpec struct {
	kind        string
	renderMode  resource.EngineRenderMode
	pkg         *packages.Package
	named       *types.Named
	value       *types.Var
	initializer ast.Expr
	pos         string
}

func resourceKind(raw string) (string, bool) {
	switch raw {
	case "skin":
		return "Skin", true
	case "effect":
		return "Effect", true
	case "particle":
		return "Particle", true
	case "instruction":
		return "Instruction", true
	case "instructionIcon":
		return "InstructionIcon", true
	case "buckets":
		return "Buckets", true
	default:
		return "", false
	}
}

func resourceDirectives(pkg *packages.Package) ([]resourceDirectiveSpec, []error) {
	typesByKind := map[string]resourceDirectiveSpec{}
	valuesByKind := map[string]resourceDirectiveSpec{}
	var errs []error
	read := func(doc *ast.CommentGroup, pos ast.Node) (string, resource.EngineRenderMode, bool) {
		var found string
		var renderMode resource.EngineRenderMode
		for _, dir := range directive.ParseDirectives(doc, directive.PrefixSonolus) {
			if dir.Cmd != directive.CmdResource {
				continue
			}
			where := pkg.Fset.Position(pos.Pos()).String()
			if found != "" {
				errs = append(errs, fmt.Errorf("%s: duplicate sonolus:resource directive", where))
				continue
			}
			if len(dir.Args) < 1 || len(dir.Args) > 2 {
				errs = append(errs, fmt.Errorf("%s: sonolus:resource requires a resource kind and optional skin render mode", where))
				continue
			}
			kind, ok := resourceKind(dir.Args[0])
			if !ok {
				errs = append(errs, fmt.Errorf("%s: unknown resource kind %q", where, dir.Args[0]))
				continue
			}
			if len(dir.Args) == 2 {
				if kind != "Skin" {
					errs = append(errs, fmt.Errorf("%s: render mode is only valid for skin resources", where))
					continue
				}
				renderMode = resource.EngineRenderMode(dir.Args[1])
				if renderMode != resource.EngineRenderModeDefault && renderMode != resource.EngineRenderModeStandard && renderMode != resource.EngineRenderModeLightweight {
					errs = append(errs, fmt.Errorf("%s: invalid skin render mode %q; allowed modes are default, standard, lightweight", where, dir.Args[1]))
					continue
				}
			}
			found = kind
		}
		return found, renderMode, found != ""
	}
	for _, file := range pkg.Syntax {
		for _, decl := range file.Decls {
			gen, ok := decl.(*ast.GenDecl)
			if !ok || (gen.Tok != token.TYPE && gen.Tok != token.VAR) {
				continue
			}
			for _, rawSpec := range gen.Specs {
				specDoc := gen.Doc
				switch spec := rawSpec.(type) {
				case *ast.TypeSpec:
					if spec.Doc != nil {
						specDoc = spec.Doc
					}
					kind, renderMode, ok := read(specDoc, spec)
					if !ok {
						continue
					}
					obj, _ := pkg.TypesInfo.Defs[spec.Name].(*types.TypeName)
					if obj == nil {
						continue
					}
					named, namedOK := namedType(obj.Type())
					if !namedOK {
						continue
					}
					if _, structOK := named.Underlying().(*types.Struct); !structOK {
						errs = append(errs, fmt.Errorf("%s: %s resource type must be a struct", pkg.Fset.Position(spec.Pos()), strings.ToLower(kind)))
						continue
					}
					entry := resourceDirectiveSpec{kind: kind, renderMode: renderMode, pkg: pkg, named: named, pos: pkg.Fset.Position(spec.Pos()).String()}
					if previous, exists := typesByKind[kind]; exists {
						errs = append(errs, fmt.Errorf("%s: duplicate %s resource type (previously declared at %s)", entry.pos, strings.ToLower(kind), previous.pos))
					} else {
						typesByKind[kind] = entry
					}
				case *ast.ValueSpec:
					if spec.Doc != nil {
						specDoc = spec.Doc
					}
					kind, renderMode, ok := read(specDoc, spec)
					if !ok {
						continue
					}
					if len(spec.Names) != 1 || len(spec.Values) != 1 {
						errs = append(errs, fmt.Errorf("%s: %s resource variable must declare exactly one initialized value", pkg.Fset.Position(spec.Pos()), strings.ToLower(kind)))
						continue
					}
					value, _ := pkg.TypesInfo.Defs[spec.Names[0]].(*types.Var)
					entry := resourceDirectiveSpec{kind: kind, renderMode: renderMode, pkg: pkg, value: value, initializer: spec.Values[0], pos: pkg.Fset.Position(spec.Pos()).String()}
					if previous, exists := valuesByKind[kind]; exists {
						errs = append(errs, fmt.Errorf("%s: duplicate %s resource value (previously declared at %s)", entry.pos, strings.ToLower(kind), previous.pos))
					} else {
						valuesByKind[kind] = entry
					}
				}
			}
		}
	}
	var result []resourceDirectiveSpec
	for _, kind := range []string{"Skin", "Effect", "Particle", "Instruction", "InstructionIcon", "Buckets"} {
		t, hasType := typesByKind[kind]
		v, hasValue := valuesByKind[kind]
		if hasType != hasValue {
			at := t.pos
			if at == "" {
				at = v.pos
			}
			errs = append(errs, fmt.Errorf("%s: %s resource requires both a type and a value declaration", at, strings.ToLower(kind)))
			continue
		}
		if hasType {
			if t.renderMode != v.renderMode {
				errs = append(errs, fmt.Errorf("%s: %s resource type and value directives must use the same options", v.pos, strings.ToLower(kind)))
				continue
			}
			t.value = v.value
			t.initializer = v.initializer
			result = append(result, t)
		}
	}
	return result, errs
}

func unwrapResourceLiteral(expr ast.Expr) (*ast.CompositeLit, bool) {
	for {
		switch value := expr.(type) {
		case *ast.ParenExpr:
			expr = value.X
		case *ast.UnaryExpr:
			if value.Op != token.AND {
				return nil, false
			}
			expr = value.X
		default:
			literal, ok := expr.(*ast.CompositeLit)
			return literal, ok
		}
	}
}

func resourceConstructor(kind string) string {
	switch kind {
	case "Skin":
		return "SkinSprite"
	case "Effect":
		return "EffectClip"
	case "Particle":
		return "ParticleEffect"
	case "Instruction":
		return "InstructionText"
	case "InstructionIcon":
		return "InstructionIcon"
	default:
		return ""
	}
}

func resourceCallName(pkg *packages.Package, kind string, expr ast.Expr) (string, bool) {
	call, ok := expr.(*ast.CallExpr)
	if !ok || len(call.Args) != 1 || call.Ellipsis.IsValid() {
		return "", false
	}
	var object types.Object
	switch fun := call.Fun.(type) {
	case *ast.Ident:
		object = pkg.TypesInfo.ObjectOf(fun)
	case *ast.SelectorExpr:
		object = pkg.TypesInfo.ObjectOf(fun.Sel)
	}
	fn, ok := object.(*types.Func)
	if !ok || fn.Pkg() == nil || fn.Pkg().Path() != source.SonolusPkgPath() || fn.Name() != resourceConstructor(kind) {
		return "", false
	}
	value := pkg.TypesInfo.Types[call.Args[0]].Value
	if value == nil || value.Kind() != constant.String {
		return "", false
	}
	return constant.StringVal(value), true
}

func calledFunction(pkg *packages.Package, expr ast.Expr, name string) (*ast.CallExpr, bool) {
	call, ok := expr.(*ast.CallExpr)
	if !ok {
		return nil, false
	}
	var object types.Object
	switch fun := call.Fun.(type) {
	case *ast.Ident:
		object = pkg.TypesInfo.ObjectOf(fun)
	case *ast.SelectorExpr:
		object = pkg.TypesInfo.ObjectOf(fun.Sel)
	}
	fn, ok := object.(*types.Func)
	if !ok {
		return nil, false
	}
	symbol, cataloged := catalog.LookupObject(fn)
	return call, cataloged && !symbol.Internal && symbol.Package == "sonolus" && symbol.Name == name && symbol.Kind == catalog.KindFunction
}

func constantFloat(pkg *packages.Package, expr ast.Expr) (float64, bool) {
	value := pkg.TypesInfo.Types[expr].Value
	if value == nil {
		return 0, false
	}
	result, exact := constant.Float64Val(constant.ToFloat(value))
	return result, exact
}

func resourceValueNames(pkg *packages.Package, kind string, fieldType types.Type, expr ast.Expr) ([]string, bool) {
	if array, ok := types.Unalias(fieldType).Underlying().(*types.Array); ok {
		if kind == "Instruction" || kind == "InstructionIcon" {
			return nil, false
		}
		literal, ok := expr.(*ast.CompositeLit)
		if !ok || int64(len(literal.Elts)) != array.Len() {
			return nil, false
		}
		names := make([]string, 0, array.Len())
		for _, element := range literal.Elts {
			if _, keyed := element.(*ast.KeyValueExpr); keyed {
				return nil, false
			}
			name, valid := resourceCallName(pkg, kind, element)
			if !valid || name == "" {
				return nil, false
			}
			names = append(names, name)
		}
		return names, true
	}
	name, ok := resourceCallName(pkg, kind, expr)
	if !ok || name == "" {
		return nil, false
	}
	return []string{name}, true
}

func resourceInitializerNames(spec resourceDirectiveSpec) (map[*types.Var][]string, []error) {
	literal, ok := unwrapResourceLiteral(spec.initializer)
	if !ok {
		return nil, []error{fmt.Errorf("%s: %s resource value must be a struct literal or pointer to one", spec.pos, strings.ToLower(spec.kind))}
	}
	st := spec.named.Underlying().(*types.Struct)
	result := map[*types.Var][]string{}
	var errs []error
	positional := 0
	for _, element := range literal.Elts {
		var field *types.Var
		value := element
		if pair, keyed := element.(*ast.KeyValueExpr); keyed {
			ident, identOK := pair.Key.(*ast.Ident)
			if !identOK {
				errs = append(errs, fmt.Errorf("%s: resource initializer field must be an identifier", spec.pkg.Fset.Position(pair.Key.Pos())))
				continue
			}
			for i := 0; i < st.NumFields(); i++ {
				if st.Field(i).Name() == ident.Name {
					field = st.Field(i)
					break
				}
			}
			value = pair.Value
		} else if positional < st.NumFields() {
			field = st.Field(positional)
			positional++
		}
		if field == nil {
			errs = append(errs, fmt.Errorf("%s: unknown or excess resource initializer field", spec.pkg.Fset.Position(element.Pos())))
			continue
		}
		names, valid := resourceValueNames(spec.pkg, spec.kind, field.Type(), value)
		if !valid {
			errs = append(errs, fmt.Errorf("%s.%s: use sonolus.%s with non-empty constant strings; fixed arrays require a complete unkeyed array literal", spec.named.Obj().Name(), field.Name(), resourceConstructor(spec.kind)))
			continue
		}
		result[field] = names
	}
	return result, errs
}

func staticResourceNames(value source.StaticValue, kind string) ([]string, bool) {
	value = dereferenceStatic(value)
	if value.Kind == source.StaticArray {
		var result []string
		for _, element := range value.Elements {
			names, ok := staticResourceNames(element, kind)
			if !ok || len(names) != 1 {
				return nil, false
			}
			result = append(result, names[0])
		}
		return result, true
	}
	if value.Kind != source.StaticFunctionCall || value.Call == nil || value.Call.Object == nil || value.Call.Receiver != nil || len(value.Call.Args) != 1 {
		return nil, false
	}
	symbol, cataloged := catalog.LookupObject(value.Call.Object)
	if !cataloged || symbol.Internal || symbol.Package != "sonolus" || symbol.Name != resourceConstructor(kind) || symbol.Kind != catalog.KindFunction {
		return nil, false
	}
	wantResult := map[string]string{"Skin": "Sprite", "Effect": "Clip", "Particle": "Effect", "Instruction": "Text", "InstructionIcon": "Icon"}[kind]
	if value.Call.Signature == nil || value.Call.Signature.Results().Len() != 1 || typeID(value.Call.Signature.Results().At(0).Type()) != rootID(wantResult) {
		return nil, false
	}
	name, ok := staticString(value.Call.Args[0])
	if !ok || name == "" {
		return nil, false
	}
	return []string{name}, true
}

func tracedResourceInitializerNames(spec resourceDirectiveSpec, tracer *source.ASTTracer) (map[*types.Var][]string, bool) {
	binding, err := tracer.EvalObject(spec.value)
	if err != nil {
		return nil, false
	}
	value := dereferenceStatic(binding.Value)
	if value.Kind != source.StaticStruct {
		return nil, false
	}
	result := map[*types.Var][]string{}
	for _, field := range value.Fields {
		names, ok := staticResourceNames(field.Value, spec.kind)
		if !ok {
			return nil, false
		}
		result[field.Field] = names
	}
	return result, true
}

func addDirectiveResource(out *ModeDeclarations, spec resourceDirectiveSpec, tracer *source.ASTTracer) []error {
	if spec.value == nil {
		return []error{fmt.Errorf("%s: invalid %s resource variable", spec.pos, strings.ToLower(spec.kind))}
	}
	valueNamed, ok := namedType(spec.value.Type())
	if !ok || valueNamed.Obj() != spec.named.Obj() {
		return []error{fmt.Errorf("%s: %s resource value must have type *%s or %s", spec.pos, strings.ToLower(spec.kind), spec.named.Obj().Name(), spec.named.Obj().Name())}
	}
	if spec.kind == "Buckets" {
		return addBucketResource(out, spec)
	}
	st := spec.named.Underlying().(*types.Struct)
	namedFields, traced := tracedResourceInitializerNames(spec, tracer)
	var errs []error
	if !traced {
		namedFields, errs = resourceInitializerNames(spec)
	}
	names := map[string]string{}
	for i := 0; i < st.NumFields(); i++ {
		field := st.Field(i)
		if field.Embedded() {
			errs = append(errs, fmt.Errorf("%s.%s: embedded fields are not allowed in resource data", spec.named.Obj().Name(), field.Name()))
			continue
		}
		want := ""
		switch spec.kind {
		case "Skin":
			want = rootID("Sprite")
		case "Effect":
			want = rootID("Clip")
		case "Particle":
			want = rootID("Effect")
		case "Instruction":
			want = rootID("Text")
		case "InstructionIcon":
			want = rootID("Icon")
		}
		actualType := field.Type()
		if array, ok := types.Unalias(actualType).Underlying().(*types.Array); ok {
			actualType = array.Elem()
		}
		if want != "" && typeID(actualType) != want {
			errs = append(errs, fmt.Errorf("%s.%s: invalid field type for %s resource", spec.named.Obj().Name(), field.Name(), strings.ToLower(spec.kind)))
			continue
		}
		fieldNames, valid := namedFields[field]
		if !valid {
			if _, alreadyReported := namedFields[field]; !alreadyReported {
				errs = append(errs, fmt.Errorf("%s.%s: resource field must be initialized", spec.named.Obj().Name(), field.Name()))
			}
			continue
		}
		for index, name := range fieldNames {
			label := field.Name()
			if len(fieldNames) > 1 {
				label = fmt.Sprintf("%s[%d]", field.Name(), index)
			}
			if previous := names[name]; previous != "" {
				errs = append(errs, fmt.Errorf("%s.%s: duplicate resource name %q (also used by %s)", spec.named.Obj().Name(), label, name, previous))
				continue
			}
			names[name] = label
			var id int
			switch spec.kind {
			case "Skin":
				id = len(out.Resources.Skin.Sprites)
				out.Resources.Skin.Sprites = append(out.Resources.Skin.Sprites, resource.EngineSkinDataSprite{Name: resource.SkinSpriteName(name), ID: id})
				out.Resources.SpriteIDs[name] = id
			case "Effect":
				id = len(out.Resources.Effect.Clips)
				out.Resources.Effect.Clips = append(out.Resources.Effect.Clips, resource.EngineEffectDataClip{Name: resource.EffectClipName(name), ID: id})
				out.Resources.EffectIDs[name] = id
			case "Particle":
				id = len(out.Resources.Particle.Effects)
				out.Resources.Particle.Effects = append(out.Resources.Particle.Effects, resource.EngineParticleDataEffect{Name: resource.ParticleEffectName(name), ID: id})
				out.Resources.ParticleIDs[name] = id
			case "Instruction":
				id = len(out.Resources.Instruction.Texts)
				out.Resources.Instruction.Texts = append(out.Resources.Instruction.Texts, resource.EngineInstructionDataText{Name: core.Text(name), ID: id})
			case "InstructionIcon":
				id = len(out.Resources.Instruction.Icons)
				out.Resources.Instruction.Icons = append(out.Resources.Instruction.Icons, resource.EngineInstructionDataIcon{Name: resource.InstructionIconName(name), ID: id})
			}
			out.Resources.FieldIDs[field] = append(out.Resources.FieldIDs[field], id)
		}
	}
	return errs
}
