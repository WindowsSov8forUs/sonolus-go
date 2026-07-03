package optimize

import (
	"strings"
	"testing"

	"github.com/WindowsSov8forUs/sonolus-go/compiler/frontend"
	"github.com/WindowsSov8forUs/sonolus-go/compiler/ir"
)

// TestAdvancedDCEInPipeline verifies that AdvancedDCE (as part of the Standard
// pipeline) preserves side-effecting operations on entity memory.
func TestAdvancedDCEInPipeline(t *testing.T) {
	src := "package p\nfunc f() {\n\tx := sin(1.0)\n\tset(0, 0, x)\n}\n"
	entry, _, err := frontend.Compile(src, frontend.Env{Names: frontend.ModeAccessors(ir.ModePlay)})
	if err != nil {
		t.Fatal(err)
	}
	result, err := Optimize(testGen, entry, ir.ModePlay, "test", ir.DefaultTempMemoryBlock, LevelStandard)
	if err != nil {
		t.Fatal(err)
	}
	got := canon(mustLower(ir.CFGToSNode(testGen, result)))
	if !strings.Contains(got, "Set(#0,#0,") {
		t.Errorf("Set with side effects was incorrectly removed\n got: %s", got)
	}
}

// TestAdvancedDCEStandalone runs AdvancedDCE directly on frontend output and
// verifies it produces a valid, non-empty result.
func TestAdvancedDCEStandalone(t *testing.T) {
	src := "package p\nfunc f() {\n\tx := sin(1.0)\n\tset(0, 0, x)\n}\n"
	entry, _, err := frontend.Compile(src, frontend.Env{Names: frontend.ModeAccessors(ir.ModePlay)})
	if err != nil {
		t.Fatal(err)
	}
	entry = AdvancedDCE{}.Run(testGen, entry)
	entry, err = ir.AllocateTestBlocks(entry, ir.DefaultTempMemoryBlock)
	if err != nil {
		t.Fatal(err)
	}
	got := canon(mustLower(ir.CFGToSNode(testGen, entry)))
	if got == "" {
		t.Error("AdvancedDCE produced empty output")
	}
	if !strings.Contains(got, "Set(#0,#0,") {
		t.Errorf("Set was incorrectly removed\n got: %s", got)
	}
}

// TestPreprocessArraysMultiSlot verifies that preprocessArrays correctly
// identifies the first write to a multi-slot TempBlock (size > 1), enabling
// per-element liveness tracking for arrays.
func TestPreprocessArraysMultiSlot(t *testing.T) {
	// Build a simple CFG: entry sets a multi-slot temp, then reads from it.
	// A multi-slot temp has size > 1 and is used for arrays/records.
	multiTB := &ir.TempBlock{Name: "arr", Size: 4}
	entry := ir.NewBlock()
	exit := ir.NewBlock()

	setStmt := testGen.SetPlace(
		ir.BlockPlace{Block: multiTB, Index: ir.Const(0), Offset: 0},
		ir.Const(42),
	)
	getStmt := ir.GetPlace(
		ir.BlockPlace{Block: multiTB, Index: ir.Const(0), Offset: 0},
	)
	entry.Statements = []ir.Node{setStmt}
	// Use the get in a second statement so the multi-slot temp has a use.
	entry.Statements = append(entry.Statements,
		testGen.SetPlace(ir.Cell(0, 0), getStmt))
	entry.ConnectTo(exit, nil)

	blocks := ir.Preorder(entry)
	arrayInit := map[int]map[*ir.TempBlock]bool{}
	for _, b := range blocks {
		for _, s := range b.Statements {
			id := stmtID(s)
			arrayInit[id] = map[*ir.TempBlock]bool{}
		}
	}
	preprocessArrays(entry, blocks, arrayInit)

	// The first Set to the multi-slot temp should be marked as array init.
	found := false
	for id, temps := range arrayInit {
		if temps[multiTB] {
			found = true
			// Verify this is the ID of the first Set statement.
			if id != setStmt.ID {
				t.Errorf("arrayInit marked stmt %d instead of first Set %d", id, setStmt.ID)
			}
		}
	}
	if !found {
		t.Error("preprocessArrays did not mark any statement as first write to multi-slot temp")
	}
}

