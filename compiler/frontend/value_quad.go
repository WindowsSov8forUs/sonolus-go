package frontend

import (
	"fmt"

	"github.com/WindowsSov8forUs/sonolus-core-go/core/resource"

	"github.com/WindowsSov8forUs/sonolus-go/compiler/ir"
)

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
