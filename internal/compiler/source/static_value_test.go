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

	"errors"
	"golang.org/x/tools/go/packages"

	"os"
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
func TestEvalPackageScalarValues(t *testing.T) {
	pkg := mustStaticPackage(t, map[string]string{
		"a.go": `package main

type Small uint8

const (
	IotaZero = iota
	IotaOne
)

const Precise = 1.0 / 10.0

var Forward = Later + 2
var Rounded float32 = 0.1
var Wide uint16 = 300
var Wrapped = Small(Wide)
var Zero int
`,
		"b.go": `package main

var Later int8 = 125
var Negative int8 = -1
var ShiftRight = Negative >> 100
var One uint8 = 1
var ShiftLeft = One << 100
var Logic = Forward == 127 && !false
`,
	})
	tracer := NewASTTracer(pkg)

	tests := []struct {
		name string
		want int64
	}{
		{name: "IotaZero", want: 0},
		{name: "IotaOne", want: 1},
		{name: "Forward", want: 127},
		{name: "Later", want: 125},
		{name: "Wrapped", want: 44},
		{name: "Zero", want: 0},
		{name: "ShiftRight", want: -1},
		{name: "ShiftLeft", want: 0},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			binding := mustEvalBinding(t, tracer, test.name)
			if got := staticInt64(t, binding.Value); got != test.want {
				t.Fatalf("%s = %d, want %d", test.name, got, test.want)
			}
			if _, ok := binding.Object.(*types.Const); ok && binding.Storage != nil {
				t.Fatalf("constant %s unexpectedly has storage", test.name)
			}
		})
	}

	precise := mustEvalBinding(t, tracer, "Precise")
	wantPrecise := constant.MakeFromLiteral("0.1", token.FLOAT, 0)
	if !constant.Compare(precise.Value.Exact, token.EQL, wantPrecise) {
		t.Fatalf("Precise = %s, want exact 0.1", precise.Value.Exact.ExactString())
	}

	rounded := mustEvalBinding(t, tracer, "Rounded")
	if got, want := staticFloat64(t, rounded.Value), float64(float32(0.1)); got != want {
		t.Fatalf("Rounded = %.20g, want %.20g", got, want)
	}
	if got := types.TypeString(mustEvalBinding(t, tracer, "Wrapped").Value.Type, nil); got != "static.test/main.Small" {
		t.Fatalf("Wrapped type = %s", got)
	}
	if !staticBoolValue(t, mustEvalBinding(t, tracer, "Logic").Value) {
		t.Fatal("Logic = false, want true")
	}
}

