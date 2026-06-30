package snode

import "github.com/WindowsSov8forUs/sonolus-core-go/core/resource"

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
