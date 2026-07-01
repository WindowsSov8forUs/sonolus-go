package optimize

import "github.com/WindowsSov8forUs/sonolus-go/compiler/ir"

// Package-local aliases that mirror the canonical ir.Op* constants so that
// optimize pass implementations can use unqualified names (opAdd, etc.).
// All values are sourced from ir.opsconst.go — the single canonical
// definition point — eliminating the previous duplication across packages.
const (
	// Arithmetic
	opAdd      = ir.OpAdd
	opSubtract = ir.OpSubtract
	opMultiply = ir.OpMultiply
	opDivide   = ir.OpDivide
	opPower    = ir.OpPower
	opMod      = ir.OpMod
	opRem      = ir.OpRem
	opNegate   = ir.OpNegate
	opAbs      = ir.OpAbs
	opSign     = ir.OpSign

	// Comparison
	opEqual     = ir.OpEqual
	opNotEqual  = ir.OpNotEqual
	opLess      = ir.OpLess
	opLessOr    = ir.OpLessOr
	opGreater   = ir.OpGreater
	opGreaterOr = ir.OpGreaterOr

	// Logic
	opAnd = ir.OpAnd
	opOr  = ir.OpOr
	opNot = ir.OpNot

	// Min / Max / Clamp
	opMax   = ir.OpMax
	opMin   = ir.OpMin
	opClamp = ir.OpClamp

	// Math
	opLog   = ir.OpLog
	opCeil  = ir.OpCeil
	opFloor = ir.OpFloor
	opRound = ir.OpRound
	opFrac  = ir.OpFrac
	opSin   = ir.OpSin
	opCos   = ir.OpCos
	opTan   = ir.OpTan
	opSinh  = ir.OpSinh
	opCosh  = ir.OpCosh
	opTanh  = ir.OpTanh
	opAsin  = ir.OpAsin
	opAcos  = ir.OpAcos
	opAtan  = ir.OpAtan
	opAtan2 = ir.OpAtan2
	opRad   = ir.OpRad
	opDeg   = ir.OpDeg

	// Interpolation / remapping
	opLerp         = ir.OpLerp
	opLerpClamped  = ir.OpLerpClamped
	opRemap        = ir.OpRemap
	opRemapClamped = ir.OpRemapClamped
)
