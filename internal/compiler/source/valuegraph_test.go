package source

import (
	"errors"
	"go/types"
	"testing"
)

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
