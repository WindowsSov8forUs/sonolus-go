package main

import (
	"go/parser"
	"go/token"
	"path/filepath"
	"strconv"
	"testing"
)

func TestCLICompilerImportsStayAtPublicBoundary(t *testing.T) {
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	allowed := map[string]bool{
		"github.com/WindowsSov8forUs/sonolus-go/internal/compiler": true,
	}
	for _, filename := range files {
		file, err := parser.ParseFile(token.NewFileSet(), filename, nil, parser.ImportsOnly)
		if err != nil {
			t.Fatalf("parse %s: %v", filename, err)
		}
		for _, spec := range file.Imports {
			path, err := strconv.Unquote(spec.Path.Value)
			if err != nil {
				t.Fatalf("unquote import in %s: %v", filename, err)
			}
			const compilerPrefix = "github.com/WindowsSov8forUs/sonolus-go/internal/compiler"
			if len(path) >= len(compilerPrefix) && path[:len(compilerPrefix)] == compilerPrefix && !allowed[path] {
				t.Errorf("%s imports compiler implementation package %q", filename, path)
			}
		}
	}
}
