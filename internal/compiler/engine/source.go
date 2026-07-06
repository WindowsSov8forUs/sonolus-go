package engine

import (
	"fmt"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"

	"github.com/WindowsSov8forUs/sonolus-go/internal/compiler/frontend"
	"github.com/WindowsSov8forUs/sonolus-go/internal/goparse"
)

// EngineSources bundles all source files for a multi-file engine compilation.
// It supports both single-file (backward compatible) and directory-based
// engine projects.
type EngineSources struct {
	// RootDir is the absolute path to the engine root directory.
	// All import paths are resolved relative to this directory.
	RootDir string

	// MainPkgName is the Go package declaration name of the main package
	// (e.g. "main", "engine", "test"). Populated during source loading.
	MainPkgName string

	// Main holds the main package source files: filename (relative to RootDir)
	// → source code. The engine name and resources (Skin, Effect, Buckets, etc.)
	// are always defined in the main package.
	Main map[string]string

	// ImportPkgNames maps import paths to their declared Go package names.
	// Populated during ResolveImports. Import paths not in this map have not
	// been resolved yet.
	ImportPkgNames map[string]string

	// Imports holds imported sub-package source files: import path →
	// (filename → source code). Populated lazily by ResolveImports.
	Imports map[string]map[string]string
}

// NewSingleFileSources creates an EngineSources for a single source string.
// This is used by tests and backward-compatible single-file entry points.
// RootDir is set to the current working directory (or "." if unavailable).
func NewSingleFileSources(src string) *EngineSources {
	pkgName, _ := extractPackageName(map[string]string{"engine.go": src})
	return &EngineSources{
		RootDir:        ".",
		MainPkgName:    pkgName,
		Main:           map[string]string{"engine.go": src},
		Imports:        map[string]map[string]string{},
		ImportPkgNames: map[string]string{},
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

	files := map[string]string{name: string(src)}
	pkgName, err := extractPackageName(files)
	if err != nil {
		return nil, fmt.Errorf("source: %w", err)
	}

	return &EngineSources{
		RootDir:        rootDir,
		MainPkgName:    pkgName,
		Main:           files,
		Imports:        map[string]map[string]string{},
		ImportPkgNames: map[string]string{},
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

	// Extract and validate the package name.
	pkgName, err := extractPackageName(files)
	if err != nil {
		return nil, fmt.Errorf("source: %w", err)
	}

	return &EngineSources{
		RootDir:        absDir,
		MainPkgName:    pkgName,
		Main:           files,
		Imports:        map[string]map[string]string{},
		ImportPkgNames: map[string]string{},
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

// extractPackageName parses all files to extract their Go package declaration
// name, validating that all files agree. Returns the package name on success.
func extractPackageName(files map[string]string) (string, error) {
	pkg, err := goparse.ParseFiles(files)
	if err != nil {
		return "", err
	}
	return pkg.Name, nil
}

// Access returns a frontend.EngineSourcesAccess for type-checking, avoiding a
// circular import (engine → frontend ← engine). The Imports field enables the
// type checker to validate sub-package callback bodies against the full prelude.
func (ess *EngineSources) Access() *frontend.EngineSourcesAccess {
	return &frontend.EngineSourcesAccess{
		RootDir:   ess.RootDir,
		MainFiles: ess.Main,
		Imports:   ess.Imports,
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
		if ess.ImportPkgNames == nil {
			ess.ImportPkgNames = map[string]string{}
		}
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
		pkgName, err := extractPackageName(files)
		if err != nil {
			return fmt.Errorf("import %q: %w", impPath, err)
		}
		if ess.ImportPkgNames == nil {
			ess.ImportPkgNames = map[string]string{}
		}
		ess.ImportPkgNames[impPath] = pkgName
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
