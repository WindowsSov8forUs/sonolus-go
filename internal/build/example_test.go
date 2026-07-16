package build

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/WindowsSov8forUs/sonolus-core-go/codec"
	"github.com/WindowsSov8forUs/sonolus-core-go/core/resource"
)

// TestEndToEndPlay verifies that a compiled play artifact can be packaged,
// written, and decoded without changing its schema.
func TestEndToEndPlay(t *testing.T) {
	data := &resource.EnginePlayData{
		Skin:     resource.EngineSkinData{},
		Effect:   resource.EngineEffectData{},
		Particle: resource.EngineParticleData{},
		Buckets:  []resource.EngineDataBucket{},
		Archetypes: []resource.EnginePlayDataArchetype{{
			Name:       "Note",
			HasInput:   true,
			Imports:    []resource.EngineDataArchetypeImport{},
			Exports:    []resource.EngineArchetypeDataName{},
			Initialize: &resource.EnginePlayDataArchetypeCallback{Index: 0},
		}},
		Nodes: []resource.EngineDataNode{resource.EngineDataValueNode{Value: 1}},
	}

	// Package and write, then round-trip back.
	pkg, err := PackagePlay(&resource.EngineConfiguration{}, data, nil)
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
	if len(got.Archetypes) != 1 {
		t.Errorf("archetypes = %d, want 1", len(got.Archetypes))
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
