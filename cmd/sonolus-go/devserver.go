package main

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"path/filepath"
	"sync"

	"github.com/fsnotify/fsnotify"

	"github.com/WindowsSov8forUs/sonolus-go/internal/build"
	"github.com/WindowsSov8forUs/sonolus-go/internal/newcompiler"
	"github.com/WindowsSov8forUs/sonolus-go/internal/newcompiler/optimize"
)

type devServerState struct {
	artifacts *newcompiler.Artifacts
	rom       []byte
}

type devServer struct {
	mu       sync.RWMutex
	patterns []string
	name     string
	fallback []byte
	stats    bool
	watcher  *fsnotify.Watcher
	watched  map[string]bool
	state    devServerState
}

func (s *devServer) recompile() error {
	compiler := newcompiler.NewCompiler(newcompiler.Options{Optimization: optimize.LevelMinimal, FallbackROM: s.fallback}, s.patterns...)
	artifacts, err := compiler.CompileAll()
	if s.stats {
		printCompileStats(compiler.Stats())
	}
	if err != nil {
		return fmt.Errorf("compile all modes: %w", err)
	}
	packaged, err := build.PackageArtifacts(artifacts)
	if err != nil {
		return err
	}
	if err := s.watchFiles(compiler.SourceFiles()); err != nil {
		return err
	}
	s.mu.Lock()
	s.state = devServerState{artifacts: artifacts, rom: packaged.ROM}
	s.mu.Unlock()
	slog.Info("[dev] recompiled", "nodes", len(artifacts.Play.Nodes), "archetypes", len(artifacts.Play.Archetypes), "options", len(artifacts.Configuration.Options))
	return nil
}

func (s *devServer) watchFiles(files []string) error {
	if s.watcher == nil {
		return nil
	}
	for _, file := range files {
		dir := filepath.Dir(file)
		if s.watched[dir] {
			continue
		}
		if err := s.watcher.Add(dir); err != nil {
			return fmt.Errorf("watch %s: %w", dir, err)
		}
		s.watched[dir] = true
	}
	return nil
}

func (s *devServer) servePayload(getter func(*newcompiler.Artifacts) any) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		s.mu.RLock()
		defer s.mu.RUnlock()
		if s.state.artifacts == nil {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		serveGzip(w, getter(s.state.artifacts))
	}
}

func (s *devServer) serveROM(w http.ResponseWriter, _ *http.Request) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if len(s.state.rom) == 0 {
		w.WriteHeader(http.StatusServiceUnavailable)
		return
	}
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Encoding", "gzip")
	if _, err := w.Write(s.state.rom); err != nil {
		slog.Warn("dev server: write ROM", "error", err)
	}
}

func (s *devServer) serveInfo(w http.ResponseWriter, _ *http.Request) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	info := map[string]any{"engine": s.name}
	if artifacts := s.state.artifacts; artifacts != nil {
		info["nodes"] = len(artifacts.Play.Nodes)
		info["arches"] = len(artifacts.Play.Archetypes)
		info["options"] = len(artifacts.Configuration.Options)
		info["hasWatch"] = artifacts.Watch != nil
		info["hasPreview"] = artifacts.Preview != nil
		info["hasTutorial"] = artifacts.Tutorial != nil
	}
	if err := json.NewEncoder(w).Encode(info); err != nil {
		slog.Warn("dev server: encode info", "error", err)
	}
}

func serveGzip(w http.ResponseWriter, data any) {
	blob, err := build.PackageAny(data)
	if err != nil {
		http.Error(w, fmt.Sprintf("packaging: %v", err), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Encoding", "gzip")
	if _, err := w.Write(blob); err != nil {
		slog.Warn("dev server: write gzip response", "error", err)
	}
}

func runDevServer(patterns []string, explicitName, addr, romPath string, stats bool) error {
	name, err := resolveEngineName(patterns, explicitName)
	if err != nil {
		return err
	}
	fallback, err := readFallbackROM(romPath)
	if err != nil {
		return err
	}
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return fmt.Errorf("fsnotify: %w", err)
	}
	defer watcher.Close()
	srv := &devServer{patterns: append([]string(nil), patterns...), name: name, fallback: fallback, stats: stats, watcher: watcher, watched: map[string]bool{}}
	if err := srv.recompile(); err != nil {
		return err
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/sonolus/engines/info", srv.serveInfo)
	mux.HandleFunc("/sonolus/engine/configuration", srv.servePayload(func(a *newcompiler.Artifacts) any { return a.Configuration }))
	mux.HandleFunc("/sonolus/engine/play-data", srv.servePayload(func(a *newcompiler.Artifacts) any { return a.Play }))
	mux.HandleFunc("/sonolus/engine/watch-data", srv.servePayload(func(a *newcompiler.Artifacts) any { return a.Watch }))
	mux.HandleFunc("/sonolus/engine/preview-data", srv.servePayload(func(a *newcompiler.Artifacts) any { return a.Preview }))
	mux.HandleFunc("/sonolus/engine/tutorial-data", srv.servePayload(func(a *newcompiler.Artifacts) any { return a.Tutorial }))
	mux.HandleFunc("/sonolus/engine/rom", srv.serveROM)

	go func() {
		for {
			select {
			case event, ok := <-watcher.Events:
				if !ok {
					return
				}
				if event.Op&(fsnotify.Write|fsnotify.Create|fsnotify.Remove|fsnotify.Rename) != 0 {
					if err := srv.recompile(); err != nil {
						slog.Error("[dev] recompile error", "error", err)
					}
				}
			case err, ok := <-watcher.Errors:
				if !ok {
					return
				}
				slog.Error("[dev] watcher error", "error", err)
			}
		}
	}()
	slog.Info("[dev] serving", "address", addr)
	return http.ListenAndServe(addr, mux)
}
