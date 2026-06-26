package engine

import (
	"strings"
	"testing"

	"github.com/WindowsSov8forUs/sonolus-core-go/codec"
	"github.com/WindowsSov8forUs/sonolus-core-go/core/resource"
)

// noteEngineSrc is an idiomatic Go engine file: a struct archetype with tagged
// fields and callback methods using receiver field access.
const noteEngineSrc = "package myengine\n\n" +
	"type Note struct {\n" +
	"\tBeat       float64 `sonolus:\"imported\"`\n" +
	"\tTargetTime float64 `sonolus:\"memory\"`\n" +
	"}\n\n" +
	"func (n Note) Initialize() {\n" +
	"\tn.TargetTime = n.Beat * 0.5\n" +
	"}\n\n" +
	"func (n Note) UpdateParallel() {\n" +
	"\tdraw(1, n.TargetTime, 0, n.TargetTime, 1, 0, 1, 0, 0)\n" +
	"}\n"

func TestCompilePlayFile(t *testing.T) {
	data, err := CompilePlayFile(noteEngineSrc)
	if err != nil {
		t.Fatalf("compile file: %v", err)
	}
	if len(data.Archetypes) != 1 {
		t.Fatalf("archetypes = %d, want 1", len(data.Archetypes))
	}
	a := data.Archetypes[0]
	if a.Name != "Note" {
		t.Errorf("name = %q, want Note", a.Name)
	}
	if len(a.Imports) != 1 || a.Imports[0].Name != "Beat" || a.Imports[0].Index != 0 {
		t.Fatalf("imports = %+v, want [{Beat 0}]", a.Imports)
	}
	if a.Initialize == nil || a.UpdateParallel == nil {
		t.Fatalf("missing callbacks: %+v", a)
	}
	if !nodeContains(data.Nodes, resource.RuntimeFunctionMultiply) {
		t.Errorf("expected Multiply (Beat*0.5)")
	}
	if !nodeContains(data.Nodes, resource.RuntimeFunctionDraw) {
		t.Errorf("expected Draw")
	}
	if _, err := codec.Decompress[resource.EnginePlayData](mustPackage(t, data)); err != nil {
		t.Fatalf("round trip: %v", err)
	}
	t.Logf("parsed+compiled Note engine: %d nodes", len(data.Nodes))
}

func TestCompilePlayFileImportedReadOnly(t *testing.T) {
	src := "package p\n" +
		"type Note struct {\n\tBeat float64 `sonolus:\"imported\"`\n}\n" +
		"func (n Note) Initialize() {\n\tn.Beat = 5\n}\n"
	_, err := CompilePlayFile(src)
	if err == nil {
		t.Fatal("expected read-only error writing n.Beat")
	}
	if !strings.Contains(err.Error(), "read-only") {
		t.Errorf("error should mention read-only: %v", err)
	}
}

func TestCompilePlayFileTouchSetsHasInput(t *testing.T) {
	src := "package p\n" +
		"type Tap struct{}\n" +
		"func (t Tap) Touch() {\n\tset(2000, 0, 1)\n}\n"
	data, err := CompilePlayFile(src)
	if err != nil {
		t.Fatal(err)
	}
	if !data.Archetypes[0].HasInput {
		t.Error("archetype with a Touch callback should have hasInput=true")
	}
}
