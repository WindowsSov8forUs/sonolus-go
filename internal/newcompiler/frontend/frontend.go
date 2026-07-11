package frontend

import (
	"encoding/binary"
	"fmt"
	"go/ast"
	"go/constant"
	"go/token"
	"go/types"
	"math"
	"os"
	"reflect"
	"sort"
	"strconv"
	"strings"

	"golang.org/x/tools/go/packages"

	"github.com/WindowsSov8forUs/sonolus-core-go/core"
	"github.com/WindowsSov8forUs/sonolus-core-go/core/resource"
	"github.com/WindowsSov8forUs/sonolus-go/internal/newcompiler/catalog"
	"github.com/WindowsSov8forUs/sonolus-go/internal/newcompiler/directive"
	"github.com/WindowsSov8forUs/sonolus-go/internal/newcompiler/intrinsic"
	"github.com/WindowsSov8forUs/sonolus-go/internal/newcompiler/mode"
	"github.com/WindowsSov8forUs/sonolus-go/internal/newcompiler/source"
)

type tagValue struct {
	flags map[string]bool
	items map[string]string
}

func parseTag(raw string) tagValue {
	t := tagValue{flags: map[string]bool{}, items: map[string]string{}}
	for _, part := range strings.Split(raw, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		key, value, ok := strings.Cut(part, "=")
		if ok {
			t.items[strings.TrimSpace(key)] = strings.TrimSpace(value)
		} else {
			t.flags[part] = true
		}
	}
	return t
}

func sonolusTag(tag string) (tagValue, bool) {
	raw, ok := reflect.StructTag(tag).Lookup("sonolus")
	if !ok {
		return tagValue{}, false
	}
	return parseTag(raw), true
}

func validateTag(where string, tag tagValue, flags, items []string) []error {
	allowedFlags := map[string]bool{}
	allowedItems := map[string]bool{}
	for _, key := range flags {
		allowedFlags[key] = true
	}
	for _, key := range items {
		allowedItems[key] = true
	}
	var errs []error
	for key := range tag.flags {
		if !allowedFlags[key] {
			errs = append(errs, fmt.Errorf("%s: unknown sonolus tag %q", where, key))
		}
	}
	for key := range tag.items {
		if !allowedItems[key] {
			errs = append(errs, fmt.Errorf("%s: unknown sonolus tag %q", where, key))
		}
	}
	return errs
}

func namedType(t types.Type) (*types.Named, bool) {
	if p, ok := t.(*types.Pointer); ok {
		t = p.Elem()
	}
	n, ok := types.Unalias(t).(*types.Named)
	return n, ok
}

func typeID(t types.Type) string {
	n, ok := namedType(t)
	if !ok || n.Obj().Pkg() == nil {
		return ""
	}
	return n.Obj().Pkg().Path() + "." + n.Obj().Name()
}

func markerID(m mode.Mode, name string) string {
	return source.SonolusPkgPath() + "/" + string(m) + "." + name
}

func rootID(name string) string { return source.SonolusPkgPath() + "." + name }

func structMarker(named *types.Named) (string, tagValue, bool) {
	st, ok := named.Underlying().(*types.Struct)
	if !ok {
		return "", tagValue{}, false
	}
	for i := 0; i < st.NumFields(); i++ {
		field := st.Field(i)
		if !field.Embedded() {
			continue
		}
		id := typeID(field.Type())
		if id == "" {
			continue
		}
		tag, _ := sonolusTag(st.Tag(i))
		return id, tag, true
	}
	return "", tagValue{}, false
}

func collectPackages(root *packages.Package) []*packages.Package {
	seen := map[string]bool{}
	var result []*packages.Package
	var visit func(*packages.Package)
	visit = func(pkg *packages.Package) {
		if pkg == nil || seen[pkg.PkgPath] || source.IsSonolusPkg(pkg) {
			return
		}
		seen[pkg.PkgPath] = true
		result = append(result, pkg)
		paths := make([]string, 0, len(pkg.Imports))
		for path := range pkg.Imports {
			paths = append(paths, path)
		}
		sort.Strings(paths)
		for _, path := range paths {
			visit(pkg.Imports[path])
		}
	}
	visit(root)
	sort.Slice(result, func(i, j int) bool { return result[i].PkgPath < result[j].PkgPath })
	return result
}