func TestEvalValueSpecOrderAndAtomicFailure(t *testing.T) {
	pkg := mustStaticPackage(t, map[string]string{
		"main.go": `package main

func dynamic() int { return 9 }

var First, Second = 1, 2
var Good, Bad = 3, dynamic()

var Lookup, Present = map[string]int{"key": 7}["key"]
var Missing, MissingOK = map[string]int{"key": 7}["missing"]

var Any any = 4
var Asserted, AssertOK = Any.(int)
var Wrong, WrongOK = Any.(string)

const (
	ConstA, ConstB = iota, iota + 10
	ConstC, ConstD
)
`,
	})
	tracer := NewASTTracer(pkg)
	any := mustEvalBinding(t, tracer, "Any").Value
	if any.Kind != StaticInterface || any.Dynamic == nil || types.TypeString(any.Dynamic.Type, nil) != "int" {
		t.Fatalf("Any dynamic value = %#v, want int", any)
	}

	bindings, err := tracer.EvalValueSpec(findValueSpec(t, pkg, "First"))
	if err != nil {
		t.Fatalf("evaluate First spec: %v", err)
	}
	if len(bindings) != 2 || bindings[0].Name != "First" || bindings[1].Name != "Second" {
		t.Fatalf("unexpected binding order: %#v", bindings)
	}
	if staticInt64(t, bindings[0].Value) != 1 || staticInt64(t, bindings[1].Value) != 2 {
		t.Fatalf("unexpected First spec values: %#v", bindings)
	}

	bindings, err = tracer.EvalValueSpec(findValueSpec(t, pkg, "Good"))
	if err != nil || len(bindings) != 2 || bindings[1].Value.Kind != StaticFunctionCall {
		t.Fatalf("Good/Bad bindings = %#v, error = %v", bindings, err)
	}

	tests := []struct {
		name       string
		wantValue  int64
		wantOK     bool
		wantString bool
	}{
		{name: "Lookup", wantValue: 7, wantOK: true},
		{name: "Missing", wantValue: 0, wantOK: false},
		{name: "Asserted", wantValue: 4, wantOK: true},
		{name: "Wrong", wantOK: false, wantString: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			results, err := tracer.EvalValueSpec(findValueSpec(t, pkg, test.name))
			if err != nil {
				t.Fatalf("evaluate %s: %v", test.name, err)
			}
			if len(results) != 2 {
				t.Fatalf("result count = %d, want 2", len(results))
			}
			if test.wantString {
				if got := staticString(t, results[0].Value); got != "" {
					t.Fatalf("zero assertion value = %q", got)
				}
			} else if got := staticInt64(t, results[0].Value); got != test.wantValue {
				t.Fatalf("value = %d, want %d", got, test.wantValue)
			}
			if got := staticBoolValue(t, results[1].Value); got != test.wantOK {
				t.Fatalf("ok = %t, want %t", got, test.wantOK)
			}
		})
	}

	constants, err := tracer.EvalValueSpec(findValueSpec(t, pkg, "ConstC"))
	if err != nil {
		t.Fatalf("evaluate omitted const spec: %v", err)
	}
	if len(constants) != 2 || staticInt64(t, constants[0].Value) != 1 || staticInt64(t, constants[1].Value) != 11 {
		t.Fatalf("unexpected omitted const values: %#v", constants)
	}
}

