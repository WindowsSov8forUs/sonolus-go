package frontend

import (
	"fmt"
	"go/token"
	"sort"

	"github.com/WindowsSov8forUs/sonolus-core-go/core/resource"

	"github.com/WindowsSov8forUs/sonolus-go/compiler/ir"
)

// NumKind distinguishes the three semantic categories of a traced value:
// scalar, record (composite with named fields), and array (indexed).
type NumKind byte

const (
	kindScalar NumKind = iota
	kindRecord
	kindArray
)

// Num is a traced value: scalar constant, IR expression, record (composite
// with named fields), or array of elements.
type Num struct {
	kind NumKind

	isConst bool
	c       float64
	e       ir.Node

	// Record: each field is tracked as a separate Num so reads can be
	// constant-folded or SSA-folded by the optimizer without a memory read.
	fields map[string]Num

	// Array: per-element Nums. Fixed length, scalar or record elements.
	arr []Num
}

func constNum(v float64) Num { return Num{kind: kindScalar, isConst: true, c: v} }
func exprNum(n ir.Node) Num  { return Num{kind: kindScalar, e: n} }

func boolNum(b bool) Num {
	if b {
		return constNum(1)
	}
	return constNum(0)
}

// compNum creates a record Num with individually tracked fields.
func compNum(fields map[string]Num) Num { return Num{kind: kindRecord, fields: fields} }

// arrayNum creates an array Num with per-element values.
func arrayNum(elems []Num) Num { return Num{kind: kindArray, arr: elems} }

// IsScalar reports whether this value is a scalar (const or expression).
func (n Num) IsScalar() bool { return n.kind == kindScalar }

// IsComposite reports whether this value is a record with named fields.
func (n Num) IsComposite() bool { return n.kind == kindRecord }

// IsArray reports whether this value is an indexed array.
func (n Num) IsArray() bool { return n.kind == kindArray }

// Len returns the element count for arrays, 0 otherwise.
func (n Num) Len() int {
	if n.kind == kindArray {
		return len(n.arr)
	}
	return 0
}

// Index returns the Num at position i for arrays.
// Returns an error if the receiver is not an array or i is out of bounds.
func (n Num) Index(i int) (Num, error) {
	if n.kind != kindArray {
		return Num{}, fmt.Errorf("Num.Index: not an array")
	}
	if i < 0 || i >= len(n.arr) {
		return Num{}, fmt.Errorf("Num.Index: index %d out of bounds [0,%d)", i, len(n.arr))
	}
	return n.arr[i], nil
}

// CompositeSize returns the number of fields in a record.
func (n Num) CompositeSize() int {
	if n.kind != kindRecord {
		return 0
	}
	return len(n.fields)
}

// CompositeFieldOrder returns field names in deterministic sorted order.
// Returns an error if the receiver is not a record.
func (n Num) CompositeFieldOrder() ([]string, error) {
	if n.kind != kindRecord {
		return nil, fmt.Errorf("Num.CompositeFieldOrder: not a record")
	}
	var out []string
	for k := range n.fields {
		out = append(out, k)
	}
	sort.Strings(out)
	return out, nil
}

// TryField returns the Num for a named field of a record, with ok=true.
// Returns ok=false if the receiver is not a record or the field does not exist.
func (n Num) TryField(name string) (Num, bool) {
	if n.kind != kindRecord {
		return Num{}, false
	}
	v, ok := n.fields[name]
	return v, ok
}

// Field returns the Num for a named field of a record, or panics.
// For user-reachable code paths that may receive non-record values,
// use TryField which returns ok=false instead.
//
// Field is safe to call in record-method implementations because the D2/D3
// type-driven dispatch system validates the receiver type before the method
// runs, guaranteeing both that the receiver is a record and that the named
// field exists.
func (n Num) Field(name string) Num {
	v, ok := n.TryField(name)
	if !ok {
		panic("Num.Field: unknown field " + name)
	}
	return v
}

// SetField updates a named field in a record.
// Returns an error if the receiver is not a record.
func (n *Num) SetField(name string, val Num) error {
	if n.kind != kindRecord {
		return fmt.Errorf("Num.SetField: not a record")
	}
	n.fields[name] = val
	return nil
}

// node returns the IR node for this value: a Const for constants, or the
// tracked expression for memory expressions.
// Returns an error for records/arrays (which must be destructured into
// individual field/element nodes before lowering).
func (n Num) node() (ir.Node, error) {
	if n.isConst {
		return ir.Const(n.c), nil
	}
	if n.kind != kindScalar {
		return nil, fmt.Errorf("Num.node: non-scalar value has no single IR node; destructure fields/elements first")
	}
	return n.e, nil
}

