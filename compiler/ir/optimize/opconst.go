package optimize

import "github.com/WindowsSov8forUs/sonolus-core-go/core/resource"

// Typed aliases for runtime functions used in optimizer passes.
// These replace bare string literals throughout the package to leverage
// Go's type system and avoid silent typos.
//
// Patterned after snode/optimize.go's rf* aliases.
var (
	// Arithmetic
	opAdd      = resource.RuntimeFunctionAdd
	opSubtract = resource.RuntimeFunctionSubtract
	opMultiply = resource.RuntimeFunctionMultiply
	opDivide   = resource.RuntimeFunctionDivide
	opPower    = resource.RuntimeFunctionPower
	opMod      = resource.RuntimeFunctionMod
	opRem      = resource.RuntimeFunctionRem
	opNegate   = resource.RuntimeFunctionNegate
	opAbs      = resource.RuntimeFunctionAbs
	opSign     = resource.RuntimeFunctionSign

	// Comparison
	opEqual     = resource.RuntimeFunctionEqual
	opNotEqual  = resource.RuntimeFunctionNotEqual
	opLess      = resource.RuntimeFunctionLess
	opLessOr    = resource.RuntimeFunctionLessOr
	opGreater   = resource.RuntimeFunctionGreater
	opGreaterOr = resource.RuntimeFunctionGreaterOr

	// Logic
	opAnd = resource.RuntimeFunctionAnd
	opOr  = resource.RuntimeFunctionOr
	opNot = resource.RuntimeFunctionNot

	// Min/Max/Clamp
	opMax   = resource.RuntimeFunctionMax
	opMin   = resource.RuntimeFunctionMin
	opClamp = resource.RuntimeFunctionClamp

	// Math
	opLog   = resource.RuntimeFunctionLog
	opCeil  = resource.RuntimeFunctionCeil
	opFloor = resource.RuntimeFunctionFloor
	opRound = resource.RuntimeFunctionRound
	opFrac  = resource.RuntimeFunctionFrac
	opSin   = resource.RuntimeFunctionSin
	opCos   = resource.RuntimeFunctionCos
	opTan   = resource.RuntimeFunctionTan
	opSinh  = resource.RuntimeFunctionSinh
	opCosh  = resource.RuntimeFunctionCosh
	opTanh  = resource.RuntimeFunctionTanh
	opAsin  = resource.RuntimeFunctionArcsin
	opAcos  = resource.RuntimeFunctionArccos
	opAtan  = resource.RuntimeFunctionArctan
	opAtan2 = resource.RuntimeFunctionArctan2
	opRad   = resource.RuntimeFunctionRadian
	opDeg   = resource.RuntimeFunctionDegree

	// Interpolation / remapping
	opLerp         = resource.RuntimeFunctionLerp
	opLerpClamped  = resource.RuntimeFunctionLerpClamped
	opRemap        = resource.RuntimeFunctionRemap
	opRemapClamped = resource.RuntimeFunctionRemapClamped
)
