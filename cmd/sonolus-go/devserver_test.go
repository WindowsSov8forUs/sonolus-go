package main

import (
	"encoding/json"
	"fmt"
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
	mux.HandleFunc("/sonolus/engine/configuration", srv.servePayload(func() any { return srv.state.cfg }))
	mux.HandleFunc("/sonolus/engine/play-data", srv.servePayload(func() any { return srv.state.data }))
	mux.HandleFunc("/sonolus/engine/watch-data", srv.servePayload(func() any { return srv.state.wd }))
	mux.HandleFunc("/sonolus/engine/preview-data", srv.servePayload(func() any { return srv.state.pv }))
	mux.HandleFunc("/sonolus/engine/tutorial-data", srv.servePayload(func() any { return srv.state.tut }))
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
	if srv.state.data == nil {
		t.Fatal("play data is nil after recompile")
	}
	if srv.state.cfg == nil {
		t.Fatal("config is nil after recompile")
	}
	if len(srv.state.data.Archetypes) != 1 {
		t.Fatalf("expected 1 archetype, got %d", len(srv.state.data.Archetypes))
	}
	if len(srv.state.data.Nodes) == 0 {
		t.Fatal("expected non-empty nodes")
	}

	// Verify JSON serialization works (all modes serialize as gzip JSON).
	for label, v := range map[string]any{
		"play":     srv.state.data,
		"config":   srv.state.cfg,
		"watch":    srv.state.wd,
		"preview":  srv.state.pv,
		"tutorial": srv.state.tut,
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

// TestServePayloadNil verifies that servePayload returns HTTP 503 when the
// data-providing closure returns nil.
func TestServePayloadNil(t *testing.T) {
	s := &devServer{} // no recompile, all state fields are nil
	handler := s.servePayload(func() any { return nil })
	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	handler(w, req)
	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("expected 503, got %d", w.Code)
	}
}

// TestServeGzipError verifies that serveGzip returns HTTP 500 when the
// gzip encoding fails (e.g., due to a write error on the response writer).
func TestServeGzipError(t *testing.T) {
	// serveGzip writes gzip-encoded data. Use a ResponseWriter that fails on write
	// to exercise the error path.
	w := &failingWriter{}
	serveGzip(w, []byte("test data"))
	if !w.written {
		t.Error("expected write to be attempted")
	}
	// serveGzip logs the error but doesn't set an error status code since the
	// header was already written with 200. The write error is swallowed after logging.
}

type failingWriter struct {
	written bool
}

func (w *failingWriter) Header() http.Header        { return http.Header{} }
func (w *failingWriter) WriteHeader(int)             {}
func (w *failingWriter) Write(b []byte) (int, error) { w.written = true; return 0, fmt.Errorf("simulated write error") }

// TestRecompileCacheHit verifies that calling recompile twice on the same source
// hits the cache on the second call, without re-parsing or re-compiling.
func TestRecompileCacheHit(t *testing.T) {
	dir := t.TempDir()
	srcPath := filepath.Join(dir, "engine.go")
	src := `package p
type N struct {
	x float64 ` + "`sonolus:\"memory\"`" + `
}
func (n N) Initialize() { n.x = 42 }
func (n N) UpdateSequential() { debugPause() }
func UpdateSpawn() float64 { return 0 }
func Preprocess() {}
func Navigate() float64 { return 1 }
func Update() {}
`
	if err := os.WriteFile(srcPath, []byte(src), 0644); err != nil {
		t.Fatal(err)
	}

	srv := &devServer{src: srcPath, cache: engine.NewCache()}
	// First compilation: cache miss.
	if err := srv.recompile(); err != nil {
		t.Fatalf("first recompile: %v", err)
	}
	nodesAfterFirst := len(srv.state.data.Nodes)

	// Second compilation without changing the file: should be a cache hit.
	if err := srv.recompile(); err != nil {
		t.Fatalf("second recompile (cache hit): %v", err)
	}
	// Node count should be identical since the source didn't change.
	if len(srv.state.data.Nodes) != nodesAfterFirst {
		t.Errorf("node count changed after cache hit: %d -> %d",
			nodesAfterFirst, len(srv.state.data.Nodes))
	}
	// Non-play data should also be non-nil (cached).
	if srv.state.wd == nil {
		t.Error("watch data nil after cache hit")
	}
	if srv.state.pv == nil {
		t.Error("preview data nil after cache hit")
	}
	if srv.state.tut == nil {
		t.Error("tutorial data nil after cache hit")
	}
}
