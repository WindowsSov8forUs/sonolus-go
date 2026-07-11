package source

import (
	"errors"
	"go/constant"
	"go/token"
	"go/types"
	"testing"
)

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
	if !errors.Is(err, ErrNotStatic) {
		t.Fatalf("Good/Bad error = %v, want ErrNotStatic", err)
	}
	if bindings != nil {
		t.Fatalf("partially returned bindings: %#v", bindings)
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
