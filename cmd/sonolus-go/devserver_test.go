package main

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/WindowsSov8forUs/sonolus-go/v2/internal/compiler"
)

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
