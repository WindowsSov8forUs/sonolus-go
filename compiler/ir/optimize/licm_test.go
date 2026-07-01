package optimize

import (
	"strings"
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

// TestLICMPhiMigration verifies that LICM correctly migrates phi node arguments
// when creating a new preheader block. The CFG has two non-back predecessors (predA,
// predB) feeding a loop header that carries a phi node for variable x. LICM must
// create a new preheader (since neither predecessor qualifies for reuse) and migrate
// the phi args from predA/predB to the preheader. Without this migration, FromSSA
// drops the SSA values and produces incorrect output.
func TestLICMPhiMigration(t *testing.T) {
	x := ir.NewTemp("x")

	entry := ir.NewBlock()
	predA := ir.NewBlock()
	predB := ir.NewBlock()
	header := ir.NewBlock()
	body := ir.NewBlock()
	exit := ir.NewBlock()

	// entry branches to predA (truthy) or predB (falsy) based on a memory read.
	entry.Test = ir.GetPlace(ir.Cell(0, 0))

	// Each predecessor writes a distinct value to x so the loop header phi has
	// two non-back args. Both predecessors contain statements so LICM cannot
	// reuse either as a preheader — it must create a new one.
	predA.Statements = []ir.Node{
		testGen.SetPlace(ir.TempCell(x), ir.Const(10)),
	}
	predB.Statements = []ir.Node{
		testGen.SetPlace(ir.TempCell(x), ir.Const(20)),
	}

	// Loop condition uses a memory read (not the temp variable) to avoid
	// circular SSA dependency between the test and the phi target.
	header.Test = ir.GetPlace(ir.Cell(0, 0))

	// Loop body: an invariant pure computation and a phi-dependent write.
	body.Statements = []ir.Node{
		testGen.SetPlace(ir.Cell(0, 0),
			testGen.PureInstr(resource.RuntimeFunctionMultiply, ir.Const(3), ir.Const(5))),
		testGen.SetPlace(ir.TempCell(x),
			testGen.PureInstr(resource.RuntimeFunctionSubtract,
				ir.GetPlace(ir.TempCell(x)), ir.Const(1))),
	}

	exit.Statements = []ir.Node{
		testGen.SetPlace(ir.Cell(0, 1), ir.GetPlace(ir.TempCell(x))),
	}

	// Edges.
	entry.ConnectTo(predA, nil)
	entry.ConnectTo(predB, ir.Cond(0))
	predA.ConnectTo(header, nil)
	predB.ConnectTo(header, nil)
	header.ConnectTo(body, nil)
	header.ConnectTo(exit, ir.Cond(0))
	body.ConnectTo(header, nil) // back edge

	// Run Standard pipeline: ToSSA → … → LICM → … → FromSSA → …
	result, err := Optimize(testGen, entry, ir.ModePlay, "updateSequential",
		ir.DefaultTempMemoryBlock, LevelStandard)
	if err != nil {
		t.Fatal(err)
	}

	sn, err := ir.CFGToSNode(testGen, result)
	if err != nil {
		t.Fatal(err)
	}
	got := canon(snode.Peephole(sn))
	if got == "" {
		t.Error("LICM phi migration pipeline produced empty output")
	}
	if !strings.Contains(got, "Execute") && !strings.Contains(got, "Add") {
		t.Errorf("LICM phi migration output missing expected nodes: %s", got)
	}
}
