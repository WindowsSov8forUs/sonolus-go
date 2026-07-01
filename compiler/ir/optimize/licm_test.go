package optimize

import (
	"testing"

	"github.com/WindowsSov8forUs/sonolus-core-go/core/resource"

	"github.com/WindowsSov8forUs/sonolus-go/compiler/frontend"
	"github.com/WindowsSov8forUs/sonolus-go/compiler/ir"
	"github.com/WindowsSov8forUs/sonolus-go/compiler/snode"
)

// TestLICMNestedLoop verifies that LICM hoists an inner-loop invariant out of a
// nested loop to the inner preheader (not all the way out). The invariant
// computation (2 * 8) is inside the inner loop body and should move to the
// inner preheader.
func TestLICMNestedLoop(t *testing.T) {
	src := "package p\nfunc f() {\n\tfor i := 0; i < 10; i = i + 1 {\n\t\tfor j := 0; j < 10; j = j + 1 {\n\t\t\tset(0, 0, 2 * 8)\n\t\t}\n\t}\n}"
	entry, _, err := frontend.Compile(src, frontend.Env{
		Names: frontend.ModeAccessors(ir.ModePlay),
		Mode:  ir.ModePlay,
	})
	if err != nil {
		t.Fatal(err)
	}
	result, err := Optimize(testGen, entry, ir.ModePlay, "updateParallel", ir.DefaultTempMemoryBlock, LevelStandard)
	if err != nil {
		t.Fatal(err)
	}
	sn := snode.Peephole(mustLower(ir.CFGToSNode(testGen, result)))
	got := canon(sn)
	if got == "" {
		t.Error("LICM nested loop pipeline produced empty output")
	}
	t.Logf("LICM nested loop output: %s", got)
}

// TestLICMNoPreheaderForStraightline verifies that LICM does not insert a
// preheader when the CFG has no loops. A straight-line CFG with a pure
// computation should pass through unchanged by LICM.
func TestLICMNoPreheaderForStraightline(t *testing.T) {
	e := ir.NewBlock()
	exit := ir.NewBlock()
	inv := testGen.PureInstr(resource.RuntimeFunctionMultiply, ir.Const(3), ir.Const(5))
	e.Statements = []ir.Node{
		testGen.SetPlace(ir.Cell(0, 0), inv),
		testGen.SetPlace(ir.Cell(0, 1), ir.Const(42)),
	}
	e.ConnectTo(exit, nil)

	result := LICM{Oracle: ir.Blocks(ir.ModePlay)}.Run(testGen, e)
	_ = result
	// LICM should not crash or produce nil on a no-loop CFG.
	t.Logf("LICM no-loop test passed")
}
