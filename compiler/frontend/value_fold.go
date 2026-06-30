package frontend

import (
	"go/token"

	"github.com/WindowsSov8forUs/sonolus-core-go/core/resource"

	"github.com/WindowsSov8forUs/sonolus-go/compiler/ir"
)

// binOps maps Go binary tokens to runtime operations.
var binOps = map[token.Token]resource.RuntimeFunction{
	token.ADD: resource.RuntimeFunctionAdd, token.SUB: resource.RuntimeFunctionSubtract,
	token.MUL: resource.RuntimeFunctionMultiply, token.QUO: resource.RuntimeFunctionDivide,
	token.REM: resource.RuntimeFunctionMod,
	token.EQL: resource.RuntimeFunctionEqual, token.NEQ: resource.RuntimeFunctionNotEqual,
	token.LSS: resource.RuntimeFunctionLess, token.LEQ: resource.RuntimeFunctionLessOr,
	token.GTR: resource.RuntimeFunctionGreater, token.GEQ: resource.RuntimeFunctionGreaterOr,
	token.LAND: resource.RuntimeFunctionAnd, token.LOR: resource.RuntimeFunctionOr,
}

func applyBinary(gen *ir.IDGen, op token.Token, a, b Num) (Num, bool) {
	rtOp, ok := binOps[op]
	if !ok {
		return Num{}, false
	}
	if a.isConst && b.isConst {
		if folded, ok := foldBinary(op, a.c, b.c); ok {
			return folded, true
		}
	}
	if !a.IsScalar() || !b.IsScalar() {
		return Num{}, false // caller will report error
	}
	return exprNum(gen.PureInstr(rtOp, a.mustNode(), b.mustNode())), true
}

func foldBinary(op token.Token, a, b float64) (Num, bool) {
	switch op {
	case token.ADD:
		return constNum(a + b), true
	case token.SUB:
		return constNum(a - b), true
	case token.MUL:
		return constNum(a * b), true
	case token.QUO:
		if b == 0 {
			return Num{}, false
		}
		return constNum(a / b), true
	case token.REM:
		if b == 0 {
			return Num{}, false
		}
		return constNum(ir.FloorMod(a, b)), true
	case token.EQL:
		return boolNum(a == b), true
	case token.NEQ:
		return boolNum(a != b), true
	case token.LSS:
		return boolNum(a < b), true
	case token.LEQ:
		return boolNum(a <= b), true
	case token.GTR:
		return boolNum(a > b), true
	case token.GEQ:
		return boolNum(a >= b), true
	case token.LAND:
		return boolNum(a != 0 && b != 0), true
	case token.LOR:
		return boolNum(a != 0 || b != 0), true
	default:
		return Num{}, false
	}
}

func applyUnary(gen *ir.IDGen, op token.Token, a Num) (Num, bool) {
	if !a.IsScalar() {
		return Num{}, false
	}
	switch op {
	case token.ADD:
		return a, true
	case token.SUB:
		if a.isConst {
			return constNum(-a.c), true
		}
		return exprNum(gen.PureInstr(resource.RuntimeFunctionNegate, a.mustNode())), true
	case token.NOT:
		if a.isConst {
			return boolNum(a.c == 0), true
		}
		return exprNum(gen.PureInstr(resource.RuntimeFunctionNot, a.mustNode())), true
	default:
		return Num{}, false
	}
}
