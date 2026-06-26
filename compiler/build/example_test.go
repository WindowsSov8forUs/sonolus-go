package build

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/WindowsSov8forUs/sonolus-core-go/codec"
	"github.com/WindowsSov8forUs/sonolus-core-go/core/resource"

	"github.com/WindowsSov8forUs/sonolus-go/compiler/play"
	"github.com/WindowsSov8forUs/sonolus-go/compiler/snode"
)

// TestEndToEndPlay drives the whole play-mode emitter: hand-authored SNode
// callbacks -> compile (optimize + no-op rules) -> assemble -> package -> write.
func TestEndToEndPlay(t *testing.T) {
	data := play.BuildPlayData(
		resource.EngineSkinData{},
		resource.EngineEffectData{},
		resource.EngineParticleData{},
		nil,
		[]play.ArchetypeDef{
			{Name: "Note", HasInput: true},
			{Name: "Stage"},
		},
	)

	val := snode.Val
	call := snode.Call
	// EntityMemory block (0) used here only as an opaque addressable target.
	const mem = 0

	results := []*play.CompileResult{
		// Note.spawnOrder = constant 0 -> omitted entirely.
		play.CompileCallback(0, play.CallbackSpawnOrder, val(0), 0),
		// Note.initialize: Set(mem, 1, 2 + 3) -> constant folds to Set(mem,1,5).
		play.CompileCallback(0, play.CallbackInitialize,
			call(resource.RuntimeFunctionSet, val(mem), val(1), call(resource.RuntimeFunctionAdd, val(2), val(3))),
			0),
		// Note.updateParallel: Execute(Set(mem,1,Get(mem,1)+1), 0) -> SetAdd + drop return.
		play.CompileCallback(0, play.CallbackUpdateParallel,
			call(resource.RuntimeFunctionExecute,
				call(resource.RuntimeFunctionSet, val(mem), val(1),
					call(resource.RuntimeFunctionAdd, call(resource.RuntimeFunctionGet, val(mem), val(1)), val(1))),
				val(0)),
			0),
		// Stage.updateParallel shares the Get(mem,1) subexpression with Note.
		play.CompileCallback(1, play.CallbackUpdateParallel,
			call(resource.RuntimeFunctionGet, val(mem), val(1)), 0),
	}

	if err := play.Assemble(data, results); err != nil {
		t.Fatalf("assemble: %v", err)
	}

	// Note.spawnOrder must be absent; initialize folded; updateParallel present.
	note := data.Archetypes[0]
	if note.SpawnOrder != nil {
		t.Errorf("spawnOrder should be omitted, got %+v", note.SpawnOrder)
	}
	if note.Initialize == nil || note.UpdateParallel == nil {
		t.Fatalf("missing callbacks on Note: %+v", note)
	}

	// Package and write, then round-trip back.
	pkg, err := PackagePlay(&resource.EngineConfiguration{}, data)
	if err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(t.TempDir(), "engine")
	if err := pkg.Write(dir); err != nil {
		t.Fatal(err)
	}

	blob, err := os.ReadFile(filepath.Join(dir, FilePlayData))
	if err != nil {
		t.Fatal(err)
	}
	got, err := codec.Decompress[resource.EnginePlayData](blob)
	if err != nil {
		t.Fatalf("decompress written file: %v", err)
	}
	if len(got.Archetypes) != 2 {
		t.Errorf("archetypes = %d, want 2", len(got.Archetypes))
	}
	if len(got.Nodes) == 0 {
		t.Errorf("expected non-empty nodes")
	}
	t.Logf("end-to-end play engine: %d nodes", len(got.Nodes))
}

// TestReadRealEngineData validates that our types + codec correctly read a real,
// already-built engine produced by the reference toolchain. Skipped if the
// local fixture is unavailable.
func TestReadRealEngineData(t *testing.T) {
	path := filepath.Join("..", "..", "..", "sonolus-notgarupa-engine", "dist", "notgarupa", "previewData")
	blob, err := os.ReadFile(path)
	if err != nil {
		t.Skipf("real engine fixture not available: %v", err)
	}
	data, err := codec.Decompress[resource.EnginePreviewData](blob)
	if err != nil {
		t.Fatalf("decompress real previewData: %v", err)
	}
	if len(data.Nodes) == 0 || len(data.Archetypes) == 0 {
		t.Fatalf("real previewData looks empty: %d nodes, %d archetypes", len(data.Nodes), len(data.Archetypes))
	}
	t.Logf("read real notgarupa previewData: %d nodes, %d archetypes", len(data.Nodes), len(data.Archetypes))
}
