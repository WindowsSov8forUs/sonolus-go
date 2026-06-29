package engine

import (
	"encoding/json"
	"testing"

	"github.com/WindowsSov8forUs/sonolus-core-go/codec"
	"github.com/WindowsSov8forUs/sonolus-core-go/core/resource"

	"github.com/WindowsSov8forUs/sonolus-go/compiler/build"
)

// TestExportedField verifies that `sonolus:\"exported\"` fields generate an exports
// entry in the archetype and a writable binding with Block=-1 (export marker).
func TestExportedField(t *testing.T) {
	src := "package p\n" +
		"type Note struct {\n" +
		"\tVal float64 `sonolus:\"exported\"`\n" +
		"}\n" +
		"func (n Note) Initialize() {\n\tn.Val = 42\n}\n"
	data, _, err := CompilePlayFile(src)
	if err != nil {
		t.Fatal(err)
	}
	a := data.Archetypes[0]
	if len(a.Exports) != 1 || a.Exports[0] != "Val" {
		t.Fatalf("exports = %v, want [Val]", a.Exports)
	}
	if !nodeContains(data.Nodes, resource.RuntimeFunctionExportValue) {
		b, _ := json.Marshal(data.Nodes)
		t.Errorf("expected ExportValue, got: %s", b)
	}
}

const realEngineSrc = "package real\n\n" +
	"type Skin struct {\n" +
	"\tNote  float64\n" +
	"\tStage float64\n" +
	"}\n\n" +
	"func dist(x, y float64) float64 { return x + y }\n\n" +
	"type Note struct {\n" +
	"\tBeat float64 `sonolus:\"imported\"`\n" +
	"\tT    float64 `sonolus:\"memory\"`\n" +
	"}\n\n" +
	"func (m Note) target() float64 { return m.Beat * 0.5 }\n\n" +
	"func (n Note) Initialize() {\n" +
	"\tn.T = n.target()\n" +
	"}\n\n" +
	"func (n Note) ShouldSpawn() {\n" +
	"\treturn time > n.T\n" +
	"}\n\n" +
	"func (n Note) UpdateParallel() {\n" +
	"\tp := vec2(sin(time), n.T)\n" +
	"\tdraw(1, p.x, p.y, dist(p.x, p.y), 1, 0, 1, 0, 0)\n" +
	"}\n"

func TestRealEngineCompiles(t *testing.T) {
	data, _, err := CompilePlayFile(realEngineSrc)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}

	if len(data.Archetypes) != 1 || data.Archetypes[0].Name != "Note" {
		t.Fatalf("archetypes = %+v", data.Archetypes)
	}
	a := data.Archetypes[0]
	if a.Initialize == nil || a.ShouldSpawn == nil || a.UpdateParallel == nil {
		t.Fatalf("missing callbacks: init=%v spawn=%v upd=%v",
			a.Initialize, a.ShouldSpawn, a.UpdateParallel)
	}

	if len(a.Imports) != 1 || a.Imports[0].Name != "Beat" {
		t.Fatalf("imports = %+v", a.Imports)
	}
	if len(data.Skin.Sprites) != 2 {
		t.Errorf("skin sprites = %d, want 2", len(data.Skin.Sprites))
	}

	pkg, err := build.PackagePlay(&resource.EngineConfiguration{}, data, nil)
	if err != nil {
		t.Fatalf("package: %v", err)
	}
	if _, rterr := codec.Decompress[resource.EnginePlayData](pkg.PlayData); rterr != nil {
		t.Fatalf("round trip: %v", rterr)
	}

	for _, want := range []resource.RuntimeFunction{
		resource.RuntimeFunctionMultiply,
		resource.RuntimeFunctionDraw,
		resource.RuntimeFunctionSin,
		resource.RuntimeFunctionGet,
		resource.RuntimeFunctionBreak,
	} {
		if !nodeContains(data.Nodes, want) {
			t.Errorf("expected %s in nodes", want)
		}
	}

	t.Logf("total nodes: %d", len(data.Nodes))
}

func TestCompositeLiterialInCallback(t *testing.T) {
	// vec2 constructor in expression position within a callback body (no helpers).
	src := "package p\n" +
		"type Note struct {\n" +
		"\tBeat float64 `sonolus:\"imported\"`\n" +
		"}\n" +
		"func (n Note) UpdateParallel() {\n" +
		"\tv := vec2(n.Beat, 0.5)\n" +
		"\tdraw(1, v.x, v.y, v.x, 1, 0, 1, 0, 0)\n" +
		"}\n"
	data, _, err := CompilePlayFile(src)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	if data.Archetypes[0].UpdateParallel == nil {
		t.Fatal("missing updateParallel")
	}
	if !nodeContains(data.Nodes, resource.RuntimeFunctionDraw) {
		t.Errorf("expected Draw")
	}
	t.Logf("literal composite engine: %d nodes", len(data.Nodes))
}

func TestVec2InCallback(t *testing.T) {
	// vec2 constructor in expression position within a callback (no helpers).
	src := "package p\n" +
		"type Note struct {\n" +
		"\tBeat float64 `sonolus:\"imported\"`\n" +
		"}\n" +
		"func (n Note) UpdateParallel() {\n" +
		"\tv := vec2(n.Beat, 0.5)\n" +
		"\tset(2000, 0, v.x)\n" +
		"}\n"
	data, _, err := CompilePlayFile(src)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	if data.Archetypes[0].UpdateParallel == nil {
		t.Fatal("missing updateParallel")
	}
	if _, err := codec.Decompress[resource.EnginePlayData](mustPackage(t, data)); err != nil {
		t.Fatalf("round trip: %v", err)
	}
	t.Logf("vec2 in callback: %d nodes", len(data.Nodes))
}

func TestScoreLifeBindings(t *testing.T) {
	b := scoreLifeBindings(true, true)
	if b["entityPerfect"].Block != 4006 || b["entityPerfect"].Index != 0 {
		t.Errorf("perfect = %+v", b["entityPerfect"])
	}
	if b["entityLifeMiss"].Block != 4007 || b["entityLifeMiss"].Index != 3 {
		t.Errorf("lifeMiss = %+v", b["entityLifeMiss"])
	}
}

func TestScoredArchetype(t *testing.T) {
	src := "package p\n" +
		"type Note struct {\n" +
		"\tBeat float64 `sonolus:\"imported\"`\n" +
		"\t_     float64 `sonolus:\"scored\"`\n" +
		"\t_     float64 `sonolus:\"lifed\"`\n" +
		"}\n" +
		"func (n Note) UpdateSequential() {\n" +
		"\tset(2000, 0, entityPerfect + entityLifePerfect)\n" +
		"}\n"
	data, _, err := CompilePlayFile(src)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	if !nodeContains(data.Nodes, resource.RuntimeFunctionAdd) {
		t.Errorf("expected Add from entityPerfect+entityLifePerfect")
	}
}
