package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	"github.com/WindowsSov8forUs/sonolus-core-go/core/resource"

	"github.com/WindowsSov8forUs/sonolus-go/compiler/build"
	"github.com/WindowsSov8forUs/sonolus-go/compiler/engine"
)

// devServer holds the in-memory compiled engine state and serves it over HTTP.
type devServer struct {
	mu          sync.RWMutex
	src         string
	cache       *engine.CompileCache
	data        *resource.EnginePlayData
	cfg         *resource.EngineConfiguration
	rom         []byte
	wd          *resource.EngineWatchData
	pv          *resource.EnginePreviewData
	tut         *resource.EngineTutorialData
	modTimeUnix atomic.Int64  // UnixNano timestamp of last known source file mod time
	modeErrors  map[string]string // mode → last compile error (watch/preview/tutorial)
}

func (s *devServer) recompile() error {
	src, err := os.ReadFile(s.src)
	if err != nil {
		return fmt.Errorf("reading source: %w", err)
	}
	srcStr := string(src)
	s.mu.Lock()
	defer s.mu.Unlock()

	// ── Play mode ──
	playKey := engine.NewCacheKey("play", srcStr)
	if d, cfg := s.cache.GetPlay(playKey); d != nil {
		s.data = d
		s.cfg = cfg
	} else {
		playData, cfg, err := engine.CompilePlayFile(srcStr)
		if err != nil {
			return fmt.Errorf("compile play: %w", err)
		}
		s.data = playData
		s.cfg = cfg
		s.cache.PutPlay(playKey, playData, cfg)
	}

	rom, err := build.DefaultROMBytes()
	if err != nil {
		return fmt.Errorf("build rom: %w", err)
	}
	s.rom = rom

	// ── Watch mode ──
	watchKey := engine.NewCacheKey("watch", srcStr)
	if d := s.cache.GetWatch(watchKey); d != nil {
		s.wd = d
		delete(s.modeErrors, "watch")
	} else if watchData, err := engine.CompileWatchFile(srcStr); err != nil {
		fmt.Fprintf(os.Stderr, "[dev] watch compile: %v\n", err)
		if s.modeErrors == nil {
			s.modeErrors = map[string]string{}
		}
		s.modeErrors["watch"] = err.Error()
	} else {
		s.wd = watchData
		s.cache.PutWatch(watchKey, watchData)
		delete(s.modeErrors, "watch")
	}

	// ── Preview mode ──
	previewKey := engine.NewCacheKey("preview", srcStr)
	if d := s.cache.GetPreview(previewKey); d != nil {
		s.pv = d
		delete(s.modeErrors, "preview")
	} else if previewData, err := engine.CompilePreviewFile(srcStr); err != nil {
		fmt.Fprintf(os.Stderr, "[dev] preview compile: %v\n", err)
		if s.modeErrors == nil {
			s.modeErrors = map[string]string{}
		}
		s.modeErrors["preview"] = err.Error()
	} else {
		s.pv = previewData
		s.cache.PutPreview(previewKey, previewData)
		delete(s.modeErrors, "preview")
	}

	// ── Tutorial mode ──
	tutorialKey := engine.NewCacheKey("tutorial", srcStr)
	if d := s.cache.GetTutorial(tutorialKey); d != nil {
		s.tut = d
		delete(s.modeErrors, "tutorial")
	} else if tutorialData, err := engine.CompileTutorialFile(srcStr); err != nil {
		fmt.Fprintf(os.Stderr, "[dev] tutorial compile: %v\n", err)
		if s.modeErrors == nil {
			s.modeErrors = map[string]string{}
		}
		s.modeErrors["tutorial"] = err.Error()
	} else {
		s.tut = tutorialData
		s.cache.PutTutorial(tutorialKey, tutorialData)
		delete(s.modeErrors, "tutorial")
	}

	fmt.Printf("[dev] recompiled: %d nodes, %d archetypes, %d options\n",
		len(s.data.Nodes), len(s.data.Archetypes), len(s.cfg.Options))
	return nil
}

// servePayload returns a handler that calls getter (under RLock) to obtain the
// current payload, then serves it as gzip JSON. The getter pattern ensures
// recompiled data is served instead of a stale pointer captured at handler
// registration time.
func (s *devServer) servePayload(getter func() any) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		s.mu.RLock()
		defer s.mu.RUnlock()
		serveGzip(w, getter())
	}
}

func (s *devServer) serveRom(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Encoding", "gzip")
	if _, err := w.Write(s.rom); err != nil {
		fmt.Fprintf(os.Stderr, "[dev] write rom: %v\n", err)
	}
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
	if len(s.modeErrors) > 0 {
		info["errors"] = s.modeErrors
	}
	if err := json.NewEncoder(w).Encode(info); err != nil {
		fmt.Fprintf(os.Stderr, "[dev] encode info: %v\n", err)
	}
}

func serveGzip(w http.ResponseWriter, data any) {
	blob, err := build.PackageAny(data)
	if err != nil {
		http.Error(w, fmt.Sprintf("packaging: %v", err), 500)
		return
	}
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Encoding", "gzip")
	if _, err := w.Write(blob); err != nil {
		slog.Warn("dev server: write gzip response failed", "error", err)
	}
}

func runDevServer(srcPath string, addr string) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	srv := &devServer{src: srcPath, cache: engine.NewCache()}
	// Initialize modTimeUnix from the current file state so the watch loop has a baseline.
	if info, err := os.Stat(srcPath); err == nil {
		srv.modTimeUnix.Store(info.ModTime().UnixNano())
	}
	if err := srv.recompile(); err != nil {
		return err
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/sonolus/engines/info", srv.serveInfo)
	mux.HandleFunc("/sonolus/engine/configuration", srv.servePayload(func() any { return srv.cfg }))
	mux.HandleFunc("/sonolus/engine/play-data", srv.servePayload(func() any { return srv.data }))
	mux.HandleFunc("/sonolus/engine/watch-data", srv.servePayload(func() any { return srv.wd }))
	mux.HandleFunc("/sonolus/engine/preview-data", srv.servePayload(func() any { return srv.pv }))
	mux.HandleFunc("/sonolus/engine/tutorial-data", srv.servePayload(func() any { return srv.tut }))
	mux.HandleFunc("/sonolus/engine/rom", srv.serveRom)

	// Watch loop for auto-recompile. Polls the source file mtime every 500ms
	// and triggers recompile when it changes. Cancelled when runDevServer returns.
	go func() {
		ticker := time.NewTicker(500 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				info, err := os.Stat(srcPath)
				if err != nil {
					continue
				}
				if info.ModTime().UnixNano() > srv.modTimeUnix.Load() {
					srv.modTimeUnix.Store(info.ModTime().UnixNano())
					if err := srv.recompile(); err != nil {
						fmt.Fprintf(os.Stderr, "[dev] recompile error: %v\n", err)
					}
				}
			}
		}
	}()

	fmt.Printf("[dev] serving on http://localhost%s\n", addr)
	return http.ListenAndServe(addr, mux)
}