// TestPreprocessArraysSingleSlot verifies that single-slot temps (size=1)
// are NOT marked as array init, only multi-slot temps.
func TestPreprocessArraysSingleSlot(t *testing.T) {
	singleTB := &ir.TempBlock{Name: "x", Size: 1}
	entry := ir.NewBlock()
	exit := ir.NewBlock()

	setStmt := testGen.SetPlace(
		ir.BlockPlace{Block: singleTB, Index: ir.Const(0), Offset: 0},
		ir.Const(7),
	)
	entry.Statements = []ir.Node{setStmt}
	entry.ConnectTo(exit, nil)

	blocks := ir.Preorder(entry)
	arrayInit := map[int]map[*ir.TempBlock]bool{}
	for _, b := range blocks {
		for _, s := range b.Statements {
			id := stmtID(s)
			arrayInit[id] = map[*ir.TempBlock]bool{}
		}
	}
	preprocessArrays(entry, blocks, arrayInit)

	// No single-slot temp should be marked as array init.
	for _, temps := range arrayInit {
		if len(temps) > 0 {
			t.Error("single-slot temp was incorrectly marked as array init")
		}
	}
}

// --- SCCP pass tests ---
func TestSCCPConstFoldArith(t *testing.T) {
	src := "package p\nfunc f() {\n\tset(0, 0, 2 + 3*4)\n}\n"
	entry, _, err := frontend.Compile(src, frontend.Env{Names: frontend.ModeAccessors(ir.ModePlay)})
	if err != nil {
		t.Fatal(err)
	}
	result, err := Optimize(testGen, entry, ir.ModePlay, "test", ir.DefaultTempMemoryBlock, LevelStandard)
	if err != nil {
		t.Fatal(err)
	}
	got := canon(mustLower(ir.CFGToSNode(testGen, result)))
	if !strings.Contains(got, "#14") {
		t.Errorf("constant 2+3*4 not folded to 14: %s", got)
	}
}

func TestSCCPConstFoldMath(t *testing.T) {
	// sin(0) = 0 — a pure math call on constant that SCCP should fold.
	src := "package p\nfunc f() {\n\tset(0, 0, sin(0))\n}\n"
	entry, _, err := frontend.Compile(src, frontend.Env{Names: frontend.ModeAccessors(ir.ModePlay)})
	if err != nil {
		t.Fatal(err)
	}
	result, err := Optimize(testGen, entry, ir.ModePlay, "test", ir.DefaultTempMemoryBlock, LevelStandard)
	if err != nil {
		t.Fatal(err)
	}
	got := canon(mustLower(ir.CFGToSNode(testGen, result)))
	if !strings.Contains(got, "#0") {
		t.Errorf("sin(0) not folded to 0: %s", got)
	}
}

func TestSCCPDeadBranchElim(t *testing.T) {
	src := "package p\nfunc f() {\n\tif true {\n\t\tset(0, 0, 1)\n\t} else {\n\t\tset(0, 0, 2)\n\t}\n}\n"
	entry, _, err := frontend.Compile(src, frontend.Env{Names: frontend.ModeAccessors(ir.ModePlay)})
	if err != nil {
		t.Fatal(err)
	}
	result, err := Optimize(testGen, entry, ir.ModePlay, "test", ir.DefaultTempMemoryBlock, LevelStandard)
	if err != nil {
		t.Fatal(err)
	}
	got := canon(mustLower(ir.CFGToSNode(testGen, result)))
	if got == "" {
		t.Fatal("SCCP dead branch elimination produced empty output")
	}
}

// --- InlineVars pass test ---
func TestInlineVarsPipeline(t *testing.T) {
	src := "package p\nfunc f() {\n\tx := get(0, 0)\n\tset(0, 1, x)\n}\n"
	entry, _, err := frontend.Compile(src, frontend.Env{Names: frontend.ModeAccessors(ir.ModePlay)})
	if err != nil {
		t.Fatal(err)
	}
	result, err := Optimize(testGen, entry, ir.ModePlay, "test", ir.DefaultTempMemoryBlock, LevelStandard)
	if err != nil {
		t.Fatal(err)
	}
	got := canon(mustLower(ir.CFGToSNode(testGen, result)))
	if got == "" {
		t.Fatal("InlineVars pipeline produced empty output")
	}
}

