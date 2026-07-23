package compiler

import (
	"fmt"
	"go/ast"
	"go/types"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/WindowsSov8forUs/sonolus-core-go/core/resource"

	"github.com/WindowsSov8forUs/sonolus-go/v2/internal/compiler/catalog"
	"github.com/WindowsSov8forUs/sonolus-go/v2/internal/compiler/frontend"
	"github.com/WindowsSov8forUs/sonolus-go/v2/internal/compiler/ir"
	"github.com/WindowsSov8forUs/sonolus-go/v2/internal/compiler/mode"
	"github.com/WindowsSov8forUs/sonolus-go/v2/internal/compiler/optimize"
	"github.com/WindowsSov8forUs/sonolus-go/v2/internal/compiler/source"
)

func parseMode(m mode.Mode, pattern string) (*frontend.ModeDeclarations, error) {
	return parseModeWithOptions(m, pattern, frontend.Options{})
}

func parseModeWithOptions(m mode.Mode, pattern string, options frontend.Options) (*frontend.ModeDeclarations, error) {
	pkgs, err := source.LoadMode(m, pattern)
	if err != nil {
		return nil, err
	}
	if len(pkgs) != 1 {
		return nil, fmt.Errorf("expected exactly one main package, got %d", len(pkgs))
	}
	parser := frontend.NewParser(options)
	if err := parser.Parse(m, pkgs[0]); err != nil {
		return nil, err
	}
	project, err := parser.GetProject()
	if err != nil {
		return nil, err
	}
	return project.Modes[m], nil
}

func TestParseDeclarationsPlay(t *testing.T) {
	decl, err := parseMode(mode.ModePlay, "./testdata/declarations")
	if err != nil {
		t.Fatal(err)
	}
	if len(decl.Archetypes) != 1 || decl.Archetypes[0].Name != "TapNote" {
		t.Fatalf("unexpected archetypes: %#v", decl.Archetypes)
	}
	a := decl.Archetypes[0]
	if !a.HasInput || len(a.Imports) != 8 || len(a.Exports) != 2 {
		t.Fatalf("unexpected archetype metadata: %#v", a)
	}
	wantImports := []resource.EngineArchetypeDataName{"#BEAT", "target.x", "target.y", "path[0].x", "path[0].y", "path[1].x", "path[1].y", "single"}
	for index, imported := range a.Imports {
		if imported.Name != wantImports[index] || imported.Index != index {
			t.Fatalf("import %d = %#v, want name %q index %d", index, imported, wantImports[index], index)
		}
	}
	if !reflect.DeepEqual(a.Exports, []resource.EngineArchetypeDataName{"hit.x", "hit.y"}) {
		t.Fatalf("exports = %v", a.Exports)
	}
	if len(a.Callbacks) != 3 {
		t.Fatalf("unexpected callbacks: %#v", a.Callbacks)
	}
	if a.Callbacks[0].IR == nil || len(a.Callbacks[0].IR.Blocks) == 0 {
		t.Fatalf("preprocess callback was not lowered: %#v", a.Callbacks[0].IR)
	}
	if decl.Resources.Skin == nil || len(decl.Resources.Skin.Sprites) != 1 {
		t.Fatalf("unexpected skin: %#v", decl.Resources.Skin)
	}
	if len(decl.Configuration.Value.Options) != 3 {
		t.Fatalf("unexpected options: %#v", decl.Configuration.Value.Options)
	}
	if decl.ROM == nil || len(decl.ROM.Values) != 3 {
		t.Fatalf("unexpected ROM: %#v", decl.ROM)
	}
}

func TestCallbackLoweringRangeAndImmediateClosure(t *testing.T) {
	decl, err := parseModeWithOptions(mode.ModePlay, "./testdata/lowering", frontend.Options{RuntimeChecks: frontend.RuntimeChecksTerminate})
	if err != nil {
		t.Fatal(err)
	}
	callback := decl.Archetypes[0].Callbacks[0]
	foundEntityRef := false
	for _, field := range decl.Archetypes[0].Fields {
		foundEntityRef = foundEntityRef || field.GoName == "Ref" && field.Storage == "imported" && field.Size == 1
	}
	if !foundEntityRef {
		t.Fatalf("EntityRef storage did not retain its one-slot imported layout: %#v", decl.Archetypes[0].Fields)
	}
	if callback.IR == nil || len(callback.IR.Blocks) < 5 {
		t.Fatalf("callback was not lowered to a loop CFG: %#v", callback.IR)
	}
	for _, local := range callback.IR.Locals {
		if local.Slots == 7 && local.Name == "[7]float64" {
			t.Fatal("dynamic indexing copied an immutable package array into callback locals")
		}
	}
	foundDraw := false
	foundCurvedDraw := false
	foundContainerMemory := false
	foundContainerStride := false
	unreachableBlocks := 0
	foundTime := false
	foundSafeArea := false
	foundScreenAspect := false
	foundSpawn := false
	uiStoreOffsets := map[int]int{}
	foundUIConfiguration := false
	foundTouchStride := false
	foundNestedArrayPlace := false
	foundMemoryArrayRange := false
	foundLevelMemoryRead := false
	foundLevelMemoryWrite := false
	foundLevelBucketRead := false
	foundLevelBucketWrite := false
	partialResultZeroes := map[int]bool{}
	containerSizeZeroed := false
	rangeLengthSnapshotted := false
	transformOffsets := map[int]bool{}
	effectfulReturns := map[resource.RuntimeFunction]int{}
	runtimeCalls := map[resource.RuntimeFunction][]ir.RuntimeCall{}
	visitExpr := func(expr ir.Expr) {}
	visitExpr = func(expr ir.Expr) {
		switch value := expr.(type) {
		case ir.Load:
			if place, ok := value.Place.(ir.IndexedLocalPlace); ok {
				foundNestedArrayPlace = foundNestedArrayPlace || place.Base == 1 && place.Length == 2 && place.Stride == 1
			}
			if place, ok := value.Place.(ir.MemoryPlace); ok {
				_, dynamicMemoryIndex := place.Index.(ir.Load)
				foundMemoryArrayRange = foundMemoryArrayRange || place.Storage == "memory" && place.Stride == 1 && dynamicMemoryIndex
				foundTime = foundTime || place.Storage == "RuntimeUpdate" && place.Offset == 0
				foundSafeArea = foundSafeArea || place.Storage == "RuntimeEnvironment" && (place.Offset == 5 || place.Offset == 6)
				foundScreenAspect = foundScreenAspect || place.Storage == "RuntimeEnvironment" && place.Offset == 1
				foundUIConfiguration = foundUIConfiguration || place.Storage == "RuntimeUIConfiguration" && place.Offset == 0
				foundTouchStride = foundTouchStride || place.Storage == "RuntimeTouch" && place.Stride == 15
				foundLevelMemoryRead = foundLevelMemoryRead || place.Storage == "LevelMemory" && place.Stride == 1
				foundLevelBucketRead = foundLevelBucketRead || place.Storage == "LevelBucket" && place.Stride == 6
			}
		case ir.RuntimeCall:
			effectfulReturns[value.Function]++
			runtimeCalls[value.Function] = append(runtimeCalls[value.Function], value)
			for _, arg := range value.Args {
				visitExpr(arg)
			}
		}
	}
	for _, block := range callback.IR.Blocks {
		if _, ok := block.Terminator.(ir.Unreachable); ok {
			unreachableBlocks++
		}
		for _, instruction := range block.Instructions {
			if store, ok := instruction.(ir.Store); ok {
				visitExpr(store.Value)
				if place, ok := store.Place.(ir.LocalPlace); ok {
					rangeLengthSnapshotted = rangeLengthSnapshotted || place.Name == "range.length"
					if constant, ok := store.Value.(ir.Const); ok && constant.Value == 0 {
						if place.Name == "partialPair.result" {
							partialResultZeroes[place.Offset] = true
						}
						containerSizeZeroed = containerSizeZeroed || place.Name == "container.size"
					}
				}
				if place, ok := store.Place.(ir.MemoryPlace); ok && place.Storage == "RuntimeUI" {
					uiStoreOffsets[place.Offset]++
				}
				if place, ok := store.Place.(ir.MemoryPlace); ok && place.Storage == "memory" {
					foundContainerMemory = true
					foundContainerStride = foundContainerStride || place.Stride == 2
				}
				if place, ok := store.Place.(ir.MemoryPlace); ok && place.Storage == "SkinTransform" {
					transformOffsets[place.Offset] = true
				}
				if place, ok := store.Place.(ir.MemoryPlace); ok && place.Storage == "LevelMemory" && place.Stride == 1 {
					foundLevelMemoryWrite = true
				}
				if place, ok := store.Place.(ir.MemoryPlace); ok && place.Storage == "LevelBucket" && place.Stride == 6 {
					foundLevelBucketWrite = true
				}
			}
			if eval, ok := instruction.(ir.Eval); ok {
				visitExpr(eval.Value)
				if call, ok := eval.Value.(ir.RuntimeCall); ok && call.Function == resource.RuntimeFunctionDraw {
					foundDraw = true
				}
				if call, ok := eval.Value.(ir.RuntimeCall); ok && call.Function == resource.RuntimeFunctionDrawCurvedB && len(call.Args) == 17 {
					foundCurvedDraw = true
				}
				if call, ok := eval.Value.(ir.RuntimeCall); ok && call.Function == resource.RuntimeFunctionSpawn {
					if len(call.Args) != 2 {
						t.Fatalf("Spawn arguments = %#v; expected archetype ID and one memory slot", call.Args)
					}
					id, ok := call.Args[0].(ir.Const)
					if !ok || id.Value != 1 {
						t.Fatalf("Spawn archetype ID = %#v; expected stable sorted ID 1", call.Args[0])
					}
					foundSpawn = true
				}
			}
		}
	}
	if !foundDraw || !foundCurvedDraw {
		t.Fatalf("resource field draw lowering mismatch: draw=%v curved=%v", foundDraw, foundCurvedDraw)
	}
	if !foundContainerMemory {
		t.Fatal("archetype container was not lowered to semantic memory")
	}
	if !foundContainerStride || unreachableBlocks == 0 {
		t.Fatalf("container bounds/stride lowering mismatch: stride=%v unreachable=%d", foundContainerStride, unreachableBlocks)
	}
	if !foundTime || !foundSafeArea || !foundScreenAspect {
		t.Fatalf("facade memory was not lowered: time=%v safeArea=%v screen=%v", foundTime, foundSafeArea, foundScreenAspect)
	}
	if !foundSpawn {
		t.Fatal("archetype Spawn was not lowered")
	}
	for offset := range 80 {
		if uiStoreOffsets[offset] != 1 {
			t.Fatalf("runtime UI store offsets = %#v; offset %d count = %d, want 1", uiStoreOffsets, offset, uiStoreOffsets[offset])
		}
	}
	if len(uiStoreOffsets) != 80 || !foundUIConfiguration {
		t.Fatalf("runtime UI lowering mismatch: offsets=%#v configuration=%v", uiStoreOffsets, foundUIConfiguration)
	}
	if !foundTouchStride {
		t.Fatal("touch lookup was not lowered with RuntimeTouch stride 15")
	}
	if !foundLevelMemoryRead || !foundLevelMemoryWrite {
		t.Fatalf("LevelMemory facade was not lowered: read=%v write=%v", foundLevelMemoryRead, foundLevelMemoryWrite)
	}
	if !foundLevelBucketRead || !foundLevelBucketWrite {
		t.Fatalf("Bucket window facade was not lowered: read=%v write=%v", foundLevelBucketRead, foundLevelBucketWrite)
	}
	if !foundNestedArrayPlace {
		t.Fatal("dynamic nested array index did not preserve base, length, and stride")
	}
	if !foundMemoryArrayRange {
		t.Fatal("archetype memory array range did not retain dynamic index, stride, and base offset")
	}
	if !partialResultZeroes[0] || !partialResultZeroes[1] || !containerSizeZeroed {
		t.Fatalf("Go zero initialization is missing: partial=%v containerSize=%v", partialResultZeroes, containerSizeZeroed)
	}
	if !rangeLengthSnapshotted {
		t.Fatal("container range length was not snapshotted before the loop")
	}
	for _, offset := range []int{0, 1, 3, 4, 5, 7, 12, 13, 15} {
		if !transformOffsets[offset] {
			t.Fatalf("SkinTransform store offsets = %#v; missing %d", transformOffsets, offset)
		}
	}
	if len(transformOffsets) != 9 {
		t.Fatalf("SkinTransform store offsets = %#v; expected nine projective 4x4 offsets", transformOffsets)
	}
	if effectfulReturns[resource.RuntimeFunctionPlayLooped] != 2 || effectfulReturns[resource.RuntimeFunctionPlayLoopedScheduled] != 1 || effectfulReturns[resource.RuntimeFunctionSpawnParticleEffect] != 1 {
		t.Fatalf("each effectful return call must execute once: %#v", effectfulReturns)
	}
	if effectfulReturns[resource.RuntimeFunctionRandom] != 1 || effectfulReturns[resource.RuntimeFunctionRandomInteger] != 1 {
		t.Fatalf("short-circuit and switch operands must execute once: %#v", effectfulReturns)
	}
	for _, runtime := range []resource.RuntimeFunction{resource.RuntimeFunctionRandom, resource.RuntimeFunctionRandomInteger} {
		call := runtimeCalls[runtime][0]
		if len(call.Args) != 2 {
			t.Fatalf("%s arguments = %#v", runtime, call.Args)
		}
		minimum, ok := call.Args[0].(ir.Const)
		if !ok || minimum.Value != 0 {
			t.Fatalf("%s minimum = %#v, want 0", runtime, call.Args[0])
		}
	}
	for _, runtime := range []resource.RuntimeFunction{
		resource.RuntimeFunctionAbs, resource.RuntimeFunctionFloor, resource.RuntimeFunctionCeil,
		resource.RuntimeFunctionTrunc, resource.RuntimeFunctionLog,
		resource.RuntimeFunctionSin, resource.RuntimeFunctionCos, resource.RuntimeFunctionTan,
		resource.RuntimeFunctionSinh, resource.RuntimeFunctionCosh, resource.RuntimeFunctionTanh,
		resource.RuntimeFunctionArcsin, resource.RuntimeFunctionArccos, resource.RuntimeFunctionArctan,
		resource.RuntimeFunctionArctan2, resource.RuntimeFunctionMin, resource.RuntimeFunctionMax,
		resource.RuntimeFunctionPower, resource.RuntimeFunctionRem, resource.RuntimeFunctionSign,
	} {
		if len(runtimeCalls[runtime]) == 0 {
			t.Fatalf("math intrinsic %s was not lowered", runtime)
		}
	}
}

