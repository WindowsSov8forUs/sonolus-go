package catalog

import (
	"sort"
	"strings"
	"testing"
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
