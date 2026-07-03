package frontend

import (
	"fmt"
	"go/ast"
	"go/importer"
	"go/parser"
	"go/token"
	"go/types"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// sentinelPkg is used as a placeholder in the loaded map during import processing,
// allowing cycle detection before the actual *types.Package is created.
var sentinelPkg = types.NewPackage("__importing__", "__importing__")

// engineImporter implements types.ImporterFrom to resolve local import paths
// relative to the engine root directory. It falls back to importer.Default()
// for standard library imports.
type engineImporter struct {
	root      string                          // engine root directory (absolute)
	fset      *token.FileSet                  // shared file set
	loaded    map[string]*types.Package       // resolved packages (key: import path)
	loading   map[string]bool                 // packages currently being loaded (cycle detection)
	preloaded map[string]map[string]string    // import path → (filename → source) from EngineSources
	mu        sync.Mutex                      // protects loaded/loading
}

// newEngineImporter creates an importer that resolves local import paths
// (e.g. "notes", "stage") to directories under rootDir. Standard library
// imports are delegated to importer.Default(). The sonolus package is
// pre-registered so engine source can use import "...sonolus-go/sonolus"
// without needing the actual package on disk.
//
// preloaded is a map of import path → (filename → source) for packages
// that have already been loaded via EngineSources (e.g., from tests or
// pre-resolved imports). When preloaded sources exist for an import path,
// they are used instead of reading from the filesystem, giving the type
// checker access to the actual sub-package source code.
func newEngineImporter(rootDir string, fset *token.FileSet, preloaded map[string]map[string]string) *engineImporter {
	imp := &engineImporter{
		root:    rootDir,
		fset:    fset,
		loaded:  map[string]*types.Package{},
		loading: map[string]bool{},
	}
	// Pre-register the sonolus declaration stub package under both short and
	// full import paths. Engine source can use either form.
	sonolusPkg := types.NewPackage("github.com/WindowsSov8forUs/sonolus-go/sonolus", "sonolus")
	imp.loaded["sonolus"] = sonolusPkg
	imp.loaded["github.com/WindowsSov8forUs/sonolus-go/sonolus"] = sonolusPkg
	// Pre-load sub-package sources into a lookup map so importLocal can use
	// them instead of requiring filesystem access. This bridges the gap
	// between EngineSources.Imports (parser layer) and engineImporter
	// (type checker layer).
	imp.preloaded = preloaded
	return imp
}

// Import implements types.Importer.
func (imp *engineImporter) Import(path string) (*types.Package, error) {
	return imp.ImportFrom(path, imp.root, 0)
}

// ImportFrom implements types.ImporterFrom.
func (imp *engineImporter) ImportFrom(path, srcDir string, _ types.ImportMode) (*types.Package, error) {
	imp.mu.Lock()

	// Check cache.
	if pkg, ok := imp.loaded[path]; ok {
		if pkg == sentinelPkg {
			imp.mu.Unlock()
			return nil, fmt.Errorf("circular import: %s", path)
		}
		imp.mu.Unlock()
		return pkg, nil
	}

	// Try stdlib first.
	if fallback, err := importer.Default().Import(path); err == nil {
		imp.loaded[path] = fallback
		imp.mu.Unlock()
		return fallback, nil
	}

	// Not stdlib — must be a local package. Mark as loading for cycle detection.
	imp.loading[path] = true
	imp.loaded[path] = sentinelPkg
	imp.mu.Unlock()

	pkg, err := imp.importLocal(path, srcDir)

	imp.mu.Lock()
	delete(imp.loading, path)
	if err != nil {
		// Remove sentinel on failure so retries work.
		delete(imp.loaded, path)
		imp.mu.Unlock()
		return nil, err
	}
	imp.loaded[path] = pkg
	imp.mu.Unlock()
	return pkg, nil
}

// importLocal resolves a local import path to a directory, parses all .go files,
// and type-checks the package. If the import path is in the preloaded source map,
// those in-memory sources are used instead of reading from the filesystem.
func (imp *engineImporter) importLocal(path, srcDir string) (*types.Package, error) {
	// Check preloaded sources first (from EngineSources.Imports).
	var files []*ast.File
	var pkgName string
	if preloaded, ok := imp.preloaded[path]; ok {
		for name, src := range preloaded {
			f, err := parser.ParseFile(imp.fset, name, src, 0)
			if err != nil {
				return nil, fmt.Errorf("import %q: parse %s: %w", path, name, err)
			}
			if pkgName == "" {
				pkgName = f.Name.Name
			} else if f.Name.Name != pkgName {
				return nil, fmt.Errorf("import %q: conflicting package names: %q uses %q, expected %q", path, name, f.Name.Name, pkgName)
			}
			files = append(files, f)
		}
	} else {
		// Fall back to filesystem.
		resolvedDir := filepath.Join(srcDir, path)
		fi, err := os.Stat(resolvedDir)
		if err != nil {
			return nil, fmt.Errorf("import %q: directory not found: %s", path, resolvedDir)
		}
		if !fi.IsDir() {
			return nil, fmt.Errorf("import %q: %s is not a directory", path, resolvedDir)
		}

		entries, err := os.ReadDir(resolvedDir)
		if err != nil {
			return nil, fmt.Errorf("import %q: %w", path, err)
		}

		for _, e := range entries {
			name := e.Name()
			if e.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
				continue
			}
			fullPath := filepath.Join(resolvedDir, name)
			src, err := os.ReadFile(fullPath)
			if err != nil {
				return nil, fmt.Errorf("import %q: read %s: %w", path, name, err)
			}
			f, err := parser.ParseFile(imp.fset, fullPath, src, 0)
			if err != nil {
				return nil, fmt.Errorf("import %q: parse %s: %w", path, name, err)
			}
			if pkgName == "" {
				pkgName = f.Name.Name
			} else if f.Name.Name != pkgName {
				return nil, fmt.Errorf("import %q: conflicting package names: %q uses %q, expected %q", path, name, f.Name.Name, pkgName)
			}
			files = append(files, f)
		}

		if len(files) == 0 {
			return nil, fmt.Errorf("import %q: no .go files found in %s", path, resolvedDir)
		}
	}

	// Use the import path as the package path (matching Go convention for local imports).
	pkgPath := path

	// Generate full prelude for the imported package so that callback
	// bodies referencing runtime functions (sin, draw, spawn, etc.) pass
	// type checking. Without this, sub-package type errors are silently
	// swallowed rather than being properly validated.
	preludeSrc := PreludeSource(pkgName, nil)
	preludeFile, err := parser.ParseFile(imp.fset, "path+`/__prelude.go`", preludeSrc, 0)
	if err != nil {
		return nil, fmt.Errorf("import %q: prelude parse: %w", path, err)
	}
	allFiles := append([]*ast.File{preludeFile}, files...)

	conf := types.Config{Importer: imp}
	pkg, err := conf.Check(pkgPath, imp.fset, allFiles, nil)
	if err != nil {
		// Filter hard errors for sub-packages too.
		if hard := filterHardErrors(err); hard != nil {
			return nil, fmt.Errorf("import %q: %w", path, hard)
		}
	}
	// If Check returns nil pkg despite soft errors, create a skeleton.
	if pkg == nil {
		pkg = types.NewPackage(pkgPath, pkgName)
	}
	return pkg, nil
}

