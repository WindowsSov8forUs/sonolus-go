package frontend

import (
	"github.com/WindowsSov8forUs/sonolus-core-go/core/resource"

	"github.com/WindowsSov8forUs/sonolus-go/compiler/ir"
)

var rectFields = []string{"t", "r", "b", "l"}

func rectW(t *tracer, r Num, args []Num) (Num, error) {
	return exprNum(t.gen.PureInstr(resource.RuntimeFunctionSubtract, r.MustField("r").mustNode(), r.MustField("l").mustNode())), nil
}

func rectH(t *tracer, r Num, args []Num) (Num, error) {
	return exprNum(t.gen.PureInstr(resource.RuntimeFunctionSubtract, r.MustField("t").mustNode(), r.MustField("b").mustNode())), nil
}

func rectCenter(t *tracer, r Num, args []Num) (Num, error) {
	return compNum(map[string]Num{
		"x": exprNum(t.gen.PureInstr(resource.RuntimeFunctionDivide,
			t.gen.PureInstr(resource.RuntimeFunctionAdd, r.MustField("l").mustNode(), r.MustField("r").mustNode()),
			ir.Const(2))),
		"y": exprNum(t.gen.PureInstr(resource.RuntimeFunctionDivide,
			t.gen.PureInstr(resource.RuntimeFunctionAdd, r.MustField("b").mustNode(), r.MustField("t").mustNode()),
			ir.Const(2))),
	}), nil
}

func rectTranslate(t *tracer, r Num, args []Num) (Num, error) {
	dx, dy := args[0], args[0]
	if len(args) > 1 {
		dy = args[1]
	}
	return compNum(map[string]Num{
		"t": exprNum(t.gen.PureInstr(resource.RuntimeFunctionAdd, r.MustField("t").mustNode(), dy.mustNode())),
		"r": exprNum(t.gen.PureInstr(resource.RuntimeFunctionAdd, r.MustField("r").mustNode(), dx.mustNode())),
		"b": exprNum(t.gen.PureInstr(resource.RuntimeFunctionAdd, r.MustField("b").mustNode(), dy.mustNode())),
		"l": exprNum(t.gen.PureInstr(resource.RuntimeFunctionAdd, r.MustField("l").mustNode(), dx.mustNode())),
	}), nil
}

func rectScale(t *tracer, r Num, args []Num) (Num, error) {
	sx, sy := args[0], args[0]
	if len(args) > 1 {
		sy = args[1]
	}
	return compNum(map[string]Num{
		"t": exprNum(t.gen.PureInstr(resource.RuntimeFunctionMultiply, r.MustField("t").mustNode(), sy.mustNode())),
		"r": exprNum(t.gen.PureInstr(resource.RuntimeFunctionMultiply, r.MustField("r").mustNode(), sx.mustNode())),
		"b": exprNum(t.gen.PureInstr(resource.RuntimeFunctionMultiply, r.MustField("b").mustNode(), sy.mustNode())),
		"l": exprNum(t.gen.PureInstr(resource.RuntimeFunctionMultiply, r.MustField("l").mustNode(), sx.mustNode())),
	}), nil
}