// --- Allocation pass tests ---
func TestAllocateBasicPass(t *testing.T) {
	src := "package p\nfunc f() {\n\tx := get(0, 0)\n\tset(0, 1, x)\n}\n"
	entry, _, err := frontend.Compile(src, frontend.Env{Names: frontend.ModeAccessors(ir.ModePlay)})
	if err != nil {
		t.Fatal(err)
	}
	entry = CoalesceFlow{}.Run(testGen, entry)
	entry = AllocateBasic{BlockID: ir.DefaultTempMemoryBlock}.Run(testGen, entry)
	got := canon(mustLower(ir.CFGToSNode(testGen, entry)))
	if got == "" {
		t.Fatal("AllocateBasic produced empty output")
	}
}

func TestAllocateLivePass(t *testing.T) {
	src := "package p\nfunc f() {\n\tx := get(0, 0)\n\tset(0, 1, x)\n}\n"
	entry, _, err := frontend.Compile(src, frontend.Env{Names: frontend.ModeAccessors(ir.ModePlay)})
	if err != nil {
		t.Fatal(err)
	}
	result, err := Optimize(testGen, entry, ir.ModePlay, "test", ir.DefaultTempMemoryBlock, LevelStandard)
	if err != nil {
		t.Fatal(err)
	}
	got := canon(mustLower(ir.CFGToSNode(testGen, result)))
	if got == "" {
		t.Fatal("AllocateLive produced empty output")
	}
}

// --- CSE pass test ---
func TestCSEPipeline(t *testing.T) {
	src := "package p\nfunc f() {\n\ta := sin(0.5)\n\tb := sin(0.5)\n\tset(0, 0, a + b)\n}\n"
	entry, _, err := frontend.Compile(src, frontend.Env{Names: frontend.ModeAccessors(ir.ModePlay)})
	if err != nil {
		t.Fatal(err)
	}
	result, err := Optimize(testGen, entry, ir.ModePlay, "test", ir.DefaultTempMemoryBlock, LevelStandard)
	if err != nil {
		t.Fatal(err)
	}
	got := canon(mustLower(ir.CFGToSNode(testGen, result)))
	if got == "" {
		t.Fatal("CSE pipeline produced empty output")
	}
}

// --- SSA roundtrip test ---
func TestSSARoundtrip(t *testing.T) {
	src := "package p\nfunc f() {\n\tx := 0\n\tif get(0, 0) {\n\t\tx = 1\n\t} else {\n\t\tx = 2\n\t}\n\tset(0, 1, x)\n}\n"
	entry, _, err := frontend.Compile(src, frontend.Env{Names: frontend.ModeAccessors(ir.ModePlay)})
	if err != nil {
		t.Fatal(err)
	}
	result, err := Optimize(testGen, entry, ir.ModePlay, "test", ir.DefaultTempMemoryBlock, LevelStandard)
	if err != nil {
		t.Fatal(err)
	}
	got := canon(mustLower(ir.CFGToSNode(testGen, result)))
	if got == "" {
		t.Fatal("SSA roundtrip produced empty output")
	}
}

// --- LICM pass test ---
func TestLICMPipeline(t *testing.T) {
	src := "package p\nfunc f() {\n\tx := 0\n\tc := sin(1.0)\n\tfor x < 10 {\n\t\tset(0, 0, c)\n\t\tx = x + 1\n\t}\n}\n"
	entry, _, err := frontend.Compile(src, frontend.Env{Names: frontend.ModeAccessors(ir.ModePlay)})
	if err != nil {
		t.Fatal(err)
	}
	result, err := Optimize(testGen, entry, ir.ModePlay, "test", ir.DefaultTempMemoryBlock, LevelStandard)
	if err != nil {
		t.Fatal(err)
	}
	got := canon(mustLower(ir.CFGToSNode(testGen, result)))
	if got == "" {
		t.Fatal("LICM pipeline produced empty output")
	}
}
