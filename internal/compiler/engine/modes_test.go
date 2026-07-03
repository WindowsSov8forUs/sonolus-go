package engine

import (
	"testing"

	"github.com/WindowsSov8forUs/sonolus-core-go/core/resource"
)

func TestCompileWatchFile(t *testing.T) {
	src := "package p\n" +
		"type Note struct {\n\tBeat float64 `sonolus:\"imported\"`\n\tT float64 `sonolus:\"memory\"`\n}\n" +
		"func (n Note) Initialize() { n.T = n.Beat * 0.5 }\n"
	data, err := CompileWatchFile(src)
	if err != nil {
		t.Fatal(err)
	}
	if len(data.Archetypes) != 1 || data.Archetypes[0].Name != "Note" {
		t.Fatalf("archetypes=%+v", data.Archetypes)
	}
	if data.Archetypes[0].Initialize == nil {
		t.Fatal("missing initialize")
	}
}

func TestCompileWatchFileSpawnTime(t *testing.T) {
	src := "package p\n" +
		"type Note struct {\n" +
		"\tBeat float64 `sonolus:\"imported\"`\n" +
		"}\n" +
		"func (n Note) SpawnTime() { set(2000, 0, n.Beat) }\n" +
		"func (n Note) DespawnTime() { set(2000, 1, n.Beat+1) }\n"
	data, err := CompileWatchFile(src)
	if err != nil {
		t.Fatal(err)
	}
	if len(data.Archetypes) != 1 || data.Archetypes[0].Name != "Note" {
		t.Fatalf("archetypes=%+v", data.Archetypes)
	}
	if data.Archetypes[0].SpawnTime == nil {
		t.Fatal("missing spawnTime")
	}
	if data.Archetypes[0].DespawnTime == nil {
		t.Fatal("missing despawnTime")
	}
	if data.Archetypes[0].SpawnTime.Index < 0 {
		t.Fatal("spawnTime node index invalid")
	}
	if data.Archetypes[0].DespawnTime.Index < 0 {
		t.Fatal("despawnTime node index invalid")
	}
}

func TestCompilePreviewFileRender(t *testing.T) {
	src := "package p\n" +
		"type Line struct {\n" +
		"\tBeat float64 `sonolus:\"imported\"`\n" +
		"}\n" +
		"func (l Line) Render() { set(2000, 0, l.Beat) }\n"
	data, err := CompilePreviewFile(src)
	if err != nil {
		t.Fatal(err)
	}
	if len(data.Archetypes) != 1 || data.Archetypes[0].Name != "Line" {
		t.Fatalf("archetypes=%+v", data.Archetypes)
	}
	if data.Archetypes[0].Render == nil {
		t.Fatal("missing render")
	}
	if data.Archetypes[0].Render.Index < 0 {
		t.Fatal("render node index invalid")
	}
}

func TestCompilePreviewFile(t *testing.T) {
	src := "package p\n" +
		"type Line struct {\n\tBeat float64 `sonolus:\"imported\"`\n}\n" +
		"func (l Line) Preprocess() { set(2000, 0, l.Beat) }\n"
	data, err := CompilePreviewFile(src)
	if err != nil {
		t.Fatal(err)
	}
	if len(data.Archetypes) != 1 || data.Archetypes[0].Preprocess == nil {
		t.Fatalf("archetypes=%+v", data.Archetypes)
	}
}

func TestCompileTutorialFile(t *testing.T) {
	src := "package p\n" +
		"func Preprocess() { set(2000, 0, time) }\n"
	data, err := CompileTutorialFile(src)
	if err != nil {
		t.Fatal(err)
	}
	if data.Preprocess < 0 {
		t.Fatal("preprocess missing")
	}
	if len(data.Instruction.Texts) != 0 || len(data.Instruction.Icons) != 0 {
		// No Instruction struct in source -- instruction data should be empty but
		// present (not nil/missing). The zero value is correct for an absent resource.
	}
}

