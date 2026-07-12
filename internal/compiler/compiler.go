package compiler

import (
	"errors"
	"fmt"
	"maps"
	"reflect"
	"sort"
	"strings"
	"sync"
	"time"

	"golang.org/x/tools/go/packages"

	"github.com/WindowsSov8forUs/sonolus-go/v2/internal/compiler/backend"
	"github.com/WindowsSov8forUs/sonolus-go/v2/internal/compiler/frontend"
	"github.com/WindowsSov8forUs/sonolus-go/v2/internal/compiler/mode"
	"github.com/WindowsSov8forUs/sonolus-go/v2/internal/compiler/optimize"
	"github.com/WindowsSov8forUs/sonolus-go/v2/internal/compiler/source"
)

type Options struct {
	Optimization optimize.Level
	FallbackROM  []byte
}

type ModeStats struct {
	Load     time.Duration
	Frontend time.Duration
}

type CompileStats struct {
	Load, Frontend, Optimize, Backend, Total time.Duration
	Modes                                    map[mode.Mode]ModeStats
	Cached                                   bool
}

type Artifacts = backend.Artifacts

type Compiler struct {
	mu             sync.Mutex
	optimizer      *optimize.Optimizer
	fallbackROM    []byte
	patterns       []string
	packages       map[mode.Mode]*packages.Package
	schemaPackages map[mode.Mode]*packages.Package
	schemaResult   *ProjectSchema
	result         *Artifacts
	stats          CompileStats
	files          []string
}

func NewCompiler(options Options, patterns ...string) *Compiler {
	return &Compiler{
		optimizer:      optimize.NewOptimizer(options.Optimization),
		fallbackROM:    append([]byte(nil), options.FallbackROM...),
		patterns:       append([]string(nil), patterns...),
		packages:       make(map[mode.Mode]*packages.Package, len(orderedModes)),
		schemaPackages: make(map[mode.Mode]*packages.Package, 3),
	}
}

func (c *Compiler) Compile(requested ...mode.Mode) (*Artifacts, error) {
	started := time.Now()
	modes, err := normalizeModes(requested)
	if err != nil {
		return nil, err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	stats := CompileStats{Modes: make(map[mode.Mode]ModeStats, len(orderedModes))}
	defer func() {
		stats.Total = time.Since(started)
		c.stats = cloneStats(stats)
	}()

	candidate := make(map[mode.Mode]*packages.Package, len(c.packages)+len(modes))
	maps.Copy(candidate, c.packages)
	var pending []mode.Mode
	needsCompilation := false
	for _, m := range modes {
		if c.packages[m] == nil {
			needsCompilation = true
		}
		if candidate[m] == nil && c.schemaPackages[m] != nil {
			candidate[m] = c.schemaPackages[m]
		}
		if candidate[m] == nil {
			pending = append(pending, m)
		}
	}
	if !needsCompilation && c.result != nil {
		stats.Cached = true
		return cloneArtifacts(c.result), nil
	}
	loadStarted := time.Now()
	loaded, loadDurations, err := c.loadModes(pending)
	stats.Load = time.Since(loadStarted)
	for m, duration := range loadDurations {
		value := stats.Modes[m]
		value.Load = duration
		stats.Modes[m] = value
	}
	if err != nil {
		return nil, err
	}
	maps.Copy(candidate, loaded)

	parser := frontend.NewParser()
	frontendStarted := time.Now()
	for _, m := range orderedModes {
		if pkg := candidate[m]; pkg != nil {
			modeStarted := time.Now()
			if err := parser.Parse(m, pkg); err != nil {
				return nil, fmt.Errorf("compiler: parse %s mode: %w", m, err)
			}
			value := stats.Modes[m]
			value.Frontend = time.Since(modeStarted)
			stats.Modes[m] = value
		}
	}
	stats.Frontend = time.Since(frontendStarted)
	project, err := parser.GetProject()
	if err != nil {
		return nil, fmt.Errorf("compiler: merge frontend project: %w", err)
	}
	if len(project.ROM) == 0 && len(c.fallbackROM) != 0 {
		project.ROM = append([]byte(nil), c.fallbackROM...)
	}
	optimizeStarted := time.Now()
	project, err = optimizeProject(c.optimizer, project)
	stats.Optimize = time.Since(optimizeStarted)
	if err != nil {
		return nil, err
	}
	backendStarted := time.Now()
	result, err := backend.Compile(project)
	stats.Backend = time.Since(backendStarted)
	if err != nil {
		return nil, err
	}
	c.packages = candidate
	c.result = cloneArtifacts(result)
	c.files = sourceFiles(candidate)
	return cloneArtifacts(result), nil
}

func (c *Compiler) CompileAll() (*Artifacts, error) {
	return c.Compile(orderedModes...)
}

var orderedModes = []mode.Mode{mode.ModePlay, mode.ModeWatch, mode.ModePreview, mode.ModeTutorial}

func normalizeModes(requested []mode.Mode) ([]mode.Mode, error) {
	if len(requested) == 0 {
		return nil, fmt.Errorf("compiler: at least one mode is required")
	}
	wanted := make(map[mode.Mode]bool, len(requested))
	var errs []error
	for _, m := range requested {
		if !m.Valid() {
			errs = append(errs, fmt.Errorf("invalid Sonolus mode %q; expected play, watch, preview, or tutorial", m))
			continue
		}
		wanted[m] = true
	}
	if len(errs) != 0 {
		return nil, errors.Join(errs...)
	}
	result := make([]mode.Mode, 0, len(wanted))
	for _, m := range orderedModes {
		if wanted[m] {
			result = append(result, m)
		}
	}
	return result, nil
}

func (c *Compiler) loadModes(modes []mode.Mode) (map[mode.Mode]*packages.Package, map[mode.Mode]time.Duration, error) {
	type loadResult struct {
		mode     mode.Mode
		pkg      *packages.Package
		err      error
		duration time.Duration
	}
	results := make(chan loadResult, len(modes))
	var wg sync.WaitGroup
	for _, m := range modes {
		wg.Add(1)
		go func(m mode.Mode) {
			defer wg.Done()
			started := time.Now()
			pkgs, err := source.LoadMode(m, c.patterns...)
			if err == nil && len(pkgs) != 1 {
				err = fmt.Errorf("expected exactly one main package, found %d", len(pkgs))
			}
			var pkg *packages.Package
			if err == nil {
				pkg = pkgs[0]
				if pkg.Name != "main" {
					err = fmt.Errorf("package %q is not main", pkg.PkgPath)
				}
			}
			results <- loadResult{mode: m, pkg: pkg, err: err, duration: time.Since(started)}
		}(m)
	}
	wg.Wait()
	close(results)

	byMode := make(map[mode.Mode]loadResult, len(modes))
	for result := range results {
		byMode[result.mode] = result
	}
	loaded := make(map[mode.Mode]*packages.Package, len(modes))
	durations := make(map[mode.Mode]time.Duration, len(modes))
	var messages []string
	for _, m := range orderedModes {
		result, ok := byMode[m]
		if !ok {
			continue
		}
		if result.err != nil {
			messages = append(messages, fmt.Sprintf("compiler: load %s mode: %v", m, result.err))
		} else {
			loaded[m] = result.pkg
		}
		durations[m] = result.duration
	}
	if len(messages) != 0 {
		return nil, durations, fmt.Errorf("%s", strings.Join(messages, "\n"))
	}
	return loaded, durations, nil
}

func (c *Compiler) Stats() CompileStats {
	c.mu.Lock()
	defer c.mu.Unlock()
	return cloneStats(c.stats)
}

func (c *Compiler) SourceFiles() []string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]string(nil), c.files...)
}

