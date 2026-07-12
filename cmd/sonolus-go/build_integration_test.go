package main

import (
	"compress/gzip"
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

	"github.com/WindowsSov8forUs/sonolus-go/internal/build"
	"github.com/WindowsSov8forUs/sonolus-go/internal/compiler"
)

func fixturePattern() string {
	return filepath.Join("..", "..", "internal", "compiler", "testdata", "multimode")
}

func publicConformancePattern() string {
	return filepath.Join("..", "..", "examples", "conformance")
}

func compilerFixturePattern(name string) string {
	return filepath.Join("..", "..", "internal", "compiler", "testdata", name)
}

func TestBuildCompilesPublicConformanceExample(t *testing.T) {
	out := t.TempDir()
	previousRoot := engineOutputRoot
	engineOutputRoot = out
	t.Cleanup(func() { engineOutputRoot = previousRoot })
	if err := cmdBuild([]string{publicConformancePattern()}, "conformance", "all", 0, "", true); err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(out, "conformance")
	for _, name := range []string{build.FileConfiguration, build.FileROM, build.FilePlayData, build.FileWatchData, build.FilePreviewData, build.FileTutorialData} {
		if _, err := os.Stat(filepath.Join(dir, name)); err != nil {
			t.Errorf("missing %s: %v", name, err)
		}
	}
	if _, err := os.Stat(filepath.Join(dir, "LevelData")); !os.IsNotExist(err) {
		t.Fatalf("build unexpectedly wrote LevelData: %v", err)
	}
	play, err := codec.Decompress[resource.EnginePlayData](mustRead(t, filepath.Join(dir, build.FilePlayData)))
	if err != nil || len(play.Skin.Sprites) == 0 {
		t.Fatalf("play round trip: sprites=%d err=%v", len(play.Skin.Sprites), err)
	}
	reader, err := gzip.NewReader(strings.NewReader(string(mustRead(t, filepath.Join(dir, build.FileROM)))))
	if err != nil {
		t.Fatal(err)
	}
	rom, err := io.ReadAll(reader)
	if err != nil {
		t.Fatal(err)
	}
	if len(rom) != 24 {
		t.Fatalf("raw ROM length = %d", len(rom))
	}
}

func TestBuildRejectsInvalidOptimization(t *testing.T) {
	err := cmdBuild([]string{fixturePattern()}, "fixture", "play", 3, "", false)
	if err == nil || !strings.Contains(err.Error(), "invalid optimization level 3") {
		t.Fatalf("error = %v", err)
	}
}

