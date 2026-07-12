package compiler

import (
	"reflect"
	"testing"

	"github.com/WindowsSov8forUs/sonolus-go/internal/compiler/mode"
	"github.com/WindowsSov8forUs/sonolus-go/internal/compiler/optimize"
)

func FuzzCompiledCallbackOptimizationSemantics(f *testing.F) {
	compiled := map[optimize.Level]executableMode{}
	for _, level := range []optimize.Level{optimize.LevelMinimal, optimize.LevelFast, optimize.LevelStandard} {
		artifacts, err := NewCompiler(Options{Optimization: level}, "./testdata/fuzzsemantics").Compile(mode.ModePlay)
		if err != nil {
			f.Fatalf("compile %s: %v", level, err)
		}
		if _, err := normalizeEngineData(artifacts.Play); err != nil {
			f.Fatalf("validate %s nodes: %v", level, err)
		}
		executable, err := decodeExecutableMode(artifacts.Play)
		if err != nil {
			f.Fatalf("decode %s: %v", level, err)
		}
		compiled[level] = executable
	}

	f.Add(int16(0))
	f.Add(int16(-1))
	f.Add(int16(6))
	f.Add(int16(127))
	f.Fuzz(func(t *testing.T, raw int16) {
		seed := float64(raw % 128)
		var baseline executionResult
		for index, level := range []optimize.Level{optimize.LevelMinimal, optimize.LevelFast, optimize.LevelStandard} {
			executable := compiled[level]
			root, ok := executable.roots["archetype[0].preprocess"]
			if !ok {
				t.Fatalf("%s preprocess root is missing", level)
			}
			result, err := executeRoot(executable, root, seed)
			if err != nil {
				t.Fatalf("execute %s with seed %v: %v", level, seed, err)
			}
			if index == 0 {
				baseline = result
			} else if !reflect.DeepEqual(result, baseline) {
				t.Fatalf("%s changed semantics for seed %v\nminimal: %#v\n%s: %#v", level, seed, baseline, level, result)
			}
		}
	})
}
