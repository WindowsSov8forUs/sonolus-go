package snode

import (
	"math"

	"github.com/WindowsSov8forUs/sonolus-core-go/core/resource"
)

// This file is a faithful port of sonolus.js-compiler's src/snode/optimize.
// Each optimizer mirrors the corresponding TypeScript file one-to-one so that,
// given the same SNode tree, the optimized result is byte-identical.

const (
	rfAdd                      = resource.RuntimeFunctionAdd
	rfSubtract                 = resource.RuntimeFunctionSubtract
	rfMultiply                 = resource.RuntimeFunctionMultiply
	rfDivide                   = resource.RuntimeFunctionDivide
	rfMod                      = resource.RuntimeFunctionMod
	rfRem                      = resource.RuntimeFunctionRem
	rfPower                    = resource.RuntimeFunctionPower
	rfIf                       = resource.RuntimeFunctionIf
	rfAnd                      = resource.RuntimeFunctionAnd
	rfGet                      = resource.RuntimeFunctionGet
	rfGetShifted               = resource.RuntimeFunctionGetShifted
	rfSet                      = resource.RuntimeFunctionSet
	rfSetShifted               = resource.RuntimeFunctionSetShifted
	rfExecute                  = resource.RuntimeFunctionExecute
	rfWhile                    = resource.RuntimeFunctionWhile
	rfSwitch                   = resource.RuntimeFunctionSwitch
	rfSwitchInteger            = resource.RuntimeFunctionSwitchInteger
	rfSwitchIntegerWithDefault = resource.RuntimeFunctionSwitchIntegerWithDefault
	rfSwitchWithDefault        = resource.RuntimeFunctionSwitchWithDefault
)

// maxSafeInteger is JS Number.MAX_SAFE_INTEGER (2^53 - 1).
const maxSafeInteger = 9007199254740991

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

// Peephole applies bottom-up peephole optimization to an SNode tree, mirroring
// optimizeSNode: children are optimized first, then the parent's optimizer (if
// any) runs on the rebuilt node.
func Peephole(snode SNode) SNode {
	if _, ok := snode.(Value); ok {
		return snode
	}

	f := snode.(Func)
	args := make([]SNode, len(f.Args))
	for i, a := range f.Args {
		args[i] = Peephole(a)
	}
	n := Func{Op: f.Op, Args: args}

	switch n.Op {
	case rfAdd:
		return optimizeArith(n, cfgAdd)
	case rfSubtract:
		return optimizeArith(n, cfgSubtract)
	case rfMultiply:
		return optimizeArith(n, cfgMultiply)
	case rfDivide:
		return optimizeArith(n, cfgDivide)
	case rfMod:
		return optimizeFlatten(n, rfMod)
	case rfRem:
		return optimizeFlatten(n, rfRem)
	case rfPower:
		return optimizeFlatten(n, rfPower)
	case rfGet:
		return optimizeGet(n)
	case rfGetShifted:
		return optimizeGetShifted(n)
	case rfIf:
		return optimizeIf(n)
	case rfSet:
		return optimizeSet(n)
	case rfSetShifted:
		return optimizeSetShifted(n)
	case rfSwitchWithDefault:
		return optimizeSwitchWithDefault(n)
	case rfWhile:
		return optimizeWhile(n)
	default:
		return n
	}
}

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

// --- If (src/snode/optimize/If.ts) ---

func optimizeIf(s Func) SNode {
	if len(s.Args) > 2 && isValueEq(s.Args[2], 0) {
		return Peephole(Func{Op: rfAnd, Args: []SNode{s.Args[0], s.Args[1]}})
	}
	return s
}

// --- Get (src/snode/optimize/Get.ts) ---

func optimizeGet(s Func) SNode {
	if len(s.Args) < 2 {
		return s
	}
	id, index := s.Args[0], s.Args[1]

	if add, ok := asFunc(index, rfAdd); ok && len(add.Args) == 2 {
		if mul, ok := asFunc(add.Args[1], rfMultiply); ok && len(mul.Args) == 2 {
			return Peephole(Func{Op: rfGetShifted, Args: []SNode{id, add.Args[0], mul.Args[0], mul.Args[1]}})
		}
	}
	return s
}

// --- GetShifted (src/snode/optimize/GetShifted.ts) ---

func optimizeGetShifted(s Func) SNode {
	if len(s.Args) < 4 {
		return s
	}
	id, x, y, sh := s.Args[0], s.Args[1], s.Args[2], s.Args[3]

	if yv, ok := asValue(y); ok {
		if sv, ok := asValue(sh); ok {
			if xv, ok := asValue(x); ok {
				return Peephole(Func{Op: rfGet, Args: []SNode{id, Value(xv + yv*sv)}})
			}
			if yv == 0 && sv == 0 {
				return Peephole(Func{Op: rfGet, Args: []SNode{id, x}})
			}
		}
	}
	return s
}

