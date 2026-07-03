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
				t.gen.PureInstr(resource.RuntimeFunctionAdd, q.MustField("blx").mustNode(), q.MustField("tlx").mustNode()),
				t.gen.PureInstr(resource.RuntimeFunctionAdd, q.MustField("trx").mustNode(), q.MustField("brx").mustNode())),
			ir.Const(4))),
		"y": exprNum(t.gen.PureInstr(resource.RuntimeFunctionDivide,
			t.gen.PureInstr(resource.RuntimeFunctionAdd,
				t.gen.PureInstr(resource.RuntimeFunctionAdd, q.MustField("bly").mustNode(), q.MustField("tly").mustNode()),
				t.gen.PureInstr(resource.RuntimeFunctionAdd, q.MustField("try").mustNode(), q.MustField("bry").mustNode())),
			ir.Const(4))),
	}), nil
}

func quadTranslate(t *tracer, q Num, args []Num) (Num, error) {
	p := args[0]
	dx, dy := p.MustField("x").mustNode(), p.MustField("y").mustNode()
	add := func(n ir.Node, d ir.Node) ir.Node { return t.gen.PureInstr(resource.RuntimeFunctionAdd, n, d) }
	return compNum(map[string]Num{
		"blx": exprNum(add(q.MustField("blx").mustNode(), dx)),
		"bly": exprNum(add(q.MustField("bly").mustNode(), dy)),
		"tlx": exprNum(add(q.MustField("tlx").mustNode(), dx)),
		"tly": exprNum(add(q.MustField("tly").mustNode(), dy)),
		"trx": exprNum(add(q.MustField("trx").mustNode(), dx)),
		"try": exprNum(add(q.MustField("try").mustNode(), dy)),
		"brx": exprNum(add(q.MustField("brx").mustNode(), dx)),
		"bry": exprNum(add(q.MustField("bry").mustNode(), dy)),
	}), nil
}

func quadScale(t *tracer, q Num, args []Num) (Num, error) {
	s := args[0]
	mul := func(n ir.Node) ir.Node { return t.gen.PureInstr(resource.RuntimeFunctionMultiply, n, s.mustNode()) }
	return compNum(map[string]Num{
		"blx": exprNum(mul(q.MustField("blx").mustNode())), "bly": exprNum(mul(q.MustField("bly").mustNode())),
		"tlx": exprNum(mul(q.MustField("tlx").mustNode())), "tly": exprNum(mul(q.MustField("tly").mustNode())),
		"trx": exprNum(mul(q.MustField("trx").mustNode())), "try": exprNum(mul(q.MustField("try").mustNode())),
		"brx": exprNum(mul(q.MustField("brx").mustNode())), "bry": exprNum(mul(q.MustField("bry").mustNode())),
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
				"blx": q.MustField("tlx"), "bly": q.MustField("tly"),
				"tlx": q.MustField("trx"), "tly": q.MustField("try"),
				"trx": q.MustField("brx"), "try": q.MustField("bry"),
				"brx": q.MustField("blx"), "bry": q.MustField("bly"),
			}), nil
		case 2:
			return compNum(map[string]Num{
				"blx": q.MustField("trx"), "bly": q.MustField("try"),
				"tlx": q.MustField("brx"), "tly": q.MustField("bry"),
				"trx": q.MustField("blx"), "try": q.MustField("bly"),
				"brx": q.MustField("tlx"), "bry": q.MustField("tly"),
			}), nil
		case 3:
			return compNum(map[string]Num{
				"blx": q.MustField("brx"), "bly": q.MustField("bry"),
				"tlx": q.MustField("blx"), "tly": q.MustField("bly"),
				"trx": q.MustField("tlx"), "try": q.MustField("tly"),
				"brx": q.MustField("trx"), "bry": q.MustField("try"),
			}), nil
		}
	}
	return Num{}, fmt.Errorf("quad.permute: non-constant rotation is not supported")
}

