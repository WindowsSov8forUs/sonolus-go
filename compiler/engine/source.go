package engine

import (
	"fmt"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/WindowsSov8forUs/sonolus-go/compiler/frontend"
)

// EngineSources bundles all source files for a multi-file engine compilation.
// It supports both single-file (backward compatible) and directory-based
// engine projects.
type EngineSources struct {
	// RootDir is the absolute path to the engine root directory.
	// All import paths are resolved relative to this directory.
	RootDir string

	// Main holds the main package source files: filename (relative to RootDir)
	// → source code. The engine name and resources (Skin, Effect, Buckets, etc.)
	// are always defined in the main package.
	Main map[string]string

	// Imports holds imported sub-package source files: import path →
	// (filename → source code). Populated lazily by resolveAndLoadImports.
	Imports map[string]map[string]string
}

// NewSingleFileSources creates an EngineSources for a single source string.
// This is used by tests and backward-compatible single-file entry points.
// RootDir is set to the current working directory (or "." if unavailable).
func NewSingleFileSources(src string) *EngineSources {
	return &EngineSources{
		RootDir: ".",
		Main:    map[string]string{"engine.go": src},
		Imports: map[string]map[string]string{},
	}
}

// LoadEngineSources detects whether path is a file or directory and loads
// the engine source accordingly.
//
// - Single file: reads the file, sets RootDir to the file's parent directory.
// - Directory: reads all *.go files (non-recursive) into the Main package.
//
// Returns an error if path does not exist, is empty, or contains no .go files.
func LoadEngineSources(path string) (*EngineSources, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("source: resolve path %q: %w", path, err)
	}

	fi, err := os.Stat(abs)
	if err != nil {
		return nil, fmt.Errorf("source: %w", err)
	}

	if fi.IsDir() {
		return loadEngineDir(abs)
	}
	return loadEngineFile(abs)
}

// loadEngineFile reads a single .go file as the main package.
func loadEngineFile(absPath string) (*EngineSources, error) {
	src, err := os.ReadFile(absPath)
	if err != nil {
		return nil, fmt.Errorf("source: read %q: %w", absPath, err)
	}
	rootDir := filepath.Dir(absPath)
	name := filepath.Base(absPath)

	return &EngineSources{
		RootDir: rootDir,
		Main:    map[string]string{name: string(src)},
		Imports: map[string]map[string]string{},
	}, nil
}

// loadEngineDir reads all .go files in a directory into the main package.
func loadEngineDir(absDir string) (*EngineSources, error) {
	files, err := collectGoFiles(absDir)
	if err != nil {
		return nil, fmt.Errorf("source: %w", err)
	}
	if len(files) == 0 {
		return nil, fmt.Errorf("source: no .go files found in %q", absDir)
	}

	// Validate that all files share the same package name.
	if err := validatePackageName(files); err != nil {
		return nil, fmt.Errorf("source: %w", err)
	}

	return &EngineSources{
		RootDir: absDir,
		Main:    files,
		Imports: map[string]map[string]string{},
	}, nil
}

// collectGoFiles reads all *.go files (non-recursive) from a directory.
// Returns filename (base name) → source content. Skips test files (*_test.go).
func collectGoFiles(dir string) (map[string]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("read dir %q: %w", dir, err)
	}

	files := make(map[string]string)
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		fullPath := filepath.Join(dir, name)
		src, err := os.ReadFile(fullPath)
		if err != nil {
			return nil, fmt.Errorf("read %q: %w", fullPath, err)
		}
		files[name] = string(src)
	}
	return files, nil
}

