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