func TestEvalPackageValueLookupErrors(t *testing.T) {
	pkg := mustStaticPackage(t, map[string]string{
		"main.go": `package main

type Number int
func Function() {}
var Value = 1
`,
	})
	tracer := NewASTTracer(pkg)

	for _, name := range []string{"Missing", "Number", "Function"} {
		_, err := tracer.EvalPackageValue(name)
		if !errors.Is(err, ErrNotStatic) {
			t.Fatalf("EvalPackageValue(%q) error = %v, want ErrNotStatic", name, err)
		}
	}
}
func TestCompositeValuesAndBuiltins(t *testing.T) {
	pkg := mustStaticPackage(t, map[string]string{
		"main.go": `package main

type Pair struct {
	Number int
	Text string
}

var Struct = Pair{Text: "set"}
var Array = [4]int{1, 3: 4}
var Slice = []int{1, 2: 3}
var DynamicKey = "key"
var Map = map[string]int{DynamicKey: 1, "other": 2, DynamicKey: 3}
var Any any = Struct
var Nested = []Pair{{Number: 1}, {Text: "two"}}

var MadeSlice = make([]int, 2, 4)
var MadeMap = make(map[string]int, 8)
var SliceLength = len(MadeSlice)
var SliceCapacity = cap(MadeSlice)
var MapLength = len(Map)

var Low = 3
var High = 8
var Minimum = min(Low, High, 5)
var Maximum = max(Low, High, 5)
var RealPart float32 = 1.25
var ImagPart float32 = -2.5
var ComplexValue = complex(RealPart, ImagPart)
var RealValue = real(ComplexValue)
var ImagValue = imag(ComplexValue)
var FloatForInt = 12.9
var IntFromFloat = int8(FloatForInt)

var Text = "é"
var Bytes = []byte(Text)
var Runes = []rune(Text)
var BytesText = string(Bytes)
var RunesText = string(Runes)
var SliceArray = [2]int(Slice)
var NilSlice []int
var EmptyArray = [0]int(NilSlice)
var EmptyArrayPointer = (*[0]int)(NilSlice)

var ViewSource = []int{1, 2, 3, 4}
var ViewSub = ViewSource[1:4]
var ViewPointer = (*[2]int)(ViewSub)
var ViewArray = *ViewPointer
var ViewFirst = &ViewPointer[0]
var SubFirst = &ViewSub[0]
var ViewSameElement = ViewFirst == SubFirst
var ViewReslice = ViewPointer[:]

var DirectArray = [2]int{5, 6}
var DirectSlice = DirectArray[:]
var DirectConverted = (*[2]int)(DirectSlice)
var DirectSamePointer = DirectConverted == &DirectArray

func dynamicBool() bool { return true }
func dynamicInt() int { return 1 }
var ShortCircuit = false && dynamicBool()
var UnevaluatedArrayLength = len([2]int{dynamicInt(), dynamicInt()})
`,
	})
	tracer := NewASTTracer(pkg)

	structValue := mustEvalBinding(t, tracer, "Struct").Value
	if structValue.Kind != StaticStruct || len(structValue.Fields) != 2 {
		t.Fatalf("Struct = %#v", structValue)
	}
	if got := staticInt64(t, structValue.Fields[0].Value); got != 0 {
		t.Fatalf("Struct.Number = %d, want 0", got)
	}
	if got := staticString(t, structValue.Fields[1].Value); got != "set" {
		t.Fatalf("Struct.Text = %q", got)
	}
	if structValue.Fields[0].Explicit || !structValue.Fields[1].Explicit {
		t.Fatalf("Struct explicit fields = %#v", structValue.Fields)
	}

	array := mustEvalBinding(t, tracer, "Array").Value
	if array.Kind != StaticArray || len(array.Elements) != 4 {
		t.Fatalf("Array = %#v", array)
	}
	wantArray := []int64{1, 0, 0, 4}
	for index, want := range wantArray {
		if got := staticInt64(t, array.Elements[index]); got != want {
			t.Fatalf("Array[%d] = %d, want %d", index, got, want)
		}
	}

	slice := mustEvalBinding(t, tracer, "Slice").Value
	if slice.Kind != StaticSliceValue || slice.Slice == nil || slice.Slice.Len != 3 || slice.Slice.Cap != 3 {
		t.Fatalf("Slice = %#v", slice)
	}
	wantSlice := []int64{1, 0, 3}
	for index, want := range wantSlice {
		path := append([]StaticPathStep(nil), slice.Slice.Path...)
		path = append(path, StaticPathStep{Kind: StaticPathElement, Index: slice.Slice.Offset + int64(index)})
		value, ok := staticValueAtAddress(&StaticAddress{Object: slice.Slice.Backing, Path: path})
		if !ok {
			t.Fatalf("load Slice[%d] from public object graph", index)
		}
		if got := staticInt64(t, value); got != want {
			t.Fatalf("Slice[%d] = %d, want %d", index, got, want)
		}
	}

	staticMap := mustEvalBinding(t, tracer, "Map").Value
	if staticMap.Kind != StaticMapValue || staticMap.Map == nil || len(staticMap.Map.Entries) != 2 {
		t.Fatalf("Map = %#v", staticMap)
	}
	if got := staticString(t, staticMap.Map.Entries[0].Key); got != "key" {
		t.Fatalf("first map key = %q, want key", got)
	}
	if got := staticInt64(t, staticMap.Map.Entries[0].Value); got != 3 {
		t.Fatalf("Map[key] = %d, want 3", got)
	}

	any := mustEvalBinding(t, tracer, "Any").Value
	if any.Kind != StaticInterface || any.Dynamic == nil || any.Dynamic.Kind != StaticStruct {
		t.Fatalf("Any = %#v", any)
	}

	for name, want := range map[string]int64{
		"SliceLength":            2,
		"SliceCapacity":          4,
		"MapLength":              2,
		"Minimum":                3,
		"Maximum":                8,
		"UnevaluatedArrayLength": 2,
	} {
		if got := staticInt64(t, mustEvalBinding(t, tracer, name).Value); got != want {
			t.Fatalf("%s = %d, want %d", name, got, want)
		}
	}
	if staticBoolValue(t, mustEvalBinding(t, tracer, "ShortCircuit").Value) {
		t.Fatal("ShortCircuit = true, want false")
	}

	if got := staticFloat64(t, mustEvalBinding(t, tracer, "RealValue").Value); got != 1.25 {
		t.Fatalf("RealValue = %v", got)
	}
	if got := staticFloat64(t, mustEvalBinding(t, tracer, "ImagValue").Value); got != -2.5 {
		t.Fatalf("ImagValue = %v", got)
	}
	if got := staticInt64(t, mustEvalBinding(t, tracer, "IntFromFloat").Value); got != 12 {
		t.Fatalf("IntFromFloat = %d, want 12", got)
	}
	complexValue := mustEvalBinding(t, tracer, "ComplexValue").Value
	if got := types.TypeString(complexValue.Type, nil); got != "complex64" {
		t.Fatalf("ComplexValue type = %s, want complex64", got)
	}

	bytes := mustEvalBinding(t, tracer, "Bytes").Value
	if bytes.Slice == nil || bytes.Slice.Len != 2 {
		t.Fatalf("Bytes = %#v", bytes)
	}
	runes := mustEvalBinding(t, tracer, "Runes").Value
	if runes.Slice == nil || runes.Slice.Len != 1 {
		t.Fatalf("Runes = %#v", runes)
	}
	if got := staticString(t, mustEvalBinding(t, tracer, "BytesText").Value); got != "é" {
		t.Fatalf("BytesText = %q", got)
	}
	if got := staticString(t, mustEvalBinding(t, tracer, "RunesText").Value); got != "é" {
		t.Fatalf("RunesText = %q", got)
	}

	sliceArray := mustEvalBinding(t, tracer, "SliceArray").Value
	if len(sliceArray.Elements) != 2 || staticInt64(t, sliceArray.Elements[0]) != 1 || staticInt64(t, sliceArray.Elements[1]) != 0 {
		t.Fatalf("SliceArray = %#v", sliceArray)
	}
	emptyArray := mustEvalBinding(t, tracer, "EmptyArray").Value
	if emptyArray.Kind != StaticArray || len(emptyArray.Elements) != 0 {
		t.Fatalf("EmptyArray = %#v", emptyArray)
	}
	if pointer := mustEvalBinding(t, tracer, "EmptyArrayPointer").Value; pointer.Kind != StaticNil {
		t.Fatalf("EmptyArrayPointer = %#v, want nil", pointer)
	}
	viewArray := mustEvalBinding(t, tracer, "ViewArray").Value
	if len(viewArray.Elements) != 2 || staticInt64(t, viewArray.Elements[0]) != 2 || staticInt64(t, viewArray.Elements[1]) != 3 {
		t.Fatalf("ViewArray = %#v", viewArray)
	}
	if !staticBoolValue(t, mustEvalBinding(t, tracer, "ViewSameElement").Value) {
		t.Fatal("slice-to-array pointer did not preserve the sub-slice start address")
	}
	viewReslice := mustEvalBinding(t, tracer, "ViewReslice").Value
	if viewReslice.Slice == nil || viewReslice.Slice.Offset != 1 || viewReslice.Slice.Len != 2 || viewReslice.Slice.Cap != 2 {
		t.Fatalf("ViewReslice = %#v", viewReslice)
	}
	if !staticBoolValue(t, mustEvalBinding(t, tracer, "DirectSamePointer").Value) {
		t.Fatal("slice-to-array pointer did not normalize the backing array address")
	}

	if madeMap := mustEvalBinding(t, tracer, "MadeMap").Value; madeMap.Kind != StaticMapValue || madeMap.Map == nil {
		t.Fatalf("MadeMap = %#v", madeMap)
	}
}

