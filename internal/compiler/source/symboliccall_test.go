package source

import (
	"errors"
	"go/types"
	"testing"
)

func TestStaticFunctionCallsPreserveTargetReceiverArgumentsAndSignature(t *testing.T) {
	pkg := mustStaticPackage(t, map[string]string{
		"calls.go": `package main

type Worker struct { Value int }
func (w Worker) Compute(add int) int { return w.Value + add }
func Identity[T any](value T) T { return value }
func Plain(value int) int { return value }

var PackageCall = Plain(1)
var GenericCall = Identity[int](2)
var MethodCall = Worker{Value: 3}.Compute(4)
var MethodExpressionCall = Worker.Compute(Worker{Value: 5}, 6)
`,
	})
	tracer := NewASTTracer(pkg)

	for _, test := range []struct {
		name, function string
		receiver       bool
		argument       int64
	}{
		{"PackageCall", "Plain", false, 1},
		{"GenericCall", "Identity", false, 2},
		{"MethodCall", "Compute", true, 4},
		{"MethodExpressionCall", "Compute", true, 6},
	} {
		t.Run(test.name, func(t *testing.T) {
			value := mustEvalBinding(t, tracer, test.name).Value
			if value.Kind != StaticFunctionCall || value.Call == nil || value.Call.Object.Name() != test.function {
				t.Fatalf("value = %#v", value)
			}
			if (value.Call.Receiver != nil) != test.receiver {
				t.Fatalf("receiver = %#v", value.Call.Receiver)
			}
			if len(value.Call.Args) != 1 || staticInt64(t, value.Call.Args[0]) != test.argument {
				t.Fatalf("arguments = %#v", value.Call.Args)
			}
			if value.Call.Signature == nil || value.Call.Pos.Filename != "calls.go" {
				t.Fatalf("signature/position = %v %v", value.Call.Signature, value.Call.Pos)
			}
			if test.name == "GenericCall" && !types.Identical(value.Call.Signature.Params().At(0).Type(), types.Typ[types.Int]) {
				t.Fatalf("generic signature = %v", value.Call.Signature)
			}
		})
	}
}

func TestStaticFunctionCallsRejectDynamicTargets(t *testing.T) {
	pkg := mustStaticPackage(t, map[string]string{
		"dynamic.go": `package main

type Computer interface { Compute() int }
type Worker struct{}
func (Worker) Compute() int { return 1 }
func Plain() int { return 1 }

var Function = Plain
var FunctionCall = Function()
var Interface Computer = Worker{}
var InterfaceCall = Interface.Compute()
`,
	})
	tracer := NewASTTracer(pkg)
	for _, name := range []string{"FunctionCall", "InterfaceCall"} {
		if _, err := tracer.EvalPackageValue(name); !errors.Is(err, ErrNotStatic) {
			t.Fatalf("%s error = %v, want ErrNotStatic", name, err)
		}
	}
}
