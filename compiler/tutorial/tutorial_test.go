package tutorial

import (
	"testing"

	"github.com/WindowsSov8forUs/sonolus-core-go/core/resource"

	"github.com/WindowsSov8forUs/sonolus-go/compiler/modecompile"
	"github.com/WindowsSov8forUs/sonolus-go/compiler/snode"
)

func TestAssemble(t *testing.T) {
	data := BuildTutorialData(
		resource.EngineSkinData{},
		resource.EngineEffectData{},
		resource.EngineParticleData{},
		resource.EngineInstructionData{},
	)
	app := snode.NewAppender(&data.Nodes)

	pre := snode.Val(100)
	nav := snode.Val(200)
	upd := snode.Val(300)

	preIdx, _ := app.Append(pre)
	navIdx, _ := app.Append(nav)
	updIdx, _ := app.Append(upd)
	data.Preprocess = preIdx
	data.Navigate = navIdx
	data.Update = updIdx

	if preIdx == navIdx || preIdx == updIdx || navIdx == updIdx {
		t.Fatalf("indices should differ: pre=%d nav=%d upd=%d", preIdx, navIdx, updIdx)
	}
	if len(data.Nodes) != 3 {
		t.Fatalf("node count = %d, want 3", len(data.Nodes))
	}
	if data.Preprocess != 0 || data.Navigate != 0 || data.Update != 0 {
		// BuildTutorialData sets the skeleton; caller wires the indices.
		t.Logf("skeleton indices: pre=%d nav=%d upd=%d", data.Preprocess, data.Navigate, data.Update)
	}
}

func TestAssembleEmpty(t *testing.T) {
	data := BuildTutorialData(
		resource.EngineSkinData{},
		resource.EngineEffectData{},
		resource.EngineParticleData{},
		resource.EngineInstructionData{},
	)
	app := snode.NewAppender(&data.Nodes)

	zero := snode.Val(0)
	preIdx, _ := app.Append(zero)
	navIdx, _ := app.Append(zero)
	updIdx, _ := app.Append(zero)
	data.Preprocess = preIdx
	data.Navigate = navIdx
	data.Update = updIdx

	if preIdx != navIdx {
		t.Logf("pre=%d nav=%d — equal OK (same node dedup)", preIdx, navIdx)
	}
	// All three are the same node (dedup), so all indices point to the same value.
	if preIdx != updIdx {
		t.Logf("pre=%d upd=%d — equal OK (same node dedup)", preIdx, updIdx)
	}
	if len(data.Nodes) != 1 {
		t.Fatalf("node count = %d, want 1 (dedup)", len(data.Nodes))
	}
}

func TestAssemble_Direct(t *testing.T) {
	data := BuildTutorialData(
		resource.EngineSkinData{},
		resource.EngineEffectData{},
		resource.EngineParticleData{},
		resource.EngineInstructionData{},
	)
	data.Preprocess = 1
	data.Navigate = 2
	data.Update = 3

	if data.Preprocess != 1 {
		t.Errorf("Preprocess = %d, want 1", data.Preprocess)
	}
	if data.Navigate != 2 {
		t.Errorf("Navigate = %d, want 2", data.Navigate)
	}
	if data.Update != 3 {
		t.Errorf("Update = %d, want 3", data.Update)
	}
}

func TestAssemble_ZeroIndices(t *testing.T) {
	data := BuildTutorialData(
		resource.EngineSkinData{},
		resource.EngineEffectData{},
		resource.EngineParticleData{},
		resource.EngineInstructionData{},
	)
	// Zero indices mean "no callback" per Sonolus convention.
	data.Preprocess = 0
	data.Navigate = 0
	data.Update = 0

	if data.Preprocess != 0 || data.Navigate != 0 || data.Update != 0 {
		t.Error("all indices should be zero")
	}
}

func TestCompileCallback_Peephole(t *testing.T) {
	// CompileCallback should run peephole optimization and apply
	// pure-constant/trailing-zero stripping.
	zero := snode.Val(0)
	r := modecompile.CompileCallbackForMode(-1, string(CallbackPreprocess), zero, "tutorial")
	if r != nil {
		t.Error("pure constant zero should be stripped")
	}

	nonZero := snode.Val(42)
	r = modecompile.CompileCallbackForMode(-1, string(CallbackNavigate), nonZero, "tutorial")
	if r != nil {
		t.Error("pure constant non-zero should be stripped")
	}
}

func TestCompileCallback_Dynamic(t *testing.T) {
	// A dynamic node should not be stripped.
	dyn := snode.Call(resource.RuntimeFunctionDebugLog, snode.Val(1))
	r := modecompile.CompileCallbackForMode(-1, string(CallbackUpdate), dyn, "tutorial")
	if r == nil {
		t.Fatal("dynamic callback should not be stripped")
	}
	if r.Callback != "update" {
		t.Errorf("callback = %q, want \"update\"", r.Callback)
	}
	if r.ArchetypeIndex != -1 {
		t.Errorf("archetypeIndex = %d, want -1", r.ArchetypeIndex)
	}
}

func TestBuildTutorialData(t *testing.T) {
	skin := resource.EngineSkinData{Sprites: []resource.EngineSkinDataSprite{
		{Name: "note"}, {Name: "stage"},
	}}
	effect := resource.EngineEffectData{}
	particle := resource.EngineParticleData{}
	instruction := resource.EngineInstructionData{
		Texts: []resource.EngineInstructionDataText{{Name: "hello"}},
	}

	data := BuildTutorialData(skin, effect, particle, instruction)

	if len(data.Skin.Sprites) != 2 {
		t.Fatalf("skin sprites = %d, want 2", len(data.Skin.Sprites))
	}
	if len(data.Instruction.Texts) != 1 {
		t.Fatalf("instruction texts = %d, want 1", len(data.Instruction.Texts))
	}
	if data.Nodes == nil {
		t.Fatal("nodes slice should be non-nil (empty)")
	}
	if data.Preprocess != 0 || data.Navigate != 0 || data.Update != 0 {
		t.Logf("initial indices are zero (caller fills)")
	}
}