func TestPointerGraphAndCopyAliasing(t *testing.T) {
	pkg := mustStaticPackage(t, map[string]string{
		"main.go": `package main

type Node struct {
	Value int
	Next *Node
}

type Holder struct {
	Pointer *int
	Slice []int
	Map map[string]int
}

var Root = Node{Value: 1}
var RootPointer = &Root
var FieldPointer = &Root.Value
var Array = [2]int{3, 4}
var ElementPointer = &Array[1]
var Slice = []int{5, 6}
var SlicePointer = &Slice[0]
var NewValue = new(int)

var Shared = 9
var Original = Holder{Pointer: &Shared, Slice: []int{7}, Map: map[string]int{"x": 8}}
var Copied = Original
var HolderArray = [1]Holder{Original}
var HolderArrayCopy = HolderArray

var ForwardPointer *Node = &ForwardNode
var ForwardNode = Node{Value: 10}
`,
	})
	tracer := NewASTTracer(pkg)

	root := mustEvalBinding(t, tracer, "Root")
	rootPointer := mustEvalBinding(t, tracer, "RootPointer").Value
	if rootPointer.Kind != StaticPointer || rootPointer.Pointer == nil || rootPointer.Pointer.Object != root.Storage {
		t.Fatalf("RootPointer does not target Root storage: %#v", rootPointer)
	}
	fieldPointer := mustEvalBinding(t, tracer, "FieldPointer").Value
	if fieldPointer.Pointer == nil || fieldPointer.Pointer.Object != root.Storage || len(fieldPointer.Pointer.Path) != 1 || fieldPointer.Pointer.Path[0].Kind != StaticPathField {
		t.Fatalf("FieldPointer = %#v", fieldPointer)
	}

	array := mustEvalBinding(t, tracer, "Array")
	elementPointer := mustEvalBinding(t, tracer, "ElementPointer").Value
	if elementPointer.Pointer == nil || elementPointer.Pointer.Object != array.Storage || len(elementPointer.Pointer.Path) != 1 || elementPointer.Pointer.Path[0].Index != 1 {
		t.Fatalf("ElementPointer = %#v", elementPointer)
	}

	slice := mustEvalBinding(t, tracer, "Slice").Value
	slicePointer := mustEvalBinding(t, tracer, "SlicePointer").Value
	if slicePointer.Pointer == nil || slice.Slice == nil || slicePointer.Pointer.Object != slice.Slice.Backing {
		t.Fatalf("SlicePointer = %#v, Slice = %#v", slicePointer, slice)
	}
	newValue := mustEvalBinding(t, tracer, "NewValue").Value
	if newValue.Pointer == nil || staticInt64(t, newValue.Pointer.Object.Value) != 0 {
		t.Fatalf("NewValue = %#v", newValue)
	}

	original := mustEvalBinding(t, tracer, "Original")
	copied := mustEvalBinding(t, tracer, "Copied")
	if original.Storage == copied.Storage {
		t.Fatal("struct copy reused package storage")
	}
	for _, fieldIndex := range []int{0, 1, 2} {
		left := original.Value.Fields[fieldIndex].Value
		right := copied.Value.Fields[fieldIndex].Value
		switch fieldIndex {
		case 0:
			if !staticAddressEqual(left.Pointer, right.Pointer) {
				t.Fatal("pointer identity was not preserved across struct copy")
			}
		case 1:
			if left.Slice == nil || right.Slice == nil || left.Slice.Backing != right.Slice.Backing {
				t.Fatal("slice backing identity was not preserved across struct copy")
			}
		case 2:
			if left.Map == nil || right.Map == nil || left.Map != right.Map {
				t.Fatal("map identity was not preserved across struct copy")
			}
		}
	}

	arrayOriginal := mustEvalBinding(t, tracer, "HolderArray")
	arrayCopy := mustEvalBinding(t, tracer, "HolderArrayCopy")
	if arrayOriginal.Storage == arrayCopy.Storage || &arrayOriginal.Value.Elements[0] == &arrayCopy.Value.Elements[0] {
		t.Fatal("array value was not copied")
	}
	leftHolder := arrayOriginal.Value.Elements[0]
	rightHolder := arrayCopy.Value.Elements[0]
	if !staticAddressEqual(leftHolder.Fields[0].Value.Pointer, rightHolder.Fields[0].Value.Pointer) {
		t.Fatal("pointer identity was not preserved across array copy")
	}

	forwardNode := mustEvalBinding(t, tracer, "ForwardNode")
	forwardPointer := mustEvalBinding(t, tracer, "ForwardPointer").Value
	if forwardPointer.Pointer == nil || forwardPointer.Pointer.Object != forwardNode.Storage {
		t.Fatalf("forward address was not preserved: %#v", forwardPointer)
	}
}

