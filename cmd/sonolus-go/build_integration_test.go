package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/WindowsSov8forUs/sonolus-core-go/codec"
	"github.com/WindowsSov8forUs/sonolus-core-go/core/resource"
	"github.com/WindowsSov8forUs/sonolus-go/compiler/build"
	"github.com/WindowsSov8forUs/sonolus-go/compiler/engine"
)

// compilePlayToDir compiles a minimal play engine and writes the packaged files
// to dir, returning the engine name. It mirrors the "build" path in main.go
// but returns errors instead of os.Exit.
func compilePlayToDir(dir, name, src string) error {
	cfgPtr := &resource.EngineConfiguration{}

	playData, playCfg, err := engine.CompilePlayFile(src)
	if err != nil {
		return err
	}
	*cfgPtr = *playCfg

	rom, err := build.BuildROM(build.DefaultROM())
	if err != nil {
		return err
	}

	pkg, err := build.PackagePlay(cfgPtr, playData, rom)
	if err != nil {
		return err
	}

	engineDir := filepath.Join(dir, name)
	if err := os.MkdirAll(engineDir, 0o755); err != nil {
		return err
	}
	return pkg.Write(engineDir)
}

func TestBuildPlayIntegration(t *testing.T) {
	dir := t.TempDir()
	src := `package engine

type Skin struct {
	Default int
}

type Archetype struct{}

func (a Archetype) UpdateParallel(dt float64) {
	set(2000, 0, get(2000, 0) + 1)
}
`
	name := "test-engine"
	if err := compilePlayToDir(dir, name, src); err != nil {
		t.Fatalf("compile: %v", err)
	}

	engineDir := filepath.Join(dir, name)

	// Verify all expected files exist.
	for _, f := range []string{
		build.FileConfiguration,
		build.FilePlayData,
		build.FileROM,
	} {
		path := filepath.Join(engineDir, f)
		if _, err := os.Stat(path); err != nil {
			t.Errorf("file %s not found: %v", f, err)
		}
	}

	// Verify play data round-trips (gzip → decompress → valid JSON).
	playData, err := os.ReadFile(filepath.Join(engineDir, build.FilePlayData))
	if err != nil {
		t.Fatal(err)
	}
	pd, err := codec.Decompress[resource.EnginePlayData](playData)
	if err != nil {
		t.Fatalf("decompress playData: %v", err)
	}
	if len(pd.Archetypes) != 1 {
		t.Errorf("archetypes = %d, want 1", len(pd.Archetypes))
	}
}

func TestBuildWithBadSourceReportsError(t *testing.T) {
	// A source with a deliberate syntax error should fail.
	src := `package engine
func (a Archetype) UpdateParallel( { oops
`
	_, _, err := engine.CompilePlayFile(src)
	if err == nil {
		t.Error("expected compile error for bad source, got nil")
	}
}