func cloneStats(stats CompileStats) CompileStats {
	result := stats
	result.Modes = make(map[mode.Mode]ModeStats, len(stats.Modes))
	maps.Copy(result.Modes, stats.Modes)
	return result
}

func sourceFiles(roots map[mode.Mode]*packages.Package) []string {
	seenPackages := map[*packages.Package]bool{}
	seenFiles := map[string]bool{}
	var visit func(*packages.Package)
	visit = func(pkg *packages.Package) {
		if pkg == nil || seenPackages[pkg] {
			return
		}
		seenPackages[pkg] = true
		if pkg.Module == nil || !pkg.Module.Main {
			return
		}
		for _, file := range append(append([]string(nil), pkg.GoFiles...), pkg.EmbedFiles...) {
			seenFiles[file] = true
		}
		for _, imported := range pkg.Imports {
			visit(imported)
		}
	}
	for _, pkg := range roots {
		visit(pkg)
	}
	files := make([]string, 0, len(seenFiles))
	for file := range seenFiles {
		files = append(files, file)
	}
	sort.Strings(files)
	return files
}

func cloneArtifacts(value *Artifacts) *Artifacts {
	if value == nil {
		return nil
	}
	cloned := cloneValue(reflect.ValueOf(value))
	return cloned.Interface().(*Artifacts)
}

func cloneValue(value reflect.Value) reflect.Value {
	if !value.IsValid() {
		return value
	}
	switch value.Kind() {
	case reflect.Pointer:
		if value.IsNil() {
			return reflect.Zero(value.Type())
		}
		result := reflect.New(value.Type().Elem())
		result.Elem().Set(cloneValue(value.Elem()))
		return result
	case reflect.Interface:
		if value.IsNil() {
			return reflect.Zero(value.Type())
		}
		result := reflect.New(value.Type()).Elem()
		result.Set(cloneValue(value.Elem()))
		return result
	case reflect.Struct:
		result := reflect.New(value.Type()).Elem()
		for i := 0; i < value.NumField(); i++ {
			result.Field(i).Set(cloneValue(value.Field(i)))
		}
		return result
	case reflect.Slice:
		if value.IsNil() {
			return reflect.Zero(value.Type())
		}
		result := reflect.MakeSlice(value.Type(), value.Len(), value.Len())
		for i := 0; i < value.Len(); i++ {
			result.Index(i).Set(cloneValue(value.Index(i)))
		}
		return result
	case reflect.Map:
		if value.IsNil() {
			return reflect.Zero(value.Type())
		}
		result := reflect.MakeMapWithSize(value.Type(), value.Len())
		iterator := value.MapRange()
		for iterator.Next() {
			result.SetMapIndex(cloneValue(iterator.Key()), cloneValue(iterator.Value()))
		}
		return result
	default:
		return value
	}
}