// TypeCheckEngine type-checks the main package and all transitively imported
// local sub-packages. It returns the main package's type info for use in
// type-driven dispatch during compilation.
func TypeCheckEngine(ess *EngineSourcesAccess) (*token.FileSet, []*ast.File, *types.Info, error) {
	fset := token.NewFileSet()

	// Convert source map to parsed AST files.
	pkgName := ""
	var userFiles []*ast.File
	for name, src := range ess.MainFiles {
		f, err := parser.ParseFile(fset, name, src, 0)
		if err != nil {
			return nil, nil, nil, fmt.Errorf("typecheck: parse %s: %w", name, err)
		}
		if pkgName == "" {
			pkgName = f.Name.Name
		} else if f.Name.Name != pkgName {
			return nil, nil, nil, fmt.Errorf("typecheck: conflicting package names: %q uses %q, expected %q", name, f.Name.Name, pkgName)
		}
		userFiles = append(userFiles, f)
	}

	// Create the engine importer for local import resolution, passing
	// pre-loaded imports from EngineSources so sub-packages are properly
	// type-checked even when not present on the filesystem.
	eimp := newEngineImporter(ess.RootDir, fset, ess.Imports)

	info, err := checkWithPreludeImp(fset, pkgName, userFiles, nil, eimp)
	if err != nil {
		return fset, userFiles, info, err
	}
	return fset, userFiles, info, nil
}

// EngineSourcesAccess provides the minimal interface TypeCheckEngine needs
// from engine.EngineSources without creating a circular import (frontend ← engine).
// Call engine.EngineSources.Access() to obtain an instance.
type EngineSourcesAccess struct {
	RootDir   string
	MainFiles map[string]string
	Imports   map[string]map[string]string // import path → (filename → source)
}
