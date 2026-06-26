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

// Vec2 methods: each returns a new composite Num by applying the operation to
// both fields individually. The optimizer folds the resulting field reads.

func vec2Add(t *tracer, v Num, args []Num) (Num, error) {
	w := args[0]
	if w.IsComposite() {
		return compNum(map[string]Num{"x": exprNum(ir.PureInstr(resource.RuntimeFunctionAdd, v.Field("x").node(), w.Field("x").node())), "y": exprNum(ir.PureInstr(resource.RuntimeFunctionAdd, v.Field("y").node(), w.Field("y").node()))}), nil
	}
	return compNum(map[string]Num{"x": exprNum(ir.PureInstr(resource.RuntimeFunctionAdd, v.Field("x").node(), w.node())), "y": exprNum(ir.PureInstr(resource.RuntimeFunctionAdd, v.Field("y").node(), w.node()))}), nil
}

func vec2Sub(t *tracer, v Num, args []Num) (Num, error) {
	w := args[0]
	return compNum(map[string]Num{"x": exprNum(ir.PureInstr(resource.RuntimeFunctionSubtract, v.Field("x").node(), w.node())), "y": exprNum(ir.PureInstr(resource.RuntimeFunctionSubtract, v.Field("y").node(), w.node()))}), nil
}

func vec2Mul(t *tracer, v Num, args []Num) (Num, error) {
	s := args[0]
	return compNum(map[string]Num{"x": exprNum(ir.PureInstr(resource.RuntimeFunctionMultiply, v.Field("x").node(), s.node())), "y": exprNum(ir.PureInstr(resource.RuntimeFunctionMultiply, v.Field("y").node(), s.node()))}), nil
}

func vec2Div(t *tracer, v Num, args []Num) (Num, error) {
	s := args[0]
	return compNum(map[string]Num{"x": exprNum(ir.PureInstr(resource.RuntimeFunctionDivide, v.Field("x").node(), s.node())), "y": exprNum(ir.PureInstr(resource.RuntimeFunctionDivide, v.Field("y").node(), s.node()))}), nil
}

var vec2Methods = map[string]func(*tracer, Num, []Num) (Num, error){
	"add": vec2Add, "sub": vec2Sub, "mul": vec2Mul, "div": vec2Div,
	"magnitude": vec2Magnitude, "dot": vec2Dot, "normalize": vec2Normalize,
}

// matFields is the field layout of a 3x2 affine matrix.
var matFields = []string{"m11", "m12", "m13", "m21", "m22", "m23"}

func matIdentity() Num {
	return compNum(map[string]Num{
		"m11": constNum(1), "m12": constNum(0), "m13": constNum(0),
		"m21": constNum(0), "m22": constNum(1), "m23": constNum(0),
	})
}

func matScale(t *tracer, m Num, args []Num) (Num, error) {
	sx, sy := args[0], args[0]
	if len(args) > 1 {
		sy = args[1]
	}
	return compNum(map[string]Num{
		"m11": exprNum(ir.PureInstr(resource.RuntimeFunctionMultiply, m.Field("m11").node(), sx.node())),
		"m12": exprNum(ir.PureInstr(resource.RuntimeFunctionMultiply, m.Field("m12").node(), sx.node())),
		"m13": exprNum(ir.PureInstr(resource.RuntimeFunctionMultiply, m.Field("m13").node(), sx.node())),
		"m21": exprNum(ir.PureInstr(resource.RuntimeFunctionMultiply, m.Field("m21").node(), sy.node())),
		"m22": exprNum(ir.PureInstr(resource.RuntimeFunctionMultiply, m.Field("m22").node(), sy.node())),
		"m23": exprNum(ir.PureInstr(resource.RuntimeFunctionMultiply, m.Field("m23").node(), sy.node())),
	}), nil
}

func matTranslate(t *tracer, m Num, args []Num) (Num, error) {
	tx, ty := args[0], args[0]
	if len(args) > 1 {
		ty = args[1]
	}
	return compNum(map[string]Num{
		"m11": m.Field("m11"), "m12": m.Field("m12"),
		"m13": exprNum(ir.PureInstr(resource.RuntimeFunctionAdd, m.Field("m13").node(), tx.node())),
		"m21": m.Field("m21"), "m22": m.Field("m22"),
		"m23": exprNum(ir.PureInstr(resource.RuntimeFunctionAdd, m.Field("m23").node(), ty.node())),
	}), nil
}

var matMethods = map[string]func(*tracer, Num, []Num) (Num, error){
	"scale": matScale, "translate": matTranslate,
}

var transFields = []string{"m11", "m12", "m13", "m21", "m22", "m23", "m31", "m32", "m33"}

func transIdentity() Num {
	return compNum(map[string]Num{
		"m11": constNum(1), "m12": constNum(0), "m13": constNum(0),
		"m21": constNum(0), "m22": constNum(1), "m23": constNum(0),
		"m31": constNum(0), "m32": constNum(0), "m33": constNum(1),
	})
}

func vec2Magnitude(t *tracer, v Num, args []Num) (Num, error) {
	x, y := v.Field("x"), v.Field("y")
	return exprNum(ir.PureInstr(resource.RuntimeFunctionPower,
		ir.PureInstr(resource.RuntimeFunctionAdd,
			ir.PureInstr(resource.RuntimeFunctionMultiply, x.node(), x.node()),
			ir.PureInstr(resource.RuntimeFunctionMultiply, y.node(), y.node())),
		ir.Const(0.5))), nil
}

