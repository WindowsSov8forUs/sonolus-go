package source

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/constant"
	"go/format"
	"go/importer"
	"go/parser"
	"go/token"
	"go/types"
	"sort"
	"strings"
	"testing"

	"golang.org/x/tools/go/packages"
)

func checkStaticPackage(sources map[string]string) (*packages.Package, error) {
	fset := token.NewFileSet()
	filenames := make([]string, 0, len(sources))
	for filename := range sources {
		filenames = append(filenames, filename)
	}
	sort.Strings(filenames)

	files := make([]*ast.File, 0, len(filenames))
	for _, filename := range filenames {
		file, err := parser.ParseFile(fset, filename, sources[filename], parser.AllErrors)
		if err != nil {
			return nil, err
		}
		files = append(files, file)
	}

	info := &types.Info{
		Types:      make(map[ast.Expr]types.TypeAndValue),
		Instances:  make(map[*ast.Ident]types.Instance),
		Defs:       make(map[*ast.Ident]types.Object),
		Uses:       make(map[*ast.Ident]types.Object),
		Implicits:  make(map[ast.Node]types.Object),
		Selections: make(map[*ast.SelectorExpr]*types.Selection),
		Scopes:     make(map[ast.Node]*types.Scope),
	}
	sizes := types.SizesFor("gc", "amd64")
	configuration := &types.Config{
		GoVersion: "go1.25",
		Importer:  importer.Default(),
		Sizes:     sizes,
	}
	typesPackage, err := configuration.Check("static.test/main", fset, files, info)
	if err != nil {
		return nil, err
	}

	return &packages.Package{
		ID:              typesPackage.Path(),
		Name:            typesPackage.Name(),
		PkgPath:         typesPackage.Path(),
		GoFiles:         filenames,
		CompiledGoFiles: filenames,
		Imports:         make(map[string]*packages.Package),
		Types:           typesPackage,
		Fset:            fset,
		IllTyped:        false,
		Syntax:          files,
		TypesInfo:       info,
		TypesSizes:      sizes,
	}, nil
}

func mustStaticPackage(t testing.TB, sources map[string]string) *packages.Package {
	t.Helper()
	pkg, err := checkStaticPackage(sources)
	if err != nil {
		t.Fatalf("type check package: %v", err)
	}
	return pkg
}

func findValueSpec(t testing.TB, pkg *packages.Package, name string) *ast.ValueSpec {
	t.Helper()
	for _, file := range pkg.Syntax {
		for _, declaration := range file.Decls {
			general, ok := declaration.(*ast.GenDecl)
			if !ok || general.Tok != token.VAR && general.Tok != token.CONST {
				continue
			}
			for _, specification := range general.Specs {
				valueSpec := specification.(*ast.ValueSpec)
				for _, ident := range valueSpec.Names {
					if ident.Name == name {
						return valueSpec
					}
				}
			}
		}
	}
	t.Fatalf("value spec %q not found", name)
	return nil
}

func findTypeSpec(t testing.TB, pkg *packages.Package, name string) *ast.TypeSpec {
	t.Helper()
	for _, file := range pkg.Syntax {
		for _, declaration := range file.Decls {
			general, ok := declaration.(*ast.GenDecl)
			if !ok || general.Tok != token.TYPE {
				continue
			}
			for _, specification := range general.Specs {
				typeSpec := specification.(*ast.TypeSpec)
				if typeSpec.Name.Name == name {
					return typeSpec
				}
			}
		}
	}
	t.Fatalf("type spec %q not found", name)
	return nil
}

func mustEvalBinding(t testing.TB, tracer *ASTTracer, name string) StaticBinding {
	t.Helper()
	binding, err := tracer.EvalPackageValue(name)
	if err != nil {
		t.Fatalf("evaluate %s: %v", name, err)
	}
	if binding.Name != name {
		t.Fatalf("binding name = %q, want %q", binding.Name, name)
	}
	return binding
}

func staticInt64(t testing.TB, value StaticValue) int64 {
	t.Helper()
	if value.Kind != StaticConstant || value.Exact == nil {
		t.Fatalf("value kind = %v, want constant", value.Kind)
	}
	result, ok := constant.Int64Val(value.Exact)
	if !ok {
		t.Fatalf("value %v is not an int64", value.Exact)
	}
	return result
}

func staticFloat64(t testing.TB, value StaticValue) float64 {
	t.Helper()
	if value.Kind != StaticConstant || value.Exact == nil {
		t.Fatalf("value kind = %v, want constant", value.Kind)
	}
	result, _ := constant.Float64Val(value.Exact)
	return result
}

