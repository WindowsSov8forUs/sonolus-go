package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"github.com/fsnotify/fsnotify"

	"github.com/WindowsSov8forUs/sonolus-go/v2/internal/build"
	"github.com/WindowsSov8forUs/sonolus-go/v2/internal/compiler"
	developmentserver "github.com/WindowsSov8forUs/sonolus-go/v2/internal/devserver"
	"github.com/WindowsSov8forUs/sonolus-go/v2/internal/level"
)

type devServerState struct {
	artifacts *compiler.Artifacts
	rom       []byte
	handler   http.Handler
}

type devServer struct {
	mu       sync.RWMutex
	patterns []string
	name     string
	fallback []byte
	stats    bool
	level    compiler.OptimizationLevel
	checks   compiler.RuntimeChecks
	watcher  *fsnotify.Watcher
	watched  map[string]bool
	state    devServerState
}

func (s *devServer) recompile() error {
	engineCompiler := compiler.NewCompiler(compiler.Options{Optimization: s.level, FallbackROM: s.fallback, RuntimeChecks: s.checks}, s.patterns...)
	artifacts, err := engineCompiler.CompileAll()
	if s.stats {
		printCompileStats(engineCompiler.Stats())
	}
	if err != nil {
		return fmt.Errorf("compile all modes: %w", err)
	}
	packaged, err := build.PackageArtifacts(artifacts)
	if err != nil {
		return err
	}
	if err := s.watchFiles(engineCompiler.SourceFiles()); err != nil {
		return err
	}
	development, err := level.LoadDevelopment(s.patterns...)
	if err != nil {
		return err
	}
	levels := make([]developmentserver.Level, len(development.Levels))
	for index, developmentLevel := range development.Levels {
		if err := level.Validate(developmentLevel.Data, artifacts); err != nil {
			return fmt.Errorf("validate development level %q: %w", developmentLevel.Name, err)
		}
		levelData, err := level.Package(developmentLevel.Data)
		if err != nil {
			return fmt.Errorf("package development level %q: %w", developmentLevel.Name, err)
		}
		levels[index] = developmentserver.Level{Name: developmentLevel.Name, Title: developmentLevel.Title, Data: levelData}
	}
	handler, err := developmentserver.New(s.name, artifacts, packaged, levels)
	if err != nil {
		return err
	}
	if err := s.watchFiles(development.Files); err != nil {
		return err
	}
	s.mu.Lock()
	s.state = devServerState{artifacts: artifacts, rom: packaged.ROM, handler: handler}
	s.mu.Unlock()
	slog.Info("[dev] recompiled", "nodes", len(artifacts.Play.Nodes), "archetypes", len(artifacts.Play.Archetypes), "options", len(artifacts.Configuration.Options))
	return nil
}

func (s *devServer) serveSonolus(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	handler := s.state.handler
	s.mu.RUnlock()
	if handler == nil {
		http.Error(w, "development server is not ready", http.StatusServiceUnavailable)
		return
	}
	handler.ServeHTTP(w, r)
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

func (s *devServer) servePayload(getter func(*compiler.Artifacts) any) http.HandlerFunc {
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

func runDevServer(patterns []string, outputName, addr string, optimization int, romPath, checksFlag string, stats bool) error {
	targets, err := compiler.DiscoverTargets(compiler.ModePlay, patterns...)
	if err != nil {
		return err
	}
	if len(targets) != 1 {
		return fmt.Errorf("dev requires exactly one engine, but package patterns matched %d", len(targets))
	}
	named, err := nameTargets(targets, outputName)
	if err != nil {
		return err
	}
	fallback, err := readFallbackROM(romPath)
	if err != nil {
		return err
	}
	level, err := parseOptLevel(optimization)
	if err != nil {
		return err
	}
	checks, err := parseRuntimeChecks(checksFlag)
	if err != nil {
		return err
	}
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return fmt.Errorf("fsnotify: %w", err)
	}
	defer watcher.Close()
	srv := &devServer{patterns: []string{named[0].target.PackagePath}, name: named[0].name, fallback: fallback, stats: stats, level: level, checks: checks, watcher: watcher, watched: map[string]bool{}}
	if err := srv.recompile(); err != nil {
		return err
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/sonolus/engines/info", srv.serveInfo)
	mux.HandleFunc("/sonolus/engine/configuration", srv.servePayload(func(a *compiler.Artifacts) any { return a.Configuration }))
	mux.HandleFunc("/sonolus/engine/play-data", srv.servePayload(func(a *compiler.Artifacts) any { return a.Play }))
	mux.HandleFunc("/sonolus/engine/watch-data", srv.servePayload(func(a *compiler.Artifacts) any { return a.Watch }))
	mux.HandleFunc("/sonolus/engine/preview-data", srv.servePayload(func(a *compiler.Artifacts) any { return a.Preview }))
	mux.HandleFunc("/sonolus/engine/tutorial-data", srv.servePayload(func(a *compiler.Artifacts) any { return a.Tutorial }))
	mux.HandleFunc("/sonolus/engine/rom", srv.serveROM)
	mux.HandleFunc("/", srv.serveSonolus)

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
	go runDevConsole(os.Stdin, os.Stdout, srv)
	slog.Info("[dev] serving", "address", addr)
	return http.ListenAndServe(addr, mux)
}

func runDevConsole(input io.Reader, output io.Writer, server *devServer) {
	scanner := bufio.NewScanner(input)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "help" {
			fmt.Fprintln(output, "commands: decode <code>, help")
			continue
		}
		parts := strings.Fields(line)
		if len(parts) != 2 || parts[0] != "decode" {
			if line != "" {
				fmt.Fprintln(output, "unknown command; type help")
			}
			continue
		}
		code, err := strconv.Atoi(parts[1])
		if err != nil {
			fmt.Fprintf(output, "invalid diagnostic code %q\n", parts[1])
			continue
		}
		server.mu.RLock()
		artifacts := server.state.artifacts
		message, exists := "", false
		if artifacts != nil {
			message, exists = artifacts.Diagnostics[code]
		}
		server.mu.RUnlock()
		if !exists {
			fmt.Fprintf(output, "unknown diagnostic code %d\n", code)
			continue
		}
		fmt.Fprintf(output, "%d: %s\n", code, message)
	}
}
