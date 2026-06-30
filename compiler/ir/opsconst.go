package ir

import "github.com/WindowsSov8forUs/sonolus-core-go/core/resource"

// Canonical short aliases for runtime functions used throughout the compiler.
// These replace bare resource.RuntimeFunction* literals and the previously
// duplicated op* / rf* aliases in ir/optimize, ir/finalize, and snode.
//
// ir/finalize.go and ir/optimize use these directly. snode keeps its own
// independent aliases (with the same names) because ir imports snode, so
// snode cannot import ir without a cycle.

const (
	// Arithmetic
	OpAdd      = resource.RuntimeFunctionAdd
	OpSubtract = resource.RuntimeFunctionSubtract
	OpMultiply = resource.RuntimeFunctionMultiply
	OpDivide   = resource.RuntimeFunctionDivide
	OpPower    = resource.RuntimeFunctionPower
	OpMod      = resource.RuntimeFunctionMod
	OpRem      = resource.RuntimeFunctionRem
	OpNegate   = resource.RuntimeFunctionNegate
	OpAbs      = resource.RuntimeFunctionAbs
	OpSign     = resource.RuntimeFunctionSign

	// Comparison
	OpEqual     = resource.RuntimeFunctionEqual
	OpNotEqual  = resource.RuntimeFunctionNotEqual
	OpLess      = resource.RuntimeFunctionLess
	OpLessOr    = resource.RuntimeFunctionLessOr
	OpGreater   = resource.RuntimeFunctionGreater
	OpGreaterOr = resource.RuntimeFunctionGreaterOr

	// Logic
	OpAnd = resource.RuntimeFunctionAnd
	OpOr  = resource.RuntimeFunctionOr
	OpNot = resource.RuntimeFunctionNot

	// Min / Max / Clamp
	OpMax   = resource.RuntimeFunctionMax
	OpMin   = resource.RuntimeFunctionMin
	OpClamp = resource.RuntimeFunctionClamp

	// Math
	OpLog   = resource.RuntimeFunctionLog
	OpCeil  = resource.RuntimeFunctionCeil
	OpFloor = resource.RuntimeFunctionFloor
	OpRound = resource.RuntimeFunctionRound
	OpFrac  = resource.RuntimeFunctionFrac
	OpSin   = resource.RuntimeFunctionSin
	OpCos   = resource.RuntimeFunctionCos
	OpTan   = resource.RuntimeFunctionTan
	OpSinh  = resource.RuntimeFunctionSinh
	OpCosh  = resource.RuntimeFunctionCosh
	OpTanh  = resource.RuntimeFunctionTanh
	OpAsin  = resource.RuntimeFunctionArcsin
	OpAcos  = resource.RuntimeFunctionArccos
	OpAtan  = resource.RuntimeFunctionArctan
	OpAtan2 = resource.RuntimeFunctionArctan2
	OpRad   = resource.RuntimeFunctionRadian
	OpDeg   = resource.RuntimeFunctionDegree

	// Interpolation / remapping
	OpLerp         = resource.RuntimeFunctionLerp
	OpLerpClamped  = resource.RuntimeFunctionLerpClamped
	OpRemap        = resource.RuntimeFunctionRemap
	OpRemapClamped = resource.RuntimeFunctionRemapClamped

	// Control flow
	OpIf                       = resource.RuntimeFunctionIf
	OpWhile                    = resource.RuntimeFunctionWhile
	OpSwitch                   = resource.RuntimeFunctionSwitch
	OpSwitchInteger            = resource.RuntimeFunctionSwitchInteger
	OpSwitchWithDefault        = resource.RuntimeFunctionSwitchWithDefault
	OpSwitchIntegerWithDefault = resource.RuntimeFunctionSwitchIntegerWithDefault

	// Memory
	OpGet        = resource.RuntimeFunctionGet
	OpGetShifted = resource.RuntimeFunctionGetShifted
	OpSet        = resource.RuntimeFunctionSet
	OpSetShifted = resource.RuntimeFunctionSetShifted

	// Block / execution
	OpExecute  = resource.RuntimeFunctionExecute
	OpBlock    = resource.RuntimeFunctionBlock
	OpJumpLoop = resource.RuntimeFunctionJumpLoop
)
