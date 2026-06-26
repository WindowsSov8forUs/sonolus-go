package engine

import (
	"encoding/json"
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

// TestHelperFunction: a free helper function is inlined at the call site, and
// the read-once temps it introduces collapse through optimization.
func TestHelperFunction(t *testing.T) {
	src := "package p\n" +
		"func dbl(x float64) float64 { return x * 2 }\n" +
		"type Note struct {\n\tBeat float64 `sonolus:\"imported\"`\n}\n" +
		"func (n Note) UpdateParallel() {\n\tset(2000, 0, dbl(n.Beat))\n}\n"
	data, err := CompilePlayFile(src)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	if data.Archetypes[0].UpdateParallel == nil {
		t.Fatal("missing callback")
	}
	// dbl(n.Beat) = n.Beat * 2 -> a Multiply over the imported field.
	if !nodeContains(data.Nodes, resource.RuntimeFunctionMultiply) {
		b, _ := json.Marshal(data.Nodes)
		t.Errorf("expected Multiply from inlined dbl, nodes: %s", b)
	}
	if _, err := codec.Decompress[resource.EnginePlayData](mustPackage(t, data)); err != nil {
		t.Fatalf("round trip: %v", err)
	}
}

// TestValueCallbackBreak: a value-returning callback yields via a Break node.
func TestValueCallbackBreak(t *testing.T) {
	src := "package p\n" +
		"type Note struct {\n\tBeat float64 `sonolus:\"imported\"`\n}\n" +
		"func (n Note) ShouldSpawn() {\n\treturn time > n.Beat\n}\n"
	data, err := CompilePlayFile(src)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	if data.Archetypes[0].ShouldSpawn == nil {
		t.Fatal("missing shouldSpawn")
	}
	if !nodeContains(data.Nodes, resource.RuntimeFunctionBreak) {
		b, _ := json.Marshal(data.Nodes)
		t.Errorf("expected Break (return value), nodes: %s", b)
	}
}

// TestMethodHelper: a non-callback method (accessing the archetype's fields via
// its own receiver) is inlined when called as receiver.Method().
func TestMethodHelper(t *testing.T) {
	src := "package p\n" +
		"type Note struct {\n\tBeat float64 `sonolus:\"imported\"`\n\tT float64 `sonolus:\"memory\"`\n}\n" +
		"func (m Note) targetTime() float64 { return m.Beat * 0.5 }\n" +
		"func (n Note) Initialize() {\n\tn.T = n.targetTime()\n}\n"
	data, err := CompilePlayFile(src)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	if data.Archetypes[0].Initialize == nil {
		t.Fatal("missing initialize")
	}
	if !nodeContains(data.Nodes, resource.RuntimeFunctionMultiply) {
		b, _ := json.Marshal(data.Nodes)
		t.Errorf("expected Multiply from inlined method, nodes: %s", b)
	}
}

func TestRecursionError(t *testing.T) {
	src := "package p\n" +
		"func f(x float64) float64 { return f(x) }\n" +
		"type A struct{}\n" +
		"func (a A) UpdateParallel() { set(2000, 0, f(1)) }\n"
	_, err := CompilePlayFile(src)
	if err == nil {
		t.Fatal("expected recursion error")
	}
	if !strings.Contains(err.Error(), "recursive") {
		t.Errorf("error should mention recursion: %v", err)
	}
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