func TestDebugRuntimeCallsAreAvailableOutsidePreprocess(t *testing.T) {
	decl, err := parseMode(mode.ModePlay, "./testdata/lowering")
	if err != nil {
		t.Fatal(err)
	}
	facts := inspectFunction(callbackByName(t, decl.Archetypes[0].Callbacks, "updateParallel"))
	if len(facts.calls[resource.RuntimeFunctionDebugLog]) != 1 || len(facts.calls[resource.RuntimeFunctionDebugPause]) != 1 {
		t.Fatalf("debug runtime calls were not lowered outside preprocess: %#v", facts.calls)
	}
}

func TestEveryPublicNativeLowersThroughPackageToIR(t *testing.T) {
	decl, err := parseMode(mode.ModePlay, "./testdata/nativecoverage")
	if err != nil {
		t.Fatal(err)
	}
	facts := inspectFunction(callbackByName(t, decl.Archetypes[0].Callbacks, "preprocess"))
	for i := range catalog.Symbols {
		symbol := &catalog.Symbols[i]
		if symbol.Kind != catalog.KindNative {
			continue
		}
		if count := len(facts.calls[symbol.Runtime]); count != 1 {
			t.Errorf("native %s (%s) lowered %d times, want 1", symbol.Key(), symbol.Runtime, count)
		}
	}
}

func TestModesAcceptSharedResourceDeclarations(t *testing.T) {
	for _, currentMode := range []mode.Mode{mode.ModePlay, mode.ModeWatch, mode.ModePreview, mode.ModeTutorial} {
		declarations, err := parseMode(currentMode, "./testdata/lowering_preview")
		if err != nil {
			t.Fatal(err)
		}
		resources := declarations.Resources
		if resources.Skin == nil || resources.Effect == nil || resources.Particle == nil || resources.Instruction == nil || len(resources.Buckets) != 1 || len(resources.Instruction.Texts) != 1 || len(resources.Instruction.Icons) != 1 {
			t.Fatalf("%s did not parse all shared resources: %#v", currentMode, resources)
		}
	}

	artifacts, err := NewCompiler(Options{}, "./testdata/lowering_preview").CompileAll()
	if err != nil {
		t.Fatal(err)
	}
	if artifacts.Configuration == nil || artifacts.Play == nil || artifacts.Watch == nil || artifacts.Preview == nil || artifacts.Tutorial == nil {
		t.Fatalf("shared declarations did not compile for all modes: %#v", artifacts)
	}
	if len(artifacts.Play.Effect.Clips) != 1 || len(artifacts.Play.Particle.Effects) != 1 || len(artifacts.Play.Buckets) != 1 || len(artifacts.Watch.Effect.Clips) != 1 || len(artifacts.Watch.Particle.Effects) != 1 || len(artifacts.Watch.Buckets) != 1 {
		t.Fatalf("play/watch resources were not selected: play=%#v watch=%#v", artifacts.Play, artifacts.Watch)
	}
	if len(artifacts.Preview.Skin.Sprites) != 1 || len(artifacts.Tutorial.Effect.Clips) != 1 || len(artifacts.Tutorial.Particle.Effects) != 1 || len(artifacts.Tutorial.Instruction.Texts) != 1 || len(artifacts.Tutorial.Instruction.Icons) != 1 {
		t.Fatalf("preview/tutorial resources were not selected: preview=%#v tutorial=%#v", artifacts.Preview, artifacts.Tutorial)
	}
}

