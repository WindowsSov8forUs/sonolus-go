package frontend

import (
	"go/ast"
	"go/token"
	"strconv"

	"github.com/WindowsSov8forUs/sonolus-core-go/core/resource"

	"github.com/WindowsSov8forUs/sonolus-go/internal/compiler/ir"
)

// ── If / For / Return / Branch ────────────────────────────────────────────────

func (t *tracer) ifStmt(n *ast.IfStmt) error {
	if n.Init != nil {
		if err := t.stmt(n.Init); err != nil {
			return err
		}
	}
	// Short-circuit logical operators: generate CFG branches instead of a single
	// expression node. `a && b` → test a, true→test b, false→falseTarget.
	// `a || b` → test a, true→thenBlock, false→test b.
	if bin, ok := n.Cond.(*ast.BinaryExpr); ok && (bin.Op == token.LAND || bin.Op == token.LOR) {
		return t.shortCircuitIf(n, bin)
	}

	cond, err := t.expr(n.Cond)
	if err != nil {
		return err
	}

	// A compile-time-constant condition is resolved here: only the taken branch
	// is traced (mirrors sonolus.py visit_If's _is_py_ handling). This avoids
	// emitting — or even tracing — the dead branch.
	if cond.isConst {
		if cond.c != 0 {
			return t.stmtList(n.Body.List)
		}
		return t.dispatchElse(n.Else)
	}

	condBlock := t.current
	condBlock.Test = cond.mustNode()

	thenBlock := ir.NewBlock()
	merge := ir.NewBlock()
	var elseBlock *ir.BasicBlock
	if n.Else != nil {
		elseBlock = ir.NewBlock()
	}

	falseTarget := merge
	if elseBlock != nil {
		falseTarget = elseBlock
	}
	condBlock.ConnectTo(falseTarget, ir.Cond(0)) // false branch
	condBlock.ConnectTo(thenBlock, nil)          // true branch

	t.enter(thenBlock)
	if err := t.stmtList(n.Body.List); err != nil {
		return err
	}
	t.fallthroughTo(merge)

	if err := t.traceElseBranch(n.Else, elseBlock, merge); err != nil {
		return err
	}

	// If neither branch reaches the merge, code after the if is unreachable.
	t.enter(merge)
	t.terminated = len(merge.Incoming) == 0
	return nil
}

// forStmt lowers Go's for in its three shapes: for{}, for cond {}, and
// for init; cond; post {}. break/continue jump to the loop exit / latch.
func (t *tracer) forStmt(n *ast.ForStmt) error {
	if n.Init != nil {
		if err := t.stmt(n.Init); err != nil {
			return err
		}
	}

	header := ir.NewBlock()
	body := ir.NewBlock()
	exit := ir.NewBlock()

	// The latch is where the back edge and `continue` go; with a post statement
	// it is a distinct block that runs post then loops to the header.
	latch := header
	if n.Post != nil {
		latch = ir.NewBlock()
	}

	t.fallthroughTo(header)
	t.enter(header)
	if n.Cond != nil {
		cond, err := t.expr(n.Cond)
		if err != nil {
			return err
		}
		header.Test = cond.mustNode()
		header.ConnectTo(exit, ir.Cond(0)) // false -> exit
		header.ConnectTo(body, nil)        // true  -> body
	} else {
		header.ConnectTo(body, nil)
	}

	t.loops = append(t.loops, loopCtx{breakTo: exit, continueTo: latch})
	t.enter(body)
	if err := t.stmtList(n.Body.List); err != nil {
		return err
	}
	t.fallthroughTo(latch)
	t.loops = t.loops[:len(t.loops)-1]

	if n.Post != nil {
		t.enter(latch)
		if err := t.stmt(n.Post); err != nil {
			return err
		}
		t.current.ConnectTo(header, nil)
	}

	// If nothing breaks out of the loop, code after it is unreachable.
	t.enter(exit)
	t.terminated = len(exit.Incoming) == 0
	return nil
}

