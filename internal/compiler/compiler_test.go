package compiler

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"go/types"
	"maps"
	"math"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"

	"github.com/WindowsSov8forUs/sonolus-core-go/core/resource"

	"golang.org/x/tools/go/packages"

	"github.com/WindowsSov8forUs/sonolus-go/v2/internal/compiler/frontend"
	"github.com/WindowsSov8forUs/sonolus-go/v2/internal/compiler/ir"
	"github.com/WindowsSov8forUs/sonolus-go/v2/internal/compiler/mode"
	"github.com/WindowsSov8forUs/sonolus-go/v2/internal/compiler/optimize"
	"github.com/WindowsSov8forUs/sonolus-go/v2/internal/compiler/source"
)

func TestCallbackCacheIdentityIsolationAndPublication(t *testing.T) {
	makeFunction := func(value float64) *ir.Function {
		b := ir.NewBuilder("cached", ir.Type{Name: "void"})
		block := b.NewBlock()
		_ = b.SetEntry(block)
		_ = b.SetCurrent(block)
		if err := b.Eval(b.RuntimeCall(resource.RuntimeFunctionDebugLog, []ir.Expr{ir.Const{Value: value}}, ir.Type{Name: "void"}, false, ir.SourcePos{File: "engine.go", Line: 1})); err != nil {
			t.Fatal(err)
		}
		if err := b.Return(ir.Value{Type: ir.Type{Name: "void"}}); err != nil {
			t.Fatal(err)
		}
		b.Function().Diagnostics = map[int]string{1: "one", 2: "two"}
		return b.Function()
	}
	context := optimize.Context{Mode: mode.ModePlay, Callback: "preprocess"}
	optimizer := optimize.NewOptimizer(optimize.LevelStandard)
	cache := new(CallbackCache)
	source := makeFunction(7)
	seed := cache.begin(optimize.LevelStandard, RuntimeChecksNone)
	result, err := seed.optimize(optimizer, context, source)
	if err != nil {
		t.Fatal(err)
	}
	want := optimize.CloneFunction(result)
	result.Name = "mutated before commit"
	result.Diagnostics[1] = "mutated"
	seed.commit()
	for round := range 2 {
		session := cache.begin(0, RuntimeChecksNone)
		got, err := session.optimize(optimizer, context, source)
		if err != nil || session.hits != 1 || !reflect.DeepEqual(got, want) {
			t.Fatalf("round %d: hits=%d error=%v result=%#v", round, session.hits, err, got)
		}
		got.Blocks[0].Instructions = nil
		got.Diagnostics[1] = "changed"
	}
	for _, test := range []struct {
		name    string
		mutate  func(*ir.Function)
		context optimize.Context
		level   optimize.Level
		checks  RuntimeChecks
	}{
		{name: "constant", mutate: func(f *ir.Function) {
			e := f.Blocks[0].Instructions[0].(ir.Eval)
			call := e.Value.(ir.RuntimeCall)
			call.Args[0] = ir.Const{Value: 8}
			e.Value = call
			f.Blocks[0].Instructions[0] = e
		}},
		{name: "position", mutate: func(f *ir.Function) {
			e := f.Blocks[0].Instructions[0].(ir.Eval)
			call := e.Value.(ir.RuntimeCall)
			call.Pos.Line++
			e.Value = call
			f.Blocks[0].Instructions[0] = e
		}},
		{name: "diagnostics", mutate: func(f *ir.Function) { f.Diagnostics[1] = "different" }},
		{name: "mode", context: optimize.Context{Mode: mode.ModeWatch, Callback: "preprocess"}},
		{name: "phase", context: optimize.Context{Mode: mode.ModePlay, Callback: "updateSequential"}},
		{name: "optimization", level: optimize.LevelMinimal},
		{name: "checks", checks: RuntimeChecksNotify},
	} {
		t.Run(test.name, func(t *testing.T) {
			fn := optimize.CloneFunction(source)
			if test.mutate != nil {
				test.mutate(fn)
			}
			ctx := test.context
			if ctx.Mode == "" {
				ctx = context
			}
			level := test.level
			if level == 0 {
				level = optimize.LevelStandard
			}
			session := cache.begin(level, test.checks)
			if _, err := session.optimize(optimize.NewOptimizer(level), ctx, fn); err != nil {
				t.Fatal(err)
			}
			if session.hits != 0 {
				t.Fatal("changed identity reused stale IR")
			}
		})
	}
	t.Run("float bits", func(t *testing.T) {
		cache := new(CallbackCache)
		values := []float64{0, math.Copysign(0, -1), math.Inf(1), math.Inf(-1), math.Float64frombits(0x7ff8000000000001), math.Float64frombits(0x7ff8000000000002)}
		session := cache.begin(0, RuntimeChecksNone)
		for _, value := range values {
			if _, err := session.optimize(optimizer, context, makeFunction(value)); err != nil {
				t.Fatal(err)
			}
		}
		session.commit()
		if len(cache.entries) != len(values) {
			t.Fatalf("distinct float bit patterns collapsed: %d", len(cache.entries))
		}
		warm := cache.begin(0, RuntimeChecksNone)
		for _, value := range values {
			if _, err := warm.optimize(optimizer, context, makeFunction(value)); err != nil {
				t.Fatal(err)
			}
		}
		if warm.hits != len(values) {
			t.Fatalf("float hits=%d", warm.hits)
		}
	})
	t.Run("concurrent snapshots", func(t *testing.T) {
		var wg sync.WaitGroup
		for range 8 {
			wg.Add(1)
			go func() {
				defer wg.Done()
				session := cache.begin(0, RuntimeChecksNone)
				got, err := session.optimize(optimizer, context, source)
				if err != nil || !reflect.DeepEqual(got, want) {
					t.Errorf("concurrent result: %v", err)
				}
				session.commit()
			}()
		}
		wg.Wait()
	})
	t.Run("failed candidate", func(t *testing.T) {
		before := maps.Clone(cache.entries)
		session := cache.begin(0, RuntimeChecksNone)
		if _, err := session.optimize(optimizer, context, makeFunction(99)); err != nil {
			t.Fatal(err)
		}
		if _, err := session.optimize(optimizer, context, nil); err == nil {
			t.Fatal("invalid candidate succeeded")
		}
		if !reflect.DeepEqual(cache.entries, before) {
			t.Fatal("failed candidate changed published cache")
		}
	})
}

