// Package frontend is the first slice of the L3 front end: it interprets a
// subset of Go source (via go/ast) over a traced value system, emitting IR into
// a CFG. This mirrors sonolus.py's script/internal tracing (visitor.py + the
// _Num value), adapted to Go: we interpret the AST rather than execute it.
//
// Supported subset (this slice): numeric literals and bools, local bindings
// (:= / =), arithmetic/comparison/logical/unary operators, if/else, and the
// get()/set() memory builtins. Local variables are SSA-style immutable values
// bound in lexical scope; assignments inside an if-branch do NOT merge back out
// (that needs memory-backed locals or SSA, a later slice).
package frontend

import (
	"go/token"
	"math"

	"github.com/WindowsSov8forUs/sonolus-core-go/core/resource"

	"github.com/WindowsSov8forUs/sonolus-go/compiler/ir"
)

// Num is a traced scalar: either a compile-time constant or an IR expression.
// Mirrors sonolus.py _Num (which is constant-backed or place/expr-backed).
type Num struct {
	isConst bool
	c       float64
	e       ir.Node
}

func constNum(v float64) Num { return Num{isConst: true, c: v} }
func exprNum(n ir.Node) Num  { return Num{e: n} }

func boolNum(b bool) Num {
	if b {
		return constNum(1)
	}
	return constNum(0)
}

// node returns the IR node for this value (a Const for constants).
func (n Num) node() ir.Node {
	if n.isConst {
		return ir.Const(n.c)
	}
	return n.e
}

// binOps maps Go binary tokens to runtime operations.
var binOps = map[token.Token]resource.RuntimeFunction{
	token.ADD:  resource.RuntimeFunctionAdd,
	token.SUB:  resource.RuntimeFunctionSubtract,
	token.MUL:  resource.RuntimeFunctionMultiply,
	token.QUO:  resource.RuntimeFunctionDivide,
	token.REM:  resource.RuntimeFunctionMod,
	token.EQL:  resource.RuntimeFunctionEqual,
	token.NEQ:  resource.RuntimeFunctionNotEqual,
	token.LSS:  resource.RuntimeFunctionLess,
	token.LEQ:  resource.RuntimeFunctionLessOr,
	token.GTR:  resource.RuntimeFunctionGreater,
	token.GEQ:  resource.RuntimeFunctionGreaterOr,
	token.LAND: resource.RuntimeFunctionAnd,
	token.LOR:  resource.RuntimeFunctionOr,
}

// applyBinary folds two constants when possible, otherwise emits a pure IR op.
func applyBinary(op token.Token, a, b Num) (Num, bool) {
	rtOp, ok := binOps[op]
	if !ok {
		return Num{}, false
	}

	if a.isConst && b.isConst {
		if folded, ok := foldBinary(op, a.c, b.c); ok {
			return folded, true
		}
	}
	return exprNum(ir.PureInstr(rtOp, a.node(), b.node())), true
}

// foldBinary computes a constant result for two constant operands. It declines
// to fold division/modulo by zero (left to runtime semantics).
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
		return constNum(math.Mod(a, b)), true
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

// applyUnary handles -, +, and ! with constant folding.
func applyUnary(op token.Token, a Num) (Num, bool) {
	switch op {
	case token.ADD:
		return a, true
	case token.SUB:
		if a.isConst {
			return constNum(-a.c), true
		}
		return exprNum(ir.PureInstr(resource.RuntimeFunctionNegate, a.node())), true
	case token.NOT:
		if a.isConst {
			return boolNum(a.c == 0), true
		}
		return exprNum(ir.PureInstr(resource.RuntimeFunctionNot, a.node())), true
	default:
		return Num{}, false
	}
}
