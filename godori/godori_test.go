package main

import (
	"testing"

	"github.com/WindowsSov8forUs/sonolus-go/v2/internal/compiler"
)

func TestGodoriCompiles(t *testing.T) {
	artifacts, err := compiler.NewCompiler(compiler.Options{}, ".").CompileAll()
	if err != nil {
		t.Fatal(err)
	}
	if artifacts.Configuration == nil || artifacts.Play == nil || artifacts.Watch == nil || artifacts.Preview == nil || artifacts.Tutorial == nil {
		t.Fatalf("incomplete four-mode artifacts: %#v", artifacts)
	}
}
