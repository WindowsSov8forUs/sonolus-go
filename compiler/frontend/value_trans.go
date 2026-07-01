package frontend

import (
	"github.com/WindowsSov8forUs/sonolus-core-go/core/resource"

	"github.com/WindowsSov8forUs/sonolus-go/compiler/ir"
)

var transFields = []string{"m11", "m12", "m13", "m21", "m22", "m23", "m31", "m32", "m33"}

func transCompose(t *tracer, m Num, args []Num) (Num, error) {
	n := args[0]
	dot := func(r [3]string, c [3]string) ir.Node {
		return t.gen.PureInstr(resource.RuntimeFunctionAdd,
			t.gen.PureInstr(resource.RuntimeFunctionAdd,
				t.gen.PureInstr(resource.RuntimeFunctionMultiply, m.MustField(r[0]).mustNode(), n.MustField(c[0]).mustNode()),
				t.gen.PureInstr(resource.RuntimeFunctionMultiply, m.MustField(r[1]).mustNode(), n.MustField(c[1]).mustNode())),
			t.gen.PureInstr(resource.RuntimeFunctionMultiply, m.MustField(r[2]).mustNode(), n.MustField(c[2]).mustNode()))
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
		"m11": m.MustField("m11"), "m12": m.MustField("m12"),
		"m13": exprNum(t.gen.PureInstr(resource.RuntimeFunctionAdd, m.MustField("m13").mustNode(), v.MustField("x").mustNode())),
		"m21": m.MustField("m21"), "m22": m.MustField("m22"),
		"m23": exprNum(t.gen.PureInstr(resource.RuntimeFunctionAdd, m.MustField("m23").mustNode(), v.MustField("y").mustNode())),
		"m31": m.MustField("m31"), "m32": m.MustField("m32"), "m33": m.MustField("m33"),
	}), nil
}

func transScale(t *tracer, m Num, args []Num) (Num, error) {
	v := args[0]
	return compNum(map[string]Num{
		"m11": exprNum(t.gen.PureInstr(resource.RuntimeFunctionMultiply, m.MustField("m11").mustNode(), v.MustField("x").mustNode())),
		"m12": exprNum(t.gen.PureInstr(resource.RuntimeFunctionMultiply, m.MustField("m12").mustNode(), v.MustField("x").mustNode())),
		"m13": exprNum(t.gen.PureInstr(resource.RuntimeFunctionMultiply, m.MustField("m13").mustNode(), v.MustField("x").mustNode())),
		"m21": exprNum(t.gen.PureInstr(resource.RuntimeFunctionMultiply, m.MustField("m21").mustNode(), v.MustField("y").mustNode())),
		"m22": exprNum(t.gen.PureInstr(resource.RuntimeFunctionMultiply, m.MustField("m22").mustNode(), v.MustField("y").mustNode())),
		"m23": exprNum(t.gen.PureInstr(resource.RuntimeFunctionMultiply, m.MustField("m23").mustNode(), v.MustField("y").mustNode())),
		"m31": m.MustField("m31"), "m32": m.MustField("m32"), "m33": m.MustField("m33"),
	}), nil
}

func transRotate(t *tracer, m Num, args []Num) (Num, error) {
	a := args[0]
	cos := exprNum(t.gen.PureInstr(resource.RuntimeFunctionCos, a.mustNode()))
	sin := exprNum(t.gen.PureInstr(resource.RuntimeFunctionSin, a.mustNode()))
	return compNum(map[string]Num{
		"m11": cos, "m12": exprNum(t.gen.PureInstr(resource.RuntimeFunctionNegate, sin.mustNode())), "m13": m.MustField("m13"),
		"m21": sin, "m22": cos, "m23": m.MustField("m23"),
		"m31": m.MustField("m31"), "m32": m.MustField("m32"), "m33": m.MustField("m33"),
	}), nil
}

// transTransformVec applies a Transform2d to a Vec2: (x*m11 + y*m12 + m13, x*m21 + y*m22 + m23).
func transTransformVec(t *tracer, m Num, args []Num) (Num, error) {
	v := args[0]
	x := v.MustField("x").mustNode()
	y := v.MustField("y").mustNode()
	add := func(a, b ir.Node) ir.Node { return t.gen.PureInstr(resource.RuntimeFunctionAdd, a, b) }
	mul := func(a, b ir.Node) ir.Node { return t.gen.PureInstr(resource.RuntimeFunctionMultiply, a, b) }
	nx := add(add(mul(m.MustField("m11").mustNode(), x), mul(m.MustField("m12").mustNode(), y)), m.MustField("m13").mustNode())
	ny := add(add(mul(m.MustField("m21").mustNode(), x), mul(m.MustField("m22").mustNode(), y)), m.MustField("m23").mustNode())
	return compNum(map[string]Num{"x": exprNum(nx), "y": exprNum(ny)}), nil
}