func TestEveryCallbackRecipeHasSuccessfulPackageToBackendFixture(t *testing.T) {
	fixtures := []struct {
		mode    mode.Mode
		pattern string
	}{
		{mode.ModePlay, "./testdata/lowering"},
		{mode.ModeWatch, "./testdata/lowering_watch"},
		{mode.ModePreview, "./testdata/lowering_preview"},
		{mode.ModeTutorial, "./testdata/lowering_tutorial"},
		{mode.ModePlay, "./testdata/conformance"},
		{mode.ModePlay, "./testdata/entityref"},
		{mode.ModePlay, "./testdata/archetypemro"},
		{mode.ModePlay, "./testdata/streams"},
		{mode.ModeWatch, "./testdata/streams"},
	}
	seen := map[string]bool{}
	var calledObject func(*types.Info, ast.Expr) types.Object
	calledObject = func(info *types.Info, expression ast.Expr) types.Object {
		switch expression := expression.(type) {
		case *ast.ParenExpr:
			return calledObject(info, expression.X)
		case *ast.IndexExpr:
			return calledObject(info, expression.X)
		case *ast.IndexListExpr:
			return calledObject(info, expression.X)
		case *ast.Ident:
			return info.ObjectOf(expression)
		case *ast.SelectorExpr:
			return info.ObjectOf(expression.Sel)
		default:
			return nil
		}
	}
	for _, fixture := range fixtures {
		currentMode, pattern := fixture.mode, fixture.pattern
		if _, err := parseMode(currentMode, pattern); err != nil {
			t.Fatalf("lower %s fixture: %v", currentMode, err)
		}
		if _, err := NewCompiler(Options{}, pattern).Compile(currentMode); err != nil {
			t.Fatalf("compile %s fixture through backend: %v", currentMode, err)
		}
		packages, err := source.LoadMode(currentMode, pattern)
		if err != nil || len(packages) != 1 {
			t.Fatalf("load %s fixture: packages=%d error=%v", currentMode, len(packages), err)
		}
		pkg := packages[0]
		for _, file := range pkg.Syntax {
			ast.Inspect(file, func(node ast.Node) bool {
				call, ok := node.(*ast.CallExpr)
				if !ok {
					return true
				}
				object := calledObject(pkg.TypesInfo, call.Fun)
				if symbol, ok := catalog.LookupObject(object); ok {
					seen[symbol.Key()] = true
				}
				return true
			})
		}
	}
	var missing []string
	for i := range catalog.Symbols {
		symbol := &catalog.Symbols[i]
		if symbol.Internal || symbol.Kind == catalog.KindNative || (symbol.Kind != catalog.KindFunction && symbol.Kind != catalog.KindMethod) {
			continue
		}
		recipe := catalog.LookupRecipe(symbol)
		switch recipe.Kind {
		case catalog.RecipeRuntime, catalog.RecipeAggregate, catalog.RecipeMemory, catalog.RecipeResource, catalog.RecipeContainer:
			if !seen[symbol.Key()] {
				missing = append(missing, symbol.Key())
			}
		}
	}
	if len(missing) != 0 {
		sort.Strings(missing)
		t.Fatalf("callback recipes without a successful Package -> IR -> backend fixture call:\n%s", strings.Join(missing, "\n"))
	}
}

func TestPromotedCallbackLowersBodyFromDefiningPackage(t *testing.T) {
	decl, err := parseMode(mode.ModePlay, "./testdata/promotedcallback")
	if err != nil {
		t.Fatal(err)
	}
	fn := callbackByName(t, decl.Archetypes[0].Callbacks, "preprocess")
	facts := inspectFunction(fn)
	calls := facts.calls[resource.RuntimeFunctionDebugLog]
	if len(calls) != 1 || len(calls[0].Args) != 1 {
		t.Fatalf("promoted callback body was not lowered from helper package: %#v", calls)
	}
	value, ok := calls[0].Args[0].(ir.Const)
	if !ok || value.Value != 7 {
		t.Fatalf("promoted callback argument = %#v, want 7", calls[0].Args[0])
	}
}

func TestStructuralMixinFieldsAndCallbacksAcrossModes(t *testing.T) {
	for _, currentMode := range []mode.Mode{mode.ModePlay, mode.ModeWatch} {
		declarations, err := parseMode(currentMode, "./testdata/structmixin")
		if err != nil {
			t.Fatalf("parse %s: %v", currentMode, err)
		}
		if len(declarations.Archetypes) != 1 {
			t.Fatalf("%s archetypes = %+v", currentMode, declarations.Archetypes)
		}
		archetype := declarations.Archetypes[0]
		if archetype.Base == nil || len(archetype.MRO) != 2 {
			t.Fatalf("%s mixin changed archetype MRO: %+v", currentMode, archetype)
		}
		gotFields := make([]string, len(archetype.Fields))
		for index, field := range archetype.Fields {
			gotFields[index] = field.GoName
		}
		wantFields := []string{"BaseValue", "Beat", "Time", "Seen", "Local"}
		if !reflect.DeepEqual(gotFields, wantFields) {
			t.Fatalf("%s fields = %v, want %v", currentMode, gotFields, wantFields)
		}
		if len(archetype.Imports) != 1 || archetype.Imports[0].Name != "#BEAT" || archetype.Imports[0].Index != 0 {
			t.Fatalf("%s imports = %+v", currentMode, archetype.Imports)
		}
		if len(archetype.Callbacks) != 1 || archetype.Callbacks[0].Name != "preprocess" {
			t.Fatalf("%s callbacks = %+v", currentMode, archetype.Callbacks)
		}
	}
	for _, level := range []optimize.Level{optimize.LevelMinimal, optimize.LevelFast, optimize.LevelStandard} {
		artifacts, err := NewCompiler(Options{Optimization: level}, "./testdata/structmixin").Compile(mode.ModePlay, mode.ModeWatch)
		if err != nil {
			t.Fatalf("level %d: %v", level, err)
		}
		if artifacts.Play == nil || artifacts.Watch == nil {
			t.Fatalf("level %d did not produce play and watch artifacts", level)
		}
	}
}

func TestStructuralMixinRejectsInvalidLayoutsAndCallbacks(t *testing.T) {
	_, err := NewCompiler(Options{}, "./testdata/invalidstructmixin").Compile(mode.ModePlay)
	if err == nil {
		t.Fatal("invalid structural mixins compiled successfully")
	}
	for _, message := range []string{
		"structural mixin Leaf is embedded more than once",
		"structural mixin field DuplicateThroughBase.Leaf.Value is embedded more than once",
		`duplicate external field name "duplicate"`,
		"data storage exceeds capacity 32",
		"InvalidCallbackMixin.Preprocess: callback must not have parameters",
	} {
		if !strings.Contains(err.Error(), message) {
			t.Errorf("missing %q in error:\n%v", message, err)
		}
	}
}

func TestArchetypeInheritanceAndReferenceMatching(t *testing.T) {
	declarations, err := parseMode(mode.ModePlay, "./testdata/archetypemro")
	if err != nil {
		t.Fatal(err)
	}
	byName := map[string]*frontend.ArchetypeDeclaration{}
	for _, archetype := range declarations.Archetypes {
		byName[archetype.Name] = archetype
	}
	base, derived, grand := byName["Base"], byName["Derived"], byName["GrandDerived"]
	concrete := byName["ConcreteNote"]
	if base == nil || derived == nil || derived.Base != base || len(derived.MRO) != 2 {
		t.Fatalf("unexpected inheritance model: base=%p derived=%+v", base, derived)
	}
	if len(derived.Fields) != 2 || derived.Fields[0].GoName != "Value" || derived.Fields[1].GoName != "Extra" {
		t.Fatalf("unexpected inherited field layout: %+v", derived.Fields)
	}
	if len(derived.Callbacks) != 1 || derived.Callbacks[0].Name != "updateSequential" || derived.Callbacks[0].Order != -4 {
		t.Fatalf("unexpected inherited callback: %+v", derived.Callbacks)
	}
	if grand == nil || grand.Base != derived || len(grand.MRO) != 3 || len(grand.Fields) != 3 || grand.Fields[2].GoName != "GrandExtra" {
		t.Fatalf("unexpected deep inheritance model: %+v", grand)
	}
	if !base.HasInput || derived.HasInput || grand.HasInput {
		t.Fatalf("hasInput must remain concrete: base=%t derived=%t grand=%t", base.HasInput, derived.HasInput, grand.HasInput)
	}
	if len(grand.Callbacks) != 1 || grand.Callbacks[0].Order != -4 {
		t.Fatalf("deep callback/order inheritance = %+v", grand.Callbacks)
	}
	if byName["AbstractNote"] != nil || concrete == nil || concrete.Base == nil || !concrete.Base.Abstract {
		t.Fatalf("abstract archetype visibility/inheritance = %+v", declarations.Archetypes)
	}
	if len(concrete.Fields) != 2 || len(concrete.Callbacks) != 1 || concrete.Callbacks[0].Name != "preprocess" || concrete.Callbacks[0].Order != -7 || !concrete.HasKey || concrete.Key != 7 {
		t.Fatalf("abstract archetype layout/callback inheritance = %+v", concrete)
	}
	for _, level := range []optimize.Level{optimize.LevelMinimal, optimize.LevelFast, optimize.LevelStandard} {
		if _, err := NewCompiler(Options{Optimization: level}, "./testdata/archetypemro").Compile(mode.ModePlay); err != nil {
			t.Fatalf("level %d: %v", level, err)
		}
	}
}

func TestAbstractArchetypeRejectsRuntimeMetadataAndSpawn(t *testing.T) {
	_, err := NewCompiler(Options{}, "./testdata/invalidabstractarchetype").Compile(mode.ModePlay)
	if err == nil || !strings.Contains(err.Error(), "abstract archetype cannot declare a runtime name") || !strings.Contains(err.Error(), "abstract archetype cannot declare hasInput") || !strings.Contains(err.Error(), "abstract archetype cannot declare a key") || !strings.Contains(err.Error(), "archetype key must be a finite number") {
		t.Fatalf("unexpected abstract metadata error: %v", err)
	}
	_, err = NewCompiler(Options{}, "./testdata/invalidabstractspawn").Compile(mode.ModePlay)
	if err == nil || !strings.Contains(err.Error(), "Spawn argument type") {
		t.Fatalf("unexpected abstract spawn error: %v", err)
	}
}

