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

func TestBuildUsesNewCompilerPackagePatterns(t *testing.T) {
	out := t.TempDir()
	if err := cmdBuild([]string{fixturePattern()}, "fixture", out, "all", 0, "", true); err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(out, "fixture")
	for _, name := range []string{build.FileConfiguration, build.FileROM, build.FilePlayData, build.FileWatchData, build.FilePreviewData, build.FileTutorialData} {
		if _, err := os.Stat(filepath.Join(dir, name)); err != nil {
			t.Errorf("missing %s: %v", name, err)
		}
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
	if len(rom) != 20 {
		t.Fatalf("raw ROM length = %d", len(rom))
	}
}

func TestBuildRejectsInvalidOptimization(t *testing.T) {
	err := cmdBuild([]string{fixturePattern()}, "fixture", t.TempDir(), "play", 3, "", false)
	if err == nil || !strings.Contains(err.Error(), "invalid optimization level 3") {
		t.Fatalf("error = %v", err)
	}
}

func TestResolveEngineName(t *testing.T) {
	if _, err := resolveEngineName([]string{"./..."}, ""); err == nil {
		t.Fatal("wildcard pattern did not require -name")
	}
	if got, err := resolveEngineName([]string{fixturePattern()}, ""); err != nil || got != "multimode" {
		t.Fatalf("name=%q err=%v", got, err)
	}
	if got, err := resolveEngineName([]string{"a", "b"}, "explicit"); err != nil || got != "explicit" {
		t.Fatalf("explicit name=%q err=%v", got, err)
	}
}

func TestDevServerRecompileIsAtomic(t *testing.T) {
	srv := &devServer{patterns: []string{fixturePattern()}, name: "fixture", watched: map[string]bool{}}
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
	if info["engine"] != "fixture" {
		t.Fatalf("info = %#v", info)
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
