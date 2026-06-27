package engine

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/WindowsSov8forUs/sonolus-core-go/codec"
	"github.com/WindowsSov8forUs/sonolus-core-go/core/resource"

	"github.com/WindowsSov8forUs/sonolus-go/compiler/build"
	"github.com/WindowsSov8forUs/sonolus-go/compiler/play"
)

// nodeContains reports whether the node list has a function node with the op.
func nodeContains(nodes []resource.EngineDataNode, op resource.RuntimeFunction) bool {
	for _, n := range nodes {
		if f, ok := n.(resource.EngineDataFunctionNode); ok && f.Func == op {
			return true
		}
	}
	return false
}

func TestCompilePlayEndToEnd(t *testing.T) {
	// A minimal but real engine: a Counter archetype that bumps a level-memory
	// cell each frame, plus a Stage archetype with a spawnOrder.
	eng := PlayEngine{
		Archetypes: []Archetype{
			{
				Name:     "Counter",
				HasInput: false,
				Callbacks: []Callback{
					{
						Name: play.CallbackUpdateParallel,
						// LevelMemory (2000): cell[0] += 1
						Body: "set(2000, 0, get(2000, 0) + 1)",
					},
				},
			},
			{
				Name: "Stage",
				Callbacks: []Callback{
					{Name: play.CallbackInitialize, Body: "set(2000, 1, 5)"},
				},
			},
		},
	}

	data, err := CompilePlay(eng)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}

	if len(data.Archetypes) != 2 {
		t.Fatalf("archetypes = %d, want 2", len(data.Archetypes))
	}
	counter := data.Archetypes[0]
	if counter.Name != "Counter" || counter.UpdateParallel == nil {
		t.Fatalf("Counter.updateParallel missing: %+v", counter)
	}
	if data.Archetypes[1].Initialize == nil {
		t.Fatalf("Stage.initialize missing")
	}
	if len(data.Nodes) == 0 {
		t.Fatal("no nodes")
	}

	// The `cell += 1` pattern should fold to a SetAdd compound assignment.
	if !nodeContains(data.Nodes, resource.RuntimeFunctionSetAdd) {
		b, _ := json.Marshal(data.Nodes)
		t.Errorf("expected a SetAdd node, nodes: %s", b)
	}

	t.Logf("compiled engine: %d archetypes, %d nodes", len(data.Archetypes), len(data.Nodes))
}

func TestCompilePlayPackages(t *testing.T) {
	eng := PlayEngine{
		Archetypes: []Archetype{{
			Name:      "Note",
			HasInput:  true,
			Callbacks: []Callback{{Name: play.CallbackUpdateParallel, Body: "set(2000, 0, get(2000, 0) + 1)"}},
		}},
	}
	data, err := CompilePlay(eng)
	if err != nil {
		t.Fatal(err)
	}

	pkg, err := build.PackagePlay(&resource.EngineConfiguration{}, data, nil)
	if err != nil {
		t.Fatal(err)
	}
	got, err := codec.Decompress[resource.EnginePlayData](pkg.PlayData)
	if err != nil {
		t.Fatalf("round trip: %v", err)
	}
	if len(got.Archetypes) != 1 || got.Archetypes[0].Name != "Note" {
		t.Fatalf("round trip lost archetype: %+v", got.Archetypes)
	}
	if got.Archetypes[0].UpdateParallel == nil {
		t.Fatal("round trip lost callback")
	}
}

// TestCompilePlayWithArray exercises array locals (incl. a dynamic index)
// through the full optimize pipeline: size-N temps survive ToSSA and the dynamic
// index is resolved by the allocator.
func TestCompilePlayWithArray(t *testing.T) {
	eng := PlayEngine{
		Archetypes: []Archetype{{
			Name: "Buffer",
			Callbacks: []Callback{{
				Name: play.CallbackUpdateParallel,
				Body: "a := array(4)\n" +
					"a[0] = get(2000, 0)\n" +
					"a[1] = a[0] + 1\n" +
					"i := get(2000, 5)\n" +
					"set(2000, 0, a[i])",
			}},
		}},
	}
	data, err := CompilePlay(eng)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	if data.Archetypes[0].UpdateParallel == nil {
		t.Fatal("missing callback")
	}
	if len(data.Nodes) == 0 {
		t.Fatal("no nodes")
	}
	// Round-trips through packaging (validates the produced nodes are well-formed).
	if _, err := codec.Decompress[resource.EnginePlayData](mustPackage(t, data)); err != nil {
		t.Fatalf("round trip: %v", err)
	}
}

func mustPackage(t *testing.T, data *resource.EnginePlayData) []byte {
	t.Helper()
	pkg, err := build.PackagePlay(&resource.EngineConfiguration{}, data, nil)
	if err != nil {
		t.Fatal(err)
	}
	return pkg.PlayData
}

