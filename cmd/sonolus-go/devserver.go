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

	"github.com/fsnotify/fsnotify"

	"github.com/WindowsSov8forUs/sonolus-core-go/core/resource"

	"github.com/WindowsSov8forUs/sonolus-go/compiler/build"
	"github.com/WindowsSov8forUs/sonolus-go/compiler/engine"
)

// devServerState holds the in-memory compiled engine state. It is swapped
// atomically under devServer.mu so that compilation does not block HTTP reads.
type devServerState struct {
	data       *resource.EnginePlayData
	cfg        *resource.EngineConfiguration
	rom        []byte
	wd         *resource.EngineWatchData
	pv         *resource.EnginePreviewData
	tut        *resource.EngineTutorialData
	modeErrors map[string]string // mode → last compile error (watch/preview/tutorial)
}

// devServer holds the in-memory compiled engine state and serves it over HTTP.
type devServer struct {
	mu    sync.RWMutex
	src   string
	cache *engine.CompileCache
	ctx   context.Context // passed to CompileOptions for cancellation support
	state devServerState
}

// recompile reads the source file and recompiles all four modes. Play mode
// failure is fatal (returns error) because the dev server cannot serve the
// engine without play data. Watch, preview, and tutorial mode failures are
// non-fatal — they are recorded in state.modeErrors and exposed via the
// /sonolus/engines/info endpoint so the user can inspect them without the
// server going down.
func (s *devServer) recompile() error {
	src, err := os.ReadFile(s.src)
	if err != nil {
		return fmt.Errorf("reading source: %w", err)
	}
	srcStr := string(src)

	// Compile to local variables first; only swap pointers under the lock.
	newState := devServerState{modeErrors: map[string]string{}}

	// ── Play mode ──
	playKey := engine.NewCacheKey("play", srcStr)
	if d, cfg := s.cache.GetPlay(playKey); d != nil {
		newState.data = d
		newState.cfg = cfg
	} else {
		playData, cfg, err := engine.CompilePlayFileWithStats(srcStr, &engine.CompileOptions{Context: s.ctx})
		if err != nil {
			return fmt.Errorf("compile play: %w", err)
		}
		newState.data = playData
		newState.cfg = cfg
		s.cache.PutPlay(playKey, playData, cfg)
	}

	rom, err := build.DefaultROMBytes()
	if err != nil {
		return fmt.Errorf("build rom: %w", err)
	}
	newState.rom = rom

	// ── Watch mode ──
	compileNonPlay(s, "watch",
		func(k engine.CacheKey) (*resource.EngineWatchData, bool) { d := s.cache.GetWatch(k); return d, d != nil },
		func(k engine.CacheKey, d *resource.EngineWatchData) { s.cache.PutWatch(k, d) },
		func(src string, opts *engine.CompileOptions) (*resource.EngineWatchData, error) {
			return engine.CompileWatchFileWithStats(src, opts)
		},
		func(d *resource.EngineWatchData) { newState.wd = d },
		srcStr, newState.modeErrors,
	)

	// ── Preview mode ──
	compileNonPlay(s, "preview",
		func(k engine.CacheKey) (*resource.EnginePreviewData, bool) { d := s.cache.GetPreview(k); return d, d != nil },
		func(k engine.CacheKey, d *resource.EnginePreviewData) { s.cache.PutPreview(k, d) },
		func(src string, opts *engine.CompileOptions) (*resource.EnginePreviewData, error) {
			return engine.CompilePreviewFileWithStats(src, opts)
		},
		func(d *resource.EnginePreviewData) { newState.pv = d },
		srcStr, newState.modeErrors,
	)

	// ── Tutorial mode ──
	compileNonPlay(s, "tutorial",
		func(k engine.CacheKey) (*resource.EngineTutorialData, bool) { d := s.cache.GetTutorial(k); return d, d != nil },
		func(k engine.CacheKey, d *resource.EngineTutorialData) { s.cache.PutTutorial(k, d) },
		func(src string, opts *engine.CompileOptions) (*resource.EngineTutorialData, error) {
			return engine.CompileTutorialFileWithStats(src, opts)
		},
		func(d *resource.EngineTutorialData) { newState.tut = d },
		srcStr, newState.modeErrors,
	)

	// Atomic pointer swap — the only part that needs the write lock.
	s.mu.Lock()
	s.state = newState
	s.mu.Unlock()

	fmt.Printf("[dev] recompiled: %d nodes, %d archetypes, %d options\n",
		len(newState.data.Nodes), len(newState.data.Archetypes), len(newState.cfg.Options))
	return nil
}