func packageNamedTypes(pkg *packages.Package) []*types.Named {
	names := pkg.Types.Scope().Names()
	result := make([]*types.Named, 0)
	for _, name := range names {
		obj, ok := pkg.Types.Scope().Lookup(name).(*types.TypeName)
		if !ok {
			continue
		}
		if named, ok := namedType(obj.Type()); ok {
			result = append(result, named)
		}
	}
	return result
}

func packageVariables(pkg *packages.Package) []*types.Var {
	var result []*types.Var
	for _, name := range pkg.Types.Scope().Names() {
		if v, ok := pkg.Types.Scope().Lookup(name).(*types.Var); ok {
			result = append(result, v)
		}
	}
	return result
}

func markerVariables(pkg *packages.Package, named *types.Named) []*types.Var {
	var result []*types.Var
	for _, v := range packageVariables(pkg) {
		vn, ok := namedType(v.Type())
		if ok && vn.Obj() == named.Obj() {
			result = append(result, v)
		}
	}
	return result
}

var callbacks = map[mode.Mode]map[string]string{
	mode.ModePlay: {
		"Preprocess": "void", "SpawnOrder": "float", "ShouldSpawn": "bool",
		"Initialize": "void", "UpdateSequential": "void", "Touch": "void",
		"UpdateParallel": "void", "Terminate": "void",
	},
	mode.ModeWatch: {
		"Preprocess": "void", "SpawnTime": "float", "DespawnTime": "float",
		"Initialize": "void", "UpdateSequential": "void", "UpdateParallel": "void", "Terminate": "void",
	},
	mode.ModePreview: {"Preprocess": "void", "Render": "void"},
}

func callbackKey(goName string) string {
	if goName == "" {
		return ""
	}
	return strings.ToLower(goName[:1]) + goName[1:]
}

func validCallbackSignature(fn *types.Func, result string, receiver bool) error {
	sig, ok := fn.Type().(*types.Signature)
	if !ok {
		return fmt.Errorf("not a function")
	}
	if receiver != (sig.Recv() != nil) {
		return fmt.Errorf("invalid receiver")
	}
	if sig.Params().Len() != 0 {
		return fmt.Errorf("callback must not have parameters")
	}
	switch result {
	case "void":
		if sig.Results().Len() != 0 {
			return fmt.Errorf("callback must not return a value")
		}
	case "float":
		if sig.Results().Len() != 1 || !types.Identical(sig.Results().At(0).Type(), types.Typ[types.Float64]) {
			return fmt.Errorf("callback must return float64")
		}
	case "bool":
		if sig.Results().Len() != 1 || !types.Identical(sig.Results().At(0).Type(), types.Typ[types.Bool]) {
			return fmt.Errorf("callback must return bool")
		}
	}
	return nil
}

func findFuncDecl(pkg *packages.Package, fn *types.Func) *ast.FuncDecl {
	for _, file := range pkg.Syntax {
		for _, decl := range file.Decls {
			fd, ok := decl.(*ast.FuncDecl)
			if ok && pkg.TypesInfo.Defs[fd.Name] == fn {
				return fd
			}
		}
	}
	return nil
}

