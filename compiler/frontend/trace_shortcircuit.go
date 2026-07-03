package frontend

import (
	"go/ast"
	"go/token"

	"github.com/WindowsSov8forUs/sonolus-go/compiler/ir"
)

// ── Short-circuit logical operators ───────────────────────────────────────────

func (t *tracer) shortCircuitIf(n *ast.IfStmt, bin *ast.BinaryExpr) error {
	if bin.Op == token.LAND {
		return t.shortCircuitAnd(n, bin)
	}
	return t.shortCircuitOr(n, bin)
}

// shortCircuitAnd lowers `left && right` into CFG edges:
//
//	const false && _ → skip to else/merge
//	const true  && right → test right
//	dyn && right → test left, false→else, true→test right
func (t *tracer) shortCircuitAnd(n *ast.IfStmt, bin *ast.BinaryExpr) error {
	thenBlock := ir.NewBlock()
	merge := ir.NewBlock()
	falseTarget, elseBlock := t.setupShortCircuit(n, merge)

	left, err := t.expr(bin.X)
	if err != nil {
		return err
	}
	if left.isConst {
		if left.c == 0 {
			return t.dispatchElse(n.Else)
		}
		right, err := t.expr(bin.Y)
		if err != nil {
			return err
		}
		t.current.Test = right.mustNode()
		t.current.ConnectTo(falseTarget, ir.Cond(0))
		t.current.ConnectTo(thenBlock, nil)
	} else {
		t.current.Test = left.mustNode()
		rightBlock := ir.NewBlock()
		t.current.ConnectTo(falseTarget, ir.Cond(0))
		t.current.ConnectTo(rightBlock, nil)
		t.enter(rightBlock)
		right, err := t.expr(bin.Y)
		if err != nil {
			return err
		}
		rightBlock.Test = right.mustNode()
		rightBlock.ConnectTo(falseTarget, ir.Cond(0))
		rightBlock.ConnectTo(thenBlock, nil)
	}

	return t.finishShortCircuit(n, thenBlock, merge, elseBlock)
}

// shortCircuitOr lowers `left || right` into CFG edges:
//
//	const truthy || _ → execute body, jump to merge
//	const falsy  || right → test right
//	dyn || right → test left, true→body, false→test right
func (t *tracer) shortCircuitOr(n *ast.IfStmt, bin *ast.BinaryExpr) error {
	thenBlock := ir.NewBlock()
	merge := ir.NewBlock()
	falseTarget, elseBlock := t.setupShortCircuit(n, merge)

	left, err := t.expr(bin.X)
	if err != nil {
		return err
	}
	if left.isConst {
		if left.c != 0 {
			if err := t.stmtList(n.Body.List); err != nil {
				return err
			}
			t.fallthroughTo(merge)
			t.enter(merge)
			return nil
		}
		right, err := t.expr(bin.Y)
		if err != nil {
			return err
		}
		t.current.Test = right.mustNode()
		t.current.ConnectTo(falseTarget, ir.Cond(0))
		t.current.ConnectTo(thenBlock, nil)
	} else {
		t.current.Test = left.mustNode()
		rightBlock := ir.NewBlock()
		t.current.ConnectTo(thenBlock, nil)
		t.current.ConnectTo(rightBlock, ir.Cond(0))
		t.enter(rightBlock)
		right, err := t.expr(bin.Y)
		if err != nil {
			return err
		}
		rightBlock.Test = right.mustNode()
		rightBlock.ConnectTo(falseTarget, ir.Cond(0))
		rightBlock.ConnectTo(thenBlock, nil)
	}

	return t.finishShortCircuit(n, thenBlock, merge, elseBlock)
}

// setupShortCircuit creates the blocks shared by short-circuit lowering and
// returns the false-branch target (elseBlock if present, otherwise merge).
func (t *tracer) setupShortCircuit(n *ast.IfStmt, merge *ir.BasicBlock) (falseTarget *ir.BasicBlock, elseBlock *ir.BasicBlock) {
	if n.Else != nil {
		elseBlock = ir.NewBlock()
		falseTarget = elseBlock
	} else {
		falseTarget = merge
	}
	return
}

// finishShortCircuit traces the then/else bodies and rejoins at merge.
func (t *tracer) finishShortCircuit(n *ast.IfStmt, thenBlock, merge, elseBlock *ir.BasicBlock) error {
	t.enter(thenBlock)
	if err := t.stmtList(n.Body.List); err != nil {
		return err
	}
	t.fallthroughTo(merge)

	if err := t.traceElseBranch(n.Else, elseBlock, merge); err != nil {
		return err
	}

	t.enter(merge)
	t.terminated = len(merge.Incoming) == 0
	return nil
}
