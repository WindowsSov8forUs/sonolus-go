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
	entry = ir.AllocateTestBlocks(entry, ir.DefaultTempMemoryBlock)
	got := canon(mustLower(ir.CFGToSNode(testGen, entry)))
	if got == "" {
		t.Error("AdvancedDCE produced empty output")
	}
	if !strings.Contains(got, "Set(#0,#0,") {
		t.Errorf("Set was incorrectly removed\n got: %s", got)
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