// TestCompilePlayWithRecord exercises a Vec2 record local through the full
// pipeline.
func TestCompilePlayWithRecord(t *testing.T) {
	eng := PlayEngine{
		Archetypes: []Archetype{{
			Name: "Mover",
			Callbacks: []Callback{{
				Name: play.CallbackUpdateParallel,
				Body: "p := vec2(get(2000, 0), get(2000, 1))\n" +
					"p.x = p.x + 1\n" +
					"set(2000, 0, p.x)\n" +
					"set(2000, 1, p.y)",
			}},
		}},
	}
	data, err := CompilePlay(eng)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	if data.Archetypes[0].UpdateParallel == nil || len(data.Nodes) == 0 {
		t.Fatal("missing callback/nodes")
	}
	if _, err := codec.Decompress[resource.EnginePlayData](mustPackage(t, data)); err != nil {
		t.Fatalf("round trip: %v", err)
	}
}

// TestCompilePlayDraws is the capstone: an engine that reads runtime state,
// does math, builds a vector, and draws — compiled end-to-end.
func TestCompilePlayDraws(t *testing.T) {
	eng := PlayEngine{
		Archetypes: []Archetype{{
			Name:     "Sprite",
			HasInput: false,
			Callbacks: []Callback{{
				Name: play.CallbackUpdateParallel,
				Body: "x := sin(time)\n" +
					"p := vec2(x, 0)\n" +
					"draw(1, p.x, p.y, p.x, 1, 0, 1, 0, 0)",
			}},
		}},
	}
	data, err := CompilePlay(eng)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	if data.Archetypes[0].UpdateParallel == nil {
		t.Fatal("missing updateParallel")
	}
	if !nodeContains(data.Nodes, resource.RuntimeFunctionDraw) {
		b, _ := json.Marshal(data.Nodes)
		t.Errorf("expected a Draw node, got: %s", b)
	}
	if !nodeContains(data.Nodes, resource.RuntimeFunctionSin) {
		t.Errorf("expected a Sin node")
	}
	if _, err := codec.Decompress[resource.EnginePlayData](mustPackage(t, data)); err != nil {
		t.Fatalf("round trip: %v", err)
	}
	t.Logf("drawing engine compiled: %d nodes", len(data.Nodes))
}

// TestArchetypeFields is the notgarupa-shaped case: an imported field feeds an
// entity-memory field computed in initialize and used in updateParallel.
func TestArchetypeFields(t *testing.T) {
	eng := PlayEngine{
		Archetypes: []Archetype{{
			Name:     "Note",
			HasInput: true,
			Imported: []ImportedField{{Name: "beat"}},
			Memory:   []string{"targetTime"},
			Callbacks: []Callback{
				{Name: play.CallbackInitialize, Body: "targetTime = beat * 0.5"},
				{Name: play.CallbackUpdateParallel, Body: "draw(1, targetTime, 0, targetTime, 1, 0, 1, 0, 0)"},
			},
		}},
	}
	data, err := CompilePlay(eng)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}

	a := data.Archetypes[0]
	// The imported field generated an import entry (beat -> entity slot 0).
	if len(a.Imports) != 1 || a.Imports[0].Name != "beat" || a.Imports[0].Index != 0 {
		t.Fatalf("imports = %+v, want [{beat 0}]", a.Imports)
	}
	if a.Initialize == nil || a.UpdateParallel == nil {
		t.Fatalf("missing callbacks: %+v", a)
	}
	// initialize computes beat*0.5 (a Multiply); updateParallel draws.
	if !nodeContains(data.Nodes, resource.RuntimeFunctionMultiply) {
		t.Errorf("expected Multiply (beat*0.5)")
	}
	if !nodeContains(data.Nodes, resource.RuntimeFunctionDraw) {
		t.Errorf("expected Draw")
	}
	if _, err := codec.Decompress[resource.EnginePlayData](mustPackage(t, data)); err != nil {
		t.Fatalf("round trip: %v", err)
	}
}

func TestArchetypeImportedFieldReadOnly(t *testing.T) {
	eng := PlayEngine{
		Archetypes: []Archetype{{
			Name:     "Note",
			Imported: []ImportedField{{Name: "beat"}},
			Callbacks: []Callback{
				{Name: play.CallbackInitialize, Body: "beat = 5"},
			},
		}},
	}
	_, err := CompilePlay(eng)
	if err == nil {
		t.Fatal("expected error writing to imported (read-only) field")
	}
	if !strings.Contains(err.Error(), "read-only") {
		t.Errorf("error should mention read-only: %v", err)
	}
}

func TestCompilePlayReportsSourceErrors(t *testing.T) {
	eng := PlayEngine{
		Archetypes: []Archetype{{
			Name:      "Bad",
			Callbacks: []Callback{{Name: play.CallbackUpdateParallel, Body: "set(2000, 0, undefinedVar)"}},
		}},
	}
	_, err := CompilePlay(eng)
	if err == nil {
		t.Fatal("expected error for undefined identifier")
	}
	if !strings.Contains(err.Error(), "Bad") || !strings.Contains(err.Error(), "updateParallel") {
		t.Errorf("error should name archetype+callback: %v", err)
	}
}