func TestCheckCompilesWithoutWritingArtifacts(t *testing.T) {
	root := t.TempDir()
	previousRoot := engineOutputRoot
	engineOutputRoot = filepath.Join(root, "dist")
	t.Cleanup(func() { engineOutputRoot = previousRoot })
	sentinel := filepath.Join(engineOutputRoot, "sentinel")
	if err := os.MkdirAll(engineOutputRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(sentinel, []byte("unchanged"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := cmdCheck([]string{publicConformancePattern()}, "all", 0, "", true); err != nil {
		t.Fatal(err)
	}
	if got := string(mustRead(t, sentinel)); got != "unchanged" {
		t.Fatalf("sentinel = %q", got)
	}
	entries, err := os.ReadDir(engineOutputRoot)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Name() != "sentinel" {
		t.Fatalf("check modified output directory: %v", entries)
	}
}

func TestCheckSupportsMultipleEnginesAndSingleMode(t *testing.T) {
	pattern := filepath.Join("..", "..", "examples", "...")
	if err := cmdCheck([]string{pattern}, "play", 1, "", false); err != nil {
		t.Fatal(err)
	}
}

func TestCheckSupportsStandardOptimization(t *testing.T) {
	if err := cmdCheck([]string{publicConformancePattern()}, "play", 2, "", false); err != nil {
		t.Fatal(err)
	}
}

func TestCheckReportsTargetCompilationFailure(t *testing.T) {
	err := cmdCheck([]string{compilerFixturePattern("invalid")}, "play", 0, "", false)
	if err == nil {
		t.Fatal("invalid engine passed check")
	}
	if !strings.Contains(err.Error(), "internal/compiler/testdata/invalid") {
		t.Fatalf("error lacks target package: %v", err)
	}
}

func TestCheckValidatesFallbackROM(t *testing.T) {
	rom := filepath.Join(t.TempDir(), "rom")
	if err := os.WriteFile(rom, []byte{1, 2, 3}, 0o644); err != nil {
		t.Fatal(err)
	}
	err := cmdCheck([]string{publicConformancePattern()}, "play", 0, rom, false)
	if err == nil || !strings.Contains(err.Error(), "length 3 is not divisible by 4") {
		t.Fatalf("error = %v", err)
	}
}

func TestCheckDoesNotParseDevelopmentLevel(t *testing.T) {
	pattern := "." + string(os.PathSeparator) + filepath.Join("testdata", "checklevel")
	if err := cmdCheck([]string{pattern}, "all", 0, "", false); err != nil {
		t.Fatalf("check parsed invalid development LevelData: %v", err)
	}
}

func TestNameTargets(t *testing.T) {
	targets := []compiler.Target{
		{PackagePath: "example.com/first/cmd/engine", ModulePath: "example.com/first"},
		{PackagePath: "example.com/second/cmd/engine", ModulePath: "example.com/second"},
	}
	named, err := nameTargets(targets, "")
	if err != nil {
		t.Fatal(err)
	}
	if named[0].name != "first" || named[1].name != "second" {
		t.Fatalf("names = %q, %q", named[0].name, named[1].name)
	}
	if _, err := nameTargets(targets, "combined"); err == nil || !strings.Contains(err.Error(), "-o requires exactly one engine") {
		t.Fatalf("multi-engine -o error = %v", err)
	}
	if got, err := nameTargets(targets[:1], "custom"); err != nil || got[0].name != "custom" {
		t.Fatalf("custom name = %#v, err = %v", got, err)
	}
	duplicate := []compiler.Target{
		{PackagePath: "example.com/shared/a", ModulePath: "example.com/shared"},
		{PackagePath: "example.com/shared/b", ModulePath: "example.com/shared"},
	}
	if _, err := nameTargets(duplicate, ""); err == nil || !strings.Contains(err.Error(), "ambiguous") {
		t.Fatalf("duplicate module name error = %v", err)
	}
}

func TestDevServerRecompileIsAtomic(t *testing.T) {
	srv := &devServer{patterns: []string{publicConformancePattern()}, name: "conformance", watched: map[string]bool{}}
	if err := srv.recompile(); err != nil {
		t.Fatal(err)
	}
	previous := srv.state.artifacts
	srv.patterns = []string{filepath.Join(t.TempDir(), "missing")}
	if err := srv.recompile(); err == nil {
		t.Fatal("invalid recompile succeeded")
	}
	if srv.state.artifacts != previous {
		t.Fatal("failed recompile replaced the served snapshot")
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/sonolus/engines/info", srv.serveInfo)
	mux.HandleFunc("/sonolus/engine/play-data", srv.servePayload(func(a *compiler.Artifacts) any { return a.Play }))
	mux.HandleFunc("/", srv.serveSonolus)
	server := httptest.NewServer(mux)
	defer server.Close()
	response, err := http.Get(server.URL + "/sonolus/engines/info")
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	var info map[string]any
	if err := json.NewDecoder(response.Body).Decode(&info); err != nil {
		t.Fatal(err)
	}
	if info["engine"] != "conformance" {
		t.Fatalf("info = %#v", info)
	}
	response, err = http.Get(server.URL + "/sonolus/info")
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("Sonolus info status = %d", response.StatusCode)
	}
	response, err = http.Get(server.URL + "/sonolus/levels/list")
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("Sonolus level list status = %d", response.StatusCode)
	}
}

func mustRead(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return data
}