func TestCompileTutorialWithInstruction(t *testing.T) {
	src := "package p\n" +
		"type Instruction struct {\n" +
		"\tWelcome float64\n" +
		"\tInfo    float64\n" +
		"}\n" +
		"func Preprocess() { set(2000, 0, time) }\n"
	data, err := CompileTutorialFile(src)
	if err != nil {
		t.Fatal(err)
	}
	if len(data.Instruction.Texts) != 2 {
		t.Fatalf("expected 2 instruction texts, got %d", len(data.Instruction.Texts))
	}
	if data.Instruction.Texts[0].ID != 0 || data.Instruction.Texts[0].Name != "Welcome" {
		t.Fatalf("text[0] = %+v, want {Welcome 0}", data.Instruction.Texts[0])
	}
	if data.Instruction.Texts[1].ID != 1 || data.Instruction.Texts[1].Name != "Info" {
		t.Fatalf("text[1] = %+v, want {Info 1}", data.Instruction.Texts[1])
	}
}

func TestComposeOrFirst(t *testing.T) {
	// Empty slice -> -1 (omitted).
	if got := composeOrFirst(nil, &[]resource.EngineDataNode{}); got != -1 {
		t.Errorf("nil slice: got %d, want -1", got)
	}
	// Single element -> returned directly.
	nodes := []resource.EngineDataNode{
		resource.EngineDataValueNode{Value: 42},
	}
	if got := composeOrFirst([]int{0}, &nodes); got != 0 {
		t.Errorf("single element: got %d, want 0", got)
	}
	// Multiple elements -> Execute composition.
	multiNodes := []resource.EngineDataNode{
		resource.EngineDataValueNode{Value: 1},
		resource.EngineDataValueNode{Value: 2},
	}
	got := composeOrFirst([]int{0, 1}, &multiNodes)
	if got != 2 {
		t.Errorf("multiple elements: got %d, want 2 (index of new Execute node)", got)
	}
	if len(multiNodes) != 3 {
		t.Fatalf("expected 3 nodes after composition, got %d", len(multiNodes))
	}
	fn, ok := multiNodes[2].(resource.EngineDataFunctionNode)
	if !ok || fn.Func != resource.RuntimeFunctionExecute {
		t.Fatalf("expected Execute function node, got %T %v", multiNodes[2], fn.Func)
	}
	// Args: [0, 1] -- matching sonolus.js-compiler tutorial assemble.ts behavior.
	if len(fn.Args) != 2 || fn.Args[0] != 0 || fn.Args[1] != 1 {
		t.Errorf("args = %v, want [0, 1]", fn.Args)
	}
}

func TestCompileWatchWithUpdateSpawn(t *testing.T) {
	src := "package p\n" +
		"type Note struct {\n" +
		"\tBeat float64 `sonolus:\"imported\"`\n" +
		"}\n" +
		"func (n Note) Preprocess() { set(2000, 0, n.Beat) }\n" +
		"func UpdateSpawn() {}\n"
	data, err := CompileWatchFile(src)
	if err != nil {
		t.Fatal(err)
	}
	// UpdateSpawn should be assigned a valid node index (>0), even for an
	// empty body whose CFG emits a Block/JumpLoop terminator.
	if data.UpdateSpawn <= 0 {
		t.Fatalf("UpdateSpawn = %d, expected positive node index", data.UpdateSpawn)
	}
}

func TestCompileWatchWithoutUpdateSpawn(t *testing.T) {
	src := "package p\n" +
		"type Note struct {\n" +
		"\tBeat float64 `sonolus:\"imported\"`\n" +
		"}\n" +
		"func (n Note) Preprocess() { set(2000, 0, n.Beat) }\n"
	data, err := CompileWatchFile(src)
	if err != nil {
		t.Fatal(err)
	}
	if data.UpdateSpawn != 0 {
		t.Fatalf("UpdateSpawn = %d, expected 0 (absent)", data.UpdateSpawn)
	}
}