func TestCallbackCacheRetentionAndCompilerReuse(t *testing.T) {
	t.Run("retention", func(t *testing.T) {
		cache := new(CallbackCache)
		optimizer := optimize.NewOptimizer(optimize.LevelMinimal)
		context := optimize.Context{Mode: mode.ModePlay, Callback: "preprocess"}
		b := ir.NewBuilder("bounded", ir.Type{Name: "void"})
		block := b.NewBlock()
		_ = b.SetEntry(block)
		_ = b.SetCurrent(block)
		_ = b.Return(ir.Value{Type: ir.Type{Name: "void"}})
		session := cache.begin(optimize.LevelMinimal, RuntimeChecksNone)
		for i := range maxCallbackCacheEntries + 1 {
			fn := optimize.CloneFunction(b.Function())
			fn.Name = fmt.Sprint(i)
			if _, err := session.optimize(optimizer, context, fn); err != nil {
				t.Fatal(err)
			}
		}
		session.commit()
		if len(cache.entries) != maxCallbackCacheEntries {
			t.Fatalf("entries=%d", len(cache.entries))
		}
		byteLimited := cache.begin(optimize.LevelMinimal, RuntimeChecksNone)
		message := strings.Repeat("x", 1<<20)
		for i := range 33 {
			fn := optimize.CloneFunction(b.Function())
			fn.Name = fmt.Sprint(i)
			fn.Diagnostics = map[int]string{1: message}
			if _, err := byteLimited.optimize(optimizer, context, fn); err != nil {
				t.Fatal(err)
			}
		}
		byteLimited.commit()
		retained := 0
		for _, entry := range cache.entries {
			retained += entry.size
		}
		if len(cache.entries) == 0 || len(cache.entries) >= 33 || retained > maxCallbackCacheBytes {
			t.Fatalf("byte limit: entries=%d bytes=%d", len(cache.entries), retained)
		}
		fn := optimize.CloneFunction(b.Function())
		fn.Diagnostics = map[int]string{1: strings.Repeat("x", maxCallbackCacheBytes+1)}
		oversize := cache.begin(optimize.LevelMinimal, RuntimeChecksNone)
		if _, err := oversize.optimize(optimizer, context, fn); err != nil {
			t.Fatal(err)
		}
		if len(oversize.pending) != 0 {
			t.Fatal("oversized callback retained")
		}
		prune := cache.begin(optimize.LevelMinimal, RuntimeChecksNone)
		if _, err := prune.optimize(optimizer, context, b.Function()); err != nil {
			t.Fatal(err)
		}
		prune.commit()
		if len(cache.entries) != 1 {
			t.Fatalf("unused generation retained: %d", len(cache.entries))
		}
	})
	t.Run("fresh compiler", func(t *testing.T) {
		cache := new(CallbackCache)
		cold := NewCompiler(Options{CallbackCache: cache}, "./testdata/callvalues")
		want, err := cold.Compile(mode.ModePlay, mode.ModeWatch)
		if err != nil {
			t.Fatal(err)
		}
		warm := NewCompiler(Options{CallbackCache: cache}, "./testdata/callvalues")
		got, err := warm.Compile(mode.ModePlay, mode.ModeWatch)
		if err != nil {
			t.Fatal(err)
		}
		if warm.Stats().Cached || warm.Stats().CallbackCacheHits == 0 || !reflect.DeepEqual(got, want) {
			t.Fatal("fresh compiler did not reuse equivalent callbacks")
		}
		before := maps.Clone(cache.entries)
		if _, err := NewCompiler(Options{CallbackCache: cache}, "./testdata/invaliddefer").Compile(mode.ModePlay); err == nil {
			t.Fatal("invalid engine succeeded")
		}
		if !reflect.DeepEqual(before, cache.entries) {
			t.Fatal("failed compilation published cache")
		}
	})
}