func TestTypedNilInterfaceSemantics(t *testing.T) {
	pkg := mustStaticPackage(t, map[string]string{
		"main.go": `package main

var Pointer *int
var PointerAny any = Pointer
var NilAny any = nil
var NilAnyCopy any = NilAny
var PointerAnyIsNil = PointerAny == nil
var NilAnyIsNil = NilAny == nil
var NilAnyCopyIsNil = NilAnyCopy == nil
var PointerAsserted, PointerOK = PointerAny.(*int)
var RoundedAny any = float32(0.1)
var RoundedAsserted = RoundedAny.(float32)

var Slice []int
var SliceAny any = Slice
var OtherSliceAny any = Slice
var PanicCompare = SliceAny == OtherSliceAny
`,
	})
	tracer := NewASTTracer(pkg)

	pointerAny := mustEvalBinding(t, tracer, "PointerAny").Value
	if pointerAny.Kind != StaticInterface || pointerAny.Dynamic == nil || pointerAny.Dynamic.Kind != StaticNil {
		t.Fatalf("PointerAny = %#v", pointerAny)
	}
	if got := types.TypeString(pointerAny.Dynamic.Type, nil); got != "*int" {
		t.Fatalf("PointerAny dynamic type = %s", got)
	}
	if mustEvalBinding(t, tracer, "NilAny").Value.Kind != StaticNil {
		t.Fatal("NilAny is not a nil interface")
	}
	if mustEvalBinding(t, tracer, "NilAnyCopy").Value.Kind != StaticNil {
		t.Fatal("NilAnyCopy is not a nil interface")
	}
	if staticBoolValue(t, mustEvalBinding(t, tracer, "PointerAnyIsNil").Value) {
		t.Fatal("typed nil pointer interface compared equal to nil")
	}
	if !staticBoolValue(t, mustEvalBinding(t, tracer, "NilAnyIsNil").Value) {
		t.Fatal("nil interface did not compare equal to nil")
	}
	if !staticBoolValue(t, mustEvalBinding(t, tracer, "NilAnyCopyIsNil").Value) {
		t.Fatal("copied nil interface did not compare equal to nil")
	}
	assertion, err := tracer.EvalValueSpec(findValueSpec(t, pkg, "PointerAsserted"))
	if err != nil {
		t.Fatalf("typed nil assertion: %v", err)
	}
	if assertion[0].Value.Kind != StaticNil || !staticBoolValue(t, assertion[1].Value) {
		t.Fatalf("typed nil assertion = %#v", assertion)
	}
	if got, want := staticFloat64(t, mustEvalBinding(t, tracer, "RoundedAsserted").Value), float64(float32(0.1)); got != want {
		t.Fatalf("RoundedAsserted = %.20g, want %.20g", got, want)
	}

	if _, err := tracer.EvalPackageValue("PanicCompare"); !errors.Is(err, ErrStaticPanic) {
		t.Fatalf("PanicCompare error = %v, want ErrStaticPanic", err)
	}
}
func TestStaticErrorGolden(t *testing.T) {
	pkg := mustStaticPackage(t, map[string]string{
		"golden.go": `package main

func dynamic() int { return 1 }

var Call = dynamic()
var Func = dynamic
var Append = append([]int{}, 1)
var Channel = make(chan int)
var Bounds = []int{1}[2]
`,
	})
	tracer := NewASTTracer(pkg)
	var actual strings.Builder
	call := mustEvalBinding(t, tracer, "Call").Value
	if call.Kind != StaticFunctionCall || call.Call == nil || call.Call.Object.Name() != "dynamic" {
		t.Fatalf("Call = %#v, want symbolic dynamic call", call)
	}
	for _, name := range []string{"Func", "Append", "Channel", "Bounds"} {
		_, err := tracer.EvalPackageValue(name)
		category := "unexpected"
		switch {
		case errors.Is(err, ErrNotStatic):
			category = "not-static"
		case errors.Is(err, ErrStaticPanic):
			category = "static-panic"
		}
		var evalErr *StaticEvalError
		if !errors.As(err, &evalErr) {
			t.Fatalf("%s error = %v, want *StaticEvalError", name, err)
		}
		fmt.Fprintf(
			&actual,
			"%s|%s|%d:%d|%s\n",
			name,
			category,
			evalErr.Pos.Line,
			evalErr.Pos.Column,
			staticExprString(evalErr.Expr),
		)
	}

	expected, err := os.ReadFile("../testdata/source/static_errors.golden")
	if err != nil {
		t.Fatalf("read golden: %v", err)
	}
	if actual.String() != string(expected) {
		t.Fatalf("error golden mismatch\nactual:\n%s\nexpected:\n%s", actual.String(), expected)
	}
}

