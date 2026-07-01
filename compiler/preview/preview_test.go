package preview

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
		// default: omit any constant; strip trailing 0 of Execute.
		{"default_const", CallbackPreprocess, val(7), true, ""},
		{"default_dyn", CallbackPreprocess, get(0), false, "Get(#1000,#0)"},
		{"default_exec2", CallbackRender, exec(get(0), val(0)), false, "Get(#1000,#0)"},
		{"default_execN", CallbackRender, exec(get(0), get(1), val(0)), false, "Execute(Get(#1000,#0),Get(#1000,#1))"},
		{"default_exec_nonzero", CallbackPreprocess, exec(get(0), val(5)), false, "Execute(Get(#1000,#0),#5)"},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := modecompile.CompileCallbackForMode(0, string(c.cb), c.node, "preview")
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
	data := BuildPreviewData(
		resource.EngineSkinData{},
		[]ArchetypeDef{{Name: "A"}, {Name: "B"}},
	)

	results := []*modecompile.Result{
		{ArchetypeIndex: 0, Callback: string(CallbackRender), Node: get(0)},
		nil,
		{ArchetypeIndex: 1, Callback: string(CallbackRender), Node: get(0)},
		{ArchetypeIndex: 0, Callback: string(CallbackPreprocess), Node: get(1)},
	}

	if err := Assemble(data, results); err != nil {
		t.Fatalf("assemble: %v", err)
	}

	if len(data.Nodes) != 5 {
		t.Fatalf("node count = %d, want 5", len(data.Nodes))
	}

	a0 := data.Archetypes[0]
	if a0.Render == nil || a0.Render.Index != 2 {
		t.Errorf("A.render = %+v, want index 2", a0.Render)
	}
	if a0.Preprocess == nil || a0.Preprocess.Index != 4 {
		t.Errorf("A.preprocess = %+v, want index 4", a0.Preprocess)
	}
	a1 := data.Archetypes[1]
	if a1.Render == nil || a1.Render.Index != 2 {
		t.Errorf("B.render = %+v, want index 2", a1.Render)
	}

	b, _ := json.Marshal(a0)
	s := string(b)
	if !strings.Contains(s, `"imports":[]`) {
		t.Errorf("archetype json missing empty imports: %s", s)
	}
}
