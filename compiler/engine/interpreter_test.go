package engine

import (
	"math"
	"testing"

	"github.com/WindowsSov8forUs/sonolus-core-go/core/resource"
)

// interpretFunc evaluates one RuntimeFunction call with the given float arguments.
// This is a minimal interpreter covering the most common operations; it is used
// to validate that compiled callback output produces correct numeric results.
func interpretFunc(fn resource.RuntimeFunction, args []float64, memory map[int]float64) float64 {
	switch fn {
	case resource.RuntimeFunctionAdd:
		if len(args) == 0 {
			return 0
		}
		r := args[0]
		for _, a := range args[1:] {
			r += a
		}
		return r
	case resource.RuntimeFunctionSubtract:
		if len(args) == 1 {
			return -args[0]
		}
		return args[0] - args[1]
	case resource.RuntimeFunctionMultiply:
		r := 1.0
		for _, a := range args {
			r *= a
		}
		return r
	case resource.RuntimeFunctionDivide:
		if len(args) < 2 || args[1] == 0 {
			return 0
		}
		return args[0] / args[1]
	case resource.RuntimeFunctionNegate:
		if len(args) == 0 {
			return 0
		}
		return -args[0]
	case resource.RuntimeFunctionAbs:
		if len(args) == 0 {
			return 0
		}
		return math.Abs(args[0])
	case resource.RuntimeFunctionEqual:
		if len(args) < 2 {
			return 0
		}
		if args[0] == args[1] {
			return 1
		}
		return 0
	case resource.RuntimeFunctionNotEqual:
		if len(args) < 2 {
			return 1
		}
		if args[0] != args[1] {
			return 1
		}
		return 0
	case resource.RuntimeFunctionLess:
		if len(args) < 2 {
			return 0
		}
		if args[0] < args[1] {
			return 1
		}
		return 0
	case resource.RuntimeFunctionGreater:
		if len(args) < 2 {
			return 0
		}
		if args[0] > args[1] {
			return 1
		}
		return 0
	case resource.RuntimeFunctionIf:
		if len(args) < 3 {
			return 0
		}
		if args[0] != 0 {
			return args[1]
		}
		return args[2]
	case resource.RuntimeFunctionSin:
		if len(args) == 0 {
			return 0
		}
		return math.Sin(args[0])
	case resource.RuntimeFunctionCos:
		if len(args) == 0 {
			return 0
		}
		return math.Cos(args[0])
	case resource.RuntimeFunctionGet:
		if len(args) < 2 {
			return 0
		}
		return memory[int(args[0])*10000+int(args[1])]
	case resource.RuntimeFunctionSet:
		if len(args) < 3 {
			return 0
		}
		memory[int(args[0])*10000+int(args[1])] = args[2]
		return args[2]
	default:
		return 0 // best-effort
	}
}

// TestInterpretArithmetic verifies the minimal interpreter on core arithmetic ops.
func TestInterpretArithmetic(t *testing.T) {
	mem := map[int]float64{}
	tests := []struct {
		name string
		fn   resource.RuntimeFunction
		args []float64
		want float64
	}{
		{"add", resource.RuntimeFunctionAdd, []float64{2, 3, 4}, 9},
		{"sub", resource.RuntimeFunctionSubtract, []float64{10, 3}, 7},
		{"negate", resource.RuntimeFunctionNegate, []float64{5}, -5},
		{"mul", resource.RuntimeFunctionMultiply, []float64{3, 4, 2}, 24},
		{"div", resource.RuntimeFunctionDivide, []float64{6, 2}, 3},
		{"eq true", resource.RuntimeFunctionEqual, []float64{5, 5}, 1},
		{"eq false", resource.RuntimeFunctionEqual, []float64{5, 3}, 0},
		{"lt", resource.RuntimeFunctionLess, []float64{2, 5}, 1},
		{"gt", resource.RuntimeFunctionGreater, []float64{7, 5}, 1},
		{"if true", resource.RuntimeFunctionIf, []float64{1, 42, 99}, 42},
		{"if false", resource.RuntimeFunctionIf, []float64{0, 42, 99}, 99},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := interpretFunc(tt.fn, tt.args, mem)
			if got != tt.want {
				t.Errorf("interpretFunc(%s, %v) = %v, want %v", tt.fn, tt.args, got, tt.want)
			}
		})
	}
}

// TestInterpretMemory verifies Get/Set operations through the memory map.
func TestInterpretMemory(t *testing.T) {
	mem := map[int]float64{}
	interpretFunc(resource.RuntimeFunctionSet, []float64{0, 5, 42}, mem)
	got := interpretFunc(resource.RuntimeFunctionGet, []float64{0, 5}, mem)
	if got != 42 {
		t.Errorf("Get after Set = %v, want 42", got)
	}
}