func loadInto(t *testing.T, parser *frontend.Parser, m mode.Mode, pattern string) {
	t.Helper()
	packages, err := source.LoadMode(m, pattern)
	if err != nil {
		t.Fatal(err)
	}
	if len(packages) != 1 {
		t.Fatalf("packages = %d, want 1", len(packages))
	}
	if err := parser.Parse(m, packages[0]); err != nil {
		t.Fatal(err)
	}
}

func TestParserBuildTagsAndAggregation(t *testing.T) {
	parser := frontend.NewParser()
	for _, currentMode := range []mode.Mode{mode.ModePlay, mode.ModeWatch, mode.ModePreview, mode.ModeTutorial} {
		loadInto(t, parser, currentMode, "./testdata/multimode")
	}
	project, err := parser.GetProject()
	if err != nil {
		t.Fatal(err)
	}
	want := map[mode.Mode]string{
		mode.ModePlay: "play.sprite", mode.ModeWatch: "watch.sprite",
		mode.ModePreview: "preview.sprite", mode.ModeTutorial: "tutorial.sprite",
	}
	var fields []*types.Var
	for _, currentMode := range []mode.Mode{mode.ModePlay, mode.ModeWatch, mode.ModePreview, mode.ModeTutorial} {
		declarations := project.Modes[currentMode]
		if declarations == nil || declarations.Resources.Skin == nil || len(declarations.Resources.Skin.Sprites) != 1 {
			t.Fatalf("%s skin = %#v", currentMode, declarations)
		}
		if got := string(declarations.Resources.Skin.Sprites[0].Name); got != want[currentMode] {
			t.Fatalf("%s sprite = %q, want %q", currentMode, got, want[currentMode])
		}
		for field := range declarations.Resources.FieldIDs {
			fields = append(fields, field)
		}
	}
	if len(fields) != 4 {
		t.Fatalf("resource fields = %d, want 4", len(fields))
	}
	for i := range fields {
		for j := i + 1; j < len(fields); j++ {
			if fields[i] == fields[j] {
				t.Fatal("resource field object was reused across mode package graphs")
			}
		}
	}
	if len(project.Configuration.Options) != 1 || len(project.ROM) != 8 {
		t.Fatalf("shared outputs: configuration=%#v ROM=%v", project.Configuration, project.ROM)
	}
}

