package main

import (
	"go/parser"
	"go/token"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestCLIDoesNotImportLegacyCompilerPipeline(t *testing.T) {
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	forbidden := []string{
		"/internal/compiler/engine",
		"/internal/compiler/ir",
		"/internal/compiler/build",
		"/internal/compiler/pack",
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
			for _, suffix := range forbidden {
				if strings.Contains(path, suffix) {
					t.Errorf("%s imports legacy compiler pipeline package %q", filename, path)
				}
			}
		}
	}
}