func callbackIntrinsics(pkg *packages.Package, decl *ast.FuncDecl, m mode.Mode, phase string) ([]IntrinsicReference, []error) {
	if decl == nil {
		return nil, nil
	}
	var refs []IntrinsicReference
	var errs []error
	ast.Inspect(decl.Body, func(node ast.Node) bool {
		if call, ok := node.(*ast.CallExpr); ok {
			if selector, ok := call.Fun.(*ast.SelectorExpr); ok {
				obj := pkg.TypesInfo.Uses[selector.Sel]
				if obj != nil && obj.Pkg() != nil && obj.Pkg().Path() == "math/rand" && obj.Name() == "Intn" && len(call.Args) == 1 {
					if value := pkg.TypesInfo.Types[call.Args[0]].Value; value != nil {
						if integer, exact := constant.Int64Val(value); exact && integer <= 0 {
							pos := pkg.Fset.Position(call.Args[0].Pos())
							errs = append(errs, fmt.Errorf("%s: math/rand.Intn constant argument must be positive", pos))
						}
					}
				}
			}
		}
		id, ok := node.(*ast.Ident)
		if !ok {
			return true
		}
		obj := pkg.TypesInfo.Uses[id]
		if obj == nil || obj.Pkg() == nil {
			return true
		}
		if source.IsSonolusPkgPath(obj.Pkg().Path()) {
			symbol, supported := catalog.LookupObject(obj)
			if !supported || symbol.Internal {
				pos := pkg.Fset.Position(id.Pos())
				errs = append(errs, fmt.Errorf("%s: Sonolus API symbol %s.%s is not part of the public catalog", pos, obj.Pkg().Path(), obj.Name()))
				return true
			}
			if !catalog.AllowsMode(symbol, string(m)) {
				pos := pkg.Fset.Position(id.Pos())
				errs = append(errs, fmt.Errorf("%s: Sonolus API %s is not available in %s mode", pos, symbol.Key(), m))
				return true
			}
			if !catalog.AllowsPhase(symbol, phase) {
				pos := pkg.Fset.Position(id.Pos())
				errs = append(errs, fmt.Errorf("%s: Sonolus API %s cannot write during %s callback", pos, symbol.Key(), phase))
				return true
			}
			for _, constructor := range []string{"SkinSprite", "EffectClip", "ParticleEffect", "InstructionText", "InstructionIcon"} {
				if symbol.Package == "sonolus" && symbol.Name == constructor {
					pos := pkg.Fset.Position(id.Pos())
					errs = append(errs, fmt.Errorf("%s: resource constructor sonolus.%s is only valid in a resource initializer", pos, constructor))
					return true
				}
			}
			refs = append(refs, IntrinsicReference{API: symbol, Object: obj})
			return true
		}
		if obj.Pkg().Path() != "math" && obj.Pkg().Path() != "math/rand" {
			return true
		}
		symbol, supported := intrinsic.LookupObject(obj)
		if !supported {
			pos := pkg.Fset.Position(id.Pos())
			errs = append(errs, fmt.Errorf("%s: standard library symbol %s.%s is not a Sonolus intrinsic", pos, obj.Pkg().Path(), obj.Name()))
			return true
		}
		refs = append(refs, IntrinsicReference{Symbol: symbol, Object: obj})
		return true
	})
	return refs, errs
}

func layoutSize(t types.Type) (int, error) {
	if basic, ok := types.Unalias(t).Underlying().(*types.Basic); ok {
		if basic.Info()&(types.IsBoolean|types.IsInteger|types.IsFloat) != 0 {
			return 1, nil
		}
	}
	switch typeID(t) {
	case rootID("Vec2"), rootID("Range"), rootID("Pair"):
		return 2, nil
	case rootID("Rect"):
		return 4, nil
	case rootID("Transform2D"), rootID("JudgmentWindow"), rootID("JudgmentWindows"):
		return 6, nil
	case rootID("Quad"):
		return 8, nil
	}
	if strings.HasPrefix(typeID(t), rootID("EntityRef")) {
		return 1, nil
	}
	return 0, fmt.Errorf("unsupported runtime field type %s", t.String())
}

func parseBool(value string) (bool, error) { return strconv.ParseBool(value) }

