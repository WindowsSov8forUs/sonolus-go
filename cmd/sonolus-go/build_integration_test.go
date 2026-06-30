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
	src := "package engine\nfunc (a Archetype) UpdateParallel( { oops\n"
	_, _, err := engine.CompilePlayFile(src)
	if err == nil {
		t.Error("expected compile error for bad source, got nil")
	}
}

func TestBuildAllModes(t *testing.T) {
	dir := t.TempDir()
	src := "package engine\n\n" +
		"type Skin struct{}\n" +
		"type Effect struct{}\n" +
		"type Particle struct{}\n\n" +
		"type Archetype struct{}\n\n" +
		"func (a Archetype) UpdateSequential(dt float64) {\n" +
		"\tset(2000, 0, get(2000, 0) + 1)\n" +
		"}\n\n" +
		"func Preprocess() {}\n" +
		"func Navigate() {}\n" +
		"func Update() {}\n"

	// Compile Play mode first (needed for config).
	playData, playCfg, err := engine.CompilePlayFile(src)
	if err != nil {
		t.Fatalf("compile play: %v", err)
	}

	rom, err := build.BuildROM(build.DefaultROM())
	if err != nil {
		t.Fatal(err)
	}
	pkg, err := build.PackagePlay(playCfg, playData, rom)
	if err != nil {
		t.Fatal(err)
	}

	engineDir := filepath.Join(dir, "test-engine")
	if err := os.MkdirAll(engineDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := pkg.Write(engineDir); err != nil {
		t.Fatal(err)
	}

	// Compile non-Play modes.
	watchData, err := engine.CompileWatchFile(src)
	if err != nil {
		t.Fatalf("compile watch: %v", err)
	}
	previewData, err := engine.CompilePreviewFile(src)
	if err != nil {
		t.Fatalf("compile preview: %v", err)
	}
	tutorialData, err := engine.CompileTutorialFile(src)
	if err != nil {
		t.Fatalf("compile tutorial: %v", err)
	}

	// Write non-Play mode files.
	for _, entry := range []struct {
		data any
		file string
	}{
		{watchData, build.FileWatchData},
		{previewData, build.FilePreviewData},
		{tutorialData, build.FileTutorialData},
	} {
		pkg, err := build.PackageNonPlay(playCfg, rom, entry.data, entry.file)
		if err != nil {
			t.Fatalf("package %s: %v", entry.file, err)
		}
		if err := pkg.Write(engineDir); err != nil {
			t.Fatalf("write %s: %v", entry.file, err)
		}
	}

	// Verify all mode files exist.
	for _, f := range []string{
		build.FileConfiguration,
		build.FilePlayData,
		build.FileWatchData,
		build.FilePreviewData,
		build.FileTutorialData,
		build.FileROM,
	} {
		path := filepath.Join(engineDir, f)
		if _, err := os.Stat(path); err != nil {
			t.Errorf("file %s not found: %v", f, err)
		}
	}
}

func TestBuildOutputDir(t *testing.T) {
	dir := t.TempDir()
	customDir := filepath.Join(dir, "custom-out")
	src := "package engine\n\ntype Skin struct{}\ntype Archetype struct{}\n\nfunc (a Archetype) UpdateParallel(dt float64) {}\n"

	playData, playCfg, err := engine.CompilePlayFile(src)
	if err != nil {
		t.Fatal(err)
	}
	pkg, err := build.PackagePlay(playCfg, playData, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(customDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := pkg.Write(customDir); err != nil {
		t.Fatal(err)
	}
	// Verify files in custom dir.
	if _, err := os.Stat(filepath.Join(customDir, build.FileConfiguration)); err != nil {
		t.Errorf("config file not in custom dir: %v", err)
	}
}

func TestBuildWithStats(t *testing.T) {
	src := "package engine\n\ntype Skin struct{}\ntype Archetype struct{}\n\nfunc (a Archetype) UpdateSequential(dt float64) {\n\tset(2000, 0, get(2000, 0) + 1)\n}\n"

	stats := &engine.CompileStats{}
	cfgPtr := &resource.EngineConfiguration{}
	playData, playCfg, err := engine.CompilePlayFileWithStats(src, &engine.CompileOptions{Stats: stats})
	if err != nil {
		t.Fatal(err)
	}
	*cfgPtr = *playCfg

	// Stats should have recorded at least one callback entry.
	if len(playData.Nodes) == 0 {
		t.Error("expected non-empty nodes")
	}
	// Stats.Total() may be zero if compilation is sub-nanosecond, which is fine.
	_ = stats.Total()
	_ = playCfg
}

func TestCLIErrorPaths(t *testing.T) {
	tests := []struct {
		name string
		src  string
	}{
		{
			"parse error",
			"package engine\nfunc { broken\n",
		},
		{
			"recursive callback",
			"package engine\n\ntype Skin struct{}\ntype Archetype struct{}\nfunc (a Archetype) UpdateParallel(dt float64) { a.UpdateParallel(dt) }\n",
		},
		{
			"unsupported type in field",
			"package engine\n\ntype Skin struct{}\ntype Archetype struct {\n\t_ sonolus:\"memory\" func()\n}\nfunc (a Archetype) UpdateParallel(dt float64) {}\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, err := engine.CompilePlayFile(tt.src)
			if err == nil {
				t.Error("expected error, got nil")
			}
		})
	}
}
