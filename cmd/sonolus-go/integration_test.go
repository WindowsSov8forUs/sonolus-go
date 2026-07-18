package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/WindowsSov8forUs/sonolus-core-go/codec"
	"github.com/WindowsSov8forUs/sonolus-core-go/core/resource"

	"fmt"
	"github.com/WindowsSov8forUs/sonolus-go/v2/internal/build"
	"github.com/WindowsSov8forUs/sonolus-go/v2/internal/compiler"
)

func fixturePattern() string {
	return filepath.Join("..", "..", "internal", "compiler", "testdata", "multimode")
}

func publicConformancePattern() string {
	return filepath.Join("..", "..", "internal", "compiler", "testdata", "conformance")
}

func godoriPattern() string {
	return filepath.Join("..", "..", "godori")
}

func multiLevelPattern() string {
	return filepath.Join("..", "..", "internal", "level", "testdata", "development")
}

func compilerFixturePattern(name string) string {
	return filepath.Join("..", "..", "internal", "compiler", "testdata", name)
}

func TestBuildCompilesGodori(t *testing.T) {
	out := t.TempDir()
	previousRoot := engineOutputRoot
	engineOutputRoot = out
	t.Cleanup(func() { engineOutputRoot = previousRoot })
	if err := cmdBuild([]string{godoriPattern()}, "godori", "all", 0, "", "none", true); err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(out, "godori")
	for _, name := range []string{build.FileConfiguration, build.FilePlayData, build.FileWatchData, build.FilePreviewData, build.FileTutorialData} {
		if _, err := os.Stat(filepath.Join(dir, name)); err != nil {
			t.Errorf("missing %s: %v", name, err)
		}
	}
	if _, err := os.Stat(filepath.Join(dir, build.FileROM)); !os.IsNotExist(err) {
		t.Fatalf("build unexpectedly wrote %s: %v", build.FileROM, err)
	}
	if _, err := os.Stat(filepath.Join(dir, "LevelData")); !os.IsNotExist(err) {
		t.Fatalf("build unexpectedly wrote LevelData: %v", err)
	}
	play, err := codec.Decompress[resource.EnginePlayData](mustRead(t, filepath.Join(dir, build.FilePlayData)))
	if err != nil || len(play.Skin.Sprites) == 0 {
		t.Fatalf("play round trip: sprites=%d err=%v", len(play.Skin.Sprites), err)
	}
}

func TestBuildRejectsInvalidOptimization(t *testing.T) {
	err := cmdBuild([]string{fixturePattern()}, "fixture", "play", 3, "", "none", false)
	if err == nil || !strings.Contains(err.Error(), "invalid optimization level 3") {
		t.Fatalf("error = %v", err)
	}
}

func TestVetCompilesWithoutWritingArtifacts(t *testing.T) {
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

	if err := cmdVet([]string{publicConformancePattern()}, "all", 0, "", "none", true); err != nil {
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
		t.Fatalf("vet modified output directory: %v", entries)
	}
}

func TestVetSupportsMultipleEnginesAndSingleMode(t *testing.T) {
	if err := cmdVet([]string{godoriPattern(), publicConformancePattern()}, "play", 1, "", "none", false); err != nil {
		t.Fatal(err)
	}
}

func TestVetSupportsStandardOptimization(t *testing.T) {
	if err := cmdVet([]string{publicConformancePattern()}, "play", 2, "", "none", false); err != nil {
		t.Fatal(err)
	}
}

func TestVetReportsTargetCompilationFailure(t *testing.T) {
	err := cmdVet([]string{compilerFixturePattern("invalid")}, "play", 0, "", "none", false)
	if err == nil {
		t.Fatal("invalid engine passed vet")
	}
	if !strings.Contains(err.Error(), "internal/compiler/testdata/invalid") {
		t.Fatalf("error lacks target package: %v", err)
	}
}

