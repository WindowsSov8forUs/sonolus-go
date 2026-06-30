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