func parseArchetype(pkg *packages.Package, named *types.Named, m mode.Mode, marker tagValue) (*ArchetypeDeclaration, []error) {
	var errs []error
	errs = append(errs, validateTag(named.Obj().Name(), marker, nil, []string{"name", "hasInput"})...)
	name := marker.items["name"]
	if name == "" {
		name = named.Obj().Name()
	}
	hasInput := false
	if raw, ok := marker.items["hasInput"]; ok {
		value, err := parseBool(raw)
		if err != nil {
			errs = append(errs, fmt.Errorf("%s: invalid hasInput %q", named.Obj().Name(), raw))
		} else {
			hasInput = value
		}
	}
	result := &ArchetypeDeclaration{PackagePath: pkg.PkgPath, TypeName: named.Obj().Name(), Name: name, HasInput: hasInput, Named: named}
	st := named.Underlying().(*types.Struct)
	offsets := map[string]int{"imported": 0, "data": 0, "memory": 0, "shared": 0, "exported": 0}
	external := map[string]bool{}
	orders := map[string]int{}
	for i := 0; i < st.NumFields(); i++ {
		field := st.Field(i)
		id := typeID(field.Type())
		if field.Embedded() {
			if id == markerID(m, "CallbackOrders") {
				tag, _ := sonolusTag(st.Tag(i))
				for key, raw := range tag.items {
					order, err := strconv.Atoi(raw)
					if err != nil {
						errs = append(errs, fmt.Errorf("%s.%s: invalid callback order %q", named.Obj().Name(), key, raw))
						continue
					}
					orders[key] = order
				}
			}
			continue
		}
		tag, ok := sonolusTag(st.Tag(i))
		if !ok {
			continue
		}
		errs = append(errs, validateTag(named.Obj().Name()+"."+field.Name(), tag,
			[]string{"imported", "data", "memory", "shared", "exported"}, []string{"name", "default"})...)
		storage := ""
		for _, candidate := range []string{"imported", "data", "memory", "shared", "exported"} {
			if tag.flags[candidate] {
				if storage != "" {
					errs = append(errs, fmt.Errorf("%s.%s: multiple storage classes", named.Obj().Name(), field.Name()))
				}
				storage = candidate
			}
		}
		if storage == "" {
			errs = append(errs, fmt.Errorf("%s.%s: missing storage class", named.Obj().Name(), field.Name()))
			continue
		}
		if storage == "exported" && m != mode.ModePlay {
			errs = append(errs, fmt.Errorf("%s.%s: exports are only available in play mode", named.Obj().Name(), field.Name()))
			continue
		}
		size, err := layoutSize(field.Type())
		if err != nil {
			errs = append(errs, fmt.Errorf("%s.%s: %w", named.Obj().Name(), field.Name(), err))
			continue
		}
		externalName := tag.items["name"]
		if externalName == "" {
			externalName = field.Name()
		}
		if (storage == "imported" || storage == "exported") && external[externalName] {
			errs = append(errs, fmt.Errorf("%s: duplicate external field name %q", named.Obj().Name(), externalName))
			continue
		}
		external[externalName] = true
		def := 0.0
		if raw, ok := tag.items["default"]; ok {
			if storage != "imported" {
				errs = append(errs, fmt.Errorf("%s.%s: default is only valid for imported fields", named.Obj().Name(), field.Name()))
			} else if value, err := strconv.ParseFloat(raw, 64); err != nil {
				errs = append(errs, fmt.Errorf("%s.%s: invalid default %q", named.Obj().Name(), field.Name(), raw))
			} else {
				def = value
			}
		}
		fd := &FieldDeclaration{GoName: field.Name(), ExternalName: externalName, Storage: storage, Offset: offsets[storage], Size: size, Default: def, Type: field.Type()}
		result.Fields = append(result.Fields, fd)
		if storage == "imported" {
			result.Imports = append(result.Imports, resource.EngineDataArchetypeImport{Name: resource.EngineArchetypeDataName(externalName), Index: offsets[storage], Def: def})
		}
		if storage == "exported" {
			result.Exports = append(result.Exports, resource.EngineArchetypeDataName(externalName))
		}
		offsets[storage] += size
	}

	methodSet := types.NewMethodSet(types.NewPointer(named))
	foundOrders := map[string]bool{}
	for i := 0; i < methodSet.Len(); i++ {
		fn, ok := methodSet.At(i).Obj().(*types.Func)
		if !ok {
			continue
		}
		want, isCallback := callbacks[m][fn.Name()]
		if !isCallback {
			continue
		}
		if err := validCallbackSignature(fn, want, true); err != nil {
			errs = append(errs, fmt.Errorf("%s.%s: %w", named.Obj().Name(), fn.Name(), err))
			continue
		}
		key := callbackKey(fn.Name())
		foundOrders[key] = true
		decl := findFuncDecl(pkg, fn)
		refs, intrinsicErrs := callbackIntrinsics(pkg, decl, m, key)
		errs = append(errs, intrinsicErrs...)
		result.Callbacks = append(result.Callbacks, &CallbackDeclaration{Name: key, Order: orders[key], Function: fn, Decl: decl, Intrinsics: refs})
	}
	for key := range orders {
		if !foundOrders[key] {
			errs = append(errs, fmt.Errorf("%s: callback order for missing or invalid callback %q", named.Obj().Name(), key))
		}
	}
	sort.Slice(result.Callbacks, func(i, j int) bool { return result.Callbacks[i].Name < result.Callbacks[j].Name })
	return result, errs
}

func staticName(tag tagValue, fallback string) string {
	if value := tag.items["name"]; value != "" {
		return value
	}
	return fallback
}

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
	return call, ok && fn.Pkg() != nil && fn.Pkg().Path() == source.SonolusPkgPath() && fn.Name() == name
}