func TestVetValidatesFallbackROM(t *testing.T) {
	rom := filepath.Join(t.TempDir(), "rom")
	if err := os.WriteFile(rom, []byte{1, 2, 3}, 0o644); err != nil {
		t.Fatal(err)
	}
	err := cmdVet([]string{publicConformancePattern()}, "play", 0, rom, "none", false)
	if err == nil || !strings.Contains(err.Error(), "length 3 is not divisible by 4") {
		t.Fatalf("error = %v", err)
	}
}

func TestVetDoesNotParseDevelopmentLevel(t *testing.T) {
	pattern := "." + string(os.PathSeparator) + filepath.Join("testdata", "checklevel")
	if err := cmdVet([]string{pattern}, "all", 0, "", "none", false); err != nil {
		t.Fatalf("vet parsed invalid development LevelData: %v", err)
	}
}

func TestListWritesStableSchemaJSONWithoutArtifacts(t *testing.T) {
	root := t.TempDir()
	previousRoot := engineOutputRoot
	engineOutputRoot = filepath.Join(root, "dist")
	t.Cleanup(func() { engineOutputRoot = previousRoot })
	var first, second bytes.Buffer
	if err := cmdList([]string{godoriPattern()}, &first); err != nil {
		t.Fatal(err)
	}
	if err := cmdList([]string{godoriPattern()}, &second); err != nil {
		t.Fatal(err)
	}
	if first.String() != second.String() {
		t.Fatalf("list output is not deterministic:\n%s\n%s", first.String(), second.String())
	}
	want := `{
  "archetypes": [
    {
      "name": "#BPM_CHANGE",
      "fields": [
        "#BEAT",
        "#BPM"
      ]
    },
    {
      "name": "#TIMESCALE_CHANGE",
      "fields": [
        "#BEAT",
        "#TIMESCALE"
      ]
    },
    {
      "name": "AccentTapNote",
      "fields": [
        "end_time",
        "#BEAT",
        "lane",
        "direction",
        "prev",
        "next"
      ]
    },
    {
      "name": "DirectionalFlickNote",
      "fields": [
        "end_time",
        "#BEAT",
        "lane",
        "direction",
        "prev",
        "next"
      ]
    },
    {
      "name": "FlickNote",
      "fields": [
        "end_time",
        "#BEAT",
        "lane",
        "direction",
        "prev",
        "next"
      ]
    },
    {
      "name": "HoldAnchorNote",
      "fields": [
        "end_time",
        "#BEAT",
        "lane",
        "direction",
        "prev",
        "next"
      ]
    },
    {
      "name": "HoldConnector",
      "fields": [
        "first",
        "second"
      ]
    },
    {
      "name": "HoldEndNote",
      "fields": [
        "end_time",
        "#BEAT",
        "lane",
        "direction",
        "prev",
        "next"
      ]
    },
    {
      "name": "HoldFlickNote",
      "fields": [
        "end_time",
        "#BEAT",
        "lane",
        "direction",
        "prev",
        "next"
      ]
    },
    {
      "name": "HoldHeadNote",
      "fields": [
        "end_time",
        "#BEAT",
        "lane",
        "direction",
        "prev",
        "next"
      ]
    },
    {
      "name": "HoldManager",
      "fields": []
    },
    {
      "name": "HoldTickNote",
      "fields": [
        "end_time",
        "#BEAT",
        "lane",
        "direction",
        "prev",
        "next"
      ]
    },
    {
      "name": "ScheduledLaneEffect",
      "fields": []
    },
    {
      "name": "SimLine",
      "fields": [
        "first",
        "second"
      ]
    },
    {
      "name": "Stage",
      "fields": []
    },
    {
      "name": "TapNote",
      "fields": [
        "end_time",
        "#BEAT",
        "lane",
        "direction",
        "prev",
        "next"
      ]
    }
  ]
}
`
	if first.String() != want {
		t.Fatalf("list output:\n%s\nwant:\n%s", first.String(), want)
	}
	if _, err := os.Stat(engineOutputRoot); !os.IsNotExist(err) {
		t.Fatalf("list unexpectedly created output directory: %v", err)
	}
}

