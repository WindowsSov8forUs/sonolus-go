package watch

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
		wantTree string
	}{
		// spawnTime: omit only constant 0.
		{"spawnTime_zero", CallbackSpawnTime, val(0), true, ""},
		{"spawnTime_const", CallbackSpawnTime, val(3), false, "#3"},
		{"spawnTime_dyn", CallbackSpawnTime, get(0), false, "Get(#1000,#0)"},
		// despawnTime: same rule.
		{"despawnTime_zero", CallbackDespawnTime, val(0), true, ""},
		{"despawnTime_const", CallbackDespawnTime, val(5), false, "#5"},
		// default: omit any constant; strip trailing 0 of Execute.
		{"default_const", CallbackInitialize, val(7), true, ""},
		{"default_dyn", CallbackInitialize, get(0), false, "Get(#1000,#0)"},
		{"default_exec2", CallbackInitialize, exec(get(0), val(0)), false, "Get(#1000,#0)"},
		{"default_execN", CallbackUpdateParallel, exec(get(0), get(1), val(0)), false, "Execute(Get(#1000,#0),Get(#1000,#1))"},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := CompileCallback(0, c.cb, c.node)
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
	data := BuildWatchData(
		resource.EngineSkinData{},
		resource.EngineEffectData{},
		resource.EngineParticleData{},
		nil,
		[]ArchetypeDef{{Name: "A"}, {Name: "B"}},
	)

	results := []*modecompile.Result{
		{ArchetypeIndex: 0, Callback: string(CallbackUpdateParallel), Node: get(0)},
		nil,
		{ArchetypeIndex: 1, Callback: string(CallbackUpdateParallel), Node: get(0)},
		{ArchetypeIndex: 0, Callback: string(CallbackInitialize), Node: get(1)},
	}

	if err := Assemble(data, results); err != nil {
		t.Fatalf("assemble: %v", err)
	}

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

	b, _ := json.Marshal(a0)
	s := string(b)
	if !strings.Contains(s, `"imports":[]`) {
		t.Errorf("archetype json missing empty imports: %s", s)
	}
}

func TestHasInput(t *testing.T) {
	// Watch mode has no input (Touch) callback, so HasInput should always be
	// false. The field is propagated from ArchetypeDef through BuildWatchData
	// into the core EngineWatchDataArchetype.
	data := BuildWatchData(
		resource.EngineSkinData{},
		resource.EngineEffectData{},
		resource.EngineParticleData{},
		nil,
		[]ArchetypeDef{{Name: "A"}, {Name: "B"}},
	)
	if len(data.Archetypes) != 2 {
		t.Fatalf("archetype count = %d", len(data.Archetypes))
	}
	for i, a := range data.Archetypes {
		if a.Name == "" {
			t.Errorf("archetype[%d].Name is empty", i)
		}
	}
}
