package catalog

import (
	"sort"
	"strings"
	"testing"

	"github.com/WindowsSov8forUs/sonolus-go/v2/internal/compiler/mode"
)

func TestEverySymbolHasRecipeClassification(t *testing.T) {
	for i := range Symbols {
		recipe := LookupRecipe(&Symbols[i])
		if recipe.Kind == "" {
			t.Fatalf("symbol %s has no recipe classification", Symbols[i].Key())
		}
		if Symbols[i].Runtime != "" && !Symbols[i].Internal && recipe.Kind != RecipeRuntime {
			t.Fatalf("native symbol %s is not a runtime recipe", Symbols[i].Key())
		}
	}
}

func TestMemoryReadonlyOracleUsesModeAndCallback(t *testing.T) {
	if MemoryReadonly(mode.ModePlay, "preprocess", "RuntimeUI") {
		t.Fatal("Play preprocess RuntimeUI was classified readonly despite setters")
	}
	if !MemoryReadonly(mode.ModePlay, "updateParallel", "RuntimeUIConfiguration") {
		t.Fatal("RuntimeUIConfiguration should be readonly in updateParallel")
	}
	if !MemoryReadonly(mode.ModePlay, "updateParallel", "EngineRom") {
		t.Fatal("EngineRom should be readonly")
	}
}

func TestEveryPublicCallableHasExplicitRecipe(t *testing.T) {
	var missing []string
	for i := range Symbols {
		symbol := &Symbols[i]
		if symbol.Internal || (symbol.Kind != KindFunction && symbol.Kind != KindMethod) {
			continue
		}
		recipe := LookupRecipe(symbol)
		if recipe.Kind == RecipeForbidden && recipe.Reason == "callback lowering recipe has not been defined" {
			missing = append(missing, symbol.Key())
		}
	}
	if len(missing) != 0 {
		sort.Strings(missing)
		t.Fatalf("public callables without an explicit recipe:\n%s", strings.Join(missing, "\n"))
	}
}

func TestConfigurationConstructorsAreCompileTimeOnly(t *testing.T) {
	for _, name := range []string{"SliderOption", "ToggleOption", "SelectOption"} {
		symbol := byKey["sonolus."+name]
		if symbol == nil {
			t.Fatalf("missing catalog symbol sonolus.%s", name)
		}
		recipe := LookupRecipe(symbol)
		if recipe.Kind != RecipeCompileTime || recipe.Reason != "configuration constructor" {
			t.Fatalf("sonolus.%s recipe = %#v", name, recipe)
		}
	}
}

func TestResourceMarkersAreCompileTimeOnly(t *testing.T) {
	for _, name := range []string{"SkinResource", "EffectResource", "ParticleResource", "BucketsResource", "InstructionResource", "InstructionIconResource"} {
		symbol := byKey["sonolus."+name]
		if symbol == nil {
			t.Fatalf("missing catalog symbol sonolus.%s", name)
		}
		if recipe := LookupRecipe(symbol); recipe.Kind != RecipeCompileTime {
			t.Fatalf("sonolus.%s recipe = %#v", name, recipe)
		}
	}
}

func TestEveryPublicNativeHasRuntimeSignature(t *testing.T) {
	for i := range Symbols {
		symbol := &Symbols[i]
		if symbol.Kind != KindNative {
			continue
		}
		if _, ok := LookupRuntimeSignature(symbol.Runtime); !ok {
			t.Fatalf("native symbol %s has no runtime signature", symbol.Key())
		}
	}
}

func TestEveryRuntimeRecipeHasRuntimeSignature(t *testing.T) {
	for i := range Symbols {
		symbol := &Symbols[i]
		recipe := LookupRecipe(symbol)
		if recipe.Kind != RecipeRuntime {
			continue
		}
		if _, ok := LookupRuntimeSignature(recipe.Runtime); !ok {
			t.Fatalf("runtime recipe %s maps to %s without a signature", symbol.Key(), recipe.Runtime)
		}
	}
}

