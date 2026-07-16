package compiler

import (
	"path/filepath"
	"testing"

	"github.com/WindowsSov8forUs/sonolus-go/internal/compiler/optimize"
)

func BenchmarkCompileAll(b *testing.B) {
	corpora := []struct {
		name    string
		pattern string
	}{
		{name: "minimal", pattern: filepath.Join("..", "..", "examples", "minimal")},
		{name: "reference", pattern: "./testdata/reference"},
	}
	levels := []optimize.Level{optimize.LevelMinimal, optimize.LevelFast, optimize.LevelStandard}
	for _, corpus := range corpora {
		for _, level := range levels {
			b.Run(corpus.name+"/"+level.String(), func(b *testing.B) {
				var nodes int
				b.ResetTimer()
				for range b.N {
					artifacts, err := NewCompiler(Options{Optimization: level}, corpus.pattern).CompileAll()
					if err != nil {
						b.Fatal(err)
					}
					nodes = len(artifacts.Play.Nodes) + len(artifacts.Watch.Nodes) + len(artifacts.Preview.Nodes) + len(artifacts.Tutorial.Nodes)
				}
				b.ReportMetric(float64(nodes), "nodes/op")
			})
		}
	}
}

func TestReferenceArtifactScale(t *testing.T) {
	for _, level := range []optimize.Level{optimize.LevelMinimal, optimize.LevelFast, optimize.LevelStandard} {
		artifacts, err := NewCompiler(Options{Optimization: level}, "./testdata/reference").CompileAll()
		if err != nil {
			t.Fatalf("compile %s: %v", level, err)
		}
		nodes := len(artifacts.Play.Nodes) + len(artifacts.Watch.Nodes) + len(artifacts.Preview.Nodes) + len(artifacts.Tutorial.Nodes)
		if nodes > 256 {
			t.Fatalf("%s reference node count grew to %d; review the structural regression before updating the limit", level, nodes)
		}
		t.Logf("%s: %d nodes", level, nodes)
	}
}