func TestListRequiresExactlyOneEngine(t *testing.T) {
	var out bytes.Buffer
	err := cmdList([]string{godoriPattern(), publicConformancePattern()}, &out)
	if err == nil || !strings.Contains(err.Error(), "list requires exactly one engine") {
		t.Fatalf("error = %v", err)
	}
	if out.Len() != 0 {
		t.Fatalf("list wrote partial output: %q", out.String())
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
	srv := &devServer{patterns: []string{godoriPattern()}, name: "godori", watched: map[string]bool{}}
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
	if info["engine"] != "godori" {
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

func TestDevServerServesMultipleDevelopmentLevels(t *testing.T) {
	srv := &devServer{patterns: []string{multiLevelPattern()}, name: "multilevel", watched: map[string]bool{}}
	if err := srv.recompile(); err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(http.HandlerFunc(srv.serveSonolus))
	defer server.Close()
	response, err := http.Get(server.URL + "/sonolus/levels/list")
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("status = %d", response.StatusCode)
	}
	var list struct {
		Items []struct {
			Name string `json:"name"`
		} `json:"items"`
	}
	if err := json.NewDecoder(response.Body).Decode(&list); err != nil {
		t.Fatal(err)
	}
	if len(list.Items) != 2 || list.Items[0].Name != "alternate" || list.Items[1].Name != "basic" {
		t.Fatalf("levels = %#v", list.Items)
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
func TestDevServerEndpoints(t *testing.T) {
	srv := &devServer{patterns: []string{fixturePattern()}, name: "fixture", watched: map[string]bool{}}
	if err := srv.recompile(); err != nil {
		t.Fatal(err)
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/sonolus/engines/info", srv.serveInfo)
	mux.HandleFunc("/sonolus/engine/configuration", srv.servePayload(func(a *compiler.Artifacts) any { return a.Configuration }))
	mux.HandleFunc("/sonolus/engine/play-data", srv.servePayload(func(a *compiler.Artifacts) any { return a.Play }))
	mux.HandleFunc("/sonolus/engine/watch-data", srv.servePayload(func(a *compiler.Artifacts) any { return a.Watch }))
	mux.HandleFunc("/sonolus/engine/preview-data", srv.servePayload(func(a *compiler.Artifacts) any { return a.Preview }))
	mux.HandleFunc("/sonolus/engine/tutorial-data", srv.servePayload(func(a *compiler.Artifacts) any { return a.Tutorial }))
	mux.HandleFunc("/sonolus/engine/rom", srv.serveROM)
	for _, path := range []string{"/sonolus/engines/info", "/sonolus/engine/configuration", "/sonolus/engine/play-data", "/sonolus/engine/watch-data", "/sonolus/engine/preview-data", "/sonolus/engine/tutorial-data", "/sonolus/engine/rom"} {
		recorder := httptest.NewRecorder()
		mux.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, path, nil))
		if recorder.Code != http.StatusOK {
			t.Errorf("%s status = %d", path, recorder.Code)
		}
	}
}

func TestServePayloadWithoutSnapshot(t *testing.T) {
	srv := &devServer{}
	recorder := httptest.NewRecorder()
	srv.servePayload(func(a *compiler.Artifacts) any { return a.Play })(recorder, httptest.NewRequest(http.MethodGet, "/", nil))
	if recorder.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d", recorder.Code)
	}
}

func TestServeGzipWriteError(t *testing.T) {
	w := &failingWriter{}
	serveGzip(w, []byte("test"))
	if !w.written {
		t.Fatal("write was not attempted")
	}
}

type failingWriter struct{ written bool }

func (w *failingWriter) Header() http.Header { return http.Header{} }
func (w *failingWriter) WriteHeader(int)     {}
func (w *failingWriter) Write([]byte) (int, error) {
	w.written = true
	return 0, fmt.Errorf("simulated write error")
}
