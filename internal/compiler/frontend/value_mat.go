package frontend

import (
	"github.com/WindowsSov8forUs/sonolus-core-go/core/resource"

	"github.com/WindowsSov8forUs/sonolus-go/internal/compiler/ir"
)

// matFields is the field layout of a 3x2 affine matrix.
var matFields = []string{"m11", "m12", "m13", "m21", "m22", "m23"}

func matScale(t *tracer, m Num, args []Num) (Num, error) {
	sx, sy := args[0], args[0]
	if len(args) > 1 {
		sy = args[1]
	}
	return compNum(map[string]Num{
		"m11": exprNum(t.gen.PureInstr(resource.RuntimeFunctionMultiply, m.MustField("m11").mustNode(), sx.mustNode())),
		"m12": exprNum(t.gen.PureInstr(resource.RuntimeFunctionMultiply, m.MustField("m12").mustNode(), sx.mustNode())),
		"m13": exprNum(t.gen.PureInstr(resource.RuntimeFunctionMultiply, m.MustField("m13").mustNode(), sx.mustNode())),
		"m21": exprNum(t.gen.PureInstr(resource.RuntimeFunctionMultiply, m.MustField("m21").mustNode(), sy.mustNode())),
		"m22": exprNum(t.gen.PureInstr(resource.RuntimeFunctionMultiply, m.MustField("m22").mustNode(), sy.mustNode())),
		"m23": exprNum(t.gen.PureInstr(resource.RuntimeFunctionMultiply, m.MustField("m23").mustNode(), sy.mustNode())),
	}), nil
}

func matTranslate(t *tracer, m Num, args []Num) (Num, error) {
	tx, ty := args[0], args[0]
	if len(args) > 1 {
		ty = args[1]
	}
	return compNum(map[string]Num{
		"m11": m.MustField("m11"), "m12": m.MustField("m12"),
		"m13": exprNum(t.gen.PureInstr(resource.RuntimeFunctionAdd, m.MustField("m13").mustNode(), tx.mustNode())),
		"m21": m.MustField("m21"), "m22": m.MustField("m22"),
		"m23": exprNum(t.gen.PureInstr(resource.RuntimeFunctionAdd, m.MustField("m23").mustNode(), ty.mustNode())),
	}), nil
}

// matCompose composes two 3×2 affine matrices: this * other.
// A 3×2 matrix represents [r0x r0y | tx; r1x r1y | ty] with an implicit
// [0 0 1] bottom row.
func matCompose(t *tracer, m Num, args []Num) (Num, error) {
	n := args[0]
	dot2 := func(r0, r1 string, c0, c1 string) ir.Node {
		return t.gen.PureInstr(resource.RuntimeFunctionAdd,
			t.gen.PureInstr(resource.RuntimeFunctionMultiply, m.MustField(r0).mustNode(), n.MustField(c0).mustNode()),
			t.gen.PureInstr(resource.RuntimeFunctionMultiply, m.MustField(r1).mustNode(), n.MustField(c1).mustNode()))
	}
	return compNum(map[string]Num{
		"m11": exprNum(dot2("m11", "m12", "m11", "m21")),
		"m12": exprNum(dot2("m11", "m12", "m12", "m22")),
		"m13": exprNum(t.addNode(dot2("m11", "m12", "m13", "m23"), m.MustField("m13").mustNode())),
		"m21": exprNum(dot2("m21", "m22", "m11", "m21")),
		"m22": exprNum(dot2("m21", "m22", "m12", "m22")),
		"m23": exprNum(t.addNode(dot2("m21", "m22", "m13", "m23"), m.MustField("m23").mustNode())),
	}), nil
}

// matRotate composes a rotation onto the receiver: this * rotate(angle).
func matRotate(t *tracer, m Num, args []Num) (Num, error) {
	a := args[0]
	cos := exprNum(t.gen.PureInstr(resource.RuntimeFunctionCos, a.mustNode()))
	sin := exprNum(t.gen.PureInstr(resource.RuntimeFunctionSin, a.mustNode()))
	negSin := exprNum(t.gen.PureInstr(resource.RuntimeFunctionNegate, sin.mustNode()))
	return compNum(map[string]Num{
		"m11": exprNum(t.addNode(t.mulNode(m.MustField("m11").mustNode(), cos.mustNode()), t.mulNode(m.MustField("m12").mustNode(), sin.mustNode()))),
		"m12": exprNum(t.addNode(t.mulNode(m.MustField("m11").mustNode(), negSin.mustNode()), t.mulNode(m.MustField("m12").mustNode(), cos.mustNode()))),
		"m13": m.MustField("m13"),
		"m21": exprNum(t.addNode(t.mulNode(m.MustField("m21").mustNode(), cos.mustNode()), t.mulNode(m.MustField("m22").mustNode(), sin.mustNode()))),
		"m22": exprNum(t.addNode(t.mulNode(m.MustField("m21").mustNode(), negSin.mustNode()), t.mulNode(m.MustField("m22").mustNode(), cos.mustNode()))),
		"m23": m.MustField("m23"),
	}), nil
}