// validatePackageName checks that all files have the same package declaration.
func validatePackageName(files map[string]string) error {
	fset := token.NewFileSet()
	var pkgName string
	// Sort filenames for deterministic error messages.
	names := make([]string, 0, len(files))
	for n := range files {
		names = append(names, n)
	}
	sort.Strings(names)

	for _, name := range names {
		src := files[name]
		f, err := parser.ParseFile(fset, name, src, parser.PackageClauseOnly)
		if err != nil {
			return fmt.Errorf("parse %q: %w", name, err)
		}
		if pkgName == "" {
			pkgName = f.Name.Name
		} else if f.Name.Name != pkgName {
			return fmt.Errorf("conflicting package names: %q uses %q, expected %q", name, f.Name.Name, pkgName)
		}
	}
	return nil
}

// Access returns a frontend.EngineSourcesAccess for type-checking, avoiding a
// circular import (engine → frontend ← engine).
func (ess *EngineSources) Access() *frontend.EngineSourcesAccess {
	return &frontend.EngineSourcesAccess{
		RootDir:   ess.RootDir,
		MainFiles: ess.Main,
	}
}

// ResolveImports scans the main package source for import declarations and
// loads each local import's source files. Standard library imports (e.g.
// "fmt", "math") are silently skipped — they are resolved by the type checker
// but not needed for compilation. Each import path must resolve to a
// subdirectory under RootDir.
//
// If Imports is already populated (e.g., from tests), scanning is skipped —
// pre-loaded imports take precedence over filesystem resolution.
func (ess *EngineSources) ResolveImports() error {
	// If imports are already pre-populated (e.g., from tests), trust them.
	if len(ess.Imports) > 0 {
		return nil
	}

	importPaths, err := scanImportPaths(ess.RootDir, ess.Main)
	if err != nil {
		return err
	}

	for _, impPath := range importPaths {
		resolvedDir := filepath.Join(ess.RootDir, impPath)
		files, err := collectGoFiles(resolvedDir)
		if err != nil {
			return fmt.Errorf("import %q: %w", impPath, err)
		}
		if len(files) == 0 {
			return fmt.Errorf("import %q: no .go files in %s", impPath, resolvedDir)
		}
		if err := validatePackageName(files); err != nil {
			return fmt.Errorf("import %q: %w", impPath, err)
		}
		ess.Imports[impPath] = files
	}
	return nil
}

// ImportedFileCount returns the total number of source files across all
// imported packages.
func (ess *EngineSources) ImportedFileCount() int {
	n := 0
	for _, pkg := range ess.Imports {
		n += len(pkg)
	}
	return n
}

// scanImportPaths parses the main package files and extracts import paths.
// Only bare-name imports (no dots, no slashes) are treated as local imports.
// Standard library paths like "fmt" are skipped because they contain no
// engine archetypes. Paths containing a "." (like "github.com/...") are
// treated as stdlib/external and skipped.
func scanImportPaths(rootDir string, files map[string]string) ([]string, error) {
	fset := token.NewFileSet()
	seen := map[string]bool{}
	var paths []string

	for name, src := range files {
		f, err := parser.ParseFile(fset, name, src, parser.ImportsOnly)
		if err != nil {
			return nil, fmt.Errorf("scan imports: parse %s: %w", name, err)
		}
		for _, imp := range f.Imports {
			// ImportSpec.Path.Value is a string literal including quotes.
			path := strings.Trim(imp.Path.Value, `"`)
			if path == "" {
				continue
			}
			// Skip stdlib/external imports (contain dots, e.g. "fmt", "math", "github.com/...").
			// Also skip the sonolus declaration stub package — it's resolved by the
			// custom engine importer and does not need filesystem access.
			if strings.Contains(path, ".") || path == "sonolus" {
				continue
			}
			if seen[path] {
				continue
			}
			seen[path] = true

			// Validate the directory exists.
			resolvedDir := filepath.Join(rootDir, path)
			fi, err := os.Stat(resolvedDir)
			if err != nil {
				return nil, fmt.Errorf("import %q: %w", path, err)
			}
			if !fi.IsDir() {
				return nil, fmt.Errorf("import %q: %s is not a directory", path, resolvedDir)
			}
			paths = append(paths, path)
		}
	}
	return paths, nil
}