// --- Set (src/snode/optimize/Set.ts) ---

func optimizeSet(s Func) SNode {
	if len(s.Args) < 3 {
		return s
	}
	id, index, value := s.Args[0], s.Args[1], s.Args[2]

	if add, ok := asFunc(index, rfAdd); ok && len(add.Args) == 2 {
		if mul, ok := asFunc(add.Args[1], rfMultiply); ok && len(mul.Args) == 2 {
			return Peephole(Func{Op: rfSetShifted, Args: []SNode{id, add.Args[0], mul.Args[0], mul.Args[1], value}})
		}
	}

	if vf, ok := asFuncs(value, rfAdd, rfSubtract, rfMultiply, rfDivide, rfRem, rfMod, rfPower); ok && len(vf.Args) == 2 {
		if g, ok := asFunc(vf.Args[0], rfGet); ok && len(g.Args) >= 2 &&
			isEquivalent(g.Args[0], id) && isEquivalent(g.Args[1], index) {
			return Func{Op: setFunc(vf.Op), Args: []SNode{id, index, vf.Args[1]}}
		}
	}

	if vf, ok := asFuncs(value, rfAdd, rfMultiply); ok && len(vf.Args) == 2 {
		if g, ok := asFunc(vf.Args[1], rfGet); ok && len(g.Args) >= 2 &&
			isEquivalent(g.Args[0], id) && isEquivalent(g.Args[1], index) {
			return Func{Op: setFunc(vf.Op), Args: []SNode{id, index, vf.Args[0]}}
		}
	}

	return s
}

// --- SetShifted (src/snode/optimize/SetShifted.ts) ---

func optimizeSetShifted(s Func) SNode {
	if len(s.Args) < 5 {
		return s
	}
	id, x, y, sh, value := s.Args[0], s.Args[1], s.Args[2], s.Args[3], s.Args[4]

	if yv, ok := asValue(y); ok {
		if sv, ok := asValue(sh); ok {
			if xv, ok := asValue(x); ok {
				return Peephole(Func{Op: rfSet, Args: []SNode{id, Value(xv + yv*sv), value}})
			}
			if yv == 0 && sv == 0 {
				return Peephole(Func{Op: rfSet, Args: []SNode{id, x, value}})
			}
		}
	}

	if vf, ok := asFuncs(value, rfAdd, rfSubtract, rfMultiply, rfDivide, rfRem, rfMod, rfPower); ok && len(vf.Args) == 2 {
		if g, ok := asFunc(vf.Args[0], rfGetShifted); ok && len(g.Args) >= 4 &&
			isEquivalent(g.Args[0], id) && isEquivalent(g.Args[1], x) &&
			isEquivalent(g.Args[2], y) && isEquivalent(g.Args[3], sh) {
			return Func{Op: setShiftedFunc(vf.Op), Args: []SNode{id, x, y, sh, vf.Args[1]}}
		}
		if g, ok := asFunc(vf.Args[1], rfGetShifted); ok && len(g.Args) >= 4 &&
			isEquivalent(g.Args[0], id) && isEquivalent(g.Args[1], x) &&
			isEquivalent(g.Args[2], y) && isEquivalent(g.Args[3], sh) {
			return Func{Op: setShiftedFunc(vf.Op), Args: []SNode{id, x, y, sh, vf.Args[0]}}
		}
	}

	return s
}

// setFuncMap maps arithmetic runtime functions to their compound-assignment
// counterparts (e.g. Add → SetAdd). The table is exhaustive for the arithmetic
// ops that appear in the Set/SetShifted optimizer paths; string concatenation
// (matching the JS reference) is kept as a fallback for future op additions.
var setFuncMap = map[resource.RuntimeFunction]resource.RuntimeFunction{
	resource.RuntimeFunctionAdd:      resource.RuntimeFunctionSetAdd,
	resource.RuntimeFunctionSubtract: resource.RuntimeFunctionSetSubtract,
	resource.RuntimeFunctionMultiply: resource.RuntimeFunctionSetMultiply,
	resource.RuntimeFunctionDivide:   resource.RuntimeFunctionSetDivide,
	resource.RuntimeFunctionPower:    resource.RuntimeFunctionSetPower,
	resource.RuntimeFunctionMod:      resource.RuntimeFunctionSetMod,
	resource.RuntimeFunctionRem:      resource.RuntimeFunctionSetRem,
}

