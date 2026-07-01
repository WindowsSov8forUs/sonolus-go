package play

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/WindowsSov8forUs/sonolus-core-go/core/resource"

	"github.com/WindowsSov8forUs/sonolus-go/compiler/modecompile"
	"github.com/WindowsSov8forUs/sonolus-go/compiler/snode"
)

// Local aliases for the shared test helpers in modecompile.
var (
	val   = modecompile.Val
	get   = modecompile.Get
	exec  = modecompile.Exec
	canon = modecompile.Canon
)

func TestCompileCallbackNoOp(t *testing.T) {
	cases := []struct {
		name     string
		cb       Callback
		node     snode.SNode
		wantNil  bool
		wantTree string // canonical form when not nil
	}{
		// spawnOrder: omit only constant 0.
		{"spawnOrder_zero", CallbackSpawnOrder, val(0), true, ""},
		{"spawnOrder_const", CallbackSpawnOrder, val(3), false, "#3"},
		{"spawnOrder_dyn", CallbackSpawnOrder, get(0), false, "Get(#1000,#0)"},

		// shouldSpawn: omit constant non-zero (always true).
		{"shouldSpawn_true", CallbackShouldSpawn, val(1), true, ""},
		{"shouldSpawn_false", CallbackShouldSpawn, val(0), false, "#0"},
		{"shouldSpawn_dyn", CallbackShouldSpawn, get(0), false, "Get(#1000,#0)"},

		// default: omit any constant; strip trailing 0 of Execute.
		{"default_const", CallbackInitialize, val(7), true, ""},
		{"default_dyn", CallbackInitialize, get(0), false, "Get(#1000,#0)"},
		{"default_exec2", CallbackInitialize, exec(get(0), val(0)), false, "Get(#1000,#0)"},
		{"default_execN", CallbackInitialize, exec(get(0), get(1), val(0)), false, "Execute(Get(#1000,#0),Get(#1000,#1))"},
		{"default_exec_nonzero", CallbackUpdateParallel, exec(get(0), val(5)), false, "Execute(Get(#1000,#0),#5)"},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := modecompile.CompileCallbackForMode(0, string(c.cb), c.node, "play")
			if c.wantNil {
				if got != nil {
					t.Fatalf("expected nil, got %s", canon(got.Node))
				}
				return
			}
			if got == nil {
				t.Fatalf("expected result, got nil")
			}
			if canon(got.Node) != c.wantTree {
				t.Errorf("tree = %s, want %s", canon(got.Node), c.wantTree)
			}
		})
	}
}

func TestAssembleDedupAndDispatch(t *testing.T) {
	data := BuildPlayData(
		resource.EngineSkinData{},
		resource.EngineEffectData{},
		resource.EngineParticleData{},
		nil,
		[]ArchetypeDef{{Name: "A"}, {Name: "B", HasInput: true}},
	)

	results := []*modecompile.Result{
		{ArchetypeIndex: 0, Callback: string(CallbackUpdateParallel), Node: get(0)},
		nil, // skipped
		{ArchetypeIndex: 1, Callback: string(CallbackUpdateParallel), Node: get(0)}, // shares nodes
		{ArchetypeIndex: 0, Callback: string(CallbackInitialize), Node: get(1)},
	}

	if err := Assemble(data, results); err != nil {
		t.Fatalf("assemble: %v", err)
	}

	// get(0) -> nodes [1000, 0, Get(0,1)] (root 2); get(1) reuses 1000 -> [.., 1, Get(0,3)] (root 4).
	if len(data.Nodes) != 5 {
		t.Fatalf("node count = %d, want 5", len(data.Nodes))
	}

	a0 := data.Archetypes[0]
	if a0.UpdateParallel == nil || a0.UpdateParallel.Index != 2 {
		t.Errorf("A.updateParallel = %+v, want index 2", a0.UpdateParallel)
	}
	if a0.Initialize == nil || a0.Initialize.Index != 4 {
		t.Errorf("A.initialize = %+v, want index 4", a0.Initialize)
	}
	a1 := data.Archetypes[1]
	if a1.UpdateParallel == nil || a1.UpdateParallel.Index != 2 {
		t.Errorf("B.updateParallel = %+v, want index 2", a1.UpdateParallel)
	}

	// Imports/exports must serialize as [] not null, matching the reference.
	b, err := json.Marshal(a0)
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)
	if !strings.Contains(s, `"imports":[]`) || !strings.Contains(s, `"exports":[]`) {
		t.Errorf("archetype json missing empty slices: %s", s)
	}
	// Omitted callbacks must not appear.
	if strings.Contains(s, `"touch"`) || strings.Contains(s, `"terminate"`) {
		t.Errorf("unexpected omitted callback present: %s", s)
	}
}