func TestStaticEvaluationErrors(t *testing.T) {
	pkg := mustStaticPackage(t, map[string]string{
		"errors.go": `package main

import "unsafe"

func dynamic() int { return 1 }

var UnsupportedCall = dynamic()
var UnsupportedFunc = dynamic
var UnsupportedAppend = append([]int{}, 1)
var UnsupportedCopy = copy(make([]int, 1), []int{1})
var UnsupportedChannel = make(chan int)
var UnsupportedUnsafe = unsafe.Sizeof(1)
var Huge float64 = 1e300
var UnsupportedInfinity = float32(Huge)
var FloatNumerator = 1.0
var FloatDenominator = 0.0
var UnsupportedFloatDivide = FloatNumerator / FloatDenominator
var UnsupportedFloatInt = int(Huge)

var PanicIndex = []int{1}[2]
var NilPointer *int
var PanicDeref = *NilPointer
type Struct struct { Field int }
var NilStruct *Struct
var PanicSelector = NilStruct.Field
var NilArrayPointer *[1]int
var PanicPointerIndex = NilArrayPointer[0]
var Any any = 1
var PanicAssertion = Any.(string)
var Numerator = 1
var Denominator = 0
var PanicDivide = Numerator / Denominator
var NegativeShift = -1
var PanicShift = 1 << NegativeShift
var NilSlice []int
var SliceAny any = NilSlice
var OtherSliceAny any = NilSlice
var PanicInterfaceCompare = SliceAny == OtherSliceAny
var BadMapKey any = []int{1}
var PanicMapLiteral = map[any]int{BadMapKey: 1}
var NilInterfaceMap map[any]int
var PanicMapLookup = NilInterfaceMap[BadMapKey]

var Good = 42
`,
	})
	tracer := NewASTTracer(pkg)
	call := mustEvalBinding(t, tracer, "UnsupportedCall").Value
	if call.Kind != StaticFunctionCall || call.Call == nil || call.Call.Object.Name() != "dynamic" {
		t.Fatalf("UnsupportedCall = %#v", call)
	}

	for _, name := range []string{
		"UnsupportedFunc",
		"UnsupportedAppend",
		"UnsupportedCopy",
		"UnsupportedChannel",
		"UnsupportedUnsafe",
		"UnsupportedInfinity",
		"UnsupportedFloatDivide",
		"UnsupportedFloatInt",
	} {
		t.Run(name, func(t *testing.T) {
			_, err := tracer.EvalPackageValue(name)
			if !errors.Is(err, ErrNotStatic) {
				t.Fatalf("error = %v, want ErrNotStatic", err)
			}
			var evalErr *StaticEvalError
			if !errors.As(err, &evalErr) || !evalErr.Pos.IsValid() || evalErr.Pos.Filename != "errors.go" {
				t.Fatalf("error has no source position: %#v", err)
			}
		})
	}

	for _, name := range []string{
		"PanicIndex",
		"PanicDeref",
		"PanicSelector",
		"PanicPointerIndex",
		"PanicAssertion",
		"PanicDivide",
		"PanicShift",
		"PanicInterfaceCompare",
		"PanicMapLiteral",
		"PanicMapLookup",
	} {
		t.Run(name, func(t *testing.T) {
			_, err := tracer.EvalPackageValue(name)
			if !errors.Is(err, ErrStaticPanic) {
				t.Fatalf("error = %v, want ErrStaticPanic", err)
			}
			var evalErr *StaticEvalError
			if !errors.As(err, &evalErr) || !evalErr.Pos.IsValid() {
				t.Fatalf("error has no source position: %#v", err)
			}
		})
	}

	if got := staticInt64(t, mustEvalBinding(t, tracer, "Good").Value); got != 42 {
		t.Fatalf("Good = %d, want 42", got)
	}
}