func vec2Dot(t *tracer, v Num, args []Num) (Num, error) {
	w := args[0]
	return exprNum(ir.PureInstr(resource.RuntimeFunctionAdd,
		ir.PureInstr(resource.RuntimeFunctionMultiply, v.Field("x").node(), w.Field("x").node()),
		ir.PureInstr(resource.RuntimeFunctionMultiply, v.Field("y").node(), w.Field("y").node()))), nil
}

func vec2Normalize(t *tracer, v Num, args []Num) (Num, error) {
	x, y := v.Field("x"), v.Field("y")
	mag := ir.PureInstr(resource.RuntimeFunctionPower,
		ir.PureInstr(resource.RuntimeFunctionAdd,
			ir.PureInstr(resource.RuntimeFunctionMultiply, x.node(), x.node()),
			ir.PureInstr(resource.RuntimeFunctionMultiply, y.node(), y.node())),
		ir.Const(0.5))
	return compNum(map[string]Num{
		"x": exprNum(ir.PureInstr(resource.RuntimeFunctionDivide, x.node(), mag)),
		"y": exprNum(ir.PureInstr(resource.RuntimeFunctionDivide, y.node(), mag)),
	}), nil
}

var rectFields = []string{"t", "r", "b", "l"}

func rectW(t *tracer, r Num, args []Num) (Num, error) {
	return exprNum(ir.PureInstr(resource.RuntimeFunctionSubtract, r.Field("r").node(), r.Field("l").node())), nil
}

func rectH(t *tracer, r Num, args []Num) (Num, error) {
	return exprNum(ir.PureInstr(resource.RuntimeFunctionSubtract, r.Field("t").node(), r.Field("b").node())), nil
}

func rectCenter(t *tracer, r Num, args []Num) (Num, error) {
	return compNum(map[string]Num{
		"x": exprNum(ir.PureInstr(resource.RuntimeFunctionDivide,
			ir.PureInstr(resource.RuntimeFunctionAdd, r.Field("l").node(), r.Field("r").node()),
			ir.Const(2))),
		"y": exprNum(ir.PureInstr(resource.RuntimeFunctionDivide,
			ir.PureInstr(resource.RuntimeFunctionAdd, r.Field("b").node(), r.Field("t").node()),
			ir.Const(2))),
	}), nil
}

func rectTranslate(t *tracer, r Num, args []Num) (Num, error) {
	dx, dy := args[0], args[0]
	if len(args) > 1 {
		dy = args[1]
	}
	return compNum(map[string]Num{
		"t": exprNum(ir.PureInstr(resource.RuntimeFunctionAdd, r.Field("t").node(), dy.node())),
		"r": exprNum(ir.PureInstr(resource.RuntimeFunctionAdd, r.Field("r").node(), dx.node())),
		"b": exprNum(ir.PureInstr(resource.RuntimeFunctionAdd, r.Field("b").node(), dy.node())),
		"l": exprNum(ir.PureInstr(resource.RuntimeFunctionAdd, r.Field("l").node(), dx.node())),
	}), nil
}

func rectScale(t *tracer, r Num, args []Num) (Num, error) {
	sx, sy := args[0], args[0]
	if len(args) > 1 {
		sy = args[1]
	}
	return compNum(map[string]Num{
		"t": exprNum(ir.PureInstr(resource.RuntimeFunctionMultiply, r.Field("t").node(), sy.node())),
		"r": exprNum(ir.PureInstr(resource.RuntimeFunctionMultiply, r.Field("r").node(), sx.node())),
		"b": exprNum(ir.PureInstr(resource.RuntimeFunctionMultiply, r.Field("b").node(), sy.node())),
		"l": exprNum(ir.PureInstr(resource.RuntimeFunctionMultiply, r.Field("l").node(), sx.node())),
	}), nil
}

var rectMethods = map[string]func(*tracer, Num, []Num) (Num, error){
	"w": rectW, "h": rectH, "center": rectCenter,
	"translate": rectTranslate, "scale": rectScale,
}

func vec2Zero() Num  { return compNum(map[string]Num{"x": constNum(0), "y": constNum(0)}) }
func vec2One() Num   { return compNum(map[string]Num{"x": constNum(1), "y": constNum(1)}) }
func vec2Up() Num    { return compNum(map[string]Num{"x": constNum(0), "y": constNum(1)}) }
func vec2Down() Num  { return compNum(map[string]Num{"x": constNum(0), "y": constNum(-1)}) }
func vec2Left() Num  { return compNum(map[string]Num{"x": constNum(-1), "y": constNum(0)}) }
func vec2Right() Num { return compNum(map[string]Num{"x": constNum(1), "y": constNum(0)}) }

func vec2Unit(t *tracer, v Num, args []Num) (Num, error) {
	angle := args[0]
	return compNum(map[string]Num{
		"x": exprNum(ir.PureInstr(resource.RuntimeFunctionCos, angle.node())),
		"y": exprNum(ir.PureInstr(resource.RuntimeFunctionSin, angle.node())),
	}), nil
}

var vec2Statics = map[string]func() Num{
	"zero": vec2Zero, "one": vec2One, "up": vec2Up, "down": vec2Down,
	"left": vec2Left, "right": vec2Right,
}
