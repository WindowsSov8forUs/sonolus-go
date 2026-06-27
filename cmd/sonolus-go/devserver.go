package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/WindowsSov8forUs/sonolus-core-go/core/resource"

	"github.com/WindowsSov8forUs/sonolus-go/compiler/build"
	"github.com/WindowsSov8forUs/sonolus-go/compiler/engine"
)

// devServer holds the in-memory compiled engine state and serves it over HTTP.
type devServer struct {
	mu      sync.RWMutex
	src     string
	data    *resource.EnginePlayData
	cfg     *resource.EngineConfiguration
	rom     []byte
	wd      *resource.EngineWatchData
	pv      *resource.EnginePreviewData
	tut     *resource.EngineTutorialData
	modTime time.Time
}

func (s *devServer) recompile() error {
	src, err := os.ReadFile(s.src)
	if err != nil {
		return fmt.Errorf("reading source: %w", err)
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	playData, cfg, err := engine.CompilePlayFile(string(src))
	if err != nil {
		return fmt.Errorf("compile play: %w", err)
	}
	s.data = playData
	s.cfg = cfg

	rom, err := build.BuildRom(build.DefaultRom())
	if err != nil {
		return fmt.Errorf("build rom: %w", err)
	}
	s.rom = rom

	// Best-effort other modes.
	wd, _ := engine.CompileWatchFile(string(src))
	s.wd = wd
	pv, _ := engine.CompilePreviewFile(string(src))
	s.pv = pv
	tut, _ := engine.CompileTutorialFile(string(src))
	s.tut = tut

	fmt.Printf("[dev] recompiled: %d nodes, %d archetypes, %d options\n",
		len(playData.Nodes), len(playData.Archetypes), len(cfg.Options))
	return nil
}

func (s *devServer) serveConfig(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	serveGzip(w, s.cfg)
}

func (s *devServer) servePlayData(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	serveGzip(w, s.data)
}

func (s *devServer) serveRom(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Encoding", "gzip")
	w.Write(s.rom)
}

func (s *devServer) serveInfo(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	info := map[string]any{
		"engine":      filepath.Base(s.src),
		"nodes":       len(s.data.Nodes),
		"arches":      len(s.data.Archetypes),
		"options":     len(s.cfg.Options),
		"hasWatch":    s.wd != nil,
		"hasPreview":  s.pv != nil,
		"hasTutorial": s.tut != nil,
	}
	json.NewEncoder(w).Encode(info)
}

func serveGzip(w http.ResponseWriter, data any) {
	blob, err := build.PackageAny(data)
	if err != nil {
		http.Error(w, fmt.Sprintf("packaging: %v", err), 500)
		return
	}
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Encoding", "gzip")
	w.Write(blob)
}

func runDevServer(srcPath string, port int) error {
	srv := &devServer{src: srcPath}
	if err := srv.recompile(); err != nil {
		return err
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/sonolus/engines/info", srv.serveInfo)
	mux.HandleFunc("/sonolus/engine/configuration", srv.serveConfig)
	mux.HandleFunc("/sonolus/engine/play-data", srv.servePlayData)
	mux.HandleFunc("/sonolus/engine/rom", srv.serveRom)

	// Watch loop for auto-recompile.
	go func() {
		for {
			info, err := os.Stat(srcPath)
			if err != nil {
				time.Sleep(time.Second)
				continue
			}
			if info.ModTime().After(srv.modTime) {
				srv.modTime = info.ModTime()
				if err := srv.recompile(); err != nil {
					fmt.Fprintf(os.Stderr, "[dev] recompile error: %v\n", err)
				}
			}
			time.Sleep(500 * time.Millisecond)
		}
	}()

	addr := fmt.Sprintf(":%d", port)
	fmt.Printf("[dev] serving on http://localhost%s\n", addr)
	return http.ListenAndServe(addr, mux)
}