func staticString(t testing.TB, value StaticValue) string {
	t.Helper()
	if value.Kind != StaticConstant || value.Exact == nil || value.Exact.Kind() != constant.String {
		t.Fatalf("value = %#v, want string constant", value)
	}
	return constant.StringVal(value.Exact)
}

func staticBoolValue(t testing.TB, value StaticValue) bool {
	t.Helper()
	result, ok := staticBool(value)
	if !ok {
		t.Fatalf("value = %#v, want bool constant", value)
	}
	return result
}

func staticBool(value StaticValue) (bool, bool) {
	if value.Kind != StaticConstant || value.Exact == nil || value.Exact.Kind() != constant.Bool {
		return false, false
	}
	return constant.BoolVal(value.Exact), true
}

func staticExprString(expr ast.Expr) string {
	if expr == nil {
		return ""
	}
	var buffer bytes.Buffer
	if err := format.Node(&buffer, token.NewFileSet(), expr); err != nil {
		return fmt.Sprintf("<%T>", expr)
	}
	return buffer.String()
}

func staticAddressEqual(left, right *StaticAddress) bool {
	if left == nil || right == nil {
		return left == right
	}
	if left.Object != right.Object || left.ArrayOffset != right.ArrayOffset || len(left.Path) != len(right.Path) {
		return false
	}
	for index := range left.Path {
		leftStep := left.Path[index]
		rightStep := right.Path[index]
		if leftStep.Kind != rightStep.Kind || leftStep.Index != rightStep.Index || leftStep.Field != rightStep.Field {
			return false
		}
	}
	return true
}

func staticValueAtAddress(address *StaticAddress) (StaticValue, bool) {
	if address == nil || address.Object == nil || address.ArrayView != nil {
		return StaticValue{}, false
	}
	value := address.Object.Value
	for _, step := range address.Path {
		switch step.Kind {
		case StaticPathField:
			if value.Kind != StaticStruct || step.Index < 0 || step.Index >= int64(len(value.Fields)) {
				return StaticValue{}, false
			}
			value = value.Fields[step.Index].Value
		case StaticPathElement:
			if value.Kind != StaticArray || step.Index < 0 || step.Index >= int64(len(value.Elements)) {
				return StaticValue{}, false
			}
			value = value.Elements[step.Index]
		default:
			return StaticValue{}, false
		}
	}
	return value, true
}

func staticValueDigest(value StaticValue) string {
	var builder strings.Builder
	visited := make(map[*StaticObject]bool)
	var writeValue func(StaticValue)
	writeObject := func(object *StaticObject) {
		if object == nil {
			builder.WriteString("object:nil")
			return
		}
		fmt.Fprintf(&builder, "object:%d:%s", object.ID, types.TypeString(object.Type, nil))
		if visited[object] {
			builder.WriteString(":seen")
			return
		}
		visited[object] = true
		builder.WriteByte('{')
		writeValue(object.Value)
		builder.WriteByte('}')
	}
	writeValue = func(value StaticValue) {
		fmt.Fprintf(&builder, "kind:%d:type:%s", value.Kind, types.TypeString(value.Type, nil))
		switch value.Kind {
		case StaticConstant:
			if value.Exact != nil {
				builder.WriteString(":")
				builder.WriteString(value.Exact.ExactString())
			}
		case StaticArray:
			for _, element := range value.Elements {
				builder.WriteByte('[')
				writeValue(element)
				builder.WriteByte(']')
			}
		case StaticStruct:
			for _, field := range value.Fields {
				builder.WriteString(field.Field.Name())
				builder.WriteByte('[')
				writeValue(field.Value)
				builder.WriteByte(']')
			}
		case StaticSliceValue:
			if value.Slice != nil {
				fmt.Fprintf(&builder, ":%d:%d:%d", value.Slice.Offset, value.Slice.Len, value.Slice.Cap)
				writeObject(value.Slice.Backing)
			}
		case StaticMapValue:
			if value.Map != nil {
				fmt.Fprintf(&builder, ":map:%d", value.Map.ID)
				for _, entry := range value.Map.Entries {
					writeValue(entry.Key)
					writeValue(entry.Value)
				}
			}
		case StaticPointer:
			if value.Pointer != nil {
				writeObject(value.Pointer.Object)
				fmt.Fprintf(&builder, ":offset:%d", value.Pointer.ArrayOffset)
				if value.Pointer.ArrayView != nil {
					builder.WriteString(":view:")
					builder.WriteString(types.TypeString(value.Pointer.ArrayView, nil))
				}
				for _, step := range value.Pointer.Path {
					fmt.Fprintf(&builder, ":path:%d:%d", step.Kind, step.Index)
				}
			}
		case StaticInterface:
			if value.Dynamic != nil {
				writeValue(*value.Dynamic)
			}
		}
	}
	writeValue(value)
	return builder.String()
}
