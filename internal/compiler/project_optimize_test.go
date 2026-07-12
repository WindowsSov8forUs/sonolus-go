package compiler

import (
	"testing"

	"github.com/WindowsSov8forUs/sonolus-go/internal/compiler/frontend"
	"github.com/WindowsSov8forUs/sonolus-go/internal/compiler/ir"
	"github.com/WindowsSov8forUs/sonolus-go/internal/compiler/mode"
	"github.com/WindowsSov8forUs/sonolus-go/internal/compiler/optimize"
)

func TestOptimizeProjectCopiesDeclarationsAndIR(t *testing.T) {
	voidType := ir.Type{Name: "void"}
	callback := &frontend.CallbackDeclaration{Name: "update", IR: &ir.Function{
		Name: "update", Result: voidType, Entry: 0,
		Blocks: []*ir.Block{
			{ID: 0, Terminator: ir.Branch{Condition: ir.Const{Value: 1}, True: 1, False: 2}},
			{ID: 1, Terminator: ir.Return{Value: ir.Value{Type: voidType}}},
			{ID: 2, Terminator: ir.Return{Value: ir.Value{Type: voidType}}},
		},
	}}
	archetype := &frontend.ArchetypeDeclaration{Name: "Note", Callbacks: []*frontend.CallbackDeclaration{callback}}
	declarations := &frontend.ModeDeclarations{Mode: mode.ModePlay, Archetypes: []*frontend.ArchetypeDeclaration{archetype}}
	project := &frontend.Project{Modes: map[mode.Mode]*frontend.ModeDeclarations{mode.ModePlay: declarations}}

	result, err := optimizeProject(optimize.NewOptimizer(0), project)
	if err != nil {
		t.Fatal(err)
	}
	optimized := result.Modes[mode.ModePlay].Archetypes[0].Callbacks[0]
	if result == project || result.Modes[mode.ModePlay] == declarations || result.Modes[mode.ModePlay].Archetypes[0] == archetype || optimized == callback || optimized.IR == callback.IR {
		t.Fatal("optimized project shares mutable declaration or IR containers")
	}
	if len(optimized.IR.Blocks) != 1 {
		t.Fatalf("optimized blocks = %d", len(optimized.IR.Blocks))
	}
	if len(callback.IR.Blocks) != 3 {
		t.Fatal("frontend callback IR was modified")
	}
}