// mustNode returns the IR node for a scalar value, or panics.
// It is intended for internal use in code paths where the Num has already been
// validated as scalar (e.g., after IsScalar() checks or in record-method
// implementations that destructure fields individually). Callers that may
// receive non-scalar values should use node() and handle errors.
func (n Num) mustNode() ir.Node {
	nd, err := n.node()
	if err != nil {
		panic(err)
	}
	return nd
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

// Vec2 methods: each returns a new composite Num by applying the operation to
// both fields individually. The optimizer folds the resulting field reads.

// vec2BinOp applies a binary operation element-wise to a Vec2.
// If w is a composite Vec2: op(v.x, w.x), op(v.y, w.y).
// Otherwise (scalar): op(v.x, w), op(v.y, w).
func vec2BinOp(gen *ir.IDGen, v Num, w Num, op resource.RuntimeFunction) (Num, error) {
	if w.IsComposite() {
		return compNum(map[string]Num{
			"x": exprNum(gen.PureInstr(op, v.Field("x").mustNode(), w.Field("x").mustNode())),
			"y": exprNum(gen.PureInstr(op, v.Field("y").mustNode(), w.Field("y").mustNode())),
		}), nil
	}
	return compNum(map[string]Num{
		"x": exprNum(gen.PureInstr(op, v.Field("x").mustNode(), w.mustNode())),
		"y": exprNum(gen.PureInstr(op, v.Field("y").mustNode(), w.mustNode())),
	}), nil
}

func vec2Add(t *tracer, v Num, args []Num) (Num, error) {
	return vec2BinOp(t.gen, v, args[0], resource.RuntimeFunctionAdd)
}

func vec2Sub(t *tracer, v Num, args []Num) (Num, error) {
	return vec2BinOp(t.gen, v, args[0], resource.RuntimeFunctionSubtract)
}

func vec2Mul(t *tracer, v Num, args []Num) (Num, error) {
	return vec2BinOp(t.gen, v, args[0], resource.RuntimeFunctionMultiply)
}

func vec2Div(t *tracer, v Num, args []Num) (Num, error) {
	return vec2BinOp(t.gen, v, args[0], resource.RuntimeFunctionDivide)
}

// matFields is the field layout of a 3x2 affine matrix.
var matFields = []string{"m11", "m12", "m13", "m21", "m22", "m23"}

func matScale(t *tracer, m Num, args []Num) (Num, error) {
	sx, sy := args[0], args[0]
	if len(args) > 1 {
		sy = args[1]
	}
	return compNum(map[string]Num{
		"m11": exprNum(t.gen.PureInstr(resource.RuntimeFunctionMultiply, m.Field("m11").mustNode(), sx.mustNode())),
		"m12": exprNum(t.gen.PureInstr(resource.RuntimeFunctionMultiply, m.Field("m12").mustNode(), sx.mustNode())),
		"m13": exprNum(t.gen.PureInstr(resource.RuntimeFunctionMultiply, m.Field("m13").mustNode(), sx.mustNode())),
		"m21": exprNum(t.gen.PureInstr(resource.RuntimeFunctionMultiply, m.Field("m21").mustNode(), sy.mustNode())),
		"m22": exprNum(t.gen.PureInstr(resource.RuntimeFunctionMultiply, m.Field("m22").mustNode(), sy.mustNode())),
		"m23": exprNum(t.gen.PureInstr(resource.RuntimeFunctionMultiply, m.Field("m23").mustNode(), sy.mustNode())),
	}), nil
}

func matTranslate(t *tracer, m Num, args []Num) (Num, error) {
	tx, ty := args[0], args[0]
	if len(args) > 1 {
		ty = args[1]
	}
	return compNum(map[string]Num{
		"m11": m.Field("m11"), "m12": m.Field("m12"),
		"m13": exprNum(t.gen.PureInstr(resource.RuntimeFunctionAdd, m.Field("m13").mustNode(), tx.mustNode())),
		"m21": m.Field("m21"), "m22": m.Field("m22"),
		"m23": exprNum(t.gen.PureInstr(resource.RuntimeFunctionAdd, m.Field("m23").mustNode(), ty.mustNode())),
	}), nil
}

// matCompose composes two 3×2 affine matrices: this * other.
// A 3×2 matrix represents [r0x r0y | tx; r1x r1y | ty] with an implicit
// [0 0 1] bottom row.
func matCompose(t *tracer, m Num, args []Num) (Num, error) {
	n := args[0]
	dot2 := func(r0, r1 string, c0, c1 string) ir.Node {
		return t.gen.PureInstr(resource.RuntimeFunctionAdd,
			t.gen.PureInstr(resource.RuntimeFunctionMultiply, m.Field(r0).mustNode(), n.Field(c0).mustNode()),
			t.gen.PureInstr(resource.RuntimeFunctionMultiply, m.Field(r1).mustNode(), n.Field(c1).mustNode()))
	}
	add := func(a, b ir.Node) ir.Node { return t.gen.PureInstr(resource.RuntimeFunctionAdd, a, b) }
	return compNum(map[string]Num{
		"m11": exprNum(dot2("m11", "m12", "m11", "m21")),
		"m12": exprNum(dot2("m11", "m12", "m12", "m22")),
		"m13": exprNum(add(dot2("m11", "m12", "m13", "m23"), m.Field("m13").mustNode())),
		"m21": exprNum(dot2("m21", "m22", "m11", "m21")),
		"m22": exprNum(dot2("m21", "m22", "m12", "m22")),
		"m23": exprNum(add(dot2("m21", "m22", "m13", "m23"), m.Field("m23").mustNode())),
	}), nil
}

// matRotate composes a rotation onto the receiver: this * rotate(angle).
func matRotate(t *tracer, m Num, args []Num) (Num, error) {
	a := args[0]
	cos := exprNum(t.gen.PureInstr(resource.RuntimeFunctionCos, a.mustNode()))
	sin := exprNum(t.gen.PureInstr(resource.RuntimeFunctionSin, a.mustNode()))
	negSin := exprNum(t.gen.PureInstr(resource.RuntimeFunctionNegate, sin.mustNode()))
	add := func(a, b ir.Node) ir.Node { return t.gen.PureInstr(resource.RuntimeFunctionAdd, a, b) }
	mul := func(a, b ir.Node) ir.Node { return t.gen.PureInstr(resource.RuntimeFunctionMultiply, a, b) }
	return compNum(map[string]Num{
		"m11": exprNum(add(mul(m.Field("m11").mustNode(), cos.mustNode()), mul(m.Field("m12").mustNode(), sin.mustNode()))),
		"m12": exprNum(add(mul(m.Field("m11").mustNode(), negSin.mustNode()), mul(m.Field("m12").mustNode(), cos.mustNode()))),
		"m13": m.Field("m13"),
		"m21": exprNum(add(mul(m.Field("m21").mustNode(), cos.mustNode()), mul(m.Field("m22").mustNode(), sin.mustNode()))),
		"m22": exprNum(add(mul(m.Field("m21").mustNode(), negSin.mustNode()), mul(m.Field("m22").mustNode(), cos.mustNode()))),
		"m23": m.Field("m23"),
	}), nil
}

var transFields = []string{"m11", "m12", "m13", "m21", "m22", "m23", "m31", "m32", "m33"}

func vec2Magnitude(t *tracer, v Num, args []Num) (Num, error) {
	x, y := v.Field("x"), v.Field("y")
	return exprNum(t.gen.PureInstr(resource.RuntimeFunctionPower,
		t.gen.PureInstr(resource.RuntimeFunctionAdd,
			t.gen.PureInstr(resource.RuntimeFunctionMultiply, x.mustNode(), x.mustNode()),
			t.gen.PureInstr(resource.RuntimeFunctionMultiply, y.mustNode(), y.mustNode())),
		ir.Const(0.5))), nil
}

func vec2Dot(t *tracer, v Num, args []Num) (Num, error) {
	w := args[0]
	return exprNum(t.gen.PureInstr(resource.RuntimeFunctionAdd,
		t.gen.PureInstr(resource.RuntimeFunctionMultiply, v.Field("x").mustNode(), w.Field("x").mustNode()),
		t.gen.PureInstr(resource.RuntimeFunctionMultiply, v.Field("y").mustNode(), w.Field("y").mustNode()))), nil
}

func vec2Normalize(t *tracer, v Num, args []Num) (Num, error) {
	x, y := v.Field("x"), v.Field("y")
	mag := t.gen.PureInstr(resource.RuntimeFunctionPower,
		t.gen.PureInstr(resource.RuntimeFunctionAdd,
			t.gen.PureInstr(resource.RuntimeFunctionMultiply, x.mustNode(), x.mustNode()),
			t.gen.PureInstr(resource.RuntimeFunctionMultiply, y.mustNode(), y.mustNode())),
		ir.Const(0.5))
	return compNum(map[string]Num{
		"x": exprNum(t.gen.PureInstr(resource.RuntimeFunctionDivide, x.mustNode(), mag)),
		"y": exprNum(t.gen.PureInstr(resource.RuntimeFunctionDivide, y.mustNode(), mag)),
	}), nil
}

var rectFields = []string{"t", "r", "b", "l"}

// knownRecordFields returns the ordered field names for a built-in or
// user-defined record type. Known built-in names: vec2, quad, mat, rect, trans.
func knownRecordFields(name string, userRecords map[string][]string) ([]string, bool) {
	switch name {
	case "vec2":
		return []string{"x", "y"}, true
	case "quad":
		return []string{"blx", "bly", "tlx", "tly", "trx", "try", "brx", "bry"}, true
	case "mat":
		return matFields, true
	case "rect":
		return rectFields, true
	case "trans":
		return transFields, true
	case "judgmentWindow":
		return judgmentWindowFields, true
	case "sprite":
		return spriteFields, true
	case "effect":
		return effectFields, true
	case "particle":
		return particleFields, true
	case "entityRef":
		return entityRefFields, true
	case "pair":
		return pairFields, true
	case "varArray":
		return varArrayFields, true
	case "arrayMap":
		return arrayMapFields, true
	case "arraySet":
		return arraySetFields, true
	case "box":
		return boxFields, true
	case "frozenNumSet":
		return frozenNumSetFields, true
	default:
		if f, ok := userRecords[name]; ok {
			return f, true
		}
		return nil, false
	}
}

func rectW(t *tracer, r Num, args []Num) (Num, error) {
	return exprNum(t.gen.PureInstr(resource.RuntimeFunctionSubtract, r.Field("r").mustNode(), r.Field("l").mustNode())), nil
}

func rectH(t *tracer, r Num, args []Num) (Num, error) {
	return exprNum(t.gen.PureInstr(resource.RuntimeFunctionSubtract, r.Field("t").mustNode(), r.Field("b").mustNode())), nil
}

func rectCenter(t *tracer, r Num, args []Num) (Num, error) {
	return compNum(map[string]Num{
		"x": exprNum(t.gen.PureInstr(resource.RuntimeFunctionDivide,
			t.gen.PureInstr(resource.RuntimeFunctionAdd, r.Field("l").mustNode(), r.Field("r").mustNode()),
			ir.Const(2))),
		"y": exprNum(t.gen.PureInstr(resource.RuntimeFunctionDivide,
			t.gen.PureInstr(resource.RuntimeFunctionAdd, r.Field("b").mustNode(), r.Field("t").mustNode()),
			ir.Const(2))),
	}), nil
}

func rectTranslate(t *tracer, r Num, args []Num) (Num, error) {
	dx, dy := args[0], args[0]
	if len(args) > 1 {
		dy = args[1]
	}
	return compNum(map[string]Num{
		"t": exprNum(t.gen.PureInstr(resource.RuntimeFunctionAdd, r.Field("t").mustNode(), dy.mustNode())),
		"r": exprNum(t.gen.PureInstr(resource.RuntimeFunctionAdd, r.Field("r").mustNode(), dx.mustNode())),
		"b": exprNum(t.gen.PureInstr(resource.RuntimeFunctionAdd, r.Field("b").mustNode(), dy.mustNode())),
		"l": exprNum(t.gen.PureInstr(resource.RuntimeFunctionAdd, r.Field("l").mustNode(), dx.mustNode())),
	}), nil
}

func rectScale(t *tracer, r Num, args []Num) (Num, error) {
	sx, sy := args[0], args[0]
	if len(args) > 1 {
		sy = args[1]
	}
	return compNum(map[string]Num{
		"t": exprNum(t.gen.PureInstr(resource.RuntimeFunctionMultiply, r.Field("t").mustNode(), sy.mustNode())),
		"r": exprNum(t.gen.PureInstr(resource.RuntimeFunctionMultiply, r.Field("r").mustNode(), sx.mustNode())),
		"b": exprNum(t.gen.PureInstr(resource.RuntimeFunctionMultiply, r.Field("b").mustNode(), sy.mustNode())),
		"l": exprNum(t.gen.PureInstr(resource.RuntimeFunctionMultiply, r.Field("l").mustNode(), sx.mustNode())),
	}), nil
}

var (
	_vec2Zero  = compNum(map[string]Num{"x": constNum(0), "y": constNum(0)})
	_vec2One   = compNum(map[string]Num{"x": constNum(1), "y": constNum(1)})
	_vec2Up    = compNum(map[string]Num{"x": constNum(0), "y": constNum(1)})
	_vec2Down  = compNum(map[string]Num{"x": constNum(0), "y": constNum(-1)})
	_vec2Left  = compNum(map[string]Num{"x": constNum(-1), "y": constNum(0)})
	_vec2Right = compNum(map[string]Num{"x": constNum(1), "y": constNum(0)})
)

func vec2Zero() Num  { return _vec2Zero }
func vec2One() Num   { return _vec2One }
func vec2Up() Num    { return _vec2Up }
func vec2Down() Num  { return _vec2Down }
func vec2Left() Num  { return _vec2Left }
func vec2Right() Num { return _vec2Right }

var vec2Statics = map[string]func() Num{
	"zero": vec2Zero, "one": vec2One, "up": vec2Up, "down": vec2Down,
	"left": vec2Left, "right": vec2Right,
}

func vec2Angle(t *tracer, v Num, args []Num) (Num, error) {
	return exprNum(t.gen.PureInstr(resource.RuntimeFunctionArctan2, v.Field("y").mustNode(), v.Field("x").mustNode())), nil
}

func vec2Rotate(t *tracer, v Num, args []Num) (Num, error) {
	a := args[0]
	cos := t.gen.PureInstr(resource.RuntimeFunctionCos, a.mustNode())
	sin := t.gen.PureInstr(resource.RuntimeFunctionSin, a.mustNode())
	return compNum(map[string]Num{
		"x": exprNum(t.gen.PureInstr(resource.RuntimeFunctionSubtract,
			t.gen.PureInstr(resource.RuntimeFunctionMultiply, v.Field("x").mustNode(), cos),
			t.gen.PureInstr(resource.RuntimeFunctionMultiply, v.Field("y").mustNode(), sin))),
		"y": exprNum(t.gen.PureInstr(resource.RuntimeFunctionAdd,
			t.gen.PureInstr(resource.RuntimeFunctionMultiply, v.Field("x").mustNode(), sin),
			t.gen.PureInstr(resource.RuntimeFunctionMultiply, v.Field("y").mustNode(), cos))),
	}), nil
}

func vec2Orthogonal(t *tracer, v Num, args []Num) (Num, error) {
	return compNum(map[string]Num{
		"x": exprNum(t.gen.PureInstr(resource.RuntimeFunctionNegate, v.Field("y").mustNode())),
		"y": v.Field("x"),
	}), nil
}

func quadCenter(t *tracer, q Num, args []Num) (Num, error) {
	return compNum(map[string]Num{
		"x": exprNum(t.gen.PureInstr(resource.RuntimeFunctionDivide,
			t.gen.PureInstr(resource.RuntimeFunctionAdd,
				t.gen.PureInstr(resource.RuntimeFunctionAdd, q.Field("blx").mustNode(), q.Field("tlx").mustNode()),
				t.gen.PureInstr(resource.RuntimeFunctionAdd, q.Field("trx").mustNode(), q.Field("brx").mustNode())),
			ir.Const(4))),
		"y": exprNum(t.gen.PureInstr(resource.RuntimeFunctionDivide,
			t.gen.PureInstr(resource.RuntimeFunctionAdd,
				t.gen.PureInstr(resource.RuntimeFunctionAdd, q.Field("bly").mustNode(), q.Field("tly").mustNode()),
				t.gen.PureInstr(resource.RuntimeFunctionAdd, q.Field("try").mustNode(), q.Field("bry").mustNode())),
			ir.Const(4))),
	}), nil
}

func quadTranslate(t *tracer, q Num, args []Num) (Num, error) {
	dx, dy := args[0], args[0]
	if len(args) > 1 {
		dy = args[1]
	}
	add := func(n ir.Node, d ir.Node) ir.Node { return t.gen.PureInstr(resource.RuntimeFunctionAdd, n, d) }
	return compNum(map[string]Num{
		"blx": exprNum(add(q.Field("blx").mustNode(), dx.mustNode())),
		"bly": exprNum(add(q.Field("bly").mustNode(), dy.mustNode())),
		"tlx": exprNum(add(q.Field("tlx").mustNode(), dx.mustNode())),
		"tly": exprNum(add(q.Field("tly").mustNode(), dy.mustNode())),
		"trx": exprNum(add(q.Field("trx").mustNode(), dx.mustNode())),
		"try": exprNum(add(q.Field("try").mustNode(), dy.mustNode())),
		"brx": exprNum(add(q.Field("brx").mustNode(), dx.mustNode())),
		"bry": exprNum(add(q.Field("bry").mustNode(), dy.mustNode())),
	}), nil
}

func quadScale(t *tracer, q Num, args []Num) (Num, error) {
	s := args[0]
	mul := func(n ir.Node) ir.Node { return t.gen.PureInstr(resource.RuntimeFunctionMultiply, n, s.mustNode()) }
	return compNum(map[string]Num{
		"blx": exprNum(mul(q.Field("blx").mustNode())), "bly": exprNum(mul(q.Field("bly").mustNode())),
		"tlx": exprNum(mul(q.Field("tlx").mustNode())), "tly": exprNum(mul(q.Field("tly").mustNode())),
		"trx": exprNum(mul(q.Field("trx").mustNode())), "try": exprNum(mul(q.Field("try").mustNode())),
		"brx": exprNum(mul(q.Field("brx").mustNode())), "bry": exprNum(mul(q.Field("bry").mustNode())),
	}), nil
}

func quadPermute(t *tracer, q Num, args []Num) (Num, error) {
	// permute(n) rotates quad corners by n positions (0-3).
	// For compile-time constant n, emit the right rotation.
	n := args[0]
	if n.isConst {
		switch int(n.c) % 4 {
		case 0:
			return q, nil
		case 1:
			return compNum(map[string]Num{
				"blx": q.Field("tlx"), "bly": q.Field("tly"),
				"tlx": q.Field("trx"), "tly": q.Field("try"),
				"trx": q.Field("brx"), "try": q.Field("bry"),
				"brx": q.Field("blx"), "bry": q.Field("bly"),
			}), nil
		case 2:
			return compNum(map[string]Num{
				"blx": q.Field("trx"), "bly": q.Field("try"),
				"tlx": q.Field("brx"), "tly": q.Field("bry"),
				"trx": q.Field("blx"), "try": q.Field("bly"),
				"brx": q.Field("tlx"), "bry": q.Field("tly"),
			}), nil
		case 3:
			return compNum(map[string]Num{
				"blx": q.Field("brx"), "bly": q.Field("bry"),
				"tlx": q.Field("blx"), "tly": q.Field("bly"),
				"trx": q.Field("tlx"), "try": q.Field("tly"),
				"brx": q.Field("trx"), "bry": q.Field("try"),
			}), nil
		}
	}
	return Num{}, fmt.Errorf("quad.permute: non-constant rotation is not supported")
}

func quadContains(t *tracer, q Num, args []Num) (Num, error) {
	p := args[0]
	px, py := p.Field("x").mustNode(), p.Field("y").mustNode()
	// Check if point is on the same side of all edges of the convex quad.
	// For each edge AB, cross product (B-A)x(P-A) should have the same sign.
	check := func(ax, ay, bx, by ir.Node) ir.Node {
		dx := t.gen.PureInstr(resource.RuntimeFunctionSubtract, bx, ax)
		dy := t.gen.PureInstr(resource.RuntimeFunctionSubtract, by, ay)
		nx := t.gen.PureInstr(resource.RuntimeFunctionSubtract, px, ax)
		ny := t.gen.PureInstr(resource.RuntimeFunctionSubtract, py, ay)
		return t.gen.PureInstr(resource.RuntimeFunctionSubtract,
			t.gen.PureInstr(resource.RuntimeFunctionMultiply, dx, ny),
			t.gen.PureInstr(resource.RuntimeFunctionMultiply, dy, nx))
	}
	v0 := check(q.Field("blx").mustNode(), q.Field("bly").mustNode(), q.Field("tlx").mustNode(), q.Field("tly").mustNode())
	v1 := check(q.Field("tlx").mustNode(), q.Field("tly").mustNode(), q.Field("trx").mustNode(), q.Field("try").mustNode())
	v2 := check(q.Field("trx").mustNode(), q.Field("try").mustNode(), q.Field("brx").mustNode(), q.Field("bry").mustNode())
	v3 := check(q.Field("brx").mustNode(), q.Field("bry").mustNode(), q.Field("blx").mustNode(), q.Field("bly").mustNode())
	// Point is inside if all cross products have the same sign (all >= 0 or all <= 0).
	same := t.gen.PureInstr(resource.RuntimeFunctionAnd,
		t.gen.PureInstr(resource.RuntimeFunctionAnd,
			t.gen.PureInstr(resource.RuntimeFunctionGreaterOr,
				t.gen.PureInstr(resource.RuntimeFunctionMultiply, v0, v1), ir.Const(0)),
			t.gen.PureInstr(resource.RuntimeFunctionGreaterOr,
				t.gen.PureInstr(resource.RuntimeFunctionMultiply, v1, v2), ir.Const(0))),
		t.gen.PureInstr(resource.RuntimeFunctionGreaterOr,
			t.gen.PureInstr(resource.RuntimeFunctionMultiply, v2, v3), ir.Const(0)))
	return exprNum(same), nil
}

func quadTop(t *tracer, q Num, args []Num) (Num, error) {
	return compNum(map[string]Num{
		"x": exprNum(t.gen.PureInstr(resource.RuntimeFunctionDivide,
			t.gen.PureInstr(resource.RuntimeFunctionAdd, q.Field("tlx").mustNode(), q.Field("trx").mustNode()), ir.Const(2))),
		"y": exprNum(t.gen.PureInstr(resource.RuntimeFunctionDivide,
			t.gen.PureInstr(resource.RuntimeFunctionAdd, q.Field("tly").mustNode(), q.Field("try").mustNode()), ir.Const(2))),
	}), nil
}
func quadRight(t *tracer, q Num, args []Num) (Num, error) {
	return compNum(map[string]Num{
		"x": exprNum(t.gen.PureInstr(resource.RuntimeFunctionDivide,
			t.gen.PureInstr(resource.RuntimeFunctionAdd, q.Field("trx").mustNode(), q.Field("brx").mustNode()), ir.Const(2))),
		"y": exprNum(t.gen.PureInstr(resource.RuntimeFunctionDivide,
			t.gen.PureInstr(resource.RuntimeFunctionAdd, q.Field("try").mustNode(), q.Field("bry").mustNode()), ir.Const(2))),
	}), nil
}
func quadBottom(t *tracer, q Num, args []Num) (Num, error) {
	return compNum(map[string]Num{
		"x": exprNum(t.gen.PureInstr(resource.RuntimeFunctionDivide,
			t.gen.PureInstr(resource.RuntimeFunctionAdd, q.Field("blx").mustNode(), q.Field("brx").mustNode()), ir.Const(2))),
		"y": exprNum(t.gen.PureInstr(resource.RuntimeFunctionDivide,
			t.gen.PureInstr(resource.RuntimeFunctionAdd, q.Field("bly").mustNode(), q.Field("bry").mustNode()), ir.Const(2))),
	}), nil
}
func quadLeft(t *tracer, q Num, args []Num) (Num, error) {
	return compNum(map[string]Num{
		"x": exprNum(t.gen.PureInstr(resource.RuntimeFunctionDivide,
			t.gen.PureInstr(resource.RuntimeFunctionAdd, q.Field("blx").mustNode(), q.Field("tlx").mustNode()), ir.Const(2))),
		"y": exprNum(t.gen.PureInstr(resource.RuntimeFunctionDivide,
			t.gen.PureInstr(resource.RuntimeFunctionAdd, q.Field("bly").mustNode(), q.Field("tly").mustNode()), ir.Const(2))),
	}), nil
}
func vec2RotateAbout(t *tracer, v Num, args []Num) (Num, error) {
	pt, angle := args[0], args[1]
	dx := exprNum(t.gen.PureInstr(resource.RuntimeFunctionSubtract, v.Field("x").mustNode(), pt.Field("x").mustNode()))
	dy := exprNum(t.gen.PureInstr(resource.RuntimeFunctionSubtract, v.Field("y").mustNode(), pt.Field("y").mustNode()))
	cs := t.gen.PureInstr(resource.RuntimeFunctionCos, angle.mustNode())
	sn := t.gen.PureInstr(resource.RuntimeFunctionSin, angle.mustNode())
	return compNum(map[string]Num{
		"x": exprNum(t.gen.PureInstr(resource.RuntimeFunctionAdd, pt.Field("x").mustNode(),
			t.gen.PureInstr(resource.RuntimeFunctionSubtract,
				t.gen.PureInstr(resource.RuntimeFunctionMultiply, dx.mustNode(), cs),
				t.gen.PureInstr(resource.RuntimeFunctionMultiply, dy.mustNode(), sn)))),
		"y": exprNum(t.gen.PureInstr(resource.RuntimeFunctionAdd, pt.Field("y").mustNode(),
			t.gen.PureInstr(resource.RuntimeFunctionAdd,
				t.gen.PureInstr(resource.RuntimeFunctionMultiply, dx.mustNode(), sn),
				t.gen.PureInstr(resource.RuntimeFunctionMultiply, dy.mustNode(), cs)))),
	}), nil
}

func vec2NormalizeOrZero(t *tracer, v Num, args []Num) (Num, error) {
	x, y := v.Field("x"), v.Field("y")
	magSq := t.gen.PureInstr(resource.RuntimeFunctionAdd,
		t.gen.PureInstr(resource.RuntimeFunctionMultiply, x.mustNode(), x.mustNode()),
		t.gen.PureInstr(resource.RuntimeFunctionMultiply, y.mustNode(), y.mustNode()))
	eps := ir.Const(1e-10)
	useZero := t.gen.PureInstr(resource.RuntimeFunctionLessOr, magSq, eps)
	mag := t.gen.PureInstr(resource.RuntimeFunctionPower, magSq, ir.Const(0.5))
	// If magnitude ≈ 0, return zero; else normalize
	normX := exprNum(t.gen.PureInstr(resource.RuntimeFunctionDivide, x.mustNode(), mag))
	normY := exprNum(t.gen.PureInstr(resource.RuntimeFunctionDivide, y.mustNode(), mag))
	// rather than Multiply-by-Not to avoid NaN·0 = NaN when mag ≈ 0.

	zero := ir.Const(0)
	return compNum(map[string]Num{
		"x": exprNum(t.gen.PureInstr(resource.RuntimeFunctionIf, useZero, zero, normX.mustNode())),
		"y": exprNum(t.gen.PureInstr(resource.RuntimeFunctionIf, useZero, zero, normY.mustNode())),
	}), nil
}

// angleDiff returns the absolute angle between two vectors: Acos(dot(a,b)/(|a||b|)).
func vec2AngleDiff(t *tracer, v Num, args []Num) (Num, error) {
	ax, ay := v.Field("x").mustNode(), v.Field("y").mustNode()
	w := args[0]
	bx, by := w.Field("x").mustNode(), w.Field("y").mustNode()
	dot := t.gen.PureInstr(resource.RuntimeFunctionAdd,
		t.gen.PureInstr(resource.RuntimeFunctionMultiply, ax, bx),
		t.gen.PureInstr(resource.RuntimeFunctionMultiply, ay, by))
	lenA := t.gen.PureInstr(resource.RuntimeFunctionPower,
		t.gen.PureInstr(resource.RuntimeFunctionAdd,
			t.gen.PureInstr(resource.RuntimeFunctionMultiply, ax, ax),
			t.gen.PureInstr(resource.RuntimeFunctionMultiply, ay, ay)),
		ir.Const(0.5))
	lenB := t.gen.PureInstr(resource.RuntimeFunctionPower,
		t.gen.PureInstr(resource.RuntimeFunctionAdd,
			t.gen.PureInstr(resource.RuntimeFunctionMultiply, bx, bx),
			t.gen.PureInstr(resource.RuntimeFunctionMultiply, by, by)),
		ir.Const(0.5))
	cos := t.gen.PureInstr(resource.RuntimeFunctionDivide, dot,
		t.gen.PureInstr(resource.RuntimeFunctionMultiply, lenA, lenB))
	return exprNum(t.gen.PureInstr(resource.RuntimeFunctionArccos,
		t.gen.PureInstr(resource.RuntimeFunctionClamp, cos, ir.Const(-1), ir.Const(1)))), nil
}

// signedAngleDiff returns the signed angle: Arctan2(cross(a,b), dot(a,b)).
func vec2SignedAngleDiff(t *tracer, v Num, args []Num) (Num, error) {
	ax, ay := v.Field("x").mustNode(), v.Field("y").mustNode()
	w := args[0]
	bx, by := w.Field("x").mustNode(), w.Field("y").mustNode()
	cross := t.gen.PureInstr(resource.RuntimeFunctionSubtract,
		t.gen.PureInstr(resource.RuntimeFunctionMultiply, ax, by),
		t.gen.PureInstr(resource.RuntimeFunctionMultiply, ay, bx))
	dot := t.gen.PureInstr(resource.RuntimeFunctionAdd,
		t.gen.PureInstr(resource.RuntimeFunctionMultiply, ax, bx),
		t.gen.PureInstr(resource.RuntimeFunctionMultiply, ay, by))
	return exprNum(t.gen.PureInstr(resource.RuntimeFunctionArctan2, cross, dot)), nil
}

func quadRotate(t *tracer, q Num, args []Num) (Num, error) {
	a := args[0]
	c := exprNum(t.gen.PureInstr(resource.RuntimeFunctionCos, a.mustNode()))
	s := exprNum(t.gen.PureInstr(resource.RuntimeFunctionSin, a.mustNode()))
	// rotate each corner around origin: (x*c - y*s, x*s + y*c)
	rot := func(fx, fy string) (Num, Num) {
		x := t.gen.PureInstr(resource.RuntimeFunctionSubtract,
			t.gen.PureInstr(resource.RuntimeFunctionMultiply, q.Field(fx).mustNode(), c.mustNode()),
			t.gen.PureInstr(resource.RuntimeFunctionMultiply, q.Field(fy).mustNode(), s.mustNode()))
		y := t.gen.PureInstr(resource.RuntimeFunctionAdd,
			t.gen.PureInstr(resource.RuntimeFunctionMultiply, q.Field(fx).mustNode(), s.mustNode()),
			t.gen.PureInstr(resource.RuntimeFunctionMultiply, q.Field(fy).mustNode(), c.mustNode()))
		return exprNum(x), exprNum(y)
	}
	blx, bly := rot("blx", "bly")
	tlx, tly := rot("tlx", "tly")
	trx, try := rot("trx", "try")
	brx, bry := rot("brx", "bry")
	return compNum(map[string]Num{"blx": blx, "bly": bly, "tlx": tlx, "tly": tly, "trx": trx, "try": try, "brx": brx, "bry": bry}), nil
}

func transCompose(t *tracer, m Num, args []Num) (Num, error) {
	n := args[0]
	dot := func(r [3]string, c [3]string) ir.Node {
		return t.gen.PureInstr(resource.RuntimeFunctionAdd,
			t.gen.PureInstr(resource.RuntimeFunctionAdd,
				t.gen.PureInstr(resource.RuntimeFunctionMultiply, m.Field(r[0]).mustNode(), n.Field(c[0]).mustNode()),
				t.gen.PureInstr(resource.RuntimeFunctionMultiply, m.Field(r[1]).mustNode(), n.Field(c[1]).mustNode())),
			t.gen.PureInstr(resource.RuntimeFunctionMultiply, m.Field(r[2]).mustNode(), n.Field(c[2]).mustNode()))
	}
	r0 := [3]string{"m11", "m12", "m13"}
	r1 := [3]string{"m21", "m22", "m23"}
	r2 := [3]string{"m31", "m32", "m33"}
	c0 := [3]string{"m11", "m21", "m31"}
	c1 := [3]string{"m12", "m22", "m32"}
	c2 := [3]string{"m13", "m23", "m33"}
	return compNum(map[string]Num{
		"m11": exprNum(dot(r0, c0)), "m12": exprNum(dot(r0, c1)), "m13": exprNum(dot(r0, c2)),
		"m21": exprNum(dot(r1, c0)), "m22": exprNum(dot(r1, c1)), "m23": exprNum(dot(r1, c2)),
		"m31": exprNum(dot(r2, c0)), "m32": exprNum(dot(r2, c1)), "m33": exprNum(dot(r2, c2)),
	}), nil
}

func transTranslate(t *tracer, m Num, args []Num) (Num, error) {
	v := args[0]
	return compNum(map[string]Num{
		"m11": m.Field("m11"), "m12": m.Field("m12"),
		"m13": exprNum(t.gen.PureInstr(resource.RuntimeFunctionAdd, m.Field("m13").mustNode(), v.Field("x").mustNode())),
		"m21": m.Field("m21"), "m22": m.Field("m22"),
		"m23": exprNum(t.gen.PureInstr(resource.RuntimeFunctionAdd, m.Field("m23").mustNode(), v.Field("y").mustNode())),
		"m31": m.Field("m31"), "m32": m.Field("m32"), "m33": m.Field("m33"),
	}), nil
}

func transScale(t *tracer, m Num, args []Num) (Num, error) {
	v := args[0]
	return compNum(map[string]Num{
		"m11": exprNum(t.gen.PureInstr(resource.RuntimeFunctionMultiply, m.Field("m11").mustNode(), v.Field("x").mustNode())),
		"m12": exprNum(t.gen.PureInstr(resource.RuntimeFunctionMultiply, m.Field("m12").mustNode(), v.Field("x").mustNode())),
		"m13": exprNum(t.gen.PureInstr(resource.RuntimeFunctionMultiply, m.Field("m13").mustNode(), v.Field("x").mustNode())),
		"m21": exprNum(t.gen.PureInstr(resource.RuntimeFunctionMultiply, m.Field("m21").mustNode(), v.Field("y").mustNode())),
		"m22": exprNum(t.gen.PureInstr(resource.RuntimeFunctionMultiply, m.Field("m22").mustNode(), v.Field("y").mustNode())),
		"m23": exprNum(t.gen.PureInstr(resource.RuntimeFunctionMultiply, m.Field("m23").mustNode(), v.Field("y").mustNode())),
		"m31": m.Field("m31"), "m32": m.Field("m32"), "m33": m.Field("m33"),
	}), nil
}

func transRotate(t *tracer, m Num, args []Num) (Num, error) {
	a := args[0]
	cos := exprNum(t.gen.PureInstr(resource.RuntimeFunctionCos, a.mustNode()))
	sin := exprNum(t.gen.PureInstr(resource.RuntimeFunctionSin, a.mustNode()))
	return compNum(map[string]Num{
		"m11": cos, "m12": exprNum(t.gen.PureInstr(resource.RuntimeFunctionNegate, sin.mustNode())), "m13": m.Field("m13"),
		"m21": sin, "m22": cos, "m23": m.Field("m23"),
		"m31": m.Field("m31"), "m32": m.Field("m32"), "m33": m.Field("m33"),
	}), nil
}

// transTransformVec applies a Transform2d to a Vec2: (x*m11 + y*m12 + m13, x*m21 + y*m22 + m23).
func transTransformVec(t *tracer, m Num, args []Num) (Num, error) {
	v := args[0]
	x := v.Field("x").mustNode()
	y := v.Field("y").mustNode()
	add := func(a, b ir.Node) ir.Node { return t.gen.PureInstr(resource.RuntimeFunctionAdd, a, b) }
	mul := func(a, b ir.Node) ir.Node { return t.gen.PureInstr(resource.RuntimeFunctionMultiply, a, b) }
	nx := add(add(mul(m.Field("m11").mustNode(), x), mul(m.Field("m12").mustNode(), y)), m.Field("m13").mustNode())
	ny := add(add(mul(m.Field("m21").mustNode(), x), mul(m.Field("m22").mustNode(), y)), m.Field("m23").mustNode())
	return compNum(map[string]Num{"x": exprNum(nx), "y": exprNum(ny)}), nil
}

// recordMethodEntry bundles a record-method implementation with arity and
// type-safety guards. minArity is the minimum number of arguments; args
// at indices in compositeArgAt (0-based relative to the supplied args,
// NOT including the receiver) must be composite values.
//
// If containerFn is set (non-nil), it is called instead of fn when the
// receiver is a container-backed variable (VarArray, ArrayMap, ArraySet),
// giving the implementation access to the containerInfo for direct IR
// emission to the backing array.
type recordMethodEntry struct {
	fn             func(*tracer, Num, []Num) (Num, error)
	containerFn    func(*tracer, *containerInfo, Num, []Num) (Num, error)
	minArity       int
	compositeArgAt []int
}

// recordMethods maps record type name → method name → implementation.
// Used by the table-driven dispatch in tracer.methodCall (trace.go).
var recordMethods = map[string]map[string]recordMethodEntry{
	"vec2": {
		"add":             {fn: vec2Add, minArity: 1},
		"sub":             {fn: vec2Sub, minArity: 1},
		"mul":             {fn: vec2Mul, minArity: 1},
		"div":             {fn: vec2Div, minArity: 1},
		"magnitude":       {fn: vec2Magnitude, minArity: 0},
		"dot":             {fn: vec2Dot, minArity: 1, compositeArgAt: []int{0}},
		"normalize":       {fn: vec2Normalize, minArity: 0},
		"normalizeOrZero": {fn: vec2NormalizeOrZero, minArity: 0},
		"angle":           {fn: vec2Angle, minArity: 0},
		"rotate":          {fn: vec2Rotate, minArity: 1},
		"orthogonal":      {fn: vec2Orthogonal, minArity: 0},
		"rotateAbout":     {fn: vec2RotateAbout, minArity: 2, compositeArgAt: []int{0}},
		"angleDiff":       {fn: vec2AngleDiff, minArity: 1, compositeArgAt: []int{0}},
		"signedAngleDiff": {fn: vec2SignedAngleDiff, minArity: 1, compositeArgAt: []int{0}},
	},
	"mat": {
		"scale":     {fn: matScale, minArity: 1},
		"translate": {fn: matTranslate, minArity: 1},
		"compose":   {fn: matCompose, minArity: 1, compositeArgAt: []int{0}},
		"rotate":    {fn: matRotate, minArity: 1},
	},
	"rect": {
		"w":         {fn: rectW, minArity: 0},
		"h":         {fn: rectH, minArity: 0},
		"center":    {fn: rectCenter, minArity: 0},
		"translate": {fn: rectTranslate, minArity: 1},
		"scale":     {fn: rectScale, minArity: 1},
	},
	"quad": {
		"center":    {fn: quadCenter, minArity: 0},
		"translate": {fn: quadTranslate, minArity: 1},
		"scale":     {fn: quadScale, minArity: 1},
		"permute":   {fn: quadPermute, minArity: 1},
		"rotate":    {fn: quadRotate, minArity: 1},
		"top":       {fn: quadTop, minArity: 0},
		"right":     {fn: quadRight, minArity: 0},
		"bottom":    {fn: quadBottom, minArity: 0},
		"left":      {fn: quadLeft, minArity: 0},
		"contains":  {fn: quadContains, minArity: 1, compositeArgAt: []int{0}},
	},
	"trans": {
		"compose":      {fn: transCompose, minArity: 1, compositeArgAt: []int{0}},
		"translate":    {fn: transTranslate, minArity: 1, compositeArgAt: []int{0}},
		"scale":        {fn: transScale, minArity: 1, compositeArgAt: []int{0}},
		"rotate":       {fn: transRotate, minArity: 1},
		"transformVec": {fn: transTransformVec, minArity: 1, compositeArgAt: []int{0}},
	},
	"judgmentWindow": {
		"judge": {fn: judgmentWindowJudge, minArity: 2},
	},
	"sprite": {
		"draw": {fn: spriteDraw, minArity: 0},
	},
	"effect": {
		"play": {fn: effectPlay, minArity: 1},
	},
	"particle": {
		"spawn": {fn: particleSpawn, minArity: 0},
	},
	"pair": {
		"lt":    {fn: pairLt, minArity: 1, compositeArgAt: []int{0}},
		"le":    {fn: pairLe, minArity: 1, compositeArgAt: []int{0}},
		"gt":    {fn: pairGt, minArity: 1, compositeArgAt: []int{0}},
		"ge":    {fn: pairGe, minArity: 1, compositeArgAt: []int{0}},
		"tuple": {fn: pairTuple, minArity: 0},
	},
	"varArray": {
		"len":       {fn: varArrayLen, containerFn: varArrayLenCI, minArity: 0},
		"capacity":  {fn: varArrayCapacity, containerFn: varArrayCapacityCI, minArity: 0},
		"isFull":    {fn: varArrayIsFull, containerFn: varArrayIsFullCI, minArity: 0},
		"append":    {fn: varArrayAppend, containerFn: varArrayAppendCI, minArity: 1},
		"pop":       {fn: varArrayPop, containerFn: varArrayPopCI, minArity: 0},
		"clear":     {fn: varArrayClear, containerFn: varArrayClearCI, minArity: 0},
		"contains":  {containerFn: varArrayContainsCI, minArity: 1},
		"index":     {containerFn: varArrayIndexCI, minArity: 1},
		"remove":    {containerFn: varArrayRemoveCI, minArity: 1},
		"insert":    {containerFn: varArrayInsertCI, minArity: 2},
		"sort":      {containerFn: varArraySortCI, minArity: 0},
		"setAdd":    {containerFn: varArraySetAddCI, minArity: 1},
		"setRemove": {containerFn: varArraySetRemoveCI, minArity: 1},
	},
	"arrayMap": {
		"len":      {fn: varArrayLen, containerFn: varArrayLenCI, minArity: 0},
		"capacity": {fn: varArrayCapacity, containerFn: varArrayCapacityCI, minArity: 0},
		"clear":    {fn: varArrayClear, containerFn: varArrayClearCI, minArity: 0},
		"get":      {containerFn: arrayMapGetCI, minArity: 1},
		"set":      {containerFn: arrayMapSetCI, minArity: 2},
		"delete":   {containerFn: arrayMapDeleteCI, minArity: 1},
		"contains": {containerFn: arrayMapContainsCI, minArity: 1},
		"pop":      {containerFn: arrayMapPopCI, minArity: 1},
		"keys":     {fn: containerSelf, minArity: 0},
		"values":   {fn: containerSelf, minArity: 0},
		"items":    {fn: containerSelf, minArity: 0},
	},
	"arraySet": {
		"len":      {fn: varArrayLen, containerFn: varArrayLenCI, minArity: 0},
		"capacity": {fn: varArrayCapacity, containerFn: varArrayCapacityCI, minArity: 0},
		"clear":    {fn: varArrayClear, containerFn: varArrayClearCI, minArity: 0},
		"add":      {containerFn: varArraySetAddCI, minArity: 1},
		"remove":   {containerFn: varArraySetRemoveCI, minArity: 1},
		"contains": {containerFn: varArrayContainsCI, minArity: 1},
	},
	"frozenNumSet": {
		"len":      {fn: varArrayLen, containerFn: varArrayLenCI, minArity: 0},
		"capacity": {fn: varArrayCapacity, containerFn: varArrayCapacityCI, minArity: 0},
		"contains": {containerFn: frozenNumSetContainsCI, minArity: 1},
	},
}

// Sprite, Effect, Particle handles are thin records wrapping a numeric runtime ID.
var spriteFields = []string{"id"}
var effectFields = []string{"id"}
var particleFields = []string{"id"}

// EntityRef wraps an entity index, enabling cross-entity data access.
var entityRefFields = []string{"index"}

// Pair is a generic two-element record.
var pairFields = []string{"first", "second"}

// VarArray is a variable-size array backed by a fixed-capacity buffer.
// It is compiled as a record with two fields: _size (current element count)
// and _array (backing storage). The _array is a multi-slot TempBlock allocated
// at declaration time with capacity slots.
var varArrayFields = []string{"_size", "_array"}

// JudgmentWindow field names.
// judgmentWindowJudge implements JudgmentWindow.judge(actual, target) → Judgment.
// Calls native judge(actual, target, perfectMin, perfectMax, greatMin, greatMax,
// goodMin, goodMax) which returns 0 (miss), 1 (perfect), 2 (great), or 3 (good).
func judgmentWindowJudge(t *tracer, w Num, args []Num) (Num, error) {
	actual, target := args[0], args[1]
	return exprNum(t.gen.PureInstr(resource.RuntimeFunctionJudge,
		actual.mustNode(),
		target.mustNode(),
		w.Field("perfectMin").mustNode(),
		w.Field("perfectMax").mustNode(),
		w.Field("greatMin").mustNode(),
		w.Field("greatMax").mustNode(),
		w.Field("goodMin").mustNode(),
		w.Field("goodMax").mustNode(),
	)), nil
}

// spriteDraw implements Sprite.draw(args...) → native Draw(id, args...).
func spriteDraw(t *tracer, s Num, args []Num) (Num, error) {
	nodes := make([]ir.Node, 1+len(args))
	nodes[0] = s.Field("id").mustNode()
	for i, a := range args {
		nodes[i+1] = a.mustNode()
	}
	return exprNum(t.gen.ImpureInstr(resource.RuntimeFunctionDraw, nodes...)), nil
}

// effectPlay implements Effect.play(distance) → native Play(id, distance).
func effectPlay(t *tracer, e Num, args []Num) (Num, error) {
	return exprNum(t.gen.ImpureInstr(resource.RuntimeFunctionPlay,
		e.Field("id").mustNode(),
		args[0].mustNode(),
	)), nil
}

// particleSpawn implements Particle.spawn(args...) → native SpawnParticleEffect(id, args...).
func particleSpawn(t *tracer, p Num, args []Num) (Num, error) {
	nodes := make([]ir.Node, 1+len(args))
	nodes[0] = p.Field("id").mustNode()
	for i, a := range args {
		nodes[i+1] = a.mustNode()
	}
	return exprNum(t.gen.ImpureInstr(resource.RuntimeFunctionSpawnParticleEffect, nodes...)), nil
}

var judgmentWindowFields = []string{
	"perfectMin", "perfectMax",
	"greatMin", "greatMax",
	"goodMin", "goodMax",
}

// recordStatics maps record type name → static-constructor name → implementation.
var recordStatics = map[string]map[string]func() (Num, error){
	"vec2": {
		"zero":  func() (Num, error) { return vec2Zero(), nil },
		"one":   func() (Num, error) { return vec2One(), nil },
		"up":    func() (Num, error) { return vec2Up(), nil },
		"down":  func() (Num, error) { return vec2Down(), nil },
		"left":  func() (Num, error) { return vec2Left(), nil },
		"right": func() (Num, error) { return vec2Right(), nil },
	},
}
