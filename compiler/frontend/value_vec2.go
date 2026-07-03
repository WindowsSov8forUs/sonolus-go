package frontend

import (
	"github.com/WindowsSov8forUs/sonolus-core-go/core/resource"

	"github.com/WindowsSov8forUs/sonolus-go/compiler/ir"
)

// Vec2 methods: each returns a new composite Num by applying the operation to
// both fields individually. The optimizer folds the resulting field reads.

// vec2BinOp applies a binary operation element-wise to a Vec2.
// If w is a composite Vec2: op(v.x, w.x), op(v.y, w.y).
// Otherwise (scalar): op(v.x, w), op(v.y, w).
func vec2BinOp(gen *ir.IDGen, v Num, w Num, op resource.RuntimeFunction) (Num, error) {
	if w.IsComposite() {
		return compNum(map[string]Num{
			"x": exprNum(gen.PureInstr(op, v.MustField("x").mustNode(), w.MustField("x").mustNode())),
			"y": exprNum(gen.PureInstr(op, v.MustField("y").mustNode(), w.MustField("y").mustNode())),
		}), nil
	}
	return compNum(map[string]Num{
		"x": exprNum(gen.PureInstr(op, v.MustField("x").mustNode(), w.mustNode())),
		"y": exprNum(gen.PureInstr(op, v.MustField("y").mustNode(), w.mustNode())),
	}), nil
}

// vec2MagSq returns x² + y² as an IR expression.
func vec2MagSq(gen *ir.IDGen, v Num) ir.Node {
	x, y := v.MustField("x"), v.MustField("y")
	return gen.PureInstr(resource.RuntimeFunctionAdd,
		gen.PureInstr(resource.RuntimeFunctionMultiply, x.mustNode(), x.mustNode()),
		gen.PureInstr(resource.RuntimeFunctionMultiply, y.mustNode(), y.mustNode()))
}

// vec2Mag returns sqrt(x² + y²) as an IR expression.
func vec2Mag(gen *ir.IDGen, v Num) ir.Node {
	return gen.PureInstr(resource.RuntimeFunctionPower, vec2MagSq(gen, v), ir.Const(0.5))
}

// makeVec2BinOp creates a binary-op method for Vec2 that applies the given
// operation element-wise.
func makeVec2BinOp(op resource.RuntimeFunction) func(*tracer, Num, []Num) (Num, error) {
	return func(t *tracer, v Num, args []Num) (Num, error) {
		return vec2BinOp(t.gen, v, args[0], op)
	}
}

var vec2Add = makeVec2BinOp(resource.RuntimeFunctionAdd)
var vec2Sub = makeVec2BinOp(resource.RuntimeFunctionSubtract)
var vec2Mul = makeVec2BinOp(resource.RuntimeFunctionMultiply)
var vec2Div = makeVec2BinOp(resource.RuntimeFunctionDivide)

func vec2Magnitude(t *tracer, v Num, args []Num) (Num, error) {
	return exprNum(vec2Mag(t.gen, v)), nil
}

func vec2Dot(t *tracer, v Num, args []Num) (Num, error) {
	w := args[0]
	return exprNum(t.gen.PureInstr(resource.RuntimeFunctionAdd,
		t.gen.PureInstr(resource.RuntimeFunctionMultiply, v.MustField("x").mustNode(), w.MustField("x").mustNode()),
		t.gen.PureInstr(resource.RuntimeFunctionMultiply, v.MustField("y").mustNode(), w.MustField("y").mustNode()))), nil
}