// compileNonPlay is a generic helper for the repeated cache-check-then-compile
// pattern used by optional (non-Play) modes. Errors are recorded in modeErrors
// but do not prevent the server from starting.
func compileNonPlay[T any](
	s *devServer,
	mode string,
	getter func(engine.CacheKey) (T, bool),
	putter func(engine.CacheKey, T),
	compiler func(string, *engine.CompileOptions) (T, error),
	setter func(T),
	srcStr string,
	modeErrors map[string]string,
) {
	key := engine.NewCacheKey(mode, srcStr)
	if val, ok := getter(key); ok {
		setter(val)
		return
	}
	val, err := compiler(srcStr, &engine.CompileOptions{Context: s.ctx})
	if err != nil {
		fmt.Fprintf(os.Stderr, "[dev] %s compile: %v\n", mode, err)
		modeErrors[mode] = err.Error()
		return
	}
	putter(key, val)
	setter(val)
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
	if _, err := w.Write(s.state.rom); err != nil {
		fmt.Fprintf(os.Stderr, "[dev] write rom: %v\n", err)
	}
}

func (s *devServer) serveInfo(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	info := map[string]any{
		"engine": filepath.Base(s.src),
	}
	if s.state.data != nil {
		info["nodes"] = len(s.state.data.Nodes)
		info["arches"] = len(s.state.data.Archetypes)
	}
	if s.state.cfg != nil {
		info["options"] = len(s.state.cfg.Options)
	}
	info["hasWatch"] = s.state.wd != nil
	info["hasPreview"] = s.state.pv != nil
	info["hasTutorial"] = s.state.tut != nil
	if len(s.state.modeErrors) > 0 {
		info["errors"] = s.state.modeErrors
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

	srv := &devServer{src: srcPath, cache: engine.NewCache(), ctx: ctx}
	if err := srv.recompile(); err != nil {
		return err
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/sonolus/engines/info", srv.serveInfo)
	mux.HandleFunc("/sonolus/engine/configuration", srv.servePayload(func() any { return srv.state.cfg }))
	mux.HandleFunc("/sonolus/engine/play-data", srv.servePayload(func() any { return srv.state.data }))
	mux.HandleFunc("/sonolus/engine/watch-data", srv.servePayload(func() any { return srv.state.wd }))
	mux.HandleFunc("/sonolus/engine/preview-data", srv.servePayload(func() any { return srv.state.pv }))
	mux.HandleFunc("/sonolus/engine/tutorial-data", srv.servePayload(func() any { return srv.state.tut }))
	mux.HandleFunc("/sonolus/engine/rom", srv.serveRom)

	// Watch for source file changes using fsnotify (event-driven, no polling).
	// Triggers recompile on Write/Create events. Cancelled when runDevServer returns.
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return fmt.Errorf("fsnotify: %w", err)
	}
	defer watcher.Close()
	if err := watcher.Add(srcPath); err != nil {
		return fmt.Errorf("fsnotify add: %w", err)
	}
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case event := <-watcher.Events:
				if event.Op&(fsnotify.Write|fsnotify.Create) != 0 {
					if err := srv.recompile(); err != nil {
						fmt.Fprintf(os.Stderr, "[dev] recompile error: %v\n", err)
					}
				}
			case err := <-watcher.Errors:
				fmt.Fprintf(os.Stderr, "[dev] watcher error: %v\n", err)
			}
		}
	}()

	fmt.Printf("[dev] serving on http://localhost%s\n", addr)
	return http.ListenAndServe(addr, mux)
}