func TestStaticErrorPositionAndCaching(t *testing.T) {
	pkg := mustStaticPackage(t, map[string]string{
		"position.go": `package main

func dynamic() int { return 1 }

var fn = dynamic
var Failure = fn()
`,
	})
	tracer := NewASTTracer(pkg)

	_, first := tracer.EvalPackageValue("Failure")
	_, second := tracer.EvalPackageValue("Failure")
	if first == nil || second == nil || first.Error() != second.Error() {
		t.Fatalf("cached errors differ: first=%v second=%v", first, second)
	}
	var evalErr *StaticEvalError
	if !errors.As(first, &evalErr) {
		t.Fatalf("error type = %T, want *StaticEvalError", first)
	}
	if evalErr.Pos.Line != 6 || evalErr.Pos.Column != 15 {
		t.Fatalf("position = %s, want position.go:6:15", evalErr.Pos)
	}
	if evalErr.Object == nil || evalErr.Object.Name() != "Failure" {
		t.Fatalf("associated object = %v, want Failure", evalErr.Object)
	}
	if !strings.Contains(first.Error(), `expression "fn()"`) {
		t.Fatalf("error text = %q", first.Error())
	}
}

func TestReachableSymbolicStorageIsPreserved(t *testing.T) {
	pkg := mustStaticPackage(t, map[string]string{
		"main.go": `package main

func dynamic() int { return 1 }

var Dynamic = dynamic()
var PointerToDynamic = &Dynamic
var Independent = 7
`,
	})
	tracer := NewASTTracer(pkg)

	if got := staticInt64(t, mustEvalBinding(t, tracer, "Independent").Value); got != 7 {
		t.Fatalf("Independent = %d", got)
	}
	pointer := mustEvalBinding(t, tracer, "PointerToDynamic").Value
	if pointer.Kind != StaticPointer || pointer.Pointer == nil || pointer.Pointer.Object == nil || pointer.Pointer.Object.Value.Kind != StaticFunctionCall {
		t.Fatalf("PointerToDynamic = %#v, want pointer to symbolic call", pointer)
	}
}