func quadContains(t *tracer, q Num, args []Num) (Num, error) {
	p := args[0]
	px, py := p.MustField("x").mustNode(), p.MustField("y").mustNode()
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
	v0 := check(q.MustField("blx").mustNode(), q.MustField("bly").mustNode(), q.MustField("tlx").mustNode(), q.MustField("tly").mustNode())
	v1 := check(q.MustField("tlx").mustNode(), q.MustField("tly").mustNode(), q.MustField("trx").mustNode(), q.MustField("try").mustNode())
	v2 := check(q.MustField("trx").mustNode(), q.MustField("try").mustNode(), q.MustField("brx").mustNode(), q.MustField("bry").mustNode())
	v3 := check(q.MustField("brx").mustNode(), q.MustField("bry").mustNode(), q.MustField("blx").mustNode(), q.MustField("bly").mustNode())
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

// quadEdge computes the midpoint of a quad edge defined by two corners.
// fx1/fy1 and fx2/fy2 are the x/y field name pairs for the two corners.
func quadEdge(t *tracer, q Num, fx1, fy1, fx2, fy2 string) (Num, error) {
	return compNum(map[string]Num{
		"x": exprNum(t.gen.PureInstr(resource.RuntimeFunctionDivide,
			t.gen.PureInstr(resource.RuntimeFunctionAdd,
				q.MustField(fx1).mustNode(), q.MustField(fx2).mustNode()), ir.Const(2))),
		"y": exprNum(t.gen.PureInstr(resource.RuntimeFunctionDivide,
			t.gen.PureInstr(resource.RuntimeFunctionAdd,
				q.MustField(fy1).mustNode(), q.MustField(fy2).mustNode()), ir.Const(2))),
	}), nil
}

func quadTop(t *tracer, q Num, args []Num) (Num, error) {
	return quadEdge(t, q, "tlx", "tly", "trx", "try")
}
func quadRight(t *tracer, q Num, args []Num) (Num, error) {
	return quadEdge(t, q, "trx", "try", "brx", "bry")
}
func quadBottom(t *tracer, q Num, args []Num) (Num, error) {
	return quadEdge(t, q, "blx", "bly", "brx", "bry")
}
func quadLeft(t *tracer, q Num, args []Num) (Num, error) {
	return quadEdge(t, q, "blx", "bly", "tlx", "tly")
}

func quadRotate(t *tracer, q Num, args []Num) (Num, error) {
	a := args[0]
	c := exprNum(t.gen.PureInstr(resource.RuntimeFunctionCos, a.mustNode()))
	s := exprNum(t.gen.PureInstr(resource.RuntimeFunctionSin, a.mustNode()))
	// rotate each corner around origin: (x*c - y*s, x*s + y*c)
	rot := func(fx, fy string) (Num, Num) {
		x := t.gen.PureInstr(resource.RuntimeFunctionSubtract,
			t.gen.PureInstr(resource.RuntimeFunctionMultiply, q.MustField(fx).mustNode(), c.mustNode()),
			t.gen.PureInstr(resource.RuntimeFunctionMultiply, q.MustField(fy).mustNode(), s.mustNode()))
		y := t.gen.PureInstr(resource.RuntimeFunctionAdd,
			t.gen.PureInstr(resource.RuntimeFunctionMultiply, q.MustField(fx).mustNode(), s.mustNode()),
			t.gen.PureInstr(resource.RuntimeFunctionMultiply, q.MustField(fy).mustNode(), c.mustNode()))
		return exprNum(x), exprNum(y)
	}
	blx, bly := rot("blx", "bly")
	tlx, tly := rot("tlx", "tly")
	trx, try := rot("trx", "try")
	brx, bry := rot("brx", "bry")
	return compNum(map[string]Num{"blx": blx, "bly": bly, "tlx": tlx, "tly": tly, "trx": trx, "try": try, "brx": brx, "bry": bry}), nil
}
