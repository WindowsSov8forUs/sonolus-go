package goparse

import (
	"fmt"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
)

// CollectGoFiles reads all *.go files (non-recursive) from a directory.
// Returns filename (base name) → source content. Skips test files (*_test.go).
func CollectGoFiles(dir string) (map[string]string, error) {
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

// Built-in import filters. Each returns true if the import path should be
// followed (loaded from a local directory). Multiple filters are ANDed:
// an import is followed only if all filters return true.

// FilterStdlib skips import paths containing a dot, which covers Go standard
// library ("fmt", "math") and external packages ("github.com/...").
func FilterStdlib(importPath string) bool {
	return !strings.Contains(importPath, ".")
}

// FilterNonSonolus skips the sonolus declaration stub package, which is
// resolved by the engine's custom type-checker importer and has no
// filesystem presence.
func FilterNonSonolus(importPath string) bool {
	return importPath != "sonolus"
}

// ExtractImportPaths parses Go source files and returns all import paths found.
// Each unique import path is returned once. Optional filters constrain which
// paths are included; if no filters are given, all imports are returned.
func ExtractImportPaths(files map[string]string, filters ...func(importPath string) bool) ([]string, error) {
	fset := token.NewFileSet()
	seen := map[string]bool{}
	var paths []string

	for name, src := range files {
		f, err := parser.ParseFile(fset, name, src, parser.ImportsOnly)
		if err != nil {
			return nil, fmt.Errorf("scan imports: parse %s: %w", name, err)
		}
		for _, imp := range f.Imports {
			path := strings.Trim(imp.Path.Value, `"`)
			if path == "" {
				continue
			}
			if !applyFilters(path, filters) {
				continue
			}
			if seen[path] {
				continue
			}
			seen[path] = true
			paths = append(paths, path)
		}
	}
	return paths, nil
}

// applyFilters returns true if path passes all filters (or no filters are given).
func applyFilters(path string, filters []func(string) bool) bool {
	for _, f := range filters {
		if !f(path) {
			return false
		}
	}
	return true
}

// LoadProject loads a Go project from a root directory and returns all
// packages keyed by import path ("" for the main package).
//
// Optional filters control which imports to resolve. An import is followed
// only if every filter returns true. Typical usage:
//
//	LoadProject("./myengine", FilterStdlib, FilterNonSonolus)
//
// When no filters are given, all imports are followed.
//
// Import resolution is non-recursive: only direct imports of the main
// package are loaded.
func LoadProject(rootDir string, filters ...func(importPath string) bool) (map[string]*Package, error) {
	abs, err := filepath.Abs(rootDir)
	if err != nil {
		return nil, fmt.Errorf("resolve path %q: %w", rootDir, err)
	}

	fi, err := os.Stat(abs)
	if err != nil {
		return nil, fmt.Errorf("stat %q: %w", abs, err)
	}

	// root is the directory used for import resolution.
	var root string
	var mainFiles map[string]string
	if fi.IsDir() {
		root = abs
		mainFiles, err = CollectGoFiles(abs)
		if err != nil {
			return nil, fmt.Errorf("main package: %w", err)
		}
		if len(mainFiles) == 0 {
			return nil, fmt.Errorf("no .go files found in %q", abs)
		}
	} else {
		root = filepath.Dir(abs)
		src, err := os.ReadFile(abs)
		if err != nil {
			return nil, fmt.Errorf("read %q: %w", abs, err)
		}
		mainFiles = map[string]string{filepath.Base(abs): string(src)}
	}

	// Parse the main package.
	mainPkg, err := ParseFiles(mainFiles)
	if err != nil {
		return nil, fmt.Errorf("main package: %w", err)
	}
	mainPkg.Path = ""

	pkgs := map[string]*Package{"": mainPkg}

	// Resolve direct imports of the main package.
	importPaths, err := ExtractImportPaths(mainFiles, filters...)
	if err != nil {
		return nil, fmt.Errorf("scan imports: %w", err)
	}

	for _, impPath := range importPaths {
		resolvedDir := filepath.Join(root, impPath)
		fi, err := os.Stat(resolvedDir)
		if err != nil {
			return nil, fmt.Errorf("import %q: %w", impPath, err)
		}
		if !fi.IsDir() {
			return nil, fmt.Errorf("import %q: %s is not a directory", impPath, resolvedDir)
		}
		importFiles, err := CollectGoFiles(resolvedDir)
		if err != nil {
			return nil, fmt.Errorf("import %q: %w", impPath, err)
		}
		if len(importFiles) == 0 {
			return nil, fmt.Errorf("import %q: no .go files in %s", impPath, resolvedDir)
		}

		impPkg, err := ParseFiles(importFiles)
		if err != nil {
			return nil, fmt.Errorf("import %q: %w", impPath, err)
		}
		impPkg.Path = impPath
		pkgs[impPath] = impPkg
	}

	return pkgs, nil
}

// LoadProjectFromFiles loads a project from in-memory source maps.
// The main package files are provided directly; this is the backward-
// compatible entry point for single-file engines and tests.
func LoadProjectFromFiles(mainFiles map[string]string) (map[string]*Package, error) {
	mainPkg, err := ParseFiles(mainFiles)
	if err != nil {
		return nil, err
	}
	mainPkg.Path = ""
	return map[string]*Package{"": mainPkg}, nil
}