func TestRuntimeSimulationInventoryIsCompleteAndConsistent(t *testing.T) {
	seen := map[string]bool{}
	for _, runtime := range RuntimeFunctions {
		if seen[string(runtime)] {
			t.Fatalf("RuntimeFunction %s appears more than once", runtime)
		}
		seen[string(runtime)] = true
		metadata, ok := LookupRuntimeSimulation(runtime)
		if !ok || metadata.Class == "" || metadata.Strategy == "" {
			t.Fatalf("RuntimeFunction %s has incomplete simulation metadata: %#v", runtime, metadata)
		}
		signature, signatureOK := LookupRuntimeSignature(runtime)
		if !metadata.SpecialShape && (!signatureOK || metadata.Signature != signature) {
			t.Fatalf("RuntimeFunction %s simulation signature = %#v, catalog signature = %#v", runtime, metadata.Signature, signature)
		}
		if metadata.SpecialShape != (metadata.Shape != "") {
			t.Fatalf("RuntimeFunction %s special-shape metadata is inconsistent: %#v", runtime, metadata)
		}
		switch metadata.Class {
		case SimulationPure, SimulationRandom, SimulationHandler:
			if metadata.Effect != EffectPure {
				t.Fatalf("RuntimeFunction %s class %s has non-pure effect %s", runtime, metadata.Class, metadata.Effect)
			}
		case SimulationControl, SimulationEffect:
			if metadata.Effect != EffectWrite {
				t.Fatalf("RuntimeFunction %s class %s has non-writing effect %s", runtime, metadata.Class, metadata.Effect)
			}
		}
	}
	for index := range Symbols {
		symbol := &Symbols[index]
		recipe := LookupRecipe(symbol)
		if recipe.Kind != RecipeRuntime {
			continue
		}
		metadata, ok := LookupRuntimeSimulation(recipe.Runtime)
		if !ok {
			t.Fatalf("runtime recipe %s maps to %s without simulation metadata", symbol.Key(), recipe.Runtime)
		}
		if metadata.Class != SimulationControl && metadata.Class != SimulationMemory && metadata.Class != SimulationRandom && (metadata.Effect == EffectWrite) != (symbol.Effect == EffectWrite) {
			t.Fatalf("runtime recipe %s effect %s conflicts with %s simulation effect %s", symbol.Key(), symbol.Effect, recipe.Runtime, metadata.Effect)
		}
	}
}

func TestNativeAvailabilityMatrixIsExplicit(t *testing.T) {
	wantModes := []string{"play", "watch", "preview", "tutorial"}
	wantPhases := []string{"preprocess", "spawnOrder", "shouldSpawn", "initialize", "updateSequential", "touch", "updateParallel", "terminate", "spawnTime", "despawnTime", "render", "updateSpawn", "navigate", "update"}
	for i := range Symbols {
		symbol := &Symbols[i]
		if symbol.Kind != KindNative {
			continue
		}
		if strings.Join(symbol.Modes, "|") != strings.Join(wantModes, "|") {
			t.Fatalf("native %s modes = %v, want %v", symbol.Key(), symbol.Modes, wantModes)
		}
		if strings.Join(symbol.Phases, "|") != strings.Join(wantPhases, "|") {
			t.Fatalf("native %s phases = %v, want %v", symbol.Key(), symbol.Phases, wantPhases)
		}
	}
}

func TestNativeEffectsMatchCompilerMetadata(t *testing.T) {
	want := map[string]Effect{
		"sonolus/native.Add":              EffectPure,
		"sonolus/native.DebugLog":         EffectWrite,
		"sonolus/native.DebugPause":       EffectWrite,
		"sonolus/native.AddLifeScheduled": EffectWrite,
		"sonolus/native.Draw":             EffectWrite,
	}
	for key, effect := range want {
		symbol := byKey[key]
		if symbol == nil || symbol.Effect != effect {
			t.Fatalf("%s effect = %#v, want %s", key, symbol, effect)
		}
	}
}
