package compiler

import (
	"encoding/binary"
	"math"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"

	"github.com/WindowsSov8forUs/sonolus-go/v2/internal/compiler/frontend"
	"github.com/WindowsSov8forUs/sonolus-go/v2/internal/compiler/ir"
	"github.com/WindowsSov8forUs/sonolus-go/v2/internal/compiler/mode"
	"github.com/WindowsSov8forUs/sonolus-go/v2/internal/compiler/optimize"
)

func TestCompilerBuildsCumulativeSnapshotAndReturnsClone(t *testing.T) {
	compiler := NewCompiler(Options{}, "./testdata/multimode")
	play, err := compiler.Compile(mode.ModePlay, mode.ModePlay)
	if err != nil {
		t.Fatal(err)
	}
	if play.Play == nil || play.Watch != nil || len(play.ROM) != 20 {
		t.Fatalf("unexpected play artifacts: %#v", play)
	}
	play.Play.Skin.Sprites[0].Name = "mutated"
	watch, err := compiler.Compile(mode.ModeWatch)
	if err != nil {
		t.Fatal(err)
	}
	if watch.Play == nil || watch.Watch == nil || watch.Play.Skin.Sprites[0].Name != "play.sprite" {
		t.Fatalf("cumulative clone was corrupted: %#v", watch)
	}
}

func TestCompilerSchemaUsesDeclarationsWithoutLoweringCallbacks(t *testing.T) {
	compiler := NewCompiler(Options{}, "./testdata/invaliddefer")
	schema, err := compiler.Schema()
	if err != nil {
		t.Fatal(err)
	}
	if len(schema.Archetypes) != 1 || schema.Archetypes[0].Name != "Note" || len(schema.Archetypes[0].Fields) != 0 {
		t.Fatalf("schema = %#v", schema)
	}
	if _, err := compiler.Compile(mode.ModePlay); err == nil {
		t.Fatal("callback lowering unexpectedly succeeded")
	}

	schema.Archetypes[0].Name = "mutated"
	again, err := compiler.Schema()
	if err != nil || again.Archetypes[0].Name != "Note" {
		t.Fatalf("cached schema was mutated: %#v, %v", again, err)
	}
}

func TestCompilerSchemaMatchesDeclarationFields(t *testing.T) {
	compiler := NewCompiler(Options{}, "./testdata/declarations")
	schema, err := compiler.Schema()
	if err != nil {
		t.Fatal(err)
	}
	if len(schema.Archetypes) != 1 || schema.Archetypes[0].Name != "TapNote" {
		t.Fatalf("schema = %#v", schema)
	}
	want := []string{"hit.x", "hit.y", "#BEAT", "target.x", "target.y", "path[0].x", "path[0].y", "path[1].x", "path[1].y", "single"}
	if !reflect.DeepEqual(schema.Archetypes[0].Fields, want) {
		t.Fatalf("fields = %v, want %v", schema.Archetypes[0].Fields, want)
	}
}

func TestCompilerSchemaRejectsInvalidArchetypeDeclarations(t *testing.T) {
	compiler := NewCompiler(Options{}, "./testdata/invalid")
	if _, err := compiler.Schema(); err == nil || !strings.Contains(err.Error(), "unknown archetype tag") {
		t.Fatalf("error = %v", err)
	}
}

func TestCompilerCompileAfterSchemaDoesNotReturnIncompleteArtifacts(t *testing.T) {
	compiler := NewCompiler(Options{Optimization: optimize.LevelMinimal}, "./testdata/declarations")
	if _, err := compiler.Schema(); err != nil {
		t.Fatal(err)
	}
	artifacts, err := compiler.Compile(mode.ModePlay)
	if err != nil {
		t.Fatal(err)
	}
	if artifacts.Play == nil || len(artifacts.Play.Archetypes) != 1 {
		t.Fatalf("artifacts = %#v", artifacts)
	}
}

func TestCompilerFailureDoesNotCommitCandidateMode(t *testing.T) {
	compiler := NewCompiler(Options{}, "./testdata/configurationmismatch")
	before, err := compiler.Compile(mode.ModePlay)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := compiler.Compile(mode.ModeWatch); err == nil || !strings.Contains(err.Error(), "configuration differs") {
		t.Fatalf("mismatch error = %v", err)
	}
	after, err := compiler.Compile(mode.ModePlay)
	if err != nil {
		t.Fatal(err)
	}
	if before.Play == nil || after.Play == nil || after.Watch != nil {
		t.Fatalf("failed candidate was committed: %#v", after)
	}
}