func constantFloat(pkg *packages.Package, expr ast.Expr) (float64, bool) {
	value := pkg.TypesInfo.Types[expr].Value
	if value == nil {
		return 0, false
	}
	result, exact := constant.Float64Val(constant.ToFloat(value))
	return result, exact
}

func resourceSpriteID(out *EngineDeclarations, pkg *packages.Package, expr ast.Expr) (int, bool) {
	if index, ok := expr.(*ast.IndexExpr); ok {
		selector, selectorOK := index.X.(*ast.SelectorExpr)
		if !selectorOK {
			return 0, false
		}
		field, fieldOK := pkg.TypesInfo.ObjectOf(selector.Sel).(*types.Var)
		value := pkg.TypesInfo.Types[index.Index].Value
		position, positionOK := constant.Int64Val(value)
		ids := out.Resources.FieldIDs[field]
		if !fieldOK || !positionOK || position < 0 || position >= int64(len(ids)) {
			return 0, false
		}
		return ids[position], true
	}
	selector, ok := expr.(*ast.SelectorExpr)
	if !ok {
		return 0, false
	}
	field, ok := pkg.TypesInfo.ObjectOf(selector.Sel).(*types.Var)
	if !ok {
		return 0, false
	}
	ids := out.Resources.FieldIDs[field]
	if len(ids) != 1 {
		return 0, false
	}
	return ids[0], true
}

func bucketSprite(out *EngineDeclarations, pkg *packages.Package, expr ast.Expr) (resource.EngineDataBucketSprite, error) {
	call, plain := calledFunction(pkg, expr, "JudgmentBucketSprite")
	if !plain {
		call, plain = calledFunction(pkg, expr, "JudgmentBucketSpriteWithFallback")
	}
	want := 6
	if !plain {
		return resource.EngineDataBucketSprite{}, fmt.Errorf("use sonolus.JudgmentBucketSprite or sonolus.JudgmentBucketSpriteWithFallback")
	}
	withFallback := false
	if objectCall, ok := calledFunction(pkg, expr, "JudgmentBucketSpriteWithFallback"); ok {
		call = objectCall
		want = 7
		withFallback = true
	}
	if len(call.Args) != want {
		return resource.EngineDataBucketSprite{}, fmt.Errorf("bucket sprite constructor has %d arguments; want %d", len(call.Args), want)
	}
	id, ok := resourceSpriteID(out, pkg, call.Args[0])
	if !ok {
		return resource.EngineDataBucketSprite{}, fmt.Errorf("bucket sprite must reference a declared skin sprite field")
	}
	result := resource.EngineDataBucketSprite{ID: id}
	start := 1
	if withFallback {
		fallback, valid := resourceSpriteID(out, pkg, call.Args[1])
		if !valid {
			return resource.EngineDataBucketSprite{}, fmt.Errorf("bucket fallback must reference a declared skin sprite field")
		}
		result.FallbackID = fallback
		start = 2
	}
	values := []*float64{&result.X, &result.Y, &result.W, &result.H, &result.Rotation}
	for index, target := range values {
		value, valid := constantFloat(pkg, call.Args[start+index])
		if !valid {
			return resource.EngineDataBucketSprite{}, fmt.Errorf("bucket sprite geometry must use constant numbers")
		}
		*target = value
	}
	return result, nil
}