func TestMissingTypeInformation(t *testing.T) {
	tracer := NewASTTracer(nil)
	if _, err := tracer.EvalPackageValue("Value"); !errors.Is(err, ErrMissingTypeInfo) {
		t.Fatalf("error = %v, want ErrMissingTypeInfo", err)
	}
	if _, err := tracer.EvalValueSpec(nil); !errors.Is(err, ErrMissingTypeInfo) {
		t.Fatalf("error = %v, want ErrMissingTypeInfo", err)
	}

	pkg := mustStaticPackage(t, map[string]string{
		"main.go": "package main\nconst Value = 1\n",
	})
	pkg.TypesSizes = nil
	tracer = NewASTTracer(pkg)
	if _, err := tracer.EvalPackageValue("Value"); !errors.Is(err, ErrMissingTypeInfo) {
		t.Fatalf("constant error = %v, want ErrMissingTypeInfo", err)
	}
	if _, err := tracer.EvalValueSpec(findValueSpec(t, pkg, "Value")); !errors.Is(err, ErrMissingTypeInfo) {
		t.Fatalf("value spec error = %v, want ErrMissingTypeInfo", err)
	}
}

func TestHugeNestedValueIsRejectedWithoutAllocation(t *testing.T) {
	pkg := mustStaticPackage(t, map[string]string{
		"main.go": "package main\nvar Huge [1048576][1048576]int\nvar Good = 1\n",
	})
	tracer := NewASTTracer(pkg)
	if _, err := tracer.EvalPackageValue("Huge"); !errors.Is(err, ErrNotStatic) {
		t.Fatalf("Huge error = %v, want ErrNotStatic", err)
	}
	if got := staticInt64(t, mustEvalBinding(t, tracer, "Good").Value); got != 1 {
		t.Fatalf("Good = %d, want 1", got)
	}
}