func TestCompilerBuildsFullPlayLoweringFixture(t *testing.T) {
	compiler := NewCompiler(Options{}, "./testdata/lowering")
	artifacts, err := compiler.Compile(mode.ModePlay)
	if err != nil {
		t.Fatal(err)
	}
	if artifacts.Play == nil || len(artifacts.Play.Archetypes) != 2 || len(artifacts.Play.Nodes) == 0 {
		t.Fatalf("unexpected play artifacts: %#v", artifacts.Play)
	}
}

func TestCompilerBuildsOtherModeLoweringFixtures(t *testing.T) {
	tests := []struct {
		name    string
		pattern string
		mode    mode.Mode
		valid   func(*Artifacts) bool
	}{
		{"watch", "./testdata/lowering_watch", mode.ModeWatch, func(value *Artifacts) bool { return value.Watch != nil && len(value.Watch.Nodes) != 0 }},
		{"preview", "./testdata/lowering_preview", mode.ModePreview, func(value *Artifacts) bool { return value.Preview != nil && len(value.Preview.Nodes) != 0 }},
		{"tutorial", "./testdata/lowering_tutorial", mode.ModeTutorial, func(value *Artifacts) bool { return value.Tutorial != nil && len(value.Tutorial.Nodes) != 0 }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			artifacts, err := NewCompiler(Options{}, test.pattern).Compile(test.mode)
			if err != nil {
				t.Fatal(err)
			}
			if !test.valid(artifacts) {
				t.Fatalf("unexpected artifacts: %#v", artifacts)
			}
		})
	}
}