func vec2Normalize(t *tracer, v Num, args []Num) (Num, error) {
	x, y := v.MustField("x"), v.MustField("y")
	mag := vec2Mag(t.gen, v)
	return compNum(map[string]Num{
		"x": exprNum(t.gen.PureInstr(resource.RuntimeFunctionDivide, x.mustNode(), mag)),
		"y": exprNum(t.gen.PureInstr(resource.RuntimeFunctionDivide, y.mustNode(), mag)),
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
	return exprNum(t.gen.PureInstr(resource.RuntimeFunctionArctan2, v.MustField("y").mustNode(), v.MustField("x").mustNode())), nil
}

func vec2Rotate(t *tracer, v Num, args []Num) (Num, error) {
	a := args[0]
	cos := t.gen.PureInstr(resource.RuntimeFunctionCos, a.mustNode())
	sin := t.gen.PureInstr(resource.RuntimeFunctionSin, a.mustNode())
	return compNum(map[string]Num{
		"x": exprNum(t.gen.PureInstr(resource.RuntimeFunctionSubtract,
			t.gen.PureInstr(resource.RuntimeFunctionMultiply, v.MustField("x").mustNode(), cos),
			t.gen.PureInstr(resource.RuntimeFunctionMultiply, v.MustField("y").mustNode(), sin))),
		"y": exprNum(t.gen.PureInstr(resource.RuntimeFunctionAdd,
			t.gen.PureInstr(resource.RuntimeFunctionMultiply, v.MustField("x").mustNode(), sin),
			t.gen.PureInstr(resource.RuntimeFunctionMultiply, v.MustField("y").mustNode(), cos))),
	}), nil
}

func vec2Orthogonal(t *tracer, v Num, args []Num) (Num, error) {
	return compNum(map[string]Num{
		"x": exprNum(t.gen.PureInstr(resource.RuntimeFunctionNegate, v.MustField("y").mustNode())),
		"y": v.MustField("x"),
	}), nil
}

func vec2RotateAbout(t *tracer, v Num, args []Num) (Num, error) {
	pt, angle := args[0], args[1]
	dx := exprNum(t.gen.PureInstr(resource.RuntimeFunctionSubtract, v.MustField("x").mustNode(), pt.MustField("x").mustNode()))
	dy := exprNum(t.gen.PureInstr(resource.RuntimeFunctionSubtract, v.MustField("y").mustNode(), pt.MustField("y").mustNode()))
	cs := t.gen.PureInstr(resource.RuntimeFunctionCos, angle.mustNode())
	sn := t.gen.PureInstr(resource.RuntimeFunctionSin, angle.mustNode())
	return compNum(map[string]Num{
		"x": exprNum(t.gen.PureInstr(resource.RuntimeFunctionAdd, pt.MustField("x").mustNode(),
			t.gen.PureInstr(resource.RuntimeFunctionSubtract,
				t.gen.PureInstr(resource.RuntimeFunctionMultiply, dx.mustNode(), cs),
				t.gen.PureInstr(resource.RuntimeFunctionMultiply, dy.mustNode(), sn)))),
		"y": exprNum(t.gen.PureInstr(resource.RuntimeFunctionAdd, pt.MustField("y").mustNode(),
			t.gen.PureInstr(resource.RuntimeFunctionAdd,
				t.gen.PureInstr(resource.RuntimeFunctionMultiply, dx.mustNode(), sn),
				t.gen.PureInstr(resource.RuntimeFunctionMultiply, dy.mustNode(), cs)))),
	}), nil
}

func vec2NormalizeOrZero(t *tracer, v Num, args []Num) (Num, error) {
	x, y := v.MustField("x"), v.MustField("y")
	magSq := vec2MagSq(t.gen, v)
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
	ax, ay := v.MustField("x").mustNode(), v.MustField("y").mustNode()
	w := args[0]
	bx, by := w.MustField("x").mustNode(), w.MustField("y").mustNode()
	dot := t.gen.PureInstr(resource.RuntimeFunctionAdd,
		t.gen.PureInstr(resource.RuntimeFunctionMultiply, ax, bx),
		t.gen.PureInstr(resource.RuntimeFunctionMultiply, ay, by))
	// vec2MagFromXY computes sqrt(x² + y²) from raw x,y nodes.
	vec2MagFromXY := func(x, y ir.Node) ir.Node {
		return t.gen.PureInstr(resource.RuntimeFunctionPower,
			t.gen.PureInstr(resource.RuntimeFunctionAdd,
				t.gen.PureInstr(resource.RuntimeFunctionMultiply, x, x),
				t.gen.PureInstr(resource.RuntimeFunctionMultiply, y, y)),
			ir.Const(0.5))
	}
	lenA := vec2MagFromXY(ax, ay)
	lenB := vec2MagFromXY(bx, by)
	cos := t.gen.PureInstr(resource.RuntimeFunctionDivide, dot,
		t.gen.PureInstr(resource.RuntimeFunctionMultiply, lenA, lenB))
	return exprNum(t.gen.PureInstr(resource.RuntimeFunctionArccos,
		t.gen.PureInstr(resource.RuntimeFunctionClamp, cos, ir.Const(-1), ir.Const(1)))), nil
}

// signedAngleDiff returns the signed angle: Arctan2(cross(a,b), dot(a,b)).
func vec2SignedAngleDiff(t *tracer, v Num, args []Num) (Num, error) {
	ax, ay := v.MustField("x").mustNode(), v.MustField("y").mustNode()
	w := args[0]
	bx, by := w.MustField("x").mustNode(), w.MustField("y").mustNode()
	cross := t.gen.PureInstr(resource.RuntimeFunctionSubtract,
		t.gen.PureInstr(resource.RuntimeFunctionMultiply, ax, by),
		t.gen.PureInstr(resource.RuntimeFunctionMultiply, ay, bx))
	dot := t.gen.PureInstr(resource.RuntimeFunctionAdd,
		t.gen.PureInstr(resource.RuntimeFunctionMultiply, ax, bx),
		t.gen.PureInstr(resource.RuntimeFunctionMultiply, ay, by))
	return exprNum(t.gen.PureInstr(resource.RuntimeFunctionArctan2, cross, dot)), nil
}