func addBucketResource(out *EngineDeclarations, spec resourceDirectiveSpec) []error {
	literal, ok := unwrapResourceLiteral(spec.initializer)
	if !ok {
		return []error{fmt.Errorf("%s: buckets resource value must be a struct literal or pointer to one", spec.pos)}
	}
	st := spec.named.Underlying().(*types.Struct)
	values := map[string]ast.Expr{}
	position := 0
	for _, element := range literal.Elts {
		if pair, keyed := element.(*ast.KeyValueExpr); keyed {
			if name, valid := pair.Key.(*ast.Ident); valid {
				values[name.Name] = pair.Value
			}
			continue
		}
		if position < st.NumFields() {
			values[st.Field(position).Name()] = element
			position++
		}
	}
	var errs []error
	for index := 0; index < st.NumFields(); index++ {
		field := st.Field(index)
		if field.Embedded() || typeID(field.Type()) != rootID("Bucket") {
			errs = append(errs, fmt.Errorf("%s.%s: buckets resource fields must be sonolus.Bucket", spec.named.Obj().Name(), field.Name()))
			continue
		}
		call, valid := calledFunction(spec.pkg, values[field.Name()], "JudgmentBucket")
		if !valid || len(call.Args) < 1 {
			errs = append(errs, fmt.Errorf("%s.%s: use sonolus.JudgmentBucket(unit, sprites...)", spec.named.Obj().Name(), field.Name()))
			continue
		}
		unitValue := spec.pkg.TypesInfo.Types[call.Args[0]].Value
		if unitValue == nil || unitValue.Kind() != constant.String {
			errs = append(errs, fmt.Errorf("%s.%s: bucket unit must be a constant string", spec.named.Obj().Name(), field.Name()))
			continue
		}
		bucket := resource.EngineDataBucket{Unit: core.Text(constant.StringVal(unitValue))}
		for _, expr := range call.Args[1:] {
			sprite, err := bucketSprite(out, spec.pkg, expr)
			if err != nil {
				errs = append(errs, fmt.Errorf("%s.%s: %w", spec.named.Obj().Name(), field.Name(), err))
				continue
			}
			bucket.Sprites = append(bucket.Sprites, sprite)
		}
		out.Resources.FieldIDs[field] = []int{len(out.Resources.Buckets)}
		out.Resources.Buckets = append(out.Resources.Buckets, bucket)
	}
	return errs
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

func addDirectiveResource(out *EngineDeclarations, spec resourceDirectiveSpec) []error {
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
	namedFields, errs := resourceInitializerNames(spec)
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

func parseOptionBase(field *types.Var, tag tagValue) resource.EngineConfigurationOptionBase {
	return resource.EngineConfigurationOptionBase{Name: core.Text(staticName(tag, field.Name())), Description: tag.items["description"], Scope: tag.items["scope"], Standard: tag.flags["standard"], Advanced: tag.flags["advanced"]}
}

func parseConfiguration(named *types.Named) (*resource.EngineConfiguration, []error) {
	cfg := &resource.EngineConfiguration{Options: []resource.EngineConfigurationOption{}}
	var errs []error
	st := named.Underlying().(*types.Struct)
	for i := 0; i < st.NumFields(); i++ {
		field := st.Field(i)
		if field.Embedded() {
			continue
		}
		tag, ok := sonolusTag(st.Tag(i))
		if !ok {
			continue
		}
		errs = append(errs, validateTag("configuration."+field.Name(), tag,
			[]string{"slider", "toggle", "select", "standard", "advanced"},
			[]string{"name", "description", "scope", "def", "min", "max", "step", "unit", "values"})...)
		base := parseOptionBase(field, tag)
		parseFloat := func(key string) float64 {
			value, err := strconv.ParseFloat(tag.items[key], 64)
			if err != nil {
				errs = append(errs, fmt.Errorf("configuration.%s: invalid %s", field.Name(), key))
				return 0
			}
			return value
		}
		switch {
		case tag.flags["slider"]:
			cfg.Options = append(cfg.Options, resource.EngineConfigurationSliderOption{EngineConfigurationOptionBase: base, Type: resource.EngineConfigurationOptionTypeSlider, Def: parseFloat("def"), Min: parseFloat("min"), Max: parseFloat("max"), Step: parseFloat("step"), Unit: core.Text(tag.items["unit"])})
		case tag.flags["toggle"]:
			def, _ := strconv.ParseBool(tag.items["def"])
			n := 0
			if def {
				n = 1
			}
			cfg.Options = append(cfg.Options, resource.EngineConfigurationToggleOption{EngineConfigurationOptionBase: base, Type: resource.EngineConfigurationOptionTypeToggle, Def: n})
		case tag.flags["select"]:
			def, err := strconv.Atoi(tag.items["def"])
			if err != nil {
				errs = append(errs, fmt.Errorf("configuration.%s: invalid def", field.Name()))
			}
			raw := tag.items["values"]
			var values []core.Text
			if raw != "" {
				for _, v := range strings.Split(raw, "|") {
					values = append(values, core.Text(v))
				}
			}
			cfg.Options = append(cfg.Options, resource.EngineConfigurationSelectOption{EngineConfigurationOptionBase: base, Type: resource.EngineConfigurationOptionTypeSelect, Def: def, Values: values})
		default:
			errs = append(errs, fmt.Errorf("configuration.%s: missing option kind", field.Name()))
		}
	}
	return cfg, errs
}

func packageROM(pkg *packages.Package) (*ROMDeclaration, []error) {
	var found *ROMDeclaration
	var errs []error
	for _, file := range pkg.Syntax {
		for _, decl := range file.Decls {
			gen, ok := decl.(*ast.GenDecl)
			if !ok || gen.Tok.String() != "var" {
				continue
			}
			for _, spec := range gen.Specs {
				vs := spec.(*ast.ValueSpec)
				for _, name := range vs.Names {
					obj, ok := pkg.TypesInfo.Defs[name].(*types.Var)
					if !ok {
						continue
					}
					id := typeID(obj.Type())
					if id != rootID("ROMValues") && id != rootID("ROMFile") {
						continue
					}
					if found != nil {
						errs = append(errs, fmt.Errorf("multiple ROM declarations"))
						continue
					}
					rom := &ROMDeclaration{PackagePath: pkg.PkgPath, Variable: name.Name}
					if id == rootID("ROMValues") {
						if len(vs.Values) != 1 {
							errs = append(errs, fmt.Errorf("%s: ROMValues requires one composite literal", name.Name))
							continue
						}
						lit, ok := vs.Values[0].(*ast.CompositeLit)
						if !ok {
							errs = append(errs, fmt.Errorf("%s: ROMValues requires a composite literal", name.Name))
							continue
						}
						for _, elt := range lit.Elts {
							tv := pkg.TypesInfo.Types[elt]
							if tv.Value == nil {
								errs = append(errs, fmt.Errorf("%s: ROM value must be constant", name.Name))
								continue
							}
							f, _ := constant.Float64Val(tv.Value)
							rom.Values = append(rom.Values, float32(f))
						}
					} else {
						if len(pkg.EmbedFiles) != 1 {
							errs = append(errs, fmt.Errorf("%s: ROMFile requires exactly one embedded file", name.Name))
							continue
						}
						data, err := os.ReadFile(pkg.EmbedFiles[0])
						if err != nil {
							errs = append(errs, err)
							continue
						}
						if len(data)%4 != 0 {
							errs = append(errs, fmt.Errorf("%s: ROM file length must be divisible by 4", name.Name))
							continue
						}
						rom.File = pkg.EmbedFiles[0]
						rom.Bytes = data
						for i := 0; i < len(data); i += 4 {
							rom.Values = append(rom.Values, math.Float32frombits(binary.LittleEndian.Uint32(data[i:])))
						}
					}
					found = rom
				}
			}
		}
	}
	return found, errs
}

func globalCallbacks(pkg *packages.Package, m mode.Mode, hasMarker bool) ([]*CallbackDeclaration, []error) {
	if !hasMarker {
		return nil, nil
	}
	wanted := map[string]string{}
	if m == mode.ModeWatch {
		wanted["UpdateSpawn"] = "float"
	}
	if m == mode.ModeTutorial {
		wanted["Preprocess"] = "void"
		wanted["Navigate"] = "void"
		wanted["Update"] = "void"
	}
	var result []*CallbackDeclaration
	var errs []error
	for name, signature := range wanted {
		obj, ok := pkg.Types.Scope().Lookup(name).(*types.Func)
		if !ok {
			continue
		}
		if err := validCallbackSignature(obj, signature, false); err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", name, err))
			continue
		}
		decl := findFuncDecl(pkg, obj)
		refs, intrinsicErrs := callbackIntrinsics(pkg, decl, m, callbackKey(name))
		errs = append(errs, intrinsicErrs...)
		result = append(result, &CallbackDeclaration{Name: callbackKey(name), Function: obj, Decl: decl, Intrinsics: refs})
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Name < result[j].Name })
	return result, errs
}