func TestParserValidationAndSharedValues(t *testing.T) {
	t.Run("duplicate mode", func(t *testing.T) {
		parser := frontend.NewParser()
		loaded, err := source.LoadMode(mode.ModePlay, "./testdata/multimode")
		if err != nil || len(loaded) != 1 {
			t.Fatalf("load: packages=%d error=%v", len(loaded), err)
		}
		if err := parser.Parse(mode.ModePlay, loaded[0]); err != nil {
			t.Fatal(err)
		}
		if err := parser.Parse(mode.ModePlay, loaded[0]); err == nil || !strings.Contains(err.Error(), "already been parsed") {
			t.Fatalf("duplicate mode error = %v", err)
		}
	})
	t.Run("input", func(t *testing.T) {
		parser := frontend.NewParser()
		if _, err := parser.GetProject(); err == nil || !strings.Contains(err.Error(), "no Sonolus modes") {
			t.Fatalf("empty project error = %v", err)
		}
		if err := parser.Parse(mode.Mode("invalid"), nil); err == nil || !strings.Contains(err.Error(), "invalid Sonolus mode") {
			t.Fatalf("invalid mode error = %v", err)
		}
		if err := parser.Parse(mode.ModePlay, nil); err == nil || !strings.Contains(err.Error(), "package is nil") {
			t.Fatalf("nil package error = %v", err)
		}
	})
	for _, test := range []struct {
		name, pattern, message string
	}{
		{"configuration mismatch", "./testdata/configurationmismatch", "configuration.options[0].def"},
		{"ROM mismatch", "./testdata/rommismatch", "first differing byte at offset"},
	} {
		t.Run(test.name, func(t *testing.T) {
			parser := frontend.NewParser()
			loadInto(t, parser, mode.ModePlay, test.pattern)
			loadInto(t, parser, mode.ModeWatch, test.pattern)
			_, err := parser.GetProject()
			if err == nil || !strings.Contains(err.Error(), test.message) || !strings.Contains(err.Error(), "play") || !strings.Contains(err.Error(), "watch") {
				t.Fatalf("mismatch error = %v", err)
			}
		})
	}
	t.Run("empty shared values", func(t *testing.T) {
		parser := frontend.NewParser()
		loadInto(t, parser, mode.ModePlay, "./testdata/emptyshared")
		loadInto(t, parser, mode.ModeWatch, "./testdata/emptyshared")
		project, err := parser.GetProject()
		if err != nil {
			t.Fatal(err)
		}
		if len(project.Configuration.Options) != 0 || len(project.ROM) != 0 || !project.ROMDeclared {
			t.Fatalf("shared outputs: configuration=%#v ROM=%v declared=%t", project.Configuration, project.ROM, project.ROMDeclared)
		}
	})
}

func TestParserFailureDoesNotCommitMode(t *testing.T) {
	parser := frontend.NewParser()
	invalid, err := source.LoadMode(mode.ModePlay, "./testdata/invalidphase")
	if err != nil || len(invalid) != 1 {
		t.Fatalf("load invalid package: packages=%d error=%v", len(invalid), err)
	}
	if err := parser.Parse(mode.ModePlay, invalid[0]); err == nil {
		t.Fatal("invalid callback unexpectedly parsed")
	}
	valid, err := source.LoadMode(mode.ModePlay, "./testdata/lowering")
	if err != nil || len(valid) != 1 {
		t.Fatalf("load valid package: packages=%d error=%v", len(valid), err)
	}
	if err := parser.Parse(mode.ModePlay, valid[0]); err != nil {
		t.Fatalf("retry after failed parse: %v", err)
	}
	project, err := parser.GetProject()
	if err != nil || project.Modes[mode.ModePlay] == nil {
		t.Fatalf("project after retry: project=%#v error=%v", project, err)
	}
}

