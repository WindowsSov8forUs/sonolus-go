package optimize

import (
	"testing"

	"github.com/WindowsSov8forUs/sonolus-core-go/core/resource"

	"github.com/WindowsSov8forUs/sonolus-go/internal/compiler/ir"
)

// TestCopyCoalesceCrossBlock verifies that CopyCoalesce can fold copies that
// span multiple basic blocks. Two predecessor blocks write the same constant
// value to the same target, which should be merged.
func TestCopyCoalesceCrossBlock(t *testing.T) {
	t1 := ir.NewTemp("x.1")
	t2 := ir.NewTemp("y.2")

	e, predA, predB, merge := ir.NewBlock(), ir.NewBlock(), ir.NewBlock(), ir.NewBlock()

	// Both predecessors write 42 into t2 via a copy of t1.
	predA.Statements = []ir.Node{
		testGen.SetPlace(ir.TempCell(t1), ir.Const(42)),
		testGen.SetPlace(ir.TempCell(t2), ir.GetPlace(ir.TempCell(t1))),
	}
	predB.Statements = []ir.Node{
		testGen.SetPlace(ir.TempCell(t1), ir.Const(42)),
		testGen.SetPlace(ir.TempCell(t2), ir.GetPlace(ir.TempCell(t1))),
	}
	merge.Statements = []ir.Node{
		testGen.SetPlace(ir.Cell(0, 0), ir.GetPlace(ir.TempCell(t2))),
	}

	e.ConnectTo(predA, nil)
	e.ConnectTo(predB, nil)
	predA.ConnectTo(merge, nil)
	predB.ConnectTo(merge, nil)

	CopyCoalesce{}.Run(testGen, e)

	// After coalesce, merge block's use of t2 should reference t1.
	lastSet := merge.Statements[0].(ir.Set)
	lastGet := lastSet.Value.(ir.Get)
	bp := lastGet.Place.(ir.BlockPlace)
	if bp.Block != t1 {
		t.Errorf("expected t1 in merge block, got %p", bp.Block)
	}
}

// TestCopyCoalesceRedundantCopyChain verifies a three-element copy chain
// (a=5, b=a, c=b) collapses so that c references the original value directly.
func TestCopyCoalesceRedundantCopyChain(t *testing.T) {
	a := ir.NewTemp("a")
	b := ir.NewTemp("b")
	c := ir.NewTemp("c")

	e := ir.NewBlock()
	e.Statements = []ir.Node{
		testGen.SetPlace(ir.TempCell(a), ir.Const(5)),
		testGen.SetPlace(ir.TempCell(b), ir.GetPlace(ir.TempCell(a))),
		testGen.SetPlace(ir.TempCell(c), ir.GetPlace(ir.TempCell(b))),
		testGen.SetPlace(ir.Cell(0, 0), ir.GetPlace(ir.TempCell(c))),
	}

	CopyCoalesce{}.Run(testGen, e)

	// After coalesce: statements should be reduced.
	// The use of c should fold back to a.
	lastSet := e.Statements[len(e.Statements)-1].(ir.Set)
	lastGet := lastSet.Value.(ir.Get)
	bp := lastGet.Place.(ir.BlockPlace)
	if bp.Block != a {
		t.Errorf("expected 'a' in use site after coalesce chain, got %p", bp.Block)
	}
	t.Logf("statements after coalesce chain: %d", len(e.Statements))
}

// TestCopyCoalescePreservesImpure ensures that side-effecting instructions are
// not coalesced through. An impure Instr (Draw) between a copy definition and
// its use must not be folded.
func TestCopyCoalescePreservesImpure(t *testing.T) {
	a := ir.NewTemp("a")
	b := ir.NewTemp("b")

	e := ir.NewBlock()
	e.Statements = []ir.Node{
		testGen.SetPlace(ir.TempCell(a), ir.Const(5)),
		testGen.SetPlace(ir.TempCell(b), ir.GetPlace(ir.TempCell(a))),  // copy
		testGen.ImpureInstr(resource.RuntimeFunctionDraw, ir.Const(1)), // side effect
		testGen.SetPlace(ir.Cell(0, 0), ir.GetPlace(ir.TempCell(b))),   // use after side effect
	}

	CopyCoalesce{}.Run(testGen, e)

	// The impure Draw instruction must still be present.
	foundDraw := false
	for _, s := range e.Statements {
		if instr, ok := s.(ir.Instr); ok && instr.Op == resource.RuntimeFunctionDraw {
			foundDraw = true
			break
		}
	}
	if !foundDraw {
		t.Error("impure Draw was incorrectly removed by CopyCoalesce")
	}
}