func TestCompilerCompileAllAndConcurrentAccumulation(t *testing.T) {
	compiler := NewCompiler(Options{}, "./testdata/multimode")
	var wg sync.WaitGroup
	errs := make(chan error, 2)
	for _, m := range []mode.Mode{mode.ModePlay, mode.ModeWatch} {
		wg.Add(1)
		go func(m mode.Mode) {
			defer wg.Done()
			_, err := compiler.Compile(m)
			errs <- err
		}(m)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	artifacts, err := compiler.CompileAll()
	if err != nil {
		t.Fatal(err)
	}
	if artifacts.Play == nil || artifacts.Watch == nil || artifacts.Preview == nil || artifacts.Tutorial == nil {
		t.Fatalf("CompileAll returned an incomplete snapshot: %#v", artifacts)
	}
}

func TestCompilerBuildsNativeCoverageThroughBackend(t *testing.T) {
	artifacts, err := NewCompiler(Options{}, "./testdata/nativecoverage").Compile(mode.ModePlay)
	if err != nil {
		t.Fatal(err)
	}
	if artifacts.Play == nil || len(artifacts.Play.Nodes) == 0 {
		t.Fatalf("native coverage produced no nodes: %#v", artifacts.Play)
	}
}

func TestCompilerValidatesModes(t *testing.T) {
	compiler := NewCompiler(Options{}, "./testdata/multimode")
	if _, err := compiler.Compile(); err == nil {
		t.Fatal("empty mode list was accepted")
	}
	if _, err := compiler.Compile(mode.Mode("invalid")); err == nil {
		t.Fatal("invalid mode was accepted")
	}
}

func TestCompilerFallbackROMAndSourcePriority(t *testing.T) {
	fallback := make([]byte, 4)
	binary.LittleEndian.PutUint32(fallback, math.Float32bits(7.5))
	empty, err := NewCompiler(Options{FallbackROM: fallback}, "./testdata/emptyshared").Compile(mode.ModePlay)
	if err != nil {
		t.Fatal(err)
	}
	if len(empty.ROM) != 16 || math.Float32frombits(binary.LittleEndian.Uint32(empty.ROM[12:])) != 7.5 {
		t.Fatalf("fallback ROM = %v", empty.ROM)
	}
	source, err := NewCompiler(Options{FallbackROM: fallback}, "./testdata/multimode").Compile(mode.ModePlay)
	if err != nil {
		t.Fatal(err)
	}
	if len(source.ROM) != 20 || math.Float32frombits(binary.LittleEndian.Uint32(source.ROM[12:])) == 7.5 {
		t.Fatalf("source ROM did not take priority: %v", source.ROM)
	}
}

func TestCompilerStatsAndSourceFiles(t *testing.T) {
	compiler := NewCompiler(Options{}, "./testdata/multimode")
	if _, err := compiler.Compile(mode.ModePlay); err != nil {
		t.Fatal(err)
	}
	stats := compiler.Stats()
	if stats.Cached || stats.Total <= 0 || stats.Load <= 0 || stats.Modes[mode.ModePlay].Load <= 0 {
		t.Fatalf("first compile stats = %#v", stats)
	}
	files := compiler.SourceFiles()
	if len(files) == 0 {
		t.Fatal("source files are empty")
	}
	foundPlay := false
	for _, file := range files {
		if filepath.Base(file) == "play.go" {
			foundPlay = true
		}
	}
	if !foundPlay {
		t.Fatalf("play build-tag file missing from %v", files)
	}
	if _, err := compiler.Compile(mode.ModePlay); err != nil {
		t.Fatal(err)
	}
	if stats := compiler.Stats(); !stats.Cached {
		t.Fatalf("cached stats = %#v", stats)
	}
}
func TestDiscoverTargets(t *testing.T) {
	targets, err := DiscoverTargets(ModePlay, "./testdata/multimode", "./testdata/conformance", "./mode")
	if err != nil {
		t.Fatal(err)
	}
	if len(targets) != 2 {
		t.Fatalf("targets = %#v", targets)
	}
	if targets[0].PackagePath >= targets[1].PackagePath {
		t.Fatalf("targets are not sorted: %#v", targets)
	}
	for _, target := range targets {
		if target.ModulePath != "github.com/WindowsSov8forUs/sonolus-go/v2" {
			t.Fatalf("module path = %q", target.ModulePath)
		}
	}
}

func TestDiscoverTargetsRejectsNoEngine(t *testing.T) {
	if _, err := DiscoverTargets(ModePlay, "./mode"); err == nil {
		t.Fatal("non-main package was accepted as an engine")
	}
}
func TestOptimizeProjectCopiesDeclarationsAndIR(t *testing.T) {
	voidType := ir.Type{Name: "void"}
	callback := &frontend.CallbackDeclaration{Name: "update", IR: &ir.Function{
		Name: "update", Result: voidType, Entry: 0,
		Blocks: []*ir.Block{
			{ID: 0, Terminator: ir.Branch{Condition: ir.Const{Value: 1}, True: 1, False: 2}},
			{ID: 1, Terminator: ir.Return{Value: ir.Value{Type: voidType}}},
			{ID: 2, Terminator: ir.Return{Value: ir.Value{Type: voidType}}},
		},
	}}
	archetype := &frontend.ArchetypeDeclaration{Name: "Note", Callbacks: []*frontend.CallbackDeclaration{callback}}
	declarations := &frontend.ModeDeclarations{Mode: mode.ModePlay, Archetypes: []*frontend.ArchetypeDeclaration{archetype}}
	project := &frontend.Project{Modes: map[mode.Mode]*frontend.ModeDeclarations{mode.ModePlay: declarations}}

	result, err := optimizeProject(optimize.NewOptimizer(0), project)
	if err != nil {
		t.Fatal(err)
	}
	optimized := result.Modes[mode.ModePlay].Archetypes[0].Callbacks[0]
	if result == project || result.Modes[mode.ModePlay] == declarations || result.Modes[mode.ModePlay].Archetypes[0] == archetype || optimized == callback || optimized.IR == callback.IR {
		t.Fatal("optimized project shares mutable declaration or IR containers")
	}
	if len(optimized.IR.Blocks) != 1 {
		t.Fatalf("optimized blocks = %d", len(optimized.IR.Blocks))
	}
	if len(callback.IR.Blocks) != 3 {
		t.Fatal("frontend callback IR was modified")
	}
}
func TestInternalConformanceFixtureCompilesAtEveryOptimizationLevel(t *testing.T) {
	pattern := "./testdata/conformance"
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
