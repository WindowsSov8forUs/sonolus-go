package snode

import "github.com/WindowsSov8forUs/sonolus-core-go/core/resource"

// This file is a faithful port of sonolus.js-compiler's src/snode/optimize.
// Each optimizer mirrors the corresponding TypeScript file one-to-one so that,
// given the same SNode tree, the optimized result is byte-identical.

// Canonical short aliases for runtime functions. These match the naming in
// ir/opsconst.go (ir.OpAdd, ir.OpGet, ...) but are defined locally because ir
// imports snode, so snode cannot import ir without a cycle.
const (
	OpAdd                      = resource.RuntimeFunctionAdd
	OpSubtract                 = resource.RuntimeFunctionSubtract
	OpMultiply                 = resource.RuntimeFunctionMultiply
	OpDivide                   = resource.RuntimeFunctionDivide
	OpMod                      = resource.RuntimeFunctionMod
	OpRem                      = resource.RuntimeFunctionRem
	OpPower                    = resource.RuntimeFunctionPower
	OpIf                       = resource.RuntimeFunctionIf
	OpAnd                      = resource.RuntimeFunctionAnd
	OpGet                      = resource.RuntimeFunctionGet
	OpGetShifted               = resource.RuntimeFunctionGetShifted
	OpSet                      = resource.RuntimeFunctionSet
	OpSetShifted               = resource.RuntimeFunctionSetShifted
	OpExecute                  = resource.RuntimeFunctionExecute
	OpWhile                    = resource.RuntimeFunctionWhile
	OpSwitch                   = resource.RuntimeFunctionSwitch
	OpSwitchInteger            = resource.RuntimeFunctionSwitchInteger
	OpSwitchIntegerWithDefault = resource.RuntimeFunctionSwitchIntegerWithDefault
	OpSwitchWithDefault        = resource.RuntimeFunctionSwitchWithDefault
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

	f, ok := snode.(Func)
	if !ok {
		return snode // non-Func nodes are returned as-is
	}
	args := make([]SNode, len(f.Args))
	for i, a := range f.Args {
		args[i] = Peephole(a)
	}
	n := Func{Op: f.Op, Args: args}

	switch n.Op {
	case OpAdd:
		return optimizeArith(n, cfgAdd)
	case OpSubtract:
		return optimizeArith(n, cfgSubtract)
	case OpMultiply:
		return optimizeArith(n, cfgMultiply)
	case OpDivide:
		return optimizeArith(n, cfgDivide)
	case OpMod:
		return optimizeFlatten(n, OpMod)
	case OpRem:
		return optimizeFlatten(n, OpRem)
	case OpPower:
		return optimizeFlatten(n, OpPower)
	case OpGet:
		return optimizeGet(n)
	case OpGetShifted:
		return optimizeGetShifted(n)
	case OpIf:
		return optimizeIf(n)
	case OpSet:
		return optimizeSet(n)
	case OpSetShifted:
		return optimizeSetShifted(n)
	case OpSwitchWithDefault:
		return optimizeSwitchWithDefault(n)
	case OpWhile:
		return optimizeWhile(n)
	default:
		return n
	}
}