func (t *tracer) returnStmt(n *ast.ReturnStmt) error {
	if len(t.returns) == 0 {
		return t.errf(n, "return outside of a function")
	}
	rc := &t.returns[len(t.returns)-1]

	// Multi-return: evaluate all results into vals slice.
	var vals []Num
	for _, r := range n.Results {
		v, err := t.expr(r)
		if err != nil {
			return err
		}
		vals = append(vals, v)
	}

	if rc.target == nil {
		// Callback: only single scalar return is valid (callback yields a float
		// via Break). Composite / multi returns are rejected at the callback level.
		if len(vals) == 0 {
			// void return — no break necessary
		} else if len(vals) == 1 {
			v := vals[0]
			if v.IsComposite() {
				return t.errf(n, "cannot return a composite value from a callback; return individual fields instead")
			}
			t.emit(t.gen.ImpureInstr(resource.RuntimeFunctionBreak, v.mustNode(), ir.Const(1)))
		} else {
			return t.errf(n, "cannot return multiple values from a callback; callbacks yield a single float")
		}
	} else {
		// Inlined function: write return values to consecutive slots in retTemp.
		// Expand rc.temp size if needed.
		if len(vals) > rc.temp.Size {
			rc.temp.Size = len(vals)
		}
		if len(vals) == 1 && vals[0].IsComposite() {
			// Single composite return — use the composite's own field names.
			order, err := vals[0].CompositeFieldOrder()
			if err != nil {
				return err
			}
			rc.compositeFields = order
			for i, f := range rc.compositeFields {
				t.emit(t.gen.SetPlace(ir.BlockPlace{Block: rc.temp, Index: ir.Const(i), Offset: 0}, vals[0].MustField(f).mustNode()))
			}
		} else if len(vals) == 1 {
			// Single scalar return.
			t.emit(t.gen.SetPlace(ir.TempCell(rc.temp), vals[0].mustNode()))
		} else if len(vals) > 1 {
			// Multi-return: auto-generate field names _0, _1, ...
			fields := make([]string, len(vals))
			for i := range vals {
				fields[i] = "_" + strconv.Itoa(i)
			}
			rc.compositeFields = fields
			for i, v := range vals {
				t.emit(t.gen.SetPlace(ir.BlockPlace{Block: rc.temp, Index: ir.Const(i), Offset: 0}, v.mustNode()))
			}
		}
		t.current.ConnectTo(rc.target, nil)
	}
	t.terminated = true
	return nil
}

func (t *tracer) branch(n *ast.BranchStmt) error {
	if len(t.loops) == 0 {
		return t.errf(n, "%s outside of a loop", n.Tok)
	}
	loop := t.loops[len(t.loops)-1]
	switch n.Tok {
	case token.BREAK:
		t.current.ConnectTo(loop.breakTo, nil)
	case token.CONTINUE:
		t.current.ConnectTo(loop.continueTo, nil)
	default:
		return t.errf(n, "unsupported branch statement %s (only break and continue are supported; no goto or fallthrough)", n.Tok)
	}
	// The rest of this block is unreachable; stmtList stops here.
	t.terminated = true
	return nil
}

// dispatchElse handles an else branch when the if-condition is a compile-time
// constant. The taken branch is executed inline and the dead branch is skipped
// entirely — no CFG edge is created.
func (t *tracer) dispatchElse(elseStmt ast.Stmt) error {
	if elseStmt == nil {
		return nil
	}
	switch e := elseStmt.(type) {
	case *ast.BlockStmt:
		return t.stmtList(e.List)
	case *ast.IfStmt:
		return t.ifStmt(e)
	default:
		return t.errf(elseStmt, "unsupported else %T (only block else {...} and else if {...} are supported)", elseStmt)
	}
}

// traceElseBranch traces the else branch of a runtime if-statement. elseBlock
// is the CFG block created by the caller (connected to the false edge of the
// condition). It is entered, the body is traced, and execution falls through to
// merge. When elseStmt is nil, there is no else branch and this is a no-op.
func (t *tracer) traceElseBranch(elseStmt ast.Stmt, elseBlock, merge *ir.BasicBlock) error {
	if elseStmt == nil {
		return nil
	}
	t.enter(elseBlock)
	switch e := elseStmt.(type) {
	case *ast.BlockStmt:
		if err := t.stmtList(e.List); err != nil {
			return err
		}
	case *ast.IfStmt:
		if err := t.ifStmt(e); err != nil {
			return err
		}
	default:
		return t.errf(elseStmt, "unsupported else %T (only block else {...} and else if {...} are supported)", elseStmt)
	}
	t.fallthroughTo(merge)
	return nil
}