func TestParserParsesModesConcurrently(t *testing.T) {
	patterns := map[mode.Mode]string{
		mode.ModePlay:     "./testdata/lowering",
		mode.ModeWatch:    "./testdata/lowering_watch",
		mode.ModePreview:  "./testdata/lowering_preview",
		mode.ModeTutorial: "./testdata/lowering_tutorial",
	}
	loaded := make(map[mode.Mode]*packages.Package, len(patterns))
	for currentMode, pattern := range patterns {
		modePackages, err := source.LoadMode(currentMode, pattern)
		if err != nil || len(modePackages) != 1 {
			t.Fatalf("load %s: packages=%d error=%v", currentMode, len(modePackages), err)
		}
		loaded[currentMode] = modePackages[0]
	}
	parser := frontend.NewParser()
	errors := make(chan error, len(loaded))
	var waitGroup sync.WaitGroup
	for currentMode, pkg := range loaded {
		waitGroup.Add(1)
		go func(currentMode mode.Mode, pkg *packages.Package) {
			defer waitGroup.Done()
			errors <- parser.Parse(currentMode, pkg)
		}(currentMode, pkg)
	}
	waitGroup.Wait()
	close(errors)
	for err := range errors {
		if err != nil {
			t.Fatal(err)
		}
	}
	project, err := parser.GetProject()
	if err != nil || len(project.Modes) != 4 {
		t.Fatalf("concurrent project: modes=%d error=%v", len(project.Modes), err)
	}
}

func TestParserDeterminismAndROMBytes(t *testing.T) {
	parse := func() *ir.Function {
		declarations, err := parseMode(mode.ModePlay, "./testdata/lowering")
		if err != nil {
			t.Fatal(err)
		}
		return callbackByName(t, declarations.Archetypes[0].Callbacks, "preprocess")
	}
	if first, second := parse(), parse(); !reflect.DeepEqual(first, second) {
		t.Fatal("callback IR differs across independent package graphs")
	}
	parser := frontend.NewParser()
	loadInto(t, parser, mode.ModePlay, "./testdata/romequivalent")
	loadInto(t, parser, mode.ModeWatch, "./testdata/romequivalent")
	project, err := parser.GetProject()
	if err != nil {
		t.Fatal(err)
	}
	if string(project.ROM) != "ABC\n" {
		t.Fatalf("ROM = %v", project.ROM)
	}
}

func TestPackageGlobalArtifactsAreDeterministic(t *testing.T) {
	var first []byte
	for run := 0; run < 8; run++ {
		artifacts, err := NewCompiler(Options{Optimization: OptimizationStandard}, "./testdata/persistentglobals").Compile(ModePlay)
		if err != nil {
			t.Fatalf("run %d: %v", run, err)
		}
		encoded, err := json.Marshal(artifacts.Play)
		if err != nil {
			t.Fatalf("run %d marshal: %v", run, err)
		}
		if run == 0 {
			first = encoded
			continue
		}
		if !bytes.Equal(encoded, first) {
			t.Fatalf("run %d package-global EnginePlayData differs from run 0", run)
		}
	}
}

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

func TestCompilerOmitsUnusedUndeclaredROM(t *testing.T) {
	artifacts, err := NewCompiler(Options{}, "./testdata/lowering").Compile(mode.ModePlay)
	if err != nil {
		t.Fatal(err)
	}
	if artifacts.ROM != nil {
		t.Fatalf("ROM = %v, want nil", artifacts.ROM)
	}
	explicit, err := NewCompiler(Options{}, "./testdata/emptyshared").Compile(mode.ModePlay)
	if err != nil {
		t.Fatal(err)
	}
	if len(explicit.ROM) != 12 {
		t.Fatalf("explicit empty ROM length = %d, want 12", len(explicit.ROM))
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
