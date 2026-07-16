package compiler

import (
	"path/filepath"
	"testing"

	"github.com/WindowsSov8forUs/sonolus-go/v2/internal/compiler/optimize"
)

func TestPublicMinimalExampleCompilesAllModes(t *testing.T) {
	pattern := filepath.Join("..", "..", "examples", "minimal")
	artifacts, err := NewCompiler(Options{}, pattern).CompileAll()
	if err != nil {
		t.Fatal(err)
	}
	if artifacts.Configuration == nil {
		t.Fatal("configuration is nil")
	}
	if artifacts.Play == nil || artifacts.Watch == nil || artifacts.Preview == nil || artifacts.Tutorial == nil {
		t.Fatalf("example produced incomplete artifacts: %#v", artifacts)
	}
	if len(artifacts.Play.Archetypes) != 1 || len(artifacts.Watch.Archetypes) != 1 || len(artifacts.Preview.Archetypes) != 1 {
		t.Fatalf("example archetypes are incomplete: play=%d watch=%d preview=%d",
			len(artifacts.Play.Archetypes), len(artifacts.Watch.Archetypes), len(artifacts.Preview.Archetypes))
	}
}

func TestPublicConformanceExampleCompilesAtEveryOptimizationLevel(t *testing.T) {
	pattern := filepath.Join("..", "..", "examples", "conformance")
	for _, level := range []optimize.Level{optimize.LevelMinimal, optimize.LevelFast, optimize.LevelStandard} {
		t.Run(level.String(), func(t *testing.T) {
			artifacts, err := NewCompiler(Options{Optimization: level}, pattern).CompileAll()
			if err != nil {
				t.Fatal(err)
			}
			for name, data := range map[string]any{
				"play": artifacts.Play, "watch": artifacts.Watch,
				"preview": artifacts.Preview, "tutorial": artifacts.Tutorial,
			} {
				if _, err := normalizeEngineData(data); err != nil {
					t.Fatalf("%s node graph: %v", name, err)
				}
			}
		})
	}
}