// setShiftedFuncMap mirrors setFuncMap for the Shifted variants.
var setShiftedFuncMap = map[resource.RuntimeFunction]resource.RuntimeFunction{
	resource.RuntimeFunctionAdd:      resource.RuntimeFunctionSetAddShifted,
	resource.RuntimeFunctionSubtract: resource.RuntimeFunctionSetSubtractShifted,
	resource.RuntimeFunctionMultiply: resource.RuntimeFunctionSetMultiplyShifted,
	resource.RuntimeFunctionDivide:   resource.RuntimeFunctionSetDivideShifted,
	resource.RuntimeFunctionPower:    resource.RuntimeFunctionSetPowerShifted,
	resource.RuntimeFunctionMod:      resource.RuntimeFunctionSetModShifted,
	resource.RuntimeFunctionRem:      resource.RuntimeFunctionSetRemShifted,
}

// setFunc maps an arithmetic function to its compound-assignment counterpart,
// e.g. Add -> SetAdd. Uses an explicit mapping table with a string-concat
// fallback for forward compatibility.
func setFunc(fn resource.RuntimeFunction) resource.RuntimeFunction {
	if m, ok := setFuncMap[fn]; ok {
		return m
	}
	return resource.RuntimeFunction("Set" + string(fn))
}

// setShiftedFunc mirrors setFunc for the Shifted variants.
func setShiftedFunc(fn resource.RuntimeFunction) resource.RuntimeFunction {
	if m, ok := setShiftedFuncMap[fn]; ok {
		return m
	}
	return resource.RuntimeFunction("Set" + string(fn) + "Shifted")
}

// --- SwitchWithDefault (src/snode/optimize/SwitchWithDefault.ts) ---

func optimizeSwitchWithDefault(s Func) SNode {
	if len(s.Args) < 2 {
		return s
	}
	discriminant := s.Args[0]
	cases := s.Args[1 : len(s.Args)-1]
	defaultCase := s.Args[len(s.Args)-1]

	removeDefault := isValueEq(defaultCase, 0)

	if a, d, ok := tryNormalize(cases); ok {
		normalizedDiscriminant := Peephole(Func{Op: rfDivide, Args: []SNode{
			Func{Op: rfSubtract, Args: []SNode{discriminant, Value(a)}},
			Value(d),
		}})

		var consequences []SNode
		for i := 1; i < len(cases); i += 2 {
			consequences = append(consequences, cases[i])
		}

		if removeDefault {
			return Func{Op: rfSwitchInteger, Args: append([]SNode{normalizedDiscriminant}, consequences...)}
		}
		return Func{Op: rfSwitchIntegerWithDefault, Args: append(append([]SNode{normalizedDiscriminant}, consequences...), defaultCase)}
	}

	if removeDefault {
		return Func{Op: rfSwitch, Args: append([]SNode{discriminant}, cases...)}
	}
	return s
}

// tryNormalize checks whether the case test values form an arithmetic sequence
// a, a+d, a+2d, ... of safe integers and returns (a, d, true) if so.
func tryNormalize(cases []SNode) (a, d float64, ok bool) {
	var tests []float64
	for i := 0; i < len(cases); i += 2 {
		v, isVal := asValue(cases[i])
		if !isVal {
			return 0, 0, false
		}
		tests = append(tests, v)
	}

	if len(tests) < 1 {
		return 0, 0, false
	}
	a = tests[0]
	if !isSafeInteger(a) {
		return 0, 0, false
	}
	if len(tests) < 2 {
		// d would be NaN in JS (tests[1] undefined), failing isSafeInteger.
		return 0, 0, false
	}
	d = tests[1] - a
	if !isSafeInteger(d) {
		return 0, 0, false
	}
	for i, value := range tests {
		if value != a+d*float64(i) {
			return 0, 0, false
		}
	}
	return a, d, true
}

// --- While (src/snode/optimize/While.ts) ---

func optimizeWhile(s Func) SNode {
	if len(s.Args) < 2 {
		return s
	}
	body, ok := asFunc(s.Args[1], rfExecute)
	if !ok || len(body.Args) == 0 {
		return s
	}
	if _, ok := asValue(body.Args[len(body.Args)-1]); !ok {
		return s
	}

	if len(body.Args) == 2 {
		return Func{Op: rfWhile, Args: []SNode{s.Args[0], body.Args[0]}}
	}
	return Func{Op: rfWhile, Args: []SNode{
		s.Args[0],
		Func{Op: rfExecute, Args: append([]SNode{}, body.Args[:len(body.Args)-1]...)},
	}}
}
