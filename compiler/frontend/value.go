package frontend

import (
	"go/token"
	"math"

	"github.com/WindowsSov8forUs/sonolus-core-go/core/resource"

	"github.com/WindowsSov8forUs/sonolus-go/compiler/ir"
)

// Num is a traced value: scalar constant, IR expression, or composite record
// whose fields are individually tracked Nums.
type Num struct {
	isConst bool
	c       float64
	e       ir.Node

	// Composite records: each field is tracked as a separate Num so reads can
	// be constant-folded or SSA-folded by the optimizer without a memory read.
	fields map[string]Num
}

func constNum(v float64) Num { return Num{isConst: true, c: v} }
func exprNum(n ir.Node) Num  { return Num{e: n} }

func boolNum(b bool) Num {
	if b {
		return constNum(1)
	}
	return constNum(0)
}

// compNum creates a composite Num with individually tracked fields.
func compNum(fields map[string]Num) Num { return Num{fields: fields} }

// IsComposite reports whether this value is a record with named fields.
func (n Num) IsComposite() bool { return n.fields != nil }

// CompositeSize returns the number of fields in a composite.
func (n Num) CompositeSize() int {
	if n.fields == nil {
		return 0
	}
	return len(n.fields)
}

// CompositeFieldOrder returns field names in declaration order (matching the
// constructor). Panics if not composite.
func CompositeFieldOrder(n *Num) []string {
	if n.fields == nil {
		panic("compositeFieldOrder: not composite")
	}
	// Use deterministic order by name.
	var out []string
	for k := range n.fields {
		out = append(out, k)
	}
	return out
}

// Field returns the Num for a named field of a composite, or panics.
func (n Num) Field(name string) Num {
	v, ok := n.fields[name]
	if !ok {
		panic("Num.Field: unknown field " + name)
	}
	return v
}

// SetField updates a named field in a composite. The receiver must be a
// composite.
func (n *Num) SetField(name string, val Num) {
	if n.fields == nil {
		panic("Num.SetField: not a composite")
	}
	n.fields[name] = val
}

// node returns the IR node for this value: a Const for constants, a Get for
// memory expressions, or the first field's node for composites (used as a
// placeholder — composites should be destructured, not passed whole).
func (n Num) node() ir.Node {
	if n.isConst {
		return ir.Const(n.c)
	}
	if n.fields != nil {
		panic("Num.node: composite value has no single IR node; destructure fields first")
	}
	return n.e
}

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