func TestTypedReplayStreamsShareLayoutAndLower(t *testing.T) {
	compiler := NewCompiler(Options{}, "./testdata/streams")
	artifacts, err := compiler.Compile(mode.ModePlay, mode.ModeWatch)
	if err != nil {
		t.Fatal(err)
	}
	if artifacts.Play == nil || artifacts.Watch == nil {
		t.Fatal("stream fixture did not produce play and watch data")
	}
	play, err := parseMode(mode.ModePlay, "./testdata/streams")
	if err != nil {
		t.Fatal(err)
	}
	facts := inspectFunction(callbackByName(t, play.Archetypes[0].Callbacks, "preprocess"))
	if len(facts.calls[resource.RuntimeFunctionStreamSet]) != 9 {
		t.Fatalf("StreamSet calls = %d, want 9", len(facts.calls[resource.RuntimeFunctionStreamSet]))
	}
}

func TestTypedReplayStreamsRejectLowLevelIDCollision(t *testing.T) {
	_, err := NewCompiler(Options{}, "./testdata/invalidstreamcollision").Compile(mode.ModePlay)
	if err == nil || !strings.Contains(err.Error(), "overlaps typed stream IDs 1..1") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestTypedLevelGlobalsAllocateAndLowerAcrossModes(t *testing.T) {
	compiler := NewCompiler(Options{}, "./testdata/levelglobals")
	artifacts, err := compiler.CompileAll()
	if err != nil {
		t.Fatal(err)
	}
	if artifacts.Play == nil || artifacts.Watch == nil || artifacts.Preview == nil || artifacts.Tutorial == nil {
		t.Fatal("level global fixture did not compile all modes")
	}

	playDeclarations, err := parseMode(mode.ModePlay, "./testdata/levelglobals")
	if err != nil {
		t.Fatal(err)
	}
	if len(playDeclarations.LevelGlobals) != 4 {
		t.Fatalf("play level globals = %+v", playDeclarations.LevelGlobals)
	}
	byType := map[string]*frontend.LevelGlobalDeclaration{}
	for _, declaration := range playDeclarations.LevelGlobals {
		byType[declaration.TypeName] = declaration
	}
	input, notes, nested, data := byType["PlayInputMemory"], byType["PlayNoteMemory"], byType["PlayNestedMemory"], byType["PlayComputedData"]
	if input == nil || input.Offset != 0 || input.Size != 5 || len(input.Fields) != 1 || input.Fields[0].Capacity != 4 {
		t.Fatalf("input memory layout = %+v", input)
	}
	if notes == nil || notes.Offset != 21 || notes.Size != 10 || len(notes.Fields) != 1 || notes.Fields[0].Capacity != 3 || notes.Fields[0].ElementSize != 3 {
		t.Fatalf("note memory layout = %+v", notes)
	}
	if nested == nil || nested.Offset != 5 || nested.Size != 16 || len(nested.Fields) != 1 || len(nested.Fields[0].Fields) != 3 || len(nested.Fields[0].Fields[1].Elements) != 2 || len(nested.Fields[0].Fields[2].Elements) != 2 {
		t.Fatalf("nested memory layout = %+v", nested)
	}
	if data == nil || data.Offset != 0 || data.Size != 32 || len(data.Fields) != 4 || data.Fields[1].Offset != 1 || data.Fields[1].Size != 4 || data.Fields[2].Offset != 5 || data.Fields[2].Size != 9 || data.Fields[3].Offset != 14 || data.Fields[3].Size != 18 {
		t.Fatalf("computed data layout = %+v", data)
	}

	archetype := playDeclarations.Archetypes[0]
	preprocess := inspectFunction(callbackByName(t, archetype.Callbacks, "preprocess"))
	sequential := inspectFunction(callbackByName(t, archetype.Callbacks, "updateSequential"))
	parallel := inspectFunction(callbackByName(t, archetype.Callbacks, "updateParallel"))
	if countMemory(preprocess.stores, "LevelData") == 0 || countMemory(preprocess.stores, "LevelMemory") == 0 {
		t.Fatalf("preprocess stores = %+v", preprocess.stores)
	}
	if countMemory(sequential.stores, "LevelMemory") == 0 {
		t.Fatalf("updateSequential stores = %+v", sequential.stores)
	}
	for _, place := range parallel.loads {
		if (place.Storage == "LevelData" || place.Storage == "LevelMemory") && place.Write {
			t.Fatalf("parallel level global load is writable: %+v", place)
		}
	}

	for _, currentMode := range []mode.Mode{mode.ModeWatch, mode.ModePreview, mode.ModeTutorial} {
		declarations, parseErr := parseMode(currentMode, "./testdata/levelglobals")
		if parseErr != nil {
			t.Fatalf("%s: %v", currentMode, parseErr)
		}
		if len(declarations.LevelGlobals) != 2 {
			t.Fatalf("%s level globals = %+v", currentMode, declarations.LevelGlobals)
		}
	}
}

func TestPersistentLevelGlobalPointersAndInterfacesCompile(t *testing.T) {
	declarations, err := parseMode(mode.ModePlay, "./testdata/persistentglobals")
	if err != nil {
		t.Fatal(err)
	}
	if len(declarations.LevelGlobals) != 1 {
		t.Fatalf("persistent level globals = %+v", declarations.LevelGlobals)
	}
	declaration := declarations.LevelGlobals[0]
	if declaration.Size != 11 || len(declaration.Fields) != 8 {
		t.Fatalf("persistent level global layout = %+v", declaration)
	}
	if declaration.Fields[2].PersistentKind != "pointer" || declaration.Fields[3].PersistentKind != "pointer" || declaration.Fields[6].PersistentKind != "interface" {
		t.Fatalf("persistent field layouts = %+v", declaration.Fields)
	}
}

func TestTypedLevelGlobalsRejectInvalidDeclarationsAndWrites(t *testing.T) {
	_, err := NewCompiler(Options{}, "./testdata/invalidlevelglobals").Compile(mode.ModePlay)
	for _, message := range []string{
		"initialize the container with sonolus.NewVarArray(capacity)",
		"runtime level global fields must have zero initial values",
		"requires exactly one singleton variable",
		"promoted level global markers are not allowed",
		"array elements must use identical container layouts",
		"persistent pointer target",
		"persistent pointer fields must have nil initial values",
	} {
		if err == nil || !strings.Contains(err.Error(), message) {
			t.Fatalf("invalid declarations error %q missing from: %v", message, err)
		}
	}

	_, err = NewCompiler(Options{}, "./testdata/invalidlevelglobalpreview").Compile(mode.ModePreview)
	if err == nil || !strings.Contains(err.Error(), "level memory globals are not available in preview mode") {
		t.Fatalf("unexpected preview memory error: %v", err)
	}

	_, err = NewCompiler(Options{}, "./testdata/invalidlevelglobalwrite").Compile(mode.ModePlay)
	if err == nil || !strings.Contains(err.Error(), "LevelData storage is read-only") {
		t.Fatalf("unexpected level data write error: %v", err)
	}
}

func TestRuntimeCheckModesAndDiagnostics(t *testing.T) {
	none, err := NewCompiler(Options{RuntimeChecks: RuntimeChecksNone}, "./testdata/simulator").Compile(mode.ModePlay)
	if err != nil {
		t.Fatal(err)
	}
	if len(none.Diagnostics) != 0 {
		t.Fatalf("none diagnostics = %#v", none.Diagnostics)
	}
	notify, err := NewCompiler(Options{RuntimeChecks: RuntimeChecksNotify}, "./testdata/simulator").Compile(mode.ModePlay)
	if err != nil {
		t.Fatal(err)
	}
	if len(notify.Diagnostics) == 0 {
		t.Fatalf("notify diagnostics = %#v", notify.Diagnostics)
	}
	foundNoteCheck := false
	for code := 1; code <= len(notify.Diagnostics); code++ {
		message, exists := notify.Diagnostics[code]
		if !exists {
			t.Fatalf("diagnostic codes must be contiguous from 1: %#v", notify.Diagnostics)
		}
		foundNoteCheck = foundNoteCheck || strings.Contains(message, "note value must be nonnegative")
	}
	if !foundNoteCheck {
		t.Fatalf("note runtime check is missing from diagnostics: %#v", notify.Diagnostics)
	}
	repeated, err := NewCompiler(Options{RuntimeChecks: RuntimeChecksNotify}, "./testdata/simulator").Compile(mode.ModePlay)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(notify.Diagnostics, repeated.Diagnostics) {
		t.Fatalf("diagnostics are not deterministic:\nfirst  %#v\nsecond %#v", notify.Diagnostics, repeated.Diagnostics)
	}
}

func TestDiagnosticsAreIndependentOfCheckoutPath(t *testing.T) {
	moduleRoot, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	compile := func(root string) map[int]string {
		t.Helper()
		files := map[string]string{
			"go.mod":  fmt.Sprintf("module example.com/diagnostics\n\ngo 1.25.12\n\nrequire github.com/WindowsSov8forUs/sonolus-go/v2 v2.0.0\nreplace github.com/WindowsSov8forUs/sonolus-go/v2 => %s\n", filepath.ToSlash(moduleRoot)),
			"main.go": "package main\n\nfunc main() {}\n",
			"play.go": `//go:build play

package main

import (
	"github.com/WindowsSov8forUs/sonolus-go/v2/sonolus"
	"github.com/WindowsSov8forUs/sonolus-go/v2/sonolus/play"
)

type Note struct {
	play.Archetype ` + "`archetype:\"name=Note\"`" + `
	Value float64 ` + "`archetype:\"memory\"`" + `
}

func (note *Note) Preprocess() {
	sonolus.Assert(note.Value >= 0, "value must be nonnegative")
}
`,
		}
		for name, contents := range files {
			if err := os.WriteFile(filepath.Join(root, name), []byte(contents), 0o644); err != nil {
				t.Fatal(err)
			}
		}
		original, err := os.Getwd()
		if err != nil {
			t.Fatal(err)
		}
		if err := os.Chdir(root); err != nil {
			t.Fatal(err)
		}
		defer func() {
			if err := os.Chdir(original); err != nil {
				t.Fatal(err)
			}
		}()
		artifacts, err := NewCompiler(Options{RuntimeChecks: RuntimeChecksNotify}, ".").Compile(mode.ModePlay)
		if err != nil {
			t.Fatal(err)
		}
		return artifacts.Diagnostics
	}
	first := compile(t.TempDir())
	second := compile(t.TempDir())
	if !reflect.DeepEqual(first, second) {
		t.Fatalf("diagnostics depend on checkout path:\nfirst  %#v\nsecond %#v", first, second)
	}
	for _, message := range first {
		if strings.Contains(message, filepath.ToSlash(filepath.Dir(moduleRoot))) {
			t.Fatalf("diagnostic contains an absolute checkout path: %q", message)
		}
	}
}

func TestIntegerZeroDivisorUsesRequiredRuntimeChecks(t *testing.T) {
	for _, checks := range []RuntimeChecks{RuntimeChecksNone, RuntimeChecksTerminate, RuntimeChecksNotify} {
		artifacts, err := NewCompiler(Options{RuntimeChecks: checks}, "./testdata/conformance").Compile(mode.ModePlay)
		if err != nil {
			t.Fatalf("checks %d: %v", checks, err)
		}
		var division, remainder bool
		for _, message := range artifacts.Diagnostics {
			division = division || strings.Contains(message, "integer division by zero")
			remainder = remainder || strings.Contains(message, "integer remainder by zero")
		}
		if checks == RuntimeChecksNotify {
			if !division || !remainder {
				t.Fatalf("notify diagnostics = %#v", artifacts.Diagnostics)
			}
		} else if division || remainder {
			t.Fatalf("checks %d unexpectedly emitted diagnostics: %#v", checks, artifacts.Diagnostics)
		}
	}

	declarations, err := parseModeWithOptions(mode.ModePlay, "./testdata/conformance", frontend.Options{RuntimeChecks: frontend.RuntimeChecksNotify})
	if err != nil {
		t.Fatal(err)
	}
	facts := inspectFunction(callbackByName(t, declarations.Archetypes[0].Callbacks, "preprocess"))
	if got := len(facts.calls[resource.RuntimeFunctionDebugLog]); got < 5 {
		t.Fatalf("DebugLog calls = %d, want four divisor evaluations plus callback log", got)
	}
}

func TestEntityRefGetDynamicLocalView(t *testing.T) {
	declarations, err := parseMode(mode.ModePlay, "./testdata/entityref")
	if err != nil {
		t.Fatal(err)
	}
	reader := entityRefArchetype(t, declarations, "Reader")
	for _, callbackName := range []string{"preprocess", "updateSequential", "touch"} {
		function := callbackByName(t, reader.Callbacks, callbackName)
		if err := ir.Validate(function); err != nil {
			t.Fatalf("%s IR: %v", callbackName, err)
		}
		facts := inspectEntityRefFunction(function)
		if facts.entityLocals == 0 || !facts.dynamicEntityIndex {
			t.Fatalf("%s did not preserve an entity-view local index: %#v", callbackName, facts)
		}
		if facts.memory["EntitySharedMemoryArray"] == 0 {
			t.Fatalf("%s did not access referenced shared memory: %#v", callbackName, facts.memory)
		}
	}
	preprocess := inspectEntityRefFunction(callbackByName(t, reader.Callbacks, "preprocess"))
	if preprocess.memory["EntityDataArray"] == 0 || preprocess.memory["EntitySharedMemoryArray"] == 0 {
		t.Fatalf("preprocess referenced memory = %#v", preprocess.memory)
	}
	if preprocess.calls[resource.RuntimeFunctionDebugLog] != 1 {
		t.Fatalf("EntityRef receiver evaluation count = %d, want 1", preprocess.calls[resource.RuntimeFunctionDebugLog])
	}
}

func TestEntityRefGetSurvivesOptimizationAndBackend(t *testing.T) {
	declarations, err := parseMode(mode.ModePlay, "./testdata/entityref")
	if err != nil {
		t.Fatal(err)
	}
	reader := entityRefArchetype(t, declarations, "Reader")
	for _, level := range []optimize.Level{optimize.LevelMinimal, optimize.LevelFast, optimize.LevelStandard} {
		t.Run(level.String(), func(t *testing.T) {
			optimizer := optimize.NewOptimizer(level)
			for _, callback := range reader.Callbacks {
				function, err := optimizer.Optimize(optimize.Context{Mode: mode.ModePlay, Callback: callback.Name}, callback.IR)
				if err != nil {
					t.Fatalf("optimize %s: %v", callback.Name, err)
				}
				if err := ir.ValidateFinal(function); err != nil {
					t.Fatalf("final %s: %v", callback.Name, err)
				}
			}
			artifacts, err := NewCompiler(Options{Optimization: level}, "./testdata/entityref").Compile(mode.ModePlay)
			if err != nil {
				t.Fatal(err)
			}
			if artifacts.Play == nil || !engineNodesUseBlock(artifacts.Play.Nodes, 4101) || !engineNodesUseBlock(artifacts.Play.Nodes, 4102) {
				t.Fatalf("backend did not retain referenced entity blocks: %#v", artifacts.Play)
			}
		})
	}
}

func TestEntityRefGetRejectsInaccessibleFieldsAndEscapes(t *testing.T) {
	_, err := parseMode(mode.ModePlay, "./testdata/invalidentityref")
	if err == nil {
		t.Fatal("expected EntityRef.Get errors")
	}
	for _, message := range []string{
		"EntityRef.Get cannot access memory field Target.Memory",
		"EntityRef.Get cannot access exported field Target.Exported",
		"entityRef.get target github.com/WindowsSov8forUs/sonolus-go/v2/internal/compiler/testdata/invalidentityref.NotArchetype is not an archetype declared in play mode",
		"EntityRef.Get views cannot be converted or stored in interfaces",
		"EntityRef.Get fields cannot be addressed",
		"EntityRef.Get views cannot be explicitly dereferenced",
		"EntityDataArray storage is read-only",
		"EntitySharedMemoryArray storage is read-only",
	} {
		if !strings.Contains(err.Error(), message) {
			t.Errorf("error does not contain %q:\n%v", message, err)
		}
	}
}

type entityRefFacts struct {
	entityLocals       int
	dynamicEntityIndex bool
	memory             map[string]int
	calls              map[resource.RuntimeFunction]int
}

func inspectEntityRefFunction(function *ir.Function) entityRefFacts {
	facts := entityRefFacts{memory: map[string]int{}, calls: map[resource.RuntimeFunction]int{}}
	entityLocals := map[int]bool{}
	for id, typ := range function.Locals {
		if typ.Slots == 1 && strings.HasPrefix(typ.Name, "entity-view:") {
			entityLocals[id] = true
			facts.entityLocals++
		}
	}
	var expression func(ir.Expr)
	var place func(ir.Place)
	expression = func(expressionValue ir.Expr) {
		switch value := expressionValue.(type) {
		case ir.Load:
			if local, ok := value.Place.(ir.LocalPlace); ok && entityLocals[local.ID] {
				facts.dynamicEntityIndex = true
			}
			place(value.Place)
		case ir.RuntimeCall:
			facts.calls[value.Function]++
			for _, argument := range value.Args {
				expression(argument)
			}
		}
	}
	place = func(placeValue ir.Place) {
		switch value := placeValue.(type) {
		case ir.MemoryPlace:
			facts.memory[value.Storage]++
			expression(value.Index)
		case ir.IndexedLocalPlace:
			expression(value.Index)
		}
	}
	for _, block := range function.Blocks {
		for _, instruction := range block.Instructions {
			switch value := instruction.(type) {
			case ir.Store:
				place(value.Place)
				expression(value.Value)
			case ir.Eval:
				expression(value.Value)
			}
		}
		switch terminator := block.Terminator.(type) {
		case ir.Branch:
			expression(terminator.Condition)
		case ir.Switch:
			expression(terminator.Value)
		case ir.Return:
			for _, value := range terminator.Value.Slots {
				expression(value)
			}
		}
	}
	return facts
}

func entityRefArchetype(t *testing.T, declarations *frontend.ModeDeclarations, name string) *frontend.ArchetypeDeclaration {
	t.Helper()
	for _, archetype := range declarations.Archetypes {
		if archetype.Name == name {
			return archetype
		}
	}
	t.Fatalf("archetype %s not found", name)
	return nil
}

func engineNodesUseBlock(nodes []resource.EngineDataNode, block float64) bool {
	for _, node := range nodes {
		function, ok := node.(resource.EngineDataFunctionNode)
		if !ok || len(function.Args) == 0 {
			continue
		}
		switch function.Func {
		case resource.RuntimeFunctionGet, resource.RuntimeFunctionGetShifted,
			resource.RuntimeFunctionSet, resource.RuntimeFunctionSetShifted:
			value, ok := nodes[function.Args[0]].(resource.EngineDataValueNode)
			if ok && value.Value == block {
				return true
			}
		}
	}
	return false
}

func TestCrossPackageGenericHelperUsesInstantiatedSignature(t *testing.T) {
	decl, err := parseMode(mode.ModePlay, "./testdata/crossgeneric")
	if err != nil {
		t.Fatal(err)
	}
	facts := inspectFunction(callbackByName(t, decl.Archetypes[0].Callbacks, "preprocess"))
	if len(facts.calls[resource.RuntimeFunctionDebugLog]) != 1 {
		t.Fatalf("cross-package generic helper did not lower: %#v", facts.calls)
	}
}

func TestContainerReturnedFromHelperRetainsBackingStorage(t *testing.T) {
	decl, err := parseMode(mode.ModePlay, "./testdata/containerreturn")
	if err != nil {
		t.Fatal(err)
	}
	fn := callbackByName(t, decl.Archetypes[0].Callbacks, "preprocess")
	facts := inspectFunction(fn)
	if len(facts.calls[resource.RuntimeFunctionDebugLog]) == 0 {
		t.Fatal("container returned from helper was not available to the caller")
	}
}

func TestResourceHandlesCannotBeFabricatedInCallbacks(t *testing.T) {
	_, err := parseMode(mode.ModePlay, "./testdata/invalidresourcehandle")
	if err == nil || !strings.Contains(err.Error(), "resource handle aggregates cannot be declared without a resource value") || !strings.Contains(err.Error(), "resource handle aggregates can only come from declared resource fields") {
		t.Fatalf("unexpected error: %v", err)
	}
}

type irFacts struct {
	calls  map[resource.RuntimeFunction][]ir.RuntimeCall
	loads  []ir.MemoryPlace
	stores []ir.MemoryPlace
}

func inspectFunction(fn *ir.Function) irFacts {
	facts := irFacts{calls: map[resource.RuntimeFunction][]ir.RuntimeCall{}}
	var expression func(ir.Expr)
	expression = func(expr ir.Expr) {
		switch value := expr.(type) {
		case ir.Load:
			if place, ok := value.Place.(ir.MemoryPlace); ok {
				facts.loads = append(facts.loads, place)
			}
		case ir.RuntimeCall:
			facts.calls[value.Function] = append(facts.calls[value.Function], value)
			for _, arg := range value.Args {
				expression(arg)
			}
		}
	}
	for _, block := range fn.Blocks {
		for _, instruction := range block.Instructions {
			switch value := instruction.(type) {
			case ir.Store:
				expression(value.Value)
				if place, ok := value.Place.(ir.MemoryPlace); ok {
					facts.stores = append(facts.stores, place)
				}
			case ir.Eval:
				expression(value.Value)
			}
		}
		switch term := block.Terminator.(type) {
		case ir.Branch:
			expression(term.Condition)
		case ir.Switch:
			expression(term.Value)
		case ir.Return:
			for _, slot := range term.Value.Slots {
				expression(slot)
			}
		}
	}
	return facts
}

func countMemory(places []ir.MemoryPlace, storage string) int {
	count := 0
	for _, place := range places {
		if place.Storage == storage {
			count++
		}
	}
	return count
}

func callbackByName(t *testing.T, callbacks []*frontend.CallbackDeclaration, name string) *ir.Function {
	t.Helper()
	for _, callback := range callbacks {
		if callback.Name == name {
			if callback.IR == nil {
				t.Fatalf("callback %s has no IR", name)
			}
			return callback.IR
		}
	}
	t.Fatalf("callback %s was not declared", name)
	return nil
}

func TestWatchFacadeLowering(t *testing.T) {
	decl, err := parseMode(mode.ModeWatch, "./testdata/lowering_watch")
	if err != nil {
		t.Fatal(err)
	}
	facts := inspectFunction(callbackByName(t, decl.Archetypes[0].Callbacks, "preprocess"))
	if countMemory(facts.stores, "CurrentInputResult") != 3 || countMemory(facts.stores, "RuntimeUI") != 100 || countMemory(facts.stores, "RuntimeBackground") != 8 {
		t.Fatalf("watch stores: %#v", facts.stores)
	}
	if calls := facts.calls[resource.RuntimeFunctionPlay]; len(calls) != 2 || len(calls[0].Args) != 2 || len(calls[1].Args) != 2 {
		t.Fatalf("watch Play calls: %#v", calls)
	}
	for _, offset := range []int{0, 1, 4} {
		found := false
		for _, place := range facts.loads {
			found = found || place.Storage == "RuntimeEnvironment" && place.Offset == offset
		}
		if !found {
			t.Fatalf("watch environment offset %d was not read: %#v", offset, facts.loads)
		}
	}
}

func TestPreviewFacadeLowering(t *testing.T) {
	decl, err := parseMode(mode.ModePreview, "./testdata/lowering_preview")
	if err != nil {
		t.Fatal(err)
	}
	callbacks := decl.Archetypes[0].Callbacks
	preprocess := inspectFunction(callbackByName(t, callbacks, "preprocess"))
	if countMemory(preprocess.stores, "RuntimeCanvas") != 2 || countMemory(preprocess.stores, "RuntimeUI") != 18 {
		t.Fatalf("preview preprocess stores: %#v", preprocess.stores)
	}
	render := inspectFunction(callbackByName(t, callbacks, "render"))
	if calls := render.calls[resource.RuntimeFunctionPrint]; len(calls) != 1 || len(calls[0].Args) != 14 {
		t.Fatalf("preview Print calls: %#v", calls)
	}
	if calls := render.calls[resource.RuntimeFunctionDraw]; len(calls) != 1 || len(calls[0].Args) != 14 {
		t.Fatalf("preview Draw calls: %#v", calls)
	}
}

func TestTutorialFacadeLowering(t *testing.T) {
	decl, err := parseMode(mode.ModeTutorial, "./testdata/lowering_tutorial")
	if err != nil {
		t.Fatal(err)
	}
	preprocess := inspectFunction(callbackByName(t, decl.Globals, "preprocess"))
	if countMemory(preprocess.loads, "TutorialData") == 0 || countMemory(preprocess.stores, "TutorialMemory") != 1 || countMemory(preprocess.stores, "RuntimeUI") != 36 || countMemory(preprocess.stores, "RuntimeBackground") != 8 {
		t.Fatalf("tutorial preprocess memory: loads=%#v stores=%#v", preprocess.loads, preprocess.stores)
	}
	navigate := inspectFunction(callbackByName(t, decl.Globals, "navigate"))
	if countMemory(navigate.stores, "TutorialInstruction") != 2 {
		t.Fatalf("tutorial instruction stores: %#v", navigate.stores)
	}
	update := inspectFunction(callbackByName(t, decl.Globals, "update"))
	if calls := update.calls[resource.RuntimeFunctionPaint]; len(calls) != 1 || len(calls[0].Args) != 7 {
		t.Fatalf("tutorial Paint calls: %#v", calls)
	}
}

func TestHelperPreservesArchetypeReferenceAndAssignmentOrder(t *testing.T) {
	decl, err := parseMode(mode.ModePlay, "./testdata/evaluation")
	if err != nil {
		t.Fatal(err)
	}
	fn := callbackByName(t, decl.Archetypes[0].Callbacks, "preprocess")
	var memoryStores []ir.Store
	pointerReceiverUpdatedOriginal := false
	valueReceiverUpdatedCopy := false
	for _, block := range fn.Blocks {
		for _, instruction := range block.Instructions {
			store, ok := instruction.(ir.Store)
			if !ok {
				continue
			}
			if place, ok := store.Place.(ir.LocalPlace); ok {
				if place.Name == "c" {
					_, pointerReceiverUpdatedOriginal = store.Value.(ir.RuntimeCall)
				}
				if constant, ok := store.Value.(ir.Const); ok && place.Name == "call.receiver" && constant.Value == 99 {
					valueReceiverUpdatedCopy = true
				}
			}
			if place, ok := store.Place.(ir.MemoryPlace); ok && place.Storage == "memory" {
				memoryStores = append(memoryStores, store)
			}
		}
	}
	if !pointerReceiverUpdatedOriginal || !valueReceiverUpdatedCopy {
		t.Fatalf("method receiver semantics mismatch: pointerOriginal=%v valueCopy=%v", pointerReceiverUpdatedOriginal, valueReceiverUpdatedCopy)
	}
	if len(memoryStores) != 6 {
		t.Fatalf("archetype memory stores = %#v; expected LHS/RHS helper writes followed by assignment writes", memoryStores)
	}
	var offsets []int
	for _, store := range memoryStores {
		offsets = append(offsets, store.Place.(ir.MemoryPlace).Offset)
	}
	want := []int{0, 1, 0, 0, 0, 1}
	if !reflect.DeepEqual(offsets, want) {
		t.Fatalf("archetype memory store order = %v, want %v", offsets, want)
	}
	facts := inspectFunction(fn)
	if countMemory(facts.stores, "data") != 2 || countMemory(facts.stores, "shared") != 1 {
		t.Fatalf("archetype data/shared stores = %#v", facts.stores)
	}
	if decl.Archetypes[0].Fields[2].Offset != 0 || decl.Archetypes[0].Fields[3].Offset != 1 {
		t.Fatalf("imported/data offsets do not share Entity Data: %#v", decl.Archetypes[0].Fields)
	}
}

func TestArchetypeStorageRejectsInvalidCallbackWrites(t *testing.T) {
	_, err := parseMode(mode.ModePlay, "./testdata/invalidstorage")
	if err == nil || !strings.Contains(err.Error(), "data storage is read-only") || !strings.Contains(err.Error(), "shared storage is read-only") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRandIntnRejectsNonPositiveConstantBound(t *testing.T) {
	_, err := parseMode(mode.ModePlay, "./testdata/invalidrand")
	if err == nil || !strings.Contains(err.Error(), "rand.Intn constant bound must be positive") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestGlobalCallbacksRejectUnknownExportedName(t *testing.T) {
	_, err := parseMode(mode.ModeTutorial, "./testdata/invalidglobal")
	if err == nil || !strings.Contains(err.Error(), "Updtae: unknown tutorial global callback") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestArchetypeStorageCapacityIncludesDataAndContainers(t *testing.T) {
	_, err := parseMode(mode.ModePlay, "./testdata/invalidcapacity")
	if err == nil || !strings.Contains(err.Error(), "data storage exceeds capacity 32") || !strings.Contains(err.Error(), "memory storage exceeds capacity 64") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestContainerFieldRequiresCapacity(t *testing.T) {
	_, err := parseMode(mode.ModePlay, "./testdata/invalidcontainer")
	if err == nil || !strings.Contains(err.Error(), "container field requires cap") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestSpawnRejectsUndeclaredArchetype(t *testing.T) {
	_, err := parseMode(mode.ModePlay, "./testdata/invalidspawn")
	if err == nil || !strings.Contains(err.Error(), "is not an archetype declared in play mode") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestParseDeclarationsNamedResourceValues(t *testing.T) {
	decl, err := parseMode(mode.ModePlay, "./testdata/namedresource")
	if err != nil {
		t.Fatal(err)
	}
	if decl.Resources.Skin == nil || len(decl.Resources.Skin.Sprites) != 4 {
		t.Fatalf("unexpected skin: %#v", decl.Resources.Skin)
	}
	if decl.Resources.Skin.RenderMode != "lightweight" {
		t.Fatalf("render mode = %q", decl.Resources.Skin.RenderMode)
	}
	if got := decl.Resources.Skin.Sprites[0].Name; got != "#NOTE_HEAD" {
		t.Fatalf("first sprite name = %q", got)
	}
	if got := decl.Resources.Skin.Sprites[1].Name; got != "custom.sprite" {
		t.Fatalf("second sprite name = %q", got)
	}
	if decl.Resources.SpriteIDs["#NOTE_HEAD"] != 0 || decl.Resources.SpriteIDs["custom.sprite"] != 1 {
		t.Fatalf("unexpected sprite IDs: %#v", decl.Resources.SpriteIDs)
	}
	if decl.Resources.SpriteIDs["group.0"] != 2 || decl.Resources.SpriteIDs["group.1"] != 3 {
		t.Fatalf("unexpected group IDs: %#v", decl.Resources.SpriteIDs)
	}
}

func TestParseDeclarationsTracesResourceAlias(t *testing.T) {
	decl, err := parseMode(mode.ModePlay, "./testdata/resourcealias")
	if err != nil {
		t.Fatal(err)
	}
	if decl.Resources.Skin == nil || len(decl.Resources.Skin.Sprites) != 1 || decl.Resources.Skin.Sprites[0].Name != "custom.alias" {
		t.Fatalf("unexpected skin: %#v", decl.Resources.Skin)
	}
}

func TestParseDeclarationsSeparateInstructionNamespaces(t *testing.T) {
	decl, err := parseMode(mode.ModeTutorial, "./testdata/instructionresource")
	if err != nil {
		t.Fatal(err)
	}
	if decl.Resources.Instruction == nil || len(decl.Resources.Instruction.Texts) != 1 || len(decl.Resources.Instruction.Icons) != 1 {
		t.Fatalf("unexpected instructions: %#v", decl.Resources.Instruction)
	}
	if decl.Resources.Instruction.Texts[0].Name != "Tap" || decl.Resources.Instruction.Icons[0].Name != "#HAND" {
		t.Fatalf("unexpected instruction names: %#v", decl.Resources.Instruction)
	}
}

func TestParseDeclarationsBuckets(t *testing.T) {
	decl, err := parseMode(mode.ModePlay, "./testdata/bucketresource")
	if err != nil {
		t.Fatal(err)
	}
	if len(decl.Resources.Buckets) != 1 || len(decl.Resources.Buckets[0].Sprites) != 2 {
		t.Fatalf("unexpected buckets: %#v", decl.Resources.Buckets)
	}
	bucket := decl.Resources.Buckets[0]
	if bucket.Unit != "#MILLISECONDS" || bucket.Sprites[0].ID != 0 || bucket.Sprites[1].FallbackID != 1 {
		t.Fatalf("unexpected bucket metadata: %#v", bucket)
	}
}

func TestParseDeclarationsRejectsUnsupportedStandardSymbol(t *testing.T) {
	_, err := parseMode(mode.ModePlay, "./testdata/invalidstdlib")
	if err == nil || !strings.Contains(err.Error(), "math/rand.Seed is not a Sonolus intrinsic") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestParseDeclarationsRejectsWrongModeAPI(t *testing.T) {
	_, err := parseMode(mode.ModePlay, "./testdata/invalidmode")
	if err == nil || !strings.Contains(err.Error(), "not available in play mode") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestParseDeclarationsRejectsWrongCallbackPhase(t *testing.T) {
	_, err := parseMode(mode.ModePlay, "./testdata/invalidphase")
	if err == nil || !strings.Contains(err.Error(), "sonolus/play.uiAPI.SetMenu cannot write during updateParallel callback") || !strings.Contains(err.Error(), "sonolus.Bucket.SetWindow cannot write during updateParallel callback") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestParseDeclarationsRejectsWrongCallbackPhaseThroughHelper(t *testing.T) {
	_, err := parseMode(mode.ModePlay, "./testdata/invalidhelper")
	if err == nil || !strings.Contains(err.Error(), "cannot write during updateParallel callback") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestParseDeclarationsRejectsWrongCallbackPhaseThroughCrossPackageHelper(t *testing.T) {
	_, err := parseMode(mode.ModePlay, "./testdata/crosshelper")
	if err == nil || !strings.Contains(err.Error(), "cannot write during updateParallel callback") || !strings.Contains(err.Error(), "inlined from") || !strings.Contains(err.Error(), "crosshelper") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestParseDeclarationsAcceptsStaticFunctionVariable(t *testing.T) {
	if _, err := parseMode(mode.ModePlay, "./testdata/dynamiccallback"); err != nil {
		t.Fatal(err)
	}
}

func TestParseDeclarationsRejectsRecursiveHelpers(t *testing.T) {
	_, err := parseMode(mode.ModePlay, "./testdata/invalidrecursion")
	if err == nil || !strings.Contains(err.Error(), "recursive helper call") || strings.Count(err.Error(), "inlined from") < 2 {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestParseDeclarationsAcceptsVariadicUserHelper(t *testing.T) {
	if _, err := parseMode(mode.ModePlay, "./testdata/invalidvariadic"); err != nil {
		t.Fatal(err)
	}
}

func TestStaticDSLCompilesAtEveryOptimizationLevel(t *testing.T) {
	for _, level := range []optimize.Level{optimize.LevelMinimal, optimize.LevelFast, optimize.LevelStandard} {
		t.Run(level.String(), func(t *testing.T) {
			artifacts, err := NewCompiler(Options{Optimization: level}, "./testdata/lowering").Compile(mode.ModePlay)
			if err != nil {
				t.Fatal(err)
			}
			if artifacts.Play == nil || len(artifacts.Play.Archetypes) == 0 || len(artifacts.Play.Nodes) == 0 {
				t.Fatalf("incomplete play artifacts: %#v", artifacts.Play)
			}
		})
	}
}

func TestParseDeclarationsRejectsInvalidStaticDSL(t *testing.T) {
	tests := []struct{ pattern, message string }{
		{"./testdata/invalidintrange", "range is only supported for int values"},
		{"./testdata/invalidvariadicescape", "variadic helper parameters cannot escape"},
		{"./testdata/invalidcallablearray", "package callable arrays are immutable"},
		{"./testdata/invalidpackagearray", "package static values are immutable in callbacks"},
		{"./testdata/invalidcurrententityref", "is not a base of current archetype"},
		{"./testdata/invalidarchetypeid", "is abstract and has no runtime ID"},
	}
	for _, test := range tests {
		t.Run(test.pattern, func(t *testing.T) {
			_, err := parseMode(mode.ModePlay, test.pattern)
			if err == nil || !strings.Contains(err.Error(), test.message) {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestParseDeclarationsRejectsRepeatedDefer(t *testing.T) {
	_, err := parseMode(mode.ModePlay, "./testdata/invaliddefer")
	if err == nil || !strings.Contains(err.Error(), "defer in loops or functions containing goto requires a runtime defer stack") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestParseDeclarationsRejectsUnsupportedBuiltin(t *testing.T) {
	_, err := parseMode(mode.ModePlay, "./testdata/invalidunsupportedbuiltin")
	if err == nil || !strings.Contains(err.Error(), "Go builtin complex is not supported") || !strings.Contains(err.Error(), "reachable path") || !strings.Contains(err.Error(), "Zero does not support compile-time-only type") || !strings.Contains(err.Error(), "Go builtin new does not support compile-time-only element type") || !strings.Contains(err.Error(), "cannot contain callback-local descriptors") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestParseDeclarationsConfigurationStaticFields(t *testing.T) {
	decl, err := parseMode(mode.ModePlay, "./testdata/configuration")
	if err != nil {
		t.Fatal(err)
	}
	if len(decl.Configuration.Value.Options) != 3 || decl.Configuration.Value.UI.Scope != "game" {
		t.Fatalf("unexpected configuration: %#v", decl.Configuration)
	}
	ui := decl.Configuration.Value.UI
	if ui.PrimaryMetric != resource.EngineConfigurationMetricErrorHeatmap || ui.SecondaryMetric != resource.EngineConfigurationMetricGreatGoodMissPercentage || ui.JudgmentErrorStyle != resource.EngineConfigurationJudgmentErrorStyleTriangleRight || ui.JudgmentErrorPlacement != resource.EngineConfigurationJudgmentErrorPlacementLeftRight {
		t.Fatalf("unexpected configuration UI enums: %#v", ui)
	}
	if ui.MenuVisibility.Scale != 0 || ui.MenuVisibility.Alpha != 0 || ui.JudgmentVisibility.Scale != 1 || ui.JudgmentVisibility.Alpha != 1 {
		t.Fatalf("unexpected explicit/default UI visibility: %#v", ui)
	}
	if ui.JudgmentAnimation.Scale.Ease != resource.EngineConfigurationAnimationTweenEaseOutInElastic || ui.JudgmentAnimation.Scale.From != 0 || ui.JudgmentAnimation.Scale.To != 1 || ui.JudgmentAnimation.Scale.Duration != 0.1 || ui.JudgmentAnimation.Alpha.Ease != resource.EngineConfigurationAnimationTweenEaseNone {
		t.Fatalf("unexpected explicit/default UI animation: %#v", ui)
	}
	slider := decl.Configuration.Value.Options[0].(resource.EngineConfigurationSliderOption)
	if slider.Name != "Speed" || slider.Title != "Speed Option" || slider.Description != "Scroll speed" || !slider.Standard || slider.Scope != "game" || slider.Unit != "#TIMES" {
		t.Fatalf("unexpected slider metadata: %#v", slider)
	}
	if got := decl.Configuration.Value.ReplayFallbackOptionNames; len(got) != 2 || got[0] != "Speed" || got[1] != "Lane" {
		t.Fatalf("unexpected replay fallback: %#v", got)
	}
}

func TestConfigurationOptionsLowerToRuntimeMemory(t *testing.T) {
	decl, err := parseMode(mode.ModePlay, "./testdata/configuration")
	if err != nil {
		t.Fatal(err)
	}
	var reader *frontend.ArchetypeDeclaration
	for _, archetype := range decl.Archetypes {
		if archetype.Name == "ConfigurationReader" {
			reader = archetype
			break
		}
	}
	if reader == nil {
		t.Fatal("ConfigurationReader declaration is missing")
	}
	facts := inspectFunction(callbackByName(t, reader.Callbacks, "spawnOrder"))
	wantOffsets := map[int]bool{0: false, 1: false, 2: false}
	for _, place := range facts.loads {
		if place.Storage == "LevelOption" {
			if _, ok := wantOffsets[place.Offset]; ok {
				wantOffsets[place.Offset] = true
			}
		}
	}
	for offset, found := range wantOffsets {
		if !found {
			t.Fatalf("LevelOption offset %d was not read: %#v", offset, facts.loads)
		}
	}
}

func TestParseDeclarationsConfigurationStaticAlias(t *testing.T) {
	decl, err := parseMode(mode.ModePlay, "./testdata/configurationalias")
	if err != nil {
		t.Fatal(err)
	}
	option := decl.Configuration.Value.Options[0].(resource.EngineConfigurationSliderOption)
	if option.Name != "Speed" || option.Def != 1 {
		t.Fatalf("unexpected aliased option: %#v", option)
	}
}

func TestParseDeclarationsRejectsInvalidConfigurationConstructors(t *testing.T) {
	tests := []struct{ pattern, message string }{
		{"./testdata/fakeconfigurationconstructor", "must use sonolus.SliderOption"},
		{"./testdata/missingconfigurationconstructor", "matching sonolus option constructor"},
		{"./testdata/invalidconfigurationselect", "default must index a non-empty static values list"},
		{"./testdata/invalidconfigurationselect", "invalid judgment error style \"arrow\""},
	}
	for _, test := range tests {
		t.Run(test.pattern, func(t *testing.T) {
			_, err := parseMode(mode.ModePlay, test.pattern)
			if err == nil || !strings.Contains(err.Error(), test.message) {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestParseDeclarationsRejectsLegacyConfigurationTag(t *testing.T) {
	_, err := parseMode(mode.ModePlay, "./testdata/legacyconfiguration")
	if err == nil || !strings.Contains(err.Error(), "struct tags are no longer supported") || !strings.Contains(err.Error(), "sonolus.SliderOption(sonolus.SliderOptionConfig") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestParseDeclarationsRejectsLegacyArchetypeTags(t *testing.T) {
	_, err := parseMode(mode.ModePlay, "./testdata/legacyarchetypetag")
	if err == nil || !strings.Contains(err.Error(), "sonolus struct tags are no longer supported for archetypes") || !strings.Contains(err.Error(), `archetype:"memory"`) || !strings.Contains(err.Error(), "main.go:") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestParseDeclarationsRejectsArchetypeTagOutsideArchetype(t *testing.T) {
	_, err := parseMode(mode.ModePlay, "./testdata/invalidarchetypetagnonarchetype")
	if err == nil || !strings.Contains(err.Error(), "archetype struct tags are only valid on archetype declarations") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestParseDeclarationsRejectsSymbolicCallInConfiguration(t *testing.T) {
	_, err := parseMode(mode.ModePlay, "./testdata/configurationcall")
	if err == nil || !strings.Contains(err.Error(), "configuration UI must be a pure static value") || !strings.Contains(err.Error(), "main.go:") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestParseDeclarationsRejectsSymbolicCallInROM(t *testing.T) {
	_, err := parseMode(mode.ModePlay, "./testdata/romcall")
	if err == nil || !strings.Contains(err.Error(), "ROMValues must be a pure static value") || !strings.Contains(err.Error(), "main.go:") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestParseDeclarationsRejectsFakeResourceConstructor(t *testing.T) {
	_, err := parseMode(mode.ModePlay, "./testdata/fakeresourceconstructor")
	if err == nil || !strings.Contains(err.Error(), "use sonolus.SkinSprite") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestParseDeclarationsRejectsInvalidResourceMarkers(t *testing.T) {
	tests := []struct{ pattern, message string }{
		{"./testdata/legacyresourcedirective", "//sonolus:resource is no longer supported"},
		{"./testdata/invalidresourcemode", `invalid skin render mode "invalid"`},
		{"./testdata/multipleresourcesingletons", "resource marker requires exactly one singleton variable"},
		{"./testdata/multipleresourcemarkers", "exactly one resource marker is required"},
		{"./testdata/indirectresourcemarker", "resource marker must be embedded directly"},
		{"./testdata/fakeresourcemarker", "resource marker must be the exact sonolus.SkinResource type"},
	}
	for _, test := range tests {
		t.Run(test.pattern, func(t *testing.T) {
			_, err := parseMode(mode.ModePlay, test.pattern)
			if err == nil || !strings.Contains(err.Error(), test.message) {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestParseDeclarationsRejectsInvalidMode(t *testing.T) {
	_, err := parseMode(mode.Mode("invalid"), "./testdata/declarations")
	if err == nil || !strings.Contains(err.Error(), "invalid Sonolus mode") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestParseDeclarationsROMFile(t *testing.T) {
	decl, err := parseMode(mode.ModePlay, "./testdata/romfile")
	if err != nil {
		t.Fatal(err)
	}
	if decl.ROM == nil || len(decl.ROM.Bytes) != 4 || len(decl.ROM.Values) != 1 {
		t.Fatalf("unexpected embedded ROM: %#v", decl.ROM)
	}
	want := math.Float32frombits(0x0a434241)
	if decl.ROM.Values[0] != want {
		t.Fatalf("ROM value = %v, want %v", decl.ROM.Values[0], want)
	}
}

func TestParseDeclarationsRejectsUnknownTags(t *testing.T) {
	_, err := parseMode(mode.ModePlay, "./testdata/invalid")
	if err == nil {
		t.Fatal("expected invalid tags to be rejected")
	}
	for _, key := range []string{
		"typo",
		"unknown",
		"configuration tags are no longer supported",
		"default is only valid for single-slot imported fields",
		`duplicate external field name "value.x"`,
		`duplicate inherited external field name "point.x"`,
	} {
		if !strings.Contains(err.Error(), key) {
			t.Errorf("error does not mention %q: %v", key, err)
		}
	}
}

func TestParseDeclarationsOtherModes(t *testing.T) {
	tests := []struct {
		mode       mode.Mode
		globals    int
		archetypes int
	}{
		{mode.ModeWatch, 1, 1},
		{mode.ModePreview, 0, 1},
		{mode.ModeTutorial, 3, 0},
	}
	for _, tt := range tests {
		t.Run(string(tt.mode), func(t *testing.T) {
			decl, err := parseMode(tt.mode, "./testdata/declarations")
			if err != nil {
				t.Fatal(err)
			}
			if len(decl.Globals) != tt.globals || len(decl.Archetypes) != tt.archetypes {
				t.Fatalf("globals=%d archetypes=%d", len(decl.Globals), len(decl.Archetypes))
			}
		})
	}
}
