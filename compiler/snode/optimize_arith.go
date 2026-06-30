package snode

import (
	"math"

	"github.com/WindowsSov8forUs/sonolus-core-go/core/resource"
)

// --- utils (src/snode/optimize/utils.ts) ---

func asValue(n SNode) (float64, bool) {
	v, ok := n.(Value)
	return float64(v), ok
}

// asFunc reports whether n is a Func with the given name and returns it.
func asFunc(n SNode, fn resource.RuntimeFunction) (Func, bool) {
	f, ok := n.(Func)
	if !ok || f.Op != fn {
		return Func{}, false
	}
	return f, true
}

// asFuncs reports whether n is a Func whose name is one of fns.
func asFuncs(n SNode, fns ...resource.RuntimeFunction) (Func, bool) {
	f, ok := n.(Func)
	if !ok {
		return Func{}, false
	}
	for _, x := range fns {
		if f.Op == x {
			return f, true
		}
	}
	return Func{}, false
}

// isValueEq reports whether n is the value v. Mirrors JS strict `n === v` where
// a Func object is never strictly equal to a number.
func isValueEq(n SNode, v float64) bool {
	x, ok := n.(Value)
	return ok && float64(x) == v
}

// isEquivalent mirrors utils.ts isEquivalent: only Get / GetShifted function
// nodes are ever considered structurally equivalent (plus equal values).
func isEquivalent(a, b SNode) bool {
	if av, ok := asValue(a); ok {
		bv, ok := asValue(b)
		return ok && av == bv
	}
	if _, ok := b.(Value); ok {
		return false
	}

	af := a.(Func)
	bf := b.(Func)
	if af.Op != bf.Op {
		return false
	}
	if len(af.Args) != len(bf.Args) {
		return false
	}
	if af.Op != rfGet && af.Op != rfGetShifted {
		return false
	}
	for i := range af.Args {
		if !isEquivalent(af.Args[i], bf.Args[i]) {
			return false
		}
	}
	return true
}

func isSafeInteger(f float64) bool {
	return !math.IsNaN(f) && !math.IsInf(f, 0) && f == math.Trunc(f) && math.Abs(f) <= maxSafeInteger
}

// --- Arithmetic optimizers (src/snode/optimize/{Add,Subtract,Multiply,Divide}.ts) ---
//
// The four arithmetic optimizers share a common structure:
//  1. Flatten nested same-op calls (all args for commutative, head-only for
//     non-commutative).
//  2. Fold constant arguments.
//  3. Simplify: eliminate identity elements, collapse single-dynamic, or
//     handle zero-annihilation (Multiply special case).
//
// They are parameterized via arithConfig and driven by optimizeArith below.
// Mod/Rem/Power use the simpler optimizeFlatten helper.

// Arithmetic optimizer configurations — parameterize the flatten → fold → simplify
// pattern shared by Add / Subtract / Multiply / Divide.
var (
	cfgAdd = arithConfig{
		op: rfAdd, identity: 0,
		combine:     func(a, b float64) float64 { return a + b },
		commutative: true,
	}
	cfgSubtract = arithConfig{
		op: rfSubtract, identity: 0,
		combine:     func(a, b float64) float64 { return a + b },
		commutative: false,
	}
	cfgMultiply = arithConfig{
		op: rfMultiply, identity: 1,
		combine:          func(a, b float64) float64 { return a * b },
		commutative:      true,
		zeroAnnihilates:  true,
	}
	cfgDivide = arithConfig{
		op: rfDivide, identity: 1,
		combine:     func(a, b float64) float64 { return a * b },
		commutative: false,
	}
)

type arithConfig struct {
	op              resource.RuntimeFunction
	identity        float64
	combine         func(a, b float64) float64 // how constants are combined
	commutative     bool                       // true: flatten all args; false: flatten head only
	zeroAnnihilates bool                       // Multiply: 0 * x = 0
}

func optimizeArith(s Func, cfg arithConfig) SNode {
	if len(s.Args) == 0 {
		return Value(cfg.identity)
	}

	if cfg.commutative {
		return optimizeCommutative(s.Args, cfg)
	}
	return optimizeNonCommutative(s.Args, cfg)
}

func optimizeCommutative(args []SNode, cfg arithConfig) SNode {
	// Flatten: Add(Add(a,b), c) → Add(a,b,c)
	var flat []SNode
	for _, arg := range args {
		if f, ok := asFunc(arg, cfg.op); ok {
			flat = append(flat, f.Args...)
		} else {
			flat = append(flat, arg)
		}
	}

	constants := cfg.identity
	var dynamics []SNode
	for _, arg := range flat {
		if v, ok := asValue(arg); ok {
			constants = cfg.combine(constants, v)
		} else {
			dynamics = append(dynamics, arg)
		}
	}

	if len(dynamics) == 0 {
		return Value(constants)
	}
	if cfg.zeroAnnihilates && constants == 0 {
		// Preserve side effects: evaluate dynamics, then yield 0.
		return Func{Op: rfExecute, Args: append(append([]SNode{}, dynamics...), Value(0))}
	}
	if constants == cfg.identity {
		if len(dynamics) == 1 {
			return dynamics[0]
		}
		return Func{Op: cfg.op, Args: dynamics}
	}
	return Func{Op: cfg.op, Args: append([]SNode{Value(constants)}, dynamics...)}
}

func optimizeNonCommutative(args []SNode, cfg arithConfig) SNode {
	// Flatten head: Subtract(Subtract(a,b), c) → Subtract(a,b,c)
	var head SNode
	var rest []SNode
	if f, ok := asFunc(args[0], cfg.op); ok {
		merged := append(append([]SNode{}, f.Args...), args[1:]...)
		head, rest = merged[0], merged[1:]
	} else {
		head, rest = args[0], args[1:]
	}

	constants := cfg.identity
	var dynamics []SNode
	for _, arg := range rest {
		if v, ok := asValue(arg); ok {
			constants = cfg.combine(constants, v)
		} else {
			dynamics = append(dynamics, arg)
		}
	}

	if constants == cfg.identity {
		if len(dynamics) == 0 {
			return head
		}
		return Func{Op: cfg.op, Args: append([]SNode{head}, dynamics...)}
	}
	return Func{Op: cfg.op, Args: append([]SNode{head, Value(constants)}, dynamics...)}
}

// --- Mod / Rem / Power (identical shape) ---

func optimizeFlatten(s Func, fn resource.RuntimeFunction) SNode {
	if len(s.Args) == 0 {
		return Value(0)
	}
	if len(s.Args) == 1 {
		return s.Args[0]
	}
	if f, ok := asFunc(s.Args[0], fn); ok {
		return Func{Op: fn, Args: append(append([]SNode{}, f.Args...), s.Args[1:]...)}
	}
	return s
}
