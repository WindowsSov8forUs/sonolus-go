package source

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"
)

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

	expected, err := os.ReadFile("testdata/static_errors.golden")
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
	if _, err := tracer.EvalPackageValue("Good"); !errors.Is(err, ErrNotStatic) {
		t.Fatalf("package preallocation error = %v, want ErrNotStatic", err)
	}
}
