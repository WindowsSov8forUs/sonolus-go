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
					return t.errf(cc, "unsupported || in switch case")
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
					return t.errf(cc, "unsupported == in switch case")
				}
				cond, ok = applyBinary(t.gen, token.LOR, cond, eq2)
				if !ok {
					return t.errf(cc, "unsupported || in switch case")
				}
			}
		}

		caseBlock := ir.NewBlock()

		if cond.isConst {
			if cond.c != 0 {
				// This case is always true: execute body and skip everything after.
				t.enter(caseBlock)
				if err := t.stmtList(cc.Body); err != nil {
					return err
				}
				t.fallthroughTo(merge)
				break
			}
			// Constant false: skip this case entirely.
			continue
		}

		// Non-constant: generate Branch.
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
		defaultBlock := ir.NewBlock()
		t.fallthroughTo(defaultBlock)
		t.enter(defaultBlock)
		if err := t.stmtList(defaultBody); err != nil {
			return err
		}
		t.fallthroughTo(merge)
	}

	t.enter(merge)
	t.terminated = len(merge.Incoming) == 0
	return nil
}
