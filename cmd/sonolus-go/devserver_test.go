package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/WindowsSov8forUs/sonolus-go/compiler/engine"
)

// TestDevServerEndpoints verifies that the dev server's HTTP endpoints return
// valid responses for a minimal engine source. The server compiles from a
// temporary file and serves the expected Sonolus API paths.
func TestDevServerEndpoints(t *testing.T) {
	// Create a minimal engine source in a temp directory.
	dir := t.TempDir()
	src := filepath.Join(dir, "engine.go")
	if err := os.WriteFile(src, []byte(`package p
type N struct {}
func (n N) Initialize() {}
`), 0644); err != nil {
		t.Fatal(err)
	}

	srv := &devServer{src: src, cache: engine.NewCache()}
	if err := srv.recompile(); err != nil {
		t.Fatalf("recompile: %v", err)
	}

	// Register handlers exactly as serve mode does.
	mux := http.NewServeMux()
	mux.HandleFunc("/sonolus/engines/info", srv.serveInfo)
	mux.HandleFunc("/sonolus/engine/configuration", srv.servePayload(func() any { return srv.cfg }))
	mux.HandleFunc("/sonolus/engine/play-data", srv.servePayload(func() any { return srv.data }))
	mux.HandleFunc("/sonolus/engine/watch-data", srv.servePayload(func() any { return srv.wd }))
	mux.HandleFunc("/sonolus/engine/preview-data", srv.servePayload(func() any { return srv.pv }))
	mux.HandleFunc("/sonolus/engine/tutorial-data", srv.servePayload(func() any { return srv.tut }))
	mux.HandleFunc("/sonolus/engine/rom", srv.serveRom)

	tests := []struct {
		path       string
		wantStatus int
	}{
		{"/sonolus/engines/info", 200},
		{"/sonolus/engine/configuration", 200},
		{"/sonolus/engine/play-data", 200},
		{"/sonolus/engine/watch-data", 200},
		{"/sonolus/engine/preview-data", 200},
		{"/sonolus/engine/tutorial-data", 200},
		{"/sonolus/engine/rom", 200},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			req := httptest.NewRequest("GET", tt.path, nil)
			w := httptest.NewRecorder()
			mux.ServeHTTP(w, req)
			if w.Code != tt.wantStatus {
				t.Errorf("got %d, want %d (body: %s)", w.Code, tt.wantStatus, trunc(w.Body.String(), 200))
			}
		})
	}
}

// TestDevServerRecompile verifies that recompile updates the server state
// without crashes when given valid source.
func TestDevServerRecompile(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "engine.go")
	if err := os.WriteFile(src, []byte(`package p
type N struct {
	x float64 `+"`sonolus:\"memory\"`"+`
}
func (n N) Initialize() {
	n.x = 42
}
`), 0644); err != nil {
		t.Fatal(err)
	}

	srv := &devServer{src: src, cache: engine.NewCache()}
	if err := srv.recompile(); err != nil {
		t.Fatalf("first recompile: %v", err)
	}

	// Verify non-empty output.
	if srv.data == nil {
		t.Fatal("play data is nil after recompile")
	}
	if srv.cfg == nil {
		t.Fatal("config is nil after recompile")
	}
	if len(srv.data.Archetypes) != 1 {
		t.Fatalf("expected 1 archetype, got %d", len(srv.data.Archetypes))
	}
	if len(srv.data.Nodes) == 0 {
		t.Fatal("expected non-empty nodes")
	}

	// Verify JSON serialization works (all modes serialize as gzip JSON).
	for label, v := range map[string]any{
		"play":     srv.data,
		"config":   srv.cfg,
		"watch":    srv.wd,
		"preview":  srv.pv,
		"tutorial": srv.tut,
	} {
		if v == nil {
			t.Errorf("%s data is nil", label)
			continue
		}
		b, err := json.Marshal(v)
		if err != nil {
			t.Errorf("%s: json marshal: %v", label, err)
		}
		if len(b) == 0 {
			t.Errorf("%s: empty json", label)
		}
	}
}

func trunc(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
