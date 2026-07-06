package engine

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/WindowsSov8forUs/sonolus-go/internal/compiler/frontend"
	"github.com/WindowsSov8forUs/sonolus-go/internal/goparse"
)

// EngineSources bundles parsed Go packages for engine compilation.
// The main package is keyed by "" in the Packages map; imported packages
// are keyed by their import path.
//
// For backward compatibility with tests, the legacy fields Main and Imports
// can also be set. When Packages is nil, ResolveImports populates it from
// the legacy fields. New code should use Packages directly or construct via
// NewEngineSources / LoadEngineSources.
type EngineSources struct {
	// RootDir is the absolute path to the engine root directory.
	RootDir string

	// Packages holds all parsed packages, keyed by import path.
	// The main package uses key ""; imported packages use their import
	// path (e.g. "notes", "stage").
	Packages map[string]*goparse.Package

	// Main is the legacy way to provide main package source files.
	// Prefer setting Packages instead.
	Main map[string]string

	// Imports is the legacy way to provide imported package source files.
	// Prefer setting Packages instead.
	Imports map[string]map[string]string
}

// MainPkg returns the main package.
func (ess *EngineSources) MainPkg() *goparse.Package { return ess.Packages[""] }

// ImportPkg returns an imported package by its import path, or nil.
func (ess *EngineSources) ImportPkg(path string) *goparse.Package {
	return ess.Packages[path]
}

// ImportedFileCount returns the total number of source files across all
// imported packages.
func (ess *EngineSources) ImportedFileCount() int {
	n := 0
	for path, pkg := range ess.Packages {
		if path != "" {
			n += len(pkg.Files)
		}
	}
	return n
}

// NewSingleFileSources creates an EngineSources for a single source string.
// This is used by tests and backward-compatible single-file entry points.
func NewSingleFileSources(src string) *EngineSources {
	pkgs, err := goparse.LoadProjectFromFiles(map[string]string{"engine.go": src})
	if err != nil {
		// Single-file source should always parse; panic on programmer error.
		panic(fmt.Sprintf("NewSingleFileSources: %v", err))
	}
	return &EngineSources{RootDir: ".", Packages: pkgs}
}

// NewEngineSources creates an EngineSources from in-memory source maps.
// This is the multi-file entry point for tests and pre-loaded engines.
func NewEngineSources(main map[string]string, imports map[string]map[string]string) (*EngineSources, error) {
	pkgs, err := goparse.LoadProjectFromFiles(main)
	if err != nil {
		return nil, err
	}
	for path, files := range imports {
		pkg, err := goparse.ParseFiles(files)
		if err != nil {
			return nil, fmt.Errorf("import %q: %w", path, err)
		}
		pkg.Path = path
		pkgs[path] = pkg
	}
	return &EngineSources{RootDir: ".", Packages: pkgs}, nil
}

// LoadEngineSources detects whether path is a file or directory and loads
// the engine source accordingly.
func LoadEngineSources(path string) (*EngineSources, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("source: resolve path %q: %w", path, err)
	}

	filter := func(importPath string) bool {
		return !strings.Contains(importPath, ".") && importPath != "sonolus"
	}

	pkgs, err := goparse.LoadProject(path, filter)
	if err != nil {
		return nil, fmt.Errorf("source: %w", err)
	}

	return &EngineSources{RootDir: abs, Packages: pkgs}, nil
}

// ResolveImports ensures Packages is populated. If Packages is already set
// (via NewEngineSources / LoadEngineSources), this is a no-op. If legacy
// Main/Imports fields are set, they are parsed into Packages lazily.
func (ess *EngineSources) ResolveImports() error {
	if ess.Packages != nil {
		return nil
	}
	// Legacy path: populate Packages from Main and Imports raw maps.
	ess2, err := NewEngineSources(ess.Main, ess.Imports)
	if err != nil {
		return err
	}
	ess.Packages = ess2.Packages
	if ess.RootDir == "" {
		ess.RootDir = "."
	}
	return nil
}

// Access returns a frontend.EngineSourcesAccess for type-checking, avoiding a
// circular import (engine → frontend ← engine).
func (ess *EngineSources) Access() *frontend.EngineSourcesAccess {
	mainFiles := ess.Packages[""].Sources

	imports := make(map[string]map[string]string)
	for path, pkg := range ess.Packages {
		if path != "" {
			imports[path] = pkg.Sources
		}
	}

	return &frontend.EngineSourcesAccess{
		RootDir:   ess.RootDir,
		MainFiles: mainFiles,
		Imports:   imports,
	}
}