// ParsePackageToFrontend parses declarations reachable from pkg for one mode.
func ParsePackageToFrontend(pkg *packages.Package, m mode.Mode) (*EngineDeclarations, error) {
	out := &EngineDeclarations{Mode: m, Configuration: &resource.EngineConfiguration{Options: []resource.EngineConfigurationOption{}}, Resources: ModeResources{SpriteIDs: map[string]int{}, EffectIDs: map[string]int{}, ParticleIDs: map[string]int{}, FieldIDs: map[*types.Var][]int{}}}
	var errs []error
	names := map[string]bool{}
	resources := map[string]bool{}
	configurationFound := false
	globalFound := false
	for _, p := range collectPackages(pkg) {
		directiveResources, directiveErrs := resourceDirectives(p)
		errs = append(errs, directiveErrs...)
		for _, spec := range directiveResources {
			if resources[spec.kind] {
				errs = append(errs, fmt.Errorf("duplicate %s resource", spec.kind))
				continue
			}
			allowed := spec.kind == "Skin" ||
				((spec.kind == "Effect" || spec.kind == "Particle") && m != mode.ModePreview) ||
				(spec.kind == "Buckets" && (m == mode.ModePlay || m == mode.ModeWatch)) ||
				((spec.kind == "Instruction" || spec.kind == "InstructionIcon") && m == mode.ModeTutorial)
			if !allowed {
				errs = append(errs, fmt.Errorf("%s: %s resources are not available in %s mode", spec.pos, strings.ToLower(spec.kind), m))
				continue
			}
			resources[spec.kind] = true
			switch spec.kind {
			case "Skin":
				renderMode := spec.renderMode
				if renderMode == "" {
					renderMode = resource.EngineRenderModeDefault
				}
				out.Resources.Skin = &resource.EngineSkinData{RenderMode: renderMode}
			case "Effect":
				out.Resources.Effect = &resource.EngineEffectData{}
			case "Particle":
				out.Resources.Particle = &resource.EngineParticleData{}
			case "Instruction", "InstructionIcon":
				if out.Resources.Instruction == nil {
					out.Resources.Instruction = &resource.EngineInstructionData{}
				}
			case "Buckets":
			}
			errs = append(errs, addDirectiveResource(out, spec)...)
		}
		rom, romErrs := packageROM(p)
		errs = append(errs, romErrs...)
		if rom != nil {
			if out.ROM != nil {
				errs = append(errs, fmt.Errorf("multiple ROM declarations"))
			} else {
				out.ROM = rom
			}
		}
		hasGlobals := false
		for _, named := range packageNamedTypes(p) {
			id, marker, ok := structMarker(named)
			if !ok {
				continue
			}
			if id == markerID(m, "Archetype") {
				a, parseErrs := parseArchetype(p, named, m, marker)
				errs = append(errs, parseErrs...)
				if names[a.Name] {
					errs = append(errs, fmt.Errorf("duplicate archetype %q", a.Name))
				} else {
					names[a.Name] = true
					out.Archetypes = append(out.Archetypes, a)
				}
				continue
			}
			if id == rootID("Configuration") {
				vars := markerVariables(p, named)
				if len(vars) != 1 {
					errs = append(errs, fmt.Errorf("%s: configuration marker requires exactly one singleton variable", named.Obj().Name()))
					continue
				}
				if configurationFound {
					errs = append(errs, fmt.Errorf("duplicate configuration declaration"))
					continue
				}
				configurationFound = true
				cfg, cfgErrs := parseConfiguration(named)
				errs = append(errs, cfgErrs...)
				out.Configuration = cfg
				continue
			}
			if id == markerID(m, "GlobalCallbacks") {
				vars := markerVariables(p, named)
				if len(vars) != 1 {
					errs = append(errs, fmt.Errorf("%s: global callback marker requires exactly one singleton variable", named.Obj().Name()))
				} else if globalFound {
					errs = append(errs, fmt.Errorf("duplicate global callback declaration"))
				} else {
					globalFound = true
					hasGlobals = true
				}
				continue
			}
		}
		globals, globalErrs := globalCallbacks(p, m, hasGlobals)
		errs = append(errs, globalErrs...)
		out.Globals = append(out.Globals, globals...)
	}
	sort.Slice(out.Archetypes, func(i, j int) bool {
		if out.Archetypes[i].Name == out.Archetypes[j].Name {
			return out.Archetypes[i].PackagePath < out.Archetypes[j].PackagePath
		}
		return out.Archetypes[i].Name < out.Archetypes[j].Name
	})
	if len(errs) > 0 {
		messages := make([]string, len(errs))
		for i, err := range errs {
			messages[i] = err.Error()
		}
		sort.Strings(messages)
		return nil, fmt.Errorf("%s", strings.Join(messages, "\n"))
	}
	return out, nil
}
