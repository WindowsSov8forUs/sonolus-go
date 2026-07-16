package main

import (
	"reflect"
	"testing"

	"github.com/WindowsSov8forUs/sonolus-core-go/core/resource"
	"github.com/WindowsSov8forUs/sonolus-go/v2/internal/compiler"
	"github.com/WindowsSov8forUs/sonolus-go/v2/internal/compiler/optimize"
	"github.com/WindowsSov8forUs/sonolus-go/v2/internal/level"
)

func TestGodoriCompilesAtEveryOptimizationLevel(t *testing.T) {
	for _, optimization := range []optimize.Level{optimize.LevelMinimal, optimize.LevelFast, optimize.LevelStandard} {
		t.Run(optimization.String(), func(t *testing.T) {
			artifacts, err := compiler.NewCompiler(compiler.Options{Optimization: optimization}, ".").CompileAll()
			if err != nil {
				t.Fatal(err)
			}
			assertArtifacts(t, artifacts)
		})
	}
}

func BenchmarkCompileAll(b *testing.B) {
	for _, optimization := range []optimize.Level{optimize.LevelMinimal, optimize.LevelFast, optimize.LevelStandard} {
		b.Run(optimization.String(), func(b *testing.B) {
			var nodes int
			for range b.N {
				artifacts, err := compiler.NewCompiler(compiler.Options{Optimization: optimization}, ".").CompileAll()
				if err != nil {
					b.Fatal(err)
				}
				nodes = len(artifacts.Play.Nodes) + len(artifacts.Watch.Nodes) + len(artifacts.Preview.Nodes) + len(artifacts.Tutorial.Nodes)
			}
			b.ReportMetric(float64(nodes), "nodes/op")
		})
	}
}

func TestGodoriCompilationIsDeterministic(t *testing.T) {
	first, err := compiler.NewCompiler(compiler.Options{}, ".").CompileAll()
	if err != nil {
		t.Fatal(err)
	}
	second, err := compiler.NewCompiler(compiler.Options{}, ".").CompileAll()
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(first, second) {
		t.Fatal("compilation is not deterministic")
	}
}

func TestGodoriSchema(t *testing.T) {
	schema, err := compiler.NewCompiler(compiler.Options{}, ".").Schema()
	if err != nil {
		t.Fatal(err)
	}
	want := &compiler.ProjectSchema{Archetypes: []compiler.ArchetypeSchema{
		{Name: "#BPM_CHANGE", Fields: []string{"#BEAT", "#BPM"}},
		{Name: "Stage", Fields: []string{}},
		{Name: "TapNote", Fields: []string{"#BEAT", "lane"}},
	}}
	if !reflect.DeepEqual(schema, want) {
		t.Fatalf("schema = %#v, want %#v", schema, want)
	}
}

func TestGodoriDevelopmentLevel(t *testing.T) {
	artifacts, err := compiler.NewCompiler(compiler.Options{}, ".").CompileAll()
	if err != nil {
		t.Fatal(err)
	}
	development, err := level.LoadDevelopment(".")
	if err != nil {
		t.Fatal(err)
	}
	if development.File == "" || len(development.Data.Entities) != 7 {
		t.Fatalf("development level = %#v", development)
	}
	if err := level.Validate(development.Data, artifacts); err != nil {
		t.Fatal(err)
	}
}

func assertArtifacts(t *testing.T, artifacts *compiler.Artifacts) {
	t.Helper()
	if artifacts.Configuration == nil || len(artifacts.Configuration.Options) != 4 || len(artifacts.ROM) == 0 {
		t.Fatalf("shared artifacts are incomplete: configuration=%#v rom=%d", artifacts.Configuration, len(artifacts.ROM))
	}
	assertNodes(t, "play", artifacts.Play.Nodes)
	assertNodes(t, "watch", artifacts.Watch.Nodes)
	assertNodes(t, "preview", artifacts.Preview.Nodes)
	assertNodes(t, "tutorial", artifacts.Tutorial.Nodes)

	playStage, playNote := findPlay(artifacts.Play, "Stage"), findPlay(artifacts.Play, "TapNote")
	watchStage, watchNote := findWatch(artifacts.Watch, "Stage"), findWatch(artifacts.Watch, "TapNote")
	previewStage, previewNote := findPreview(artifacts.Preview, "Stage"), findPreview(artifacts.Preview, "TapNote")
	if playStage == nil || playNote == nil || watchStage == nil || watchNote == nil || previewStage == nil || previewNote == nil {
		t.Fatal("Stage and TapNote must exist in Play, Watch, and Preview")
	}
	if playStage.Preprocess == nil || playNote.Touch == nil || watchNote.UpdateSequential == nil || previewNote.Render == nil {
		t.Fatal("gameplay callbacks were omitted")
	}
	assertIndex(t, "play Stage preprocess", playStage.Preprocess.Index, len(artifacts.Play.Nodes))
	assertIndex(t, "play TapNote touch", playNote.Touch.Index, len(artifacts.Play.Nodes))
	assertIndex(t, "watch TapNote replay", watchNote.UpdateSequential.Index, len(artifacts.Watch.Nodes))
	assertIndex(t, "watch updateSpawn", artifacts.Watch.UpdateSpawn, len(artifacts.Watch.Nodes))
	assertIndex(t, "preview TapNote render", previewNote.Render.Index, len(artifacts.Preview.Nodes))
	assertIndex(t, "tutorial preprocess", artifacts.Tutorial.Preprocess, len(artifacts.Tutorial.Nodes))
	assertIndex(t, "tutorial navigate", artifacts.Tutorial.Navigate, len(artifacts.Tutorial.Nodes))
	assertIndex(t, "tutorial update", artifacts.Tutorial.Update, len(artifacts.Tutorial.Nodes))
}

func assertNodes(t *testing.T, mode string, nodes []resource.EngineDataNode) {
	t.Helper()
	if len(nodes) == 0 {
		t.Fatalf("%s node pool is empty", mode)
	}
	for i, node := range nodes {
		function, ok := node.(resource.EngineDataFunctionNode)
		if !ok {
			continue
		}
		for _, argument := range function.Args {
			if argument < 0 || argument >= len(nodes) {
				t.Fatalf("%s node %d has invalid argument %d", mode, i, argument)
			}
		}
	}
}

func assertIndex(t *testing.T, name string, index, length int) {
	t.Helper()
	if index < 0 || index >= length {
		t.Fatalf("%s index %d is outside node pool of length %d", name, index, length)
	}
}

func findPlay(data *resource.EnginePlayData, name resource.EngineArchetypeName) *resource.EnginePlayDataArchetype {
	for i := range data.Archetypes {
		if data.Archetypes[i].Name == name {
			return &data.Archetypes[i]
		}
	}
	return nil
}

func findWatch(data *resource.EngineWatchData, name resource.EngineArchetypeName) *resource.EngineWatchDataArchetype {
	for i := range data.Archetypes {
		if data.Archetypes[i].Name == name {
			return &data.Archetypes[i]
		}
	}
	return nil
}

func findPreview(data *resource.EnginePreviewData, name resource.EngineArchetypeName) *resource.EnginePreviewDataArchetype {
	for i := range data.Archetypes {
		if data.Archetypes[i].Name == name {
			return &data.Archetypes[i]
		}
	}
	return nil
}
