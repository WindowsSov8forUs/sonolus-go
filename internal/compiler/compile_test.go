package compiler

import (
	"fmt"
	"go/ast"
	"go/types"
	"math"
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
	pkgs, err := source.LoadMode(m, pattern)
	if err != nil {
		return nil, err
	}
	if len(pkgs) != 1 {
		return nil, fmt.Errorf("expected exactly one main package, got %d", len(pkgs))
	}
	parser := frontend.NewParser()
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
	if !a.HasInput || len(a.Imports) != 1 || len(a.Exports) != 1 {
		t.Fatalf("unexpected archetype metadata: %#v", a)
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
	decl, err := parseMode(mode.ModePlay, "./testdata/lowering")
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
	for _, offset := range []int{0, 1, 3, 4, 5, 7} {
		if !transformOffsets[offset] {
			t.Fatalf("SkinTransform store offsets = %#v; missing %d", transformOffsets, offset)
		}
	}
	if len(transformOffsets) != 6 {
		t.Fatalf("SkinTransform store offsets = %#v; expected six affine 4x4 offsets", transformOffsets)
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
		resource.RuntimeFunctionRound, resource.RuntimeFunctionTrunc, resource.RuntimeFunctionLog,
		resource.RuntimeFunctionSin, resource.RuntimeFunctionCos, resource.RuntimeFunctionTan,
		resource.RuntimeFunctionArcsin, resource.RuntimeFunctionArccos, resource.RuntimeFunctionArctan,
		resource.RuntimeFunctionArctan2, resource.RuntimeFunctionMin, resource.RuntimeFunctionMax,
		resource.RuntimeFunctionPower, resource.RuntimeFunctionMod,
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

func TestEveryCallbackRecipeHasSuccessfulPackageToBackendFixture(t *testing.T) {
	fixtures := map[mode.Mode]string{
		mode.ModePlay:     "./testdata/lowering",
		mode.ModeWatch:    "./testdata/lowering_watch",
		mode.ModePreview:  "./testdata/lowering_preview",
		mode.ModeTutorial: "./testdata/lowering_tutorial",
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
	for currentMode, pattern := range fixtures {
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

func TestContainerHelperRejectsRuntimeBackingSelection(t *testing.T) {
	_, err := parseMode(mode.ModePlay, "./testdata/invalidcontainerreturn")
	if err == nil || !strings.Contains(err.Error(), "cannot select between different backing stores at runtime") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestContainerLocalRejectsRuntimeBackingSelection(t *testing.T) {
	_, err := parseMode(mode.ModePlay, "./testdata/invalidcontainerbranch")
	if err == nil || !strings.Contains(err.Error(), "cannot select a different backing store in runtime control flow") {
		t.Fatalf("unexpected error: %v", err)
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
	if err == nil || !strings.Contains(err.Error(), "sonolus/play.uiAPI.SetMenu cannot write during updateParallel callback") {
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
		{"./testdata/invalidcallablereassign", "static callable variables cannot be reassigned"},
		{"./testdata/invalidcallabledynamic", "cannot be initialized in runtime control flow"},
		{"./testdata/invalidcallablereturn", "function values cannot be returned"},
		{"./testdata/invalidpackagecallable", "unsupported identifier fn"},
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

func TestParseDeclarationsRejectsDefer(t *testing.T) {
	_, err := parseMode(mode.ModePlay, "./testdata/invaliddefer")
	if err == nil || !strings.Contains(err.Error(), "unsupported callback statement *ast.DeferStmt") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestParseDeclarationsRejectsUnregisteredBuiltin(t *testing.T) {
	_, err := parseMode(mode.ModePlay, "./testdata/invalidbuiltin")
	if err == nil || !strings.Contains(err.Error(), "Go builtin max is not supported") {
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
	for _, key := range []string{"typo", "unknown", "configuration tags are no longer supported"} {
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
