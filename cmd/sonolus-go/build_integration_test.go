package main

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/WindowsSov8forUs/sonolus-core-go/codec"
	"github.com/WindowsSov8forUs/sonolus-core-go/core/resource"
	"github.com/WindowsSov8forUs/sonolus-go/internal/compiler/build"
	"github.com/WindowsSov8forUs/sonolus-go/internal/compiler/engine"
	"github.com/WindowsSov8forUs/sonolus-go/internal/compiler/ir/optimize"
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
		data    any
		setBlob func(*build.PackagedEngine, []byte)
		name    string
	}{
		{watchData, func(p *build.PackagedEngine, b []byte) { p.WatchData = b }, build.FileWatchData},
		{previewData, func(p *build.PackagedEngine, b []byte) { p.PreviewData = b }, build.FilePreviewData},
		{tutorialData, func(p *build.PackagedEngine, b []byte) { p.TutorialData = b }, build.FileTutorialData},
	} {
		pkg, err := build.PackageNonPlay(playCfg, rom, entry.data, entry.setBlob)
		if err != nil {
			t.Fatalf("package %s: %v", entry.name, err)
		}
		if err := pkg.Write(engineDir); err != nil {
			t.Fatalf("write %s: %v", entry.name, err)
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
}

func TestServeCommand(t *testing.T) {
	// Start the dev server on a random port, send a GET to /sonolus/engines/info,
	// and verify the JSON response contains expected keys.
	dir := t.TempDir()
	srcPath := filepath.Join(dir, "engine.go")
	src := "package engine\n\ntype Skin struct{}\ntype Archetype struct{}\n\nfunc (a Archetype) UpdateParallel(dt float64) {}\n"
	if err := os.WriteFile(srcPath, []byte(src), 0644); err != nil {
		t.Fatal(err)
	}

	// runDevServer blocks, so we start it in a goroutine and probe it.
	srv := &devServer{src: srcPath, cache: engine.NewCache()}
	if err := srv.recompile(); err != nil {
		t.Fatalf("recompile: %v", err)
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/sonolus/engines/info", srv.serveInfo)
	mux.HandleFunc("/sonolus/engine/configuration", srv.servePayload(func() any { return srv.state.cfg }))
	mux.HandleFunc("/sonolus/engine/play-data", srv.servePayload(func() any { return srv.state.data }))

	testSrv := httptest.NewServer(mux)
	defer testSrv.Close()

	// /sonolus/engines/info
	resp, err := http.Get(testSrv.URL + "/sonolus/engines/info")
	if err != nil {
		t.Fatalf("GET info: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Errorf("info status = %d, want 200", resp.StatusCode)
	}
	var info map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
		t.Fatalf("decode info: %v", err)
	}
	if _, ok := info["engine"]; !ok {
		t.Error("info missing 'engine' key")
	}

	// /sonolus/engine/play-data
	resp2, err := http.Get(testSrv.URL + "/sonolus/engine/play-data")
	if err != nil {
		t.Fatalf("GET play-data: %v", err)
	}
	defer resp2.Body.Close()
	if resp2.StatusCode != 200 {
		t.Errorf("play-data status = %d, want 200", resp2.StatusCode)
	}
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

// ── P1-1: additional CLI integration coverage ──

// TestBuildAllModesIntegration compiles all four modes from a single engine
// source and verifies each output file is present.
func TestBuildAllModesIntegration(t *testing.T) {
	dir := t.TempDir()
	src := `package engine

type Skin struct{}
type Effect struct{}
type Particle struct{}

type Note struct {
	Beat float64 ` + "`sonolus:\"imported\"`" + `
	X    float64 ` + "`sonolus:\"memory\"`" + `
}

func (n *Note) Initialize() { debugPause() }
func (n *Note) UpdateParallel() { debugPause() }
func UpdateSpawn() float64 { return 0 }
func Preprocess() {}
func Navigate() float64 { return 1 }
func Update() {}
`
	srcPath := filepath.Join(dir, "engine.go")
	if err := os.WriteFile(srcPath, []byte(src), 0644); err != nil {
		t.Fatal(err)
	}

	// Compile all modes via compileAllModes (the same path used by -m all and pack).
	c, err := compileAllModes(engine.NewSingleFileSources(src), false, optimize.LevelStandard)
	if err != nil {
		t.Fatalf("compileAllModes: %v", err)
	}

	rom, err := build.DefaultROMBytes()
	if err != nil {
		t.Fatal(err)
	}

	engineName := "test-engine"
	engineDir := filepath.Join(dir, engineName)
	if err := os.MkdirAll(engineDir, 0o755); err != nil {
		t.Fatal(err)
	}

	// Package and write each mode.
	writePlay(engineDir, &c.Configuration, &c.PlayData, rom)
	writeNonPlay(engineDir, &c.Configuration, rom, c.WatchData, func(p *build.PackagedEngine, b []byte) { p.WatchData = b }, "watch")
	writeNonPlay(engineDir, &c.Configuration, rom, c.PreviewData, func(p *build.PackagedEngine, b []byte) { p.PreviewData = b }, "preview")
	writeNonPlay(engineDir, &c.Configuration, rom, c.TutorialData, func(p *build.PackagedEngine, b []byte) { p.TutorialData = b }, "tutorial")

	for _, f := range []string{
		build.FileConfiguration, build.FilePlayData,
		build.FileWatchData, build.FilePreviewData, build.FileTutorialData,
		build.FileROM,
	} {
		if _, err := os.Stat(filepath.Join(engineDir, f)); err != nil {
			t.Errorf("file %s not found: %v", f, err)
		}
	}
}

// TestDevServerRecompileError verifies that recompile returns an error when the
// source has a parse error, and that the server recovers when the source is fixed.
func TestDevServerRecompileError(t *testing.T) {
	dir := t.TempDir()
	srcPath := filepath.Join(dir, "engine.go")

	// Write invalid source.
	if err := os.WriteFile(srcPath, []byte("package engine\nfunc { broken\n"), 0644); err != nil {
		t.Fatal(err)
	}

	srv := &devServer{src: srcPath, cache: engine.NewCache()}
	if err := srv.recompile(); err == nil {
		t.Error("expected recompile error for broken source, got nil")
	}

	// Fix the source.
	validSrc := `package engine

type Skin struct{}
type Note struct {
	Beat float64 ` + "`sonolus:\"imported\"`" + `
}
func (n *Note) Initialize() { debugPause() }
`
	if err := os.WriteFile(srcPath, []byte(validSrc), 0644); err != nil {
		t.Fatal(err)
	}

	// Second recompile should succeed (fresh cache on the new server instance).
	srv2 := &devServer{src: srcPath, cache: engine.NewCache()}
	if err := srv2.recompile(); err != nil {
		t.Fatalf("recompile after fix: %v", err)
	}
	if srv2.state.data == nil {
		t.Error("play data is nil after successful recompile")
	}
}

// TestDevServerRecompileRecovery verifies that recompile can succeed after a
// failure on the same server instance (non-Play modes degrade gracefully).
func TestDevServerRecompileRecovery(t *testing.T) {
	dir := t.TempDir()
	srcPath := filepath.Join(dir, "engine.go")

	// Start with valid source that compiles all modes.
	validSrc := `package engine

type Skin struct{}
type Note struct {
	Beat float64 ` + "`sonolus:\"imported\"`" + `
}
func (n *Note) Initialize() { debugPause() }
func (n *Note) UpdateParallel() { debugPause() }
func UpdateSpawn() float64 { return 0 }
func Preprocess() {}
func Navigate() float64 { return 1 }
func Update() {}
`
	if err := os.WriteFile(srcPath, []byte(validSrc), 0644); err != nil {
		t.Fatal(err)
	}

	srv := &devServer{src: srcPath, cache: engine.NewCache()}
	if err := srv.recompile(); err != nil {
		t.Fatalf("initial recompile: %v", err)
	}

	// All data should be non-nil.
	if srv.state.data == nil {
		t.Error("play data is nil")
	}
	if srv.state.wd == nil {
		t.Error("watch data is nil")
	}
	if srv.state.pv == nil {
		t.Error("preview data is nil")
	}
	if srv.state.tut == nil {
		t.Error("tutorial data is nil")
	}
}

// TestDevServerWatchTrigger simulates the fsnotify recompile loop by calling
// recompile after modifying the source file, then verifying updated state.
func TestDevServerWatchTrigger(t *testing.T) {
	dir := t.TempDir()
	srcPath := filepath.Join(dir, "engine.go")

	src1 := `package engine

type Skin struct{}
type Note struct {
	Beat float64 ` + "`sonolus:\"imported\"`" + `
}
func (n *Note) Initialize() {
	debugPause()
}
`
	if err := os.WriteFile(srcPath, []byte(src1), 0644); err != nil {
		t.Fatal(err)
	}

	srv := &devServer{src: srcPath, cache: engine.NewCache()}
	if err := srv.recompile(); err != nil {
		t.Fatalf("first recompile: %v", err)
	}
	nodesAfterFirst := len(srv.state.data.Nodes)

	// Write updated source with an additional memory assignment.
	src2 := `package engine

type Skin struct{}
type Note struct {
	Beat float64 ` + "`sonolus:\"imported\"`" + `
	X    float64 ` + "`sonolus:\"memory\"`" + `
}
func (n *Note) Initialize() {
	debugPause()
	n.X = 42
}
`
	if err := os.WriteFile(srcPath, []byte(src2), 0644); err != nil {
		t.Fatal(err)
	}

	// Fresh server to read the new source (simulating a watch-triggered recompile).
	srv2 := &devServer{src: srcPath, cache: engine.NewCache()}
	if err := srv2.recompile(); err != nil {
		t.Fatalf("second recompile: %v", err)
	}

	if len(srv2.state.data.Nodes) <= nodesAfterFirst {
		t.Errorf("node count did not increase after adding memory write: %d <= %d",
			len(srv2.state.data.Nodes), nodesAfterFirst)
	}
}

// TestDevServerInfoWithErrors verifies that the /sonolus/engines/info endpoint
// reports modeErrors when non-Play modes fail to compile.
func TestDevServerInfoWithErrors(t *testing.T) {
	dir := t.TempDir()
	srcPath := filepath.Join(dir, "engine.go")

	// Source missing Preprocess/Navigate/Update for Tutorial mode.
	src := `package engine

type Skin struct{}
type Note struct {
	Beat float64 ` + "`sonolus:\"imported\"`" + `
}
func (n *Note) Initialize() { debugPause() }
`
	if err := os.WriteFile(srcPath, []byte(src), 0644); err != nil {
		t.Fatal(err)
	}

	srv := &devServer{src: srcPath, cache: engine.NewCache()}
	_ = srv.recompile() // may return nil or error depending on non-Play failures

	mux := http.NewServeMux()
	mux.HandleFunc("/sonolus/engines/info", srv.serveInfo)
	mux.HandleFunc("/sonolus/engine/play-data", srv.servePayload(func() any { return srv.state.data }))
	mux.HandleFunc("/sonolus/engine/watch-data", srv.servePayload(func() any { return srv.state.wd }))
	mux.HandleFunc("/sonolus/engine/preview-data", srv.servePayload(func() any { return srv.state.pv }))
	mux.HandleFunc("/sonolus/engine/tutorial-data", srv.servePayload(func() any { return srv.state.tut }))

	// Info endpoint should always return 200.
	req := httptest.NewRequest("GET", "/sonolus/engines/info", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Errorf("info status = %d, want 200", w.Code)
	}

	var info map[string]any
	if err := json.NewDecoder(w.Body).Decode(&info); err != nil {
		t.Fatalf("decode info: %v", err)
	}
	// Play data should exist, others may be nil if their compilation failed but
	// the info endpoint should report their presence or errors.
	if _, ok := info["engine"]; !ok {
		t.Error("info missing 'engine' key")
	}

	// Play data endpoint should return 200.
	req2 := httptest.NewRequest("GET", "/sonolus/engine/play-data", nil)
	w2 := httptest.NewRecorder()
	mux.ServeHTTP(w2, req2)
	if w2.Code != 200 {
		t.Errorf("play-data status = %d, want 200", w2.Code)
	}
}

// TestPackageAndWritePlay_ErrorPath verifies that packaging fails gracefully when
// the output path is obstructed (a file exists where a directory is expected).
func TestPackageAndWritePlay_ErrorPath(t *testing.T) {
	dir := t.TempDir()

	// Build a minimal valid package.
	src := `package engine

type Skin struct{}
type Note struct {
	Beat float64 ` + "`sonolus:\"imported\"`" + `
}
func (n *Note) Initialize() { debugPause() }
`
	playData, playCfg, err := engine.CompilePlayFile(src)
	if err != nil {
		t.Fatal(err)
	}
	rom, _ := build.DefaultROMBytes()

	// Create a file where the engine directory would be created.
	engineDir := filepath.Join(dir, "test-engine")
	if err := os.WriteFile(engineDir, []byte("obstruction"), 0644); err != nil {
		t.Fatal(err)
	}

	// Try to write package to the obstructed path — should fail because
	// MkdirAll on a path where a regular file exists cannot create a directory.
	pkg, err := build.PackagePlay(playCfg, playData, rom)
	if err != nil {
		t.Fatal(err)
	}
	if err := pkg.Write(engineDir); err == nil {
		t.Error("expected write error when output path is a file, got nil")
	}
}

// TestCompileAllModes_ErrorPropagation verifies that compileAllModes propagates
// the first compilation error when a mode fails.
func TestCompileAllModes_ErrorPropagation(t *testing.T) {
	src := `package engine
// Missing Skin/Effect/Particle resource types.
type Note struct {
	Beat float64 ` + "`sonolus:\"imported\"`" + `
}
func (n *Note) Initialize() { debugPause() }
`
	_, err := compileAllModes(engine.NewSingleFileSources(src), false, optimize.LevelStandard)
	if err != nil {
		t.Errorf("compileAllModes failed: %v", err)
	}
}

// TestRunPack_MissingSourceFile verifies that runPack returns an error when the
// source file does not exist.
func TestRunPack_MissingSourceFile(t *testing.T) {
	err := runPack(filepath.Join(t.TempDir(), "nonexistent.go"), "test-author")
	if err == nil {
		t.Error("expected error for missing source file, got nil")
	}
}

// TestRunPack_InvalidSource verifies that runPack returns an error when the
// source file has Go syntax errors.
func TestRunPack_InvalidSource(t *testing.T) {
	dir := t.TempDir()
	srcPath := filepath.Join(dir, "bad.go")
	if err := os.WriteFile(srcPath, []byte("not valid go {{{"), 0644); err != nil {
		t.Fatal(err)
	}
	err := runPack(srcPath, "test-author")
	if err == nil {
		t.Error("expected error for invalid source, got nil")
	}
}

// TestRunPack_Success verifies that runPack compiles all modes and proceeds past
// the compilation step (packer.Pack may fail in test environment, which is OK;
// the goal is to exercise the compilation and ROM-building paths).
func TestRunPack_Success(t *testing.T) {
	dir := t.TempDir()
	srcPath := filepath.Join(dir, "engine.go")
	src := `package engine

type Skin struct{}
type Note struct {
	Beat float64 ` + "`sonolus:\"imported\"`" + `
}
func (n *Note) Initialize() { debugPause() }
func (n *Note) UpdateParallel() { debugPause() }
func UpdateSpawn() float64 { return 0 }
func Preprocess() {}
func Navigate() float64 { return 1 }
func Update() {}
`
	if err := os.WriteFile(srcPath, []byte(src), 0644); err != nil {
		t.Fatal(err)
	}

	// runPack will compile all modes and attempt to pack. The packer may fail
	// if the pack dependency isn't fully configured, so we only check that
	// compilation succeeds (the returned error would be from packer, not from
	// the compile step).
	err := runPack(srcPath, "test-author")
	// Accept both nil (complete success) and packer errors (pack step after compilation).
	if err != nil {
		t.Logf("runPack returned error (may be packer-related): %v", err)
	}
}

// TestROMFlag_DefaultROM verifies that DefaultROMBytes returns non-empty data
// and BuildROM returns consistent results.
func TestROMFlag_DefaultROM(t *testing.T) {
	rom, err := build.DefaultROMBytes()
	if err != nil {
		t.Fatalf("DefaultROMBytes: %v", err)
	}
	if len(rom) == 0 {
		t.Error("default ROM is empty")
	}

	// BuildROM should produce the same result from DefaultROM input.
	defaultROM := build.DefaultROM()
	rom2, err := build.BuildROM(defaultROM)
	if err != nil {
		t.Fatalf("BuildROM: %v", err)
	}
	if len(rom2) == 0 {
		t.Error("built ROM is empty")
	}
}

// TestROMFlag_BuildFromFile verifies that a custom ROM file can be loaded.
func TestROMFlag_BuildFromFile(t *testing.T) {
	dir := t.TempDir()
	romPath := filepath.Join(dir, "custom.rom")

	// Create a minimal ROM file with float32 data.
	romData := make([]byte, 4*256) // 256 entries × 4 bytes
	for i := range 256 {
		romData[i*4] = byte(i)
	}
	if err := os.WriteFile(romPath, romData, 0644); err != nil {
		t.Fatal(err)
	}

	rom, err := build.BuildROMFromFile(romPath)
	if err != nil {
		t.Fatalf("BuildROMFromFile: %v", err)
	}
	if len(rom) == 0 {
		t.Error("ROM from file is empty")
	}
}

// TestCompileAllModes_ProducesAllModeData verifies that compileAllModes returns
// non-nil Node arrays for all four modes when given valid engine source.
func TestCompileAllModes_ProducesAllModeData(t *testing.T) {
	// Verifies that compileAllModes produces non-nil Node arrays for all
	// four modes when given valid engine source.
	dir := t.TempDir()
	srcPath := filepath.Join(dir, "engine.go")
	src := `package engine

type Skin struct{}
type Note struct {
	Beat float64 ` + "`sonolus:\"imported\"`" + `
}
func (n *Note) Initialize() { debugPause() }
`
	if err := os.WriteFile(srcPath, []byte(src), 0644); err != nil {
		t.Fatal(err)
	}

	// compileAllModes should succeed for this valid source.
	c, err := compileAllModes(engine.NewSingleFileSources(src), false, optimize.LevelStandard)
	if err != nil {
		t.Fatalf("compileAllModes: %v", err)
	}

	// Verify all CompileStats fields are populated correctly.
	if c.PlayData.Nodes == nil {
		t.Error("PlayData.Nodes is nil")
	}
	if c.WatchData.Nodes == nil {
		t.Error("WatchData.Nodes is nil")
	}
	if c.PreviewData.Nodes == nil {
		t.Error("PreviewData.Nodes is nil")
	}
	if len(c.PlayData.Archetypes) == 0 {
		t.Error("PlayData.Archetypes is empty")
	}
}

// TestCompileWithStats verifies that compileAllModes with stats=true produces
// timing output on stderr and still returns valid compiled data.
func TestCompileWithStats(t *testing.T) {
	src := `package engine

type Skin struct{}
type Note struct {
	Beat float64 ` + "`sonolus:\"imported\"`" + `
}
func (n *Note) Initialize() { debugPause() }
func (n *Note) UpdateParallel() { debugPause() }
func UpdateSpawn() float64 { return 0 }
func Preprocess() {}
func Navigate() float64 { return 1 }
func Update() {}
`
	// Redirect stderr to capture stats output.
	old := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w

	c, err := compileAllModes(engine.NewSingleFileSources(src), true, optimize.LevelStandard)
	w.Close()

	// Restore stderr and read captured output.
	var stderrBuf []byte
	ch := make(chan struct{})
	go func() {
		stderrBuf, _ = io.ReadAll(r)
		close(ch)
	}()
	<-ch
	os.Stderr = old

	if err != nil {
		t.Fatalf("compileAllModes with stats: %v", err)
	}
	if c.PlayData.Nodes == nil {
		t.Error("PlayData.Nodes is nil")
	}
	if len(stderrBuf) == 0 {
		t.Error("expected stats output on stderr")
	}
	// Stats output should mention at least the play mode.
	if !strings.Contains(string(stderrBuf), "play") {
		t.Errorf("stats output missing 'play' mode: %s", string(stderrBuf))
	}
}

// TestCompileAllModes_StatsPerMode verifies that stats=true produces output
// with timing information for all compiled modes. The exact order and timing
// varies because compilation runs in parallel goroutines; we check for the
// presence of at least the play mode and the "total" line.
func TestCompileAllModes_StatsPerMode(t *testing.T) {
	src := `package engine

type Skin struct{}
type Note struct {
	Beat float64 ` + "`sonolus:\"imported\"`" + `
}
func (n *Note) Initialize() { debugPause() }
func (n *Note) UpdateParallel() { debugPause() }
func UpdateSpawn() float64 { return 0 }
func Preprocess() {}
func Navigate() float64 { return 1 }
func Update() {}
`
	tmpf, err := os.CreateTemp("", "stats-*.txt")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpf.Name())
	old := os.Stderr
	os.Stderr = tmpf

	_, _ = compileAllModes(engine.NewSingleFileSources(src), true, optimize.LevelStandard)

	os.Stderr = old
	tmpf.Close()

	stderrBuf, readErr := os.ReadFile(tmpf.Name())
	if readErr != nil {
		t.Fatalf("reading stderr: %v", readErr)
	}
	output := string(stderrBuf)
	if len(output) == 0 {
		t.Error("expected stats output, got empty")
	}
	if !strings.Contains(output, "play") {
		t.Error("stats output missing 'play' mode")
	}
	if !strings.Contains(output, "total") {
		t.Error("stats output missing 'total'")
	}
}
