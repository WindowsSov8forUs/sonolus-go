package frontend

import (
	"go/ast"

	"github.com/WindowsSov8forUs/sonolus-core-go/core/resource"

	"github.com/WindowsSov8forUs/sonolus-go/internal/compiler/ir"
)

func (t *tracer) touchField(n *ast.CallExpr, args []Num, fieldOffset int) (Num, error) {
	if len(args) != 1 {
		return Num{}, t.errf(n, "touch field expects 1 argument (touch index)")
	}
	const touchBlock, touchStride = ir.BlockRuntimeTouch, ir.TouchFieldStride
	index := t.gen.PureInstr(resource.RuntimeFunctionAdd,
		t.gen.PureInstr(resource.RuntimeFunctionMultiply, args[0].mustNode(), ir.Const(touchStride)),
		ir.Const(fieldOffset))
	return exprNum(ir.GetPlace(ir.NewBlockPlace(ir.Const(touchBlock), index, 0))), nil
}

func (t *tracer) screenFunc(n *ast.CallExpr) (Num, error) {
	if len(n.Args) != 0 {
		return Num{}, t.errf(n, "screen() takes no arguments")
	}
	ar := exprNum(ir.GetPlace(ir.Cell(ir.BlockRuntimeEnvironment, 1)))
	negAr := exprNum(t.gen.PureInstr(resource.RuntimeFunctionNegate, ar.mustNode()))
	return compNumTyped("rect", map[string]Num{"t": constNum(1), "r": ar, "b": constNum(-1), "l": negAr}), nil
}

func (t *tracer) safeAreaFunc(n *ast.CallExpr) (Num, error) {
	if len(n.Args) != 0 {
		return Num{}, t.errf(n, "safeArea() takes no arguments")
	}
	return compNumTyped("rect", map[string]Num{
		"t": exprNum(ir.GetPlace(ir.Cell(ir.BlockRuntimeEnvironment, 8))),
		"r": exprNum(ir.GetPlace(ir.Cell(ir.BlockRuntimeEnvironment, 6))),
		"b": exprNum(ir.GetPlace(ir.Cell(ir.BlockRuntimeEnvironment, 7))),
		"l": exprNum(ir.GetPlace(ir.Cell(ir.BlockRuntimeEnvironment, 5))),
	}), nil
}

// pnpolyFunc emits a point-in-quad test: 1 if point is inside the convex quad.
func (t *tracer) pnpolyFunc(n *ast.CallExpr, args []Num) (Num, error) {
	if len(args) != 2 {
		return Num{}, t.errf(n, "pnpoly expects (point, quad)")
	}
	// Delegate to the shared quadContains in value.go.
	return quadContains(t, args[1], args[0:1])
}

func (t *tracer) offsetAdjustedTimeFunc(n *ast.CallExpr) (Num, error) {
	if len(n.Args) != 0 {
		return Num{}, t.errf(n, "offsetAdjustedTime() takes no arguments")
	}
	tm := exprNum(ir.GetPlace(ir.Cell(ir.BlockRuntimeEnvironment, 0)))
	ao := exprNum(ir.GetPlace(ir.Cell(ir.BlockRuntimeEnvironment, 2)))
	return exprNum(t.gen.PureInstr(resource.RuntimeFunctionSubtract, tm.mustNode(), ao.mustNode())), nil
}

func (t *tracer) prevTimeFunc(n *ast.CallExpr) (Num, error) {
	if len(n.Args) != 0 {
		return Num{}, t.errf(n, "prevTime() takes no arguments")
	}
	tm := exprNum(ir.GetPlace(ir.Cell(ir.BlockRuntimeEnvironment, 0)))
	dt := exprNum(ir.GetPlace(ir.Cell(ir.BlockRuntimeEnvironment, 1)))
	return exprNum(t.gen.PureInstr(resource.RuntimeFunctionSubtract, tm.mustNode(), dt.mustNode())), nil
}

func (t *tracer) perspectiveApproachFunc(n *ast.CallExpr, args []Num) (Num, error) {
	if len(args) != 2 {
		return Num{}, t.errf(n, "perspectiveApproach expects (x, y)")
	}
	// perspectiveApproach(x, y) = 1 / (1 - x * y)
	denom := t.gen.PureInstr(resource.RuntimeFunctionSubtract,
		ir.Const(1),
		t.gen.PureInstr(resource.RuntimeFunctionMultiply, args[0].mustNode(), args[1].mustNode()))
	return exprNum(t.gen.PureInstr(resource.RuntimeFunctionDivide, ir.Const(1), denom)), nil
}
