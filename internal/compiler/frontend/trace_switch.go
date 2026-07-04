package frontend

import (
	"go/ast"
	"go/token"

	"github.com/WindowsSov8forUs/sonolus-go/internal/compiler/ir"
)

func (t *tracer) switchStmt(n *ast.SwitchStmt) error {
	if n.Init != nil {
		if err := t.stmt(n.Init); err != nil {
			return err
		}
	}

	// Evaluate the tag expression (or treat as untagged-switch with tautology).
	var tag Num
	if n.Tag != nil {
		v, err := t.expr(n.Tag)
		if err != nil {
			return err
		}
		tag = v
	}

	merge := ir.NewBlock()
	var defaultBody []ast.Stmt

	// Save and reset fallthrough state for nested switches.
	savedFallthrough := t.fallthroughTarget
	t.fallthroughTarget = nil

	for _, clause := range n.Body.List {
		cc, ok := clause.(*ast.CaseClause)
		if !ok {
			continue
		}

		if cc.List == nil {
			// Defer default case — process after all other cases so that
			// it is connected from the last non-default case's nextBlock.
			defaultBody = cc.Body
			continue
		}

		// Evaluate every case expression once, then build the condition.
		caseValues := make([]Num, len(cc.List))
		for j, expr := range cc.List {
			cv, err := t.expr(expr)
			if err != nil {
				return err
			}
			caseValues[j] = cv
		}

		var cond Num
		if n.Tag == nil {
			// Untagged: expressions are boolean conditions directly.
			cond = caseValues[0]
			for _, cv := range caseValues[1:] {
				var ok bool
				cond, ok = applyBinary(t.gen, token.LOR, cond, cv)
				if !ok {
					return t.errf(cc, "unsupported || in switch case (conditions in untagged switches must be simple boolean expressions)")
				}
			}
		} else {
			// Tagged: build tag == caseVal for each.
			eq2, ok := applyBinary(t.gen, token.EQL, tag, caseValues[0])
			if !ok {
				return t.errf(cc, "unsupported == in switch case")
			}
			cond = eq2
			for _, cv := range caseValues[1:] {
				eq2, ok = applyBinary(t.gen, token.EQL, tag, cv)
				if !ok {
					return t.errf(cc, "unsupported == in switch case (switch case values must be comparable scalar expressions)")
				}
				cond, ok = applyBinary(t.gen, token.LOR, cond, eq2)
				if !ok {
					return t.errf(cc, "unsupported || in switch case (tagged switch cases only support combining multiple == checks with ,)")
				}
			}
		}

		// Pick up the fallthrough target from the previous case (if any).
		// When a case body ends with fallthrough, it creates a target block
		// that the next case should use as its body, bypassing condition check.
		caseBlock := t.fallthroughTarget
		if caseBlock == nil {
			caseBlock = ir.NewBlock()
		}

		if cond.isConst {
			if cond.c != 0 {
				// This case is always true: execute body.
				t.current.ConnectTo(caseBlock, nil)
				t.fallthroughTarget = nil
				t.enter(caseBlock)
				if err := t.stmtList(cc.Body); err != nil {
					return err
				}
				t.fallthroughTo(merge)
				if t.fallthroughTarget == nil {
					// No fallthrough — remaining cases are unreachable.
					break
				}
				// Body ended with fallthrough. Advance t.current to a dummy
				// block so the next case can set up its condition without
				// clobbering the current body block.
				t.enter(ir.NewBlock())
				continue
			}
			// Constant false: skip this case's condition check —
			// but the body may still be reachable via fallthrough.
			if t.fallthroughTarget != nil {
				// Reached via fallthrough from the previous case.
				// caseBlock already equals t.fallthroughTarget (set above).
				t.fallthroughTarget = nil
				t.enter(caseBlock)
				if err := t.stmtList(cc.Body); err != nil {
					return err
				}
				t.fallthroughTo(merge)
				if t.fallthroughTarget == nil {
					// No fallthrough — remaining cases unreachable.
					break
				}
				t.enter(ir.NewBlock())
				continue
			}
			continue
		}

		// Non-constant: generate Branch.
		t.fallthroughTarget = nil
		nextBlock := ir.NewBlock()
		t.current.Test = cond.mustNode()
		t.current.ConnectTo(nextBlock, ir.Cond(0)) // false → next case
		t.current.ConnectTo(caseBlock, nil)        // true → this case

		t.enter(caseBlock)
		if err := t.stmtList(cc.Body); err != nil {
			return err
		}
		t.fallthroughTo(merge)

		t.enter(nextBlock)
	}

	// Process default case after all non-default cases so that t.current
	// (the last nextBlock) correctly falls through to the default body.
	if defaultBody != nil {
		defaultBlock := t.fallthroughTarget
		if defaultBlock == nil {
			defaultBlock = ir.NewBlock()
		}
		t.fallthroughTarget = nil
		t.fallthroughTo(defaultBlock)
		t.enter(defaultBlock)
		if err := t.stmtList(defaultBody); err != nil {
			return err
		}
		t.fallthroughTo(merge)
	}

	t.fallthroughTarget = savedFallthrough
	t.enter(merge)
	t.terminated = len(merge.Incoming) == 0
	return nil
}
