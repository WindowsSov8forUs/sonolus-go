package sim

import (
	"encoding/json"
	"errors"
	"math"
	"os"
	"reflect"
	"testing"

	"github.com/WindowsSov8forUs/sonolus-core-go/core/resource"
	"github.com/WindowsSov8forUs/sonolus-go/v2/internal/compiler/catalog"
	"github.com/WindowsSov8forUs/sonolus-go/v2/internal/simexec"
)

func TestRuntimeCatalogIsClassified(t *testing.T) {
	seen := map[resource.RuntimeFunction]bool{}
	for _, function := range catalog.RuntimeFunctions {
		if seen[function] {
			t.Errorf("%s appears more than once in the generated RuntimeFunction inventory", function)
		}
		seen[function] = true
		if class := simexec.Classify(function); class == "" {
			t.Errorf("%s is unclassified", function)
		}
	}
}

func TestRuntimeSimulationArityContracts(t *testing.T) {
	machine := simexec.NewMachine()
	for _, function := range catalog.RuntimeFunctions {
		metadata, ok := catalog.LookupRuntimeSimulation(function)
		if !ok {
			continue
		}
		if metadata.SpecialShape {
			valid, invalid := specialShapeArities(metadata.Shape)
			for _, count := range valid {
				if _, _, err := machine.Builtin(function, make([]float64, count)...); err != nil {
					var executionErr *ExecutionError
					if errors.As(err, &executionErr) && executionErr.Kind == ExecutionErrorInvalidArity {
						t.Errorf("%s rejected shape-valid arity %d: %v", function, count, err)
					}
				}
			}
			for _, count := range invalid {
				_, _, err := machine.Builtin(function, make([]float64, count)...)
				var executionErr *ExecutionError
				if !errors.As(err, &executionErr) || executionErr.Kind != ExecutionErrorInvalidArity {
					t.Errorf("%s accepted shape-invalid arity %d: %v", function, count, err)
				}
			}
			continue
		}
		checkValid := func(count int) {
			if count < 0 {
				return
			}
			_, _, err := machine.Builtin(function, make([]float64, count)...)
			var executionErr *ExecutionError
			if errors.As(err, &executionErr) && executionErr.Kind == ExecutionErrorInvalidArity {
				t.Errorf("%s rejected catalog-valid arity %d: %v", function, count, err)
			}
		}
		checkInvalid := func(count int) {
			if count < 0 {
				return
			}
			_, _, err := machine.Builtin(function, make([]float64, count)...)
			var executionErr *ExecutionError
			if !errors.As(err, &executionErr) || executionErr.Kind != ExecutionErrorInvalidArity {
				t.Errorf("%s accepted catalog-invalid arity %d: %v", function, count, err)
			}
		}
		checkValid(metadata.Signature.MinArgs)
		if metadata.Signature.MaxArgs >= 0 {
			checkValid(metadata.Signature.MaxArgs)
		}
		if metadata.Signature.MinArgs > 0 {
			checkInvalid(metadata.Signature.MinArgs - 1)
		}
		if metadata.Signature.MaxArgs >= 0 {
			checkInvalid(metadata.Signature.MaxArgs + 1)
		}
	}
}

func specialShapeArities(shape string) (valid, invalid []int) {
	switch shape {
	case "variadic":
		return []int{0, 1, 4}, nil
	case "if":
		return []int{3}, []int{2, 4}
	case "switch":
		return []int{1, 3, 5}, []int{0, 2, 4}
	case "switch-default":
		return []int{2, 4, 6}, []int{0, 1, 3, 5}
	case "switch-integer", "jump-loop":
		return []int{1, 3}, []int{0}
	case "switch-integer-default":
		return []int{2, 4}, []int{0, 1}
	case "block":
		return []int{1}, []int{0, 2}
	case "binary-control":
		return []int{2}, []int{0, 1, 3}
	default:
		return nil, []int{0}
	}
}

func TestEveryBuiltinSimulationStrategyHasAnImplementation(t *testing.T) {
	for _, function := range catalog.RuntimeFunctions {
		metadata, ok := catalog.LookupRuntimeSimulation(function)
		if !ok || metadata.Strategy != "builtin" || metadata.Class == catalog.SimulationControl {
			continue
		}
		arguments := make([]float64, metadata.Signature.MinArgs)
		_, handled, _ := simexec.NewMachine().Builtin(function, arguments...)
		if !handled {
			t.Errorf("%s is registered as builtin but has no implementation", function)
		}
	}
}

func TestRuntimeSimulationArgumentContracts(t *testing.T) {
	validArguments := map[string]func(int) []float64{
		"": func(count int) []float64 { return make([]float64, count) },
		"memory": func(count int) []float64 {
			return make([]float64, count)
		},
		"pointed-memory": func(count int) []float64 {
			return make([]float64, count)
		},
		"shifted-memory": func(count int) []float64 {
			return make([]float64, count)
		},
		"copy": func(count int) []float64 {
			return make([]float64, count)
		},
		"stream": func(count int) []float64 {
			return make([]float64, count)
		},
		"integer-range": func(count int) []float64 {
			arguments := make([]float64, count)
			arguments[1] = 1
			return arguments
		},
	}
	invalidArguments := map[string]func(int) []float64{
		"memory": func(count int) []float64 {
			arguments := make([]float64, count)
			arguments[1] = -1
			return arguments
		},
		"pointed-memory": func(count int) []float64 {
			arguments := make([]float64, count)
			arguments[1] = -1
			return arguments
		},
		"shifted-memory": func(count int) []float64 {
			arguments := make([]float64, count)
			arguments[3] = 0.5
			return arguments
		},
		"copy": func(count int) []float64 {
			arguments := make([]float64, count)
			arguments[4] = -1
			return arguments
		},
		"stream": func(count int) []float64 {
			arguments := make([]float64, count)
			arguments[1] = math.Inf(1)
			return arguments
		},
		"integer-range": func(count int) []float64 {
			arguments := make([]float64, count)
			arguments[0] = 0.5
			return arguments
		},
	}
	for _, function := range catalog.RuntimeFunctions {
		metadata := catalog.RuntimeSimulations[function]
		makeValid, known := validArguments[metadata.Arguments]
		if !known {
			t.Errorf("%s has unknown argument contract %q", function, metadata.Arguments)
			continue
		}
		if metadata.SpecialShape {
			continue
		}
		arguments := makeValid(metadata.Signature.MinArgs)
		if _, _, err := simexec.NewMachine().Builtin(function, arguments...); err != nil && metadata.Arguments != "" {
			var executionErr *ExecutionError
			if errors.As(err, &executionErr) && executionErr.Kind == ExecutionErrorInvalidArgument {
				t.Errorf("%s rejected generated valid arguments %v: %v", function, arguments, err)
			}
		}
		makeInvalid, constrained := invalidArguments[metadata.Arguments]
		if !constrained {
			continue
		}
		arguments = makeInvalid(metadata.Signature.MinArgs)
		_, _, err := simexec.NewMachine().Builtin(function, arguments...)
		var executionErr *ExecutionError
		if !errors.As(err, &executionErr) || executionErr.Kind != ExecutionErrorInvalidArgument {
			t.Errorf("%s accepted invalid %s arguments %v: %v", function, metadata.Arguments, arguments, err)
		}
	}
}

func TestPinnedJavaScriptNativeResults(t *testing.T) {
	data, err := os.ReadFile("../../internal/compiler/testdata/backend/runtime_native_golden.json")
	if err != nil {
		t.Fatal(err)
	}
	var golden struct {
		SchemaVersion      int    `json:"schemaVersion"`
		JavaScriptCommit   string `json:"javascriptCommit"`
		NativeSourceSHA256 string `json:"nativeSourceSha256"`
		Cases              []struct {
			Function  resource.RuntimeFunction `json:"function"`
			Arguments []json.RawMessage        `json:"arguments"`
			Result    json.RawMessage          `json:"result"`
		} `json:"cases"`
	}
	if err := json.Unmarshal(data, &golden); err != nil {
		t.Fatal(err)
	}
	if golden.SchemaVersion != 2 || golden.JavaScriptCommit != "37b0eee5aa16d1e01973d33d625d86f5ef72d268" || len(golden.NativeSourceSHA256) != 64 {
		t.Fatalf("unexpected JS native golden provenance: schema=%d commit=%q source=%q", golden.SchemaVersion, golden.JavaScriptCommit, golden.NativeSourceSHA256)
	}
	for _, item := range golden.Cases {
		arguments := make([]float64, len(item.Arguments))
		for index, raw := range item.Arguments {
			arguments[index] = decodeJavaScriptNumber(t, raw)
		}
		expected := decodeJavaScriptNumber(t, item.Result)
		value, handled, err := simexec.NewMachine().Builtin(item.Function, arguments...)
		if err != nil || !handled {
			t.Errorf("%s%v: value=%v handled=%v err=%v", item.Function, arguments, value, handled, err)
			continue
		}
		if !equalJavaScriptNumber(value, expected) {
			t.Errorf("%s%v = %.17g, JS wants %.17g", item.Function, arguments, value, expected)
		}
	}
}

func decodeJavaScriptNumber(t *testing.T, raw json.RawMessage) float64 {
	t.Helper()
	var number float64
	if len(raw) != 0 && raw[0] != '"' {
		if err := json.Unmarshal(raw, &number); err != nil {
			t.Fatal(err)
		}
		return number
	}
	var special string
	if err := json.Unmarshal(raw, &special); err != nil {
		t.Fatal(err)
	}
	switch special {
	case "NaN":
		return math.NaN()
	case "+Inf":
		return math.Inf(1)
	case "-Inf":
		return math.Inf(-1)
	case "-0":
		return math.Copysign(0, -1)
	default:
		t.Fatalf("unknown JavaScript number %q", special)
		return 0
	}
}

func equalJavaScriptNumber(left, right float64) bool {
	if math.IsNaN(left) || math.IsNaN(right) {
		return math.IsNaN(left) && math.IsNaN(right)
	}
	if left == 0 || right == 0 || math.IsInf(left, 0) || math.IsInf(right, 0) {
		return left == right && math.Signbit(left) == math.Signbit(right)
	}
	tolerance := 1e-12 * max(1, math.Abs(right))
	return math.Abs(left-right) <= tolerance
}

func TestHandlerAndEffectSimulationContracts(t *testing.T) {
	for _, function := range catalog.RuntimeFunctions {
		metadata := catalog.RuntimeSimulations[function]
		if metadata.Class != catalog.SimulationHandler && metadata.Class != catalog.SimulationEffect {
			continue
		}
		nodes := make([]resource.EngineDataNode, metadata.Signature.MinArgs+1)
		arguments := make([]int, metadata.Signature.MinArgs)
		for index := range arguments {
			arguments[index] = index + 1
			nodes[index+1] = resource.EngineDataValueNode{Value: float64(index + 1)}
		}
		nodes[0] = resource.EngineDataFunctionNode{Func: function, Args: arguments}
		calls := 0
		result, err := simexec.Execute(nodes, 0, simexec.Request{Handler: func(got resource.RuntimeFunction, values []float64) (float64, error) {
			calls++
			if got != function || len(values) != metadata.Signature.MinArgs {
				t.Fatalf("%s handler call = %s%v", function, got, values)
			}
			return 37, nil
		}})
		if err != nil {
			t.Errorf("%s: %v", function, err)
			continue
		}
		if calls != 1 || result.Value != 37 {
			t.Errorf("%s handler result = value %v calls %d", function, result.Value, calls)
		}
		if metadata.Class == catalog.SimulationHandler && len(result.Effects) != 0 {
			t.Errorf("pure handler %s emitted effects: %+v", function, result.Effects)
		}
		if metadata.Class == catalog.SimulationEffect && (len(result.Effects) != 1 || result.Effects[0].Function != function) {
			t.Errorf("effect %s log = %+v", function, result.Effects)
		}
	}
}

func TestRandomSimulationIsDeterministicAndBounded(t *testing.T) {
	nodes := []resource.EngineDataNode{
		resource.EngineDataFunctionNode{Func: resource.RuntimeFunctionAdd, Args: []int{1, 2, 3, 4}},
		resource.EngineDataFunctionNode{Func: resource.RuntimeFunctionRandom, Args: []int{5, 6}},
		resource.EngineDataFunctionNode{Func: resource.RuntimeFunctionRandomInteger, Args: []int{5, 6}},
		resource.EngineDataFunctionNode{Func: resource.RuntimeFunctionRandom, Args: []int{5, 6}},
		resource.EngineDataFunctionNode{Func: resource.RuntimeFunctionRandomInteger, Args: []int{5, 6}},
		resource.EngineDataValueNode{Value: -2},
		resource.EngineDataValueNode{Value: 3},
	}
	run := func(seed int64) float64 {
		t.Helper()
		result, err := simexec.Execute(nodes, 0, simexec.Request{RandomSeed: seed})
		if err != nil {
			t.Fatal(err)
		}
		return result.Value
	}
	first, second, different := run(7), run(7), run(8)
	if first != second {
		t.Fatalf("same seed differs: %v vs %v", first, second)
	}
	if first == different {
		t.Fatalf("different seeds produced the same sequence: %v", first)
	}
	for functionIndex, function := range []resource.RuntimeFunction{resource.RuntimeFunctionRandom, resource.RuntimeFunctionRandomInteger} {
		result, err := simexec.Execute(nodes, functionIndex+1, simexec.Request{RandomSeed: 11})
		if err != nil {
			t.Fatal(err)
		}
		if result.Value < -2 || result.Value >= 3 || function == resource.RuntimeFunctionRandomInteger && result.Value != math.Trunc(result.Value) {
			t.Fatalf("%s returned out-of-range value %v", function, result.Value)
		}
	}
}

func TestEasingSimulationRelationships(t *testing.T) {
	machine := simexec.NewMachine()
	call := func(name string, value float64) float64 {
		t.Helper()
		result, handled, err := machine.Builtin(resource.RuntimeFunction(name), value)
		if err != nil || !handled {
			t.Fatalf("%s(%v): value=%v handled=%v err=%v", name, value, result, handled, err)
		}
		return result
	}
	for _, family := range []string{"Sine", "Quad", "Cubic", "Quart", "Quint", "Expo", "Circ", "Back", "Elastic"} {
		for _, direction := range []string{"In", "Out", "InOut", "OutIn"} {
			name := "Ease" + direction + family
			if start, end := call(name, 0), call(name, 1); math.Abs(start) > 1e-12 || math.Abs(end-1) > 1e-12 {
				t.Errorf("%s endpoints = (%v,%v), want (0,1)", name, start, end)
			}
		}
		for _, value := range []float64{0.125, 0.375, 0.625, 0.875} {
			in := call("EaseIn"+family, value)
			outMirror := 1 - call("EaseOut"+family, 1-value)
			if math.Abs(in-outMirror) > 1e-12 {
				t.Errorf("%s in/out duality at %v: %v vs %v", family, value, in, outMirror)
			}
		}
	}
}

func TestFinalEngineDataExecution(t *testing.T) {
	var reference Result
	for index, optimization := range []OptimizationLevel{OptimizationMinimal, OptimizationFast, OptimizationStandard} {
		engine, err := Compile(Options{Optimization: optimization}, "../../internal/compiler/testdata/simulator")
		if err != nil {
			t.Fatal(err)
		}
		result, err := engine.Run(Request{
			Mode:       ModePlay,
			Archetype:  "Note",
			Callback:   "preprocess",
			RandomSeed: 7,
			Handler: func(resource.RuntimeFunction, []float64) (float64, error) {
				return 0, nil
			},
		})
		if err != nil {
			t.Fatal(err)
		}
		if result.Steps == 0 || result.Memory == nil {
			t.Fatalf("unexpected simulation result: %+v", result)
		}
		if index == 0 {
			reference = result
			continue
		}
		if !equalNumber(result.Value, reference.Value) || !equalMemory(result.Memory, reference.Memory) || !reflect.DeepEqual(result.Streams, reference.Streams) || !reflect.DeepEqual(result.Effects, reference.Effects) {
			t.Fatalf("optimization %d changed semantics:\nwant %+v\ngot  %+v", optimization, reference, result)
		}
	}
}

func TestFiniteVariantsAndContainersMatchAcrossOptimizations(t *testing.T) {
	for selector := 0; selector < 2; selector++ {
		temporary := make([]float64, 256)
		for index := range temporary {
			temporary[index] = float64(index + 17)
		}
		var reference Result
		for index, optimization := range []OptimizationLevel{OptimizationMinimal, OptimizationFast, OptimizationStandard} {
			engine, err := Compile(Options{Optimization: optimization}, "../../internal/compiler/testdata/simulator")
			if err != nil {
				t.Fatal(err)
			}
			result, err := engine.Run(Request{
				Mode:      ModePlay,
				Archetype: "VariantNote",
				Callback:  "preprocess",
				Memory: map[int][]float64{
					4001:  {float64(selector)},
					10000: append([]float64(nil), temporary...),
				},
			})
			if err != nil {
				t.Fatalf("selector %d optimization %d: %v", selector, optimization, err)
			}
			if len(result.Memory[4000]) == 0 || result.Memory[4000][0] == 0 {
				t.Fatalf("optimization %d did not execute the variant callback: %+v", optimization, result)
			}
			if len(result.Memory[4000]) <= 2 {
				t.Fatalf("optimization %d omitted container result memory: %+v", optimization, result)
			}
			if got := result.Memory[4000][2]; got != 62 {
				t.Fatalf("optimization %d container helpers = %v, want 62", optimization, got)
			}
			wantContainerVariant := []float64{314, 482}[selector]
			if got := result.Memory[4000][3]; got != wantContainerVariant {
				t.Fatalf("optimization %d container variants = %v, want %v", optimization, got, wantContainerVariant)
			}
			wantNewPointer := []float64{1, 18}[selector]
			if got := result.Memory[4000][4]; got != wantNewPointer {
				t.Fatalf("optimization %d new/nil pointer = %v, want %v", optimization, got, wantNewPointer)
			}
			wantPointer := float64(16 + selector*4)
			if got := result.Memory[4000][1]; got != wantPointer {
				t.Fatalf("optimization %d pointer variants = %v, want %v", optimization, got, wantPointer)
			}
			if index == 0 {
				reference = result
				continue
			}
			if !equalNumber(result.Value, reference.Value) || !equalMemory(result.Memory, reference.Memory) || !reflect.DeepEqual(result.Streams, reference.Streams) || !reflect.DeepEqual(result.Effects, reference.Effects) {
				t.Fatalf("selector %d optimization %d changed semantics:\nwant %+v\ngot  %+v", selector, optimization, reference, result)
			}
		}
	}
}

func TestStaticLanguageExtensionsMatchAcrossOptimizations(t *testing.T) {
	for selector, expected := range []float64{16287, 16321} {
		var reference Result
		for index, optimization := range []OptimizationLevel{OptimizationMinimal, OptimizationFast, OptimizationStandard} {
			engine, err := Compile(Options{Optimization: optimization}, "../../internal/compiler/testdata/simulator")
			if err != nil {
				t.Fatalf("selector %d optimization %d: %v", selector, optimization, err)
			}
			result, err := engine.Run(Request{
				Mode: ModePlay, Archetype: "StaticLanguageNote", Callback: "preprocess",
				Memory: map[int][]float64{4001: {float64(selector)}},
			})
			if err != nil {
				t.Fatalf("selector %d optimization %d: %v", selector, optimization, err)
			}
			if got := result.Memory[4000][0]; got != expected {
				t.Fatalf("selector %d optimization %d method expressions = %v, want %v", selector, optimization, got, expected)
			}
			if index == 0 {
				reference = result
				continue
			}
			if !equalNumber(result.Value, reference.Value) || !equalMemory(result.Memory, reference.Memory) || !reflect.DeepEqual(result.Streams, reference.Streams) || !reflect.DeepEqual(result.Effects, reference.Effects) {
				t.Fatalf("selector %d optimization %d changed semantics:\nwant %+v\ngot  %+v", selector, optimization, reference, result)
			}
		}
	}
}

func TestPackageCallableArraysMatchAcrossOptimizations(t *testing.T) {
	for selector, expected := range []float64{11, 9} {
		for _, optimization := range []OptimizationLevel{OptimizationMinimal, OptimizationFast, OptimizationStandard} {
			engine, err := Compile(Options{Optimization: optimization}, "../../internal/compiler/testdata/simulator")
			if err != nil {
				t.Fatalf("selector %d optimization %d: %v", selector, optimization, err)
			}
			result, err := engine.Run(Request{
				Mode: ModePlay, Archetype: "PackageCallableArrayNote", Callback: "preprocess",
				Memory: map[int][]float64{4001: {float64(selector)}},
			})
			if err != nil {
				t.Fatalf("selector %d optimization %d: %v", selector, optimization, err)
			}
			if got := result.Memory[4000][0]; got != expected {
				t.Fatalf("selector %d optimization %d value = %v, want %v", selector, optimization, got, expected)
			}
		}
	}
}

func TestPackageStaticArraysMatchAcrossOptimizations(t *testing.T) {
	for selector, expected := range []float64{27, 33, 32} {
		for _, optimization := range []OptimizationLevel{OptimizationMinimal, OptimizationFast, OptimizationStandard} {
			engine, err := Compile(Options{Optimization: optimization}, "../../internal/compiler/testdata/simulator")
			if err != nil {
				t.Fatalf("selector %d optimization %d: %v", selector, optimization, err)
			}
			result, err := engine.Run(Request{
				Mode: ModePlay, Archetype: "PackageStaticArrayNote", Callback: "preprocess",
				Memory: map[int][]float64{4001: {float64(selector)}},
			})
			if err != nil {
				t.Fatalf("selector %d optimization %d: %v", selector, optimization, err)
			}
			if got := result.Memory[4000][0]; got != expected {
				t.Fatalf("selector %d optimization %d value = %v, want %v", selector, optimization, got, expected)
			}
		}
	}
}

func TestRangeOperationsMatchAcrossOptimizations(t *testing.T) {
	for _, input := range []float64{2, 9} {
		for _, optimization := range []OptimizationLevel{OptimizationMinimal, OptimizationFast, OptimizationStandard} {
			engine, err := Compile(Options{Optimization: optimization}, "../../internal/compiler/testdata/simulator")
			if err != nil {
				t.Fatalf("input %v optimization %d: %v", input, optimization, err)
			}
			result, err := engine.Run(Request{
				Mode: ModePlay, Archetype: "RangeNote", Callback: "preprocess",
				Memory: map[int][]float64{4001: {input}},
			})
			if err != nil {
				t.Fatalf("input %v optimization %d: %v", input, optimization, err)
			}
			wantContains := 0.0
			wantClamp := 6.0
			if input == 2 {
				wantContains = 1
				wantClamp = 2
			}
			want := []float64{8, 0, wantContains, 1, 2, 0, 0.5, wantClamp, 42, 0.375, 0, 0.15625, 0.103515625, 1, 0, 0.5, 2.5, 1, 0}
			if got := result.Memory[4000]; !reflect.DeepEqual(got, want) {
				t.Fatalf("input %v optimization %d range values = %v, want %v", input, optimization, got, want)
			}
		}
	}
}

func TestProjectiveGeometryMatchesAcrossOptimizations(t *testing.T) {
	want := []float64{
		2.0 / 3.0, 5.0 / 6.0,
		2, 4,
		7, 4, -3, -2,
		2, 4, 2, 0, 0, 0, 0, 4,
		1.0 / 3.0,
	}
	for _, optimization := range []OptimizationLevel{OptimizationMinimal, OptimizationFast, OptimizationStandard} {
		engine, err := Compile(Options{Optimization: optimization}, "../../internal/compiler/testdata/simulator")
		if err != nil {
			t.Fatalf("optimization %d: %v", optimization, err)
		}
		result, err := engine.Run(Request{Mode: ModePlay, Archetype: "GeometryNote", Callback: "preprocess"})
		if err != nil {
			t.Fatalf("optimization %d: %v", optimization, err)
		}
		got := result.Memory[4000]
		if len(got) != len(want) {
			t.Fatalf("optimization %d geometry slots = %v, want %v", optimization, got, want)
		}
		for index := range want {
			if math.Abs(got[index]-want[index]) > 1e-9 {
				t.Fatalf("optimization %d geometry slot %d = %v, want %v", optimization, index, got[index], want[index])
			}
		}
	}
}

func TestTypedLevelGlobalsMatchAcrossOptimizations(t *testing.T) {
	for _, optimization := range []OptimizationLevel{OptimizationMinimal, OptimizationFast, OptimizationStandard} {
		engine, err := Compile(Options{Optimization: optimization}, "../../internal/compiler/testdata/levelglobals")
		if err != nil {
			t.Fatalf("optimization %d: %v", optimization, err)
		}
		preprocessed, err := engine.Run(Request{Mode: ModePlay, Archetype: "GlobalNote", Callback: "preprocess"})
		if err != nil {
			t.Fatalf("optimization %d preprocess: %v", optimization, err)
		}
		if got := preprocessed.Memory[2001]; len(got) < 5 || got[0] != 2 || got[3] != 3 || got[4] != 4 {
			t.Fatalf("optimization %d LevelData = %v", optimization, got)
		}
		for dynamicIndex := 0; dynamicIndex < 2; dynamicIndex++ {
			memoryInput := map[int][]float64{}
			for block, values := range preprocessed.Memory {
				memoryInput[block] = append([]float64(nil), values...)
			}
			memoryInput[1001] = []float64{float64(dynamicIndex)}
			sequential, runErr := engine.Run(Request{
				Mode: ModePlay, Archetype: "GlobalNote", Callback: "updateSequential",
				Memory: memoryInput,
			})
			if runErr != nil {
				t.Fatalf("optimization %d index %d updateSequential: %v", optimization, dynamicIndex, runErr)
			}
			memory := sequential.Memory[2000]
			setOffset := 9 + dynamicIndex*3
			groupOffset := 15 + dynamicIndex*3
			if len(memory) < 25 || memory[0] != 1 || memory[1] != 1 || memory[5] != 1 || memory[6] != 1 || memory[setOffset] != 1 || memory[setOffset+1] != 1 || memory[groupOffset] != 1 || memory[groupOffset+1] != 1 || memory[21] != 1 || memory[22] != 1 || memory[23] != 3 || memory[24] != 4 {
				t.Fatalf("optimization %d index %d LevelMemory = %v", optimization, dynamicIndex, memory)
			}
		}
	}
}

func TestTouchIteratorMatchesAcrossOptimizations(t *testing.T) {
	touches := make([]float64, 30)
	touches[0], touches[13] = 2, 5
	touches[15], touches[28] = 3, 7
	for _, optimization := range []OptimizationLevel{OptimizationMinimal, OptimizationFast, OptimizationStandard} {
		engine, err := Compile(Options{Optimization: optimization}, "../../internal/compiler/testdata/simulator")
		if err != nil {
			t.Fatalf("optimization %d: %v", optimization, err)
		}
		result, err := engine.Run(Request{
			Mode: ModePlay, Archetype: "TouchIteratorNote", Callback: "touch",
			Memory: map[int][]float64{1001: {0, 0, 0, 2}, 1002: touches},
		})
		if err != nil {
			t.Fatalf("optimization %d: %v", optimization, err)
		}
		if got := result.Memory[4000][0]; got != 18 {
			t.Fatalf("optimization %d touch iterator sum = %v, want 18", optimization, got)
		}
	}
}

func TestEntityRefRuntimeKeyMatchesAcrossOptimizations(t *testing.T) {
	for _, optimization := range []OptimizationLevel{OptimizationMinimal, OptimizationFast, OptimizationStandard} {
		engine, err := Compile(Options{Optimization: optimization}, "../../internal/compiler/testdata/simulator")
		if err != nil {
			t.Fatalf("optimization %d: %v", optimization, err)
		}
		ids := map[string]int{}
		for id, archetype := range engine.artifacts.Play.Archetypes {
			ids[string(archetype.Name)] = id
		}
		for _, test := range []struct {
			target string
			want   float64
		}{{"EntityKeyTarget", 34}, {"EntityKeyDefault", 10}} {
			entityInfo := make([]float64, 9)
			entityInfo[7] = float64(ids[test.target])
			result, runErr := engine.Run(Request{
				Mode: ModePlay, Archetype: "EntityKeyNote", Callback: "preprocess",
				Memory: map[int][]float64{4001: {2}, 4103: entityInfo},
			})
			if runErr != nil {
				t.Fatalf("optimization %d target %s: %v", optimization, test.target, runErr)
			}
			if got := result.Memory[4000][0]; got != test.want {
				t.Fatalf("optimization %d target %s key sum = %v, want %v", optimization, test.target, got, test.want)
			}
		}
	}
}

func TestNilPointerDereferenceTerminatesAtEveryCheckLevel(t *testing.T) {
	for _, checks := range []RuntimeChecks{RuntimeChecksNone, RuntimeChecksTerminate, RuntimeChecksNotify} {
		engine, err := Compile(Options{Optimization: OptimizationStandard, RuntimeChecks: checks}, "../../internal/compiler/testdata/simulator")
		if err != nil {
			t.Fatalf("checks %d: %v", checks, err)
		}
		valid, err := engine.Run(Request{
			Mode: ModePlay, Archetype: "NilPointerNote", Callback: "preprocess",
			Memory: map[int][]float64{4001: {0}},
		})
		if err != nil {
			t.Fatalf("checks %d valid pointer comparison: %v", checks, err)
		}
		if got := valid.Memory[4000][0]; got != 1 {
			t.Fatalf("checks %d nil comparison = %v, want 1", checks, got)
		}
		terminated, err := engine.Run(Request{
			Mode: ModePlay, Archetype: "NilPointerNote", Callback: "preprocess",
			Memory: map[int][]float64{4001: {1}},
		})
		if err != nil {
			t.Fatalf("checks %d nil dereference: %v", checks, err)
		}
		if len(terminated.Memory[4000]) != 0 && terminated.Memory[4000][0] != 0 {
			t.Fatalf("checks %d continued after nil dereference: %+v", checks, terminated)
		}
		if checks == RuntimeChecksNotify {
			if len(terminated.Effects) != 2 || terminated.Effects[0].Function != resource.RuntimeFunctionDebugLog || terminated.Effects[1].Function != resource.RuntimeFunctionDebugPause {
				t.Fatalf("notify nil dereference effects = %+v", terminated.Effects)
			}
		} else if len(terminated.Effects) != 0 {
			t.Fatalf("checks %d nil dereference effects = %+v", checks, terminated.Effects)
		}
	}
}

func TestNilCallableTerminatesAtEveryCheckLevel(t *testing.T) {
	for _, checks := range []RuntimeChecks{RuntimeChecksNone, RuntimeChecksTerminate, RuntimeChecksNotify} {
		engine, err := Compile(Options{Optimization: OptimizationStandard, RuntimeChecks: checks}, "../../internal/compiler/testdata/simulator")
		if err != nil {
			t.Fatalf("checks %d: %v", checks, err)
		}
		valid, err := engine.Run(Request{
			Mode: ModePlay, Archetype: "NilCallableNote", Callback: "preprocess",
			Memory: map[int][]float64{4001: {0}},
		})
		if err != nil {
			t.Fatalf("checks %d valid callable: %v", checks, err)
		}
		if got := valid.Memory[4000][0]; got != 8 {
			t.Fatalf("checks %d valid callable = %v, want 8", checks, got)
		}
		terminated, err := engine.Run(Request{
			Mode: ModePlay, Archetype: "NilCallableNote", Callback: "preprocess",
			Memory: map[int][]float64{4001: {1}},
		})
		if err != nil {
			t.Fatalf("checks %d nil callable: %v", checks, err)
		}
		got := 0.0
		if values := terminated.Memory[4000]; len(values) != 0 {
			got = values[0]
		}
		if got != 0 {
			t.Fatalf("checks %d nil callable wrote %v", checks, got)
		}
		if checks == RuntimeChecksNotify && len(terminated.Effects) == 0 {
			t.Fatalf("checks %d nil callable emitted no diagnostic", checks)
		}
	}
}

func TestDiagnosticControlHelpersMatchRuntimeCheckLevels(t *testing.T) {
	for _, checks := range []RuntimeChecks{RuntimeChecksNone, RuntimeChecksTerminate, RuntimeChecksNotify} {
		engine, err := Compile(Options{Optimization: OptimizationStandard, RuntimeChecks: checks}, "../../internal/compiler/testdata/simulator")
		if err != nil {
			t.Fatalf("checks %d: %v", checks, err)
		}
		for selector := 0; selector < 3; selector++ {
			result, runErr := engine.Run(Request{
				Mode: ModePlay, Archetype: "DiagnosticControlNote", Callback: "preprocess",
				Memory: map[int][]float64{4001: {float64(selector)}},
			})
			if runErr != nil {
				t.Fatalf("checks %d selector %d: %v", checks, selector, runErr)
			}
			if selector == 0 {
				if got := result.Memory[4000][0]; got != 1 {
					t.Fatalf("checks %d notify continuation = %v, want 1", checks, got)
				}
			} else if len(result.Memory[4000]) != 0 && result.Memory[4000][0] != 0 {
				t.Fatalf("checks %d selector %d continued after termination: %+v", checks, selector, result)
			}
			wantEffects := 0
			if checks == RuntimeChecksNotify {
				wantEffects = 2
				if selector != 0 {
					wantEffects = 4
				}
			}
			if len(result.Effects) != wantEffects {
				t.Fatalf("checks %d selector %d effects=%+v, want %d", checks, selector, result.Effects, wantEffects)
			}
		}
	}
}

func TestIntegerZeroDivisionTerminatesAtEveryCheckLevel(t *testing.T) {
	for _, checks := range []RuntimeChecks{RuntimeChecksNone, RuntimeChecksTerminate, RuntimeChecksNotify} {
		engine, err := Compile(Options{Optimization: OptimizationStandard, RuntimeChecks: checks}, "../../internal/compiler/testdata/simulator")
		if err != nil {
			t.Fatalf("checks %d: %v", checks, err)
		}
		for operation := 0; operation < 4; operation++ {
			result, err := engine.Run(Request{
				Mode:      ModePlay,
				Archetype: "ArithmeticNote",
				Callback:  "preprocess",
				Memory:    map[int][]float64{4001: {0, float64(operation)}},
			})
			if err != nil {
				t.Fatalf("checks %d operation %d: %v", checks, operation, err)
			}
			if values := result.Memory[4000]; len(values) != 0 && values[0] != 0 {
				t.Fatalf("checks %d operation %d continued after integer zero division: memory=%v", checks, operation, values)
			}
			var logs, pauses int
			for _, effect := range result.Effects {
				switch effect.Function {
				case resource.RuntimeFunctionDebugLog:
					logs++
				case resource.RuntimeFunctionDebugPause:
					pauses++
				}
			}
			if checks == RuntimeChecksNotify {
				if logs != 2 || pauses != 1 {
					t.Fatalf("notify operation %d effects = %+v, want divisor log + diagnostic log + pause", operation, result.Effects)
				}
			} else if logs != 1 || pauses != 0 {
				t.Fatalf("checks %d operation %d effects = %+v, want only divisor evaluation log", checks, operation, result.Effects)
			}
		}
	}
}

func TestDynamicRandIntnBoundTerminatesAtEveryCheckLevel(t *testing.T) {
	for _, checks := range []RuntimeChecks{RuntimeChecksNone, RuntimeChecksTerminate, RuntimeChecksNotify} {
		engine, err := Compile(Options{Optimization: OptimizationStandard, RuntimeChecks: checks}, "../../internal/compiler/testdata/simulator")
		if err != nil {
			t.Fatalf("checks %d: %v", checks, err)
		}
		result, err := engine.Run(Request{
			Mode:      ModePlay,
			Archetype: "RandomBoundNote",
			Callback:  "preprocess",
			Memory:    map[int][]float64{4000: {0, 0}},
		})
		if err != nil {
			t.Fatalf("checks %d: %v", checks, err)
		}
		if values := result.Memory[4000]; len(values) != 0 && values[0] != 0 {
			t.Fatalf("checks %d continued after invalid rand.Intn bound: memory=%v", checks, values)
		}
		wantEffects := 0
		if checks == RuntimeChecksNotify {
			wantEffects = 2
		}
		if len(result.Effects) != wantEffects {
			t.Fatalf("checks %d effects=%+v, want %d", checks, result.Effects, wantEffects)
		}
		result, err = engine.Run(Request{
			Mode:       ModePlay,
			Archetype:  "RandomBoundNote",
			Callback:   "preprocess",
			Memory:     map[int][]float64{4000: {0, 3}},
			RandomSeed: 7,
		})
		if err != nil {
			t.Fatalf("checks %d positive bound: %v", checks, err)
		}
		if got := result.Memory[4000][0]; got < 0 || got >= 3 || got != math.Trunc(got) {
			t.Fatalf("checks %d rand.Intn(3) = %v", checks, got)
		}
		if len(result.Effects) != 1 || result.Effects[0].Function != resource.RuntimeFunctionDebugLog {
			t.Fatalf("checks %d positive bound effects=%+v", checks, result.Effects)
		}
	}
}

func TestGoMathAndSonolusNumberSemanticsStayDistinct(t *testing.T) {
	for _, optimization := range []OptimizationLevel{OptimizationMinimal, OptimizationFast, OptimizationStandard} {
		engine, err := Compile(Options{Optimization: optimization}, "../../internal/compiler/testdata/simulator")
		if err != nil {
			t.Fatalf("optimization %d: %v", optimization, err)
		}
		for _, test := range []struct {
			operation float64
			want      float64
		}{{4, -2}, {5, 1}, {6, -2}, {7, -1}} {
			result, err := engine.Run(Request{
				Mode:      ModePlay,
				Archetype: "ArithmeticNote",
				Callback:  "preprocess",
				Memory:    map[int][]float64{4001: {1, test.operation}},
			})
			if err != nil {
				t.Fatalf("optimization %d operation %v: %v", optimization, test.operation, err)
			}
			if got := result.Memory[4000][0]; got != test.want {
				t.Fatalf("optimization %d operation %v = %v, want %v", optimization, test.operation, got, test.want)
			}
		}
	}
}

func TestRuntimeChecksEnabledIsCompileTimeAndUnreachableIsPruned(t *testing.T) {
	for _, test := range []struct {
		checks RuntimeChecks
		want   float64
	}{
		{RuntimeChecksNone, 32},
		{RuntimeChecksTerminate, 31},
		{RuntimeChecksNotify, 31},
	} {
		engine, err := Compile(Options{Optimization: OptimizationStandard, RuntimeChecks: test.checks}, "../../internal/compiler/testdata/simulator")
		if err != nil {
			t.Fatal(err)
		}
		result, err := engine.Run(Request{Mode: ModePlay, Archetype: "MetaNote", Callback: "preprocess"})
		if err != nil {
			t.Fatal(err)
		}
		if got := result.Memory[4000][0]; got != test.want {
			t.Errorf("checks %v produced %v, want %v", test.checks, got, test.want)
		}
	}
}

func TestLabeledControlFlowFallthroughAndGotoAcrossOptimizations(t *testing.T) {
	for _, optimization := range []OptimizationLevel{OptimizationMinimal, OptimizationFast, OptimizationStandard} {
		engine, err := Compile(Options{Optimization: optimization}, "../../internal/compiler/testdata/simulator")
		if err != nil {
			t.Fatal(err)
		}
		for selector, want := range []float64{10, 9, 7} {
			result, err := engine.Run(Request{
				Mode:      ModePlay,
				Archetype: "ControlNote",
				Callback:  "preprocess",
				Memory:    map[int][]float64{4001: {float64(selector)}},
			})
			if err != nil {
				t.Fatal(err)
			}
			if got := result.Memory[4000][0]; got != want {
				t.Errorf("optimization %d selector %d produced %v, want %v", optimization, selector, got, want)
			}
		}
	}
}

func TestNestedDynamicMemoryRangeAcrossOptimizations(t *testing.T) {
	for _, optimization := range []OptimizationLevel{OptimizationMinimal, OptimizationFast, OptimizationStandard} {
		engine, err := Compile(Options{Optimization: optimization}, "../../internal/compiler/testdata/simulator")
		if err != nil {
			t.Fatal(err)
		}
		entityData := make([]float64, 64)
		entityData[0], entityData[1] = 1, 2
		entityData[32], entityData[33] = 4, 7
		for _, test := range []struct {
			selector float64
			want     float64
		}{{0, 22}, {1, 6}} {
			result, err := engine.Run(Request{
				Mode:      ModePlay,
				Archetype: "ViewRangeNote",
				Callback:  "preprocess",
				Memory: map[int][]float64{
					4001: {1, 0, test.selector},
					4101: entityData,
				},
			})
			if err != nil {
				t.Fatal(err)
			}
			if got := result.Memory[4000][0]; got != test.want {
				t.Errorf("optimization %d selector %v produced %v, want %v", optimization, test.selector, got, test.want)
			}
		}
	}
}

func TestLinkedEntitySortIsStableAcrossOptimizations(t *testing.T) {
	entityData := make([]float64, 96)
	entityData[0], entityData[32], entityData[64] = 2, 1, 2
	shared := make([]float64, 96)
	shared[0], shared[1] = 1, -1
	shared[32], shared[33] = 2, 0
	shared[64], shared[65] = -1, 1
	for _, optimization := range []OptimizationLevel{OptimizationMinimal, OptimizationFast, OptimizationStandard} {
		engine, err := Compile(Options{Optimization: optimization}, "../../internal/compiler/testdata/simulator")
		if err != nil {
			t.Fatal(err)
		}
		result, err := engine.Run(Request{
			Mode:      ModePlay,
			Archetype: "LinkedNote",
			Callback:  "preprocess",
			Memory: map[int][]float64{
				4000: {0},
				4101: append([]float64(nil), entityData...),
				4102: append([]float64(nil), shared...),
			},
		})
		if err != nil {
			t.Fatalf("optimization %d: %v", optimization, err)
		}
		if got := result.Memory[4000][0]; got != 1 {
			t.Fatalf("optimization %d head = %v, want 1; shared=%v effects=%v", optimization, got, result.Memory[4102], result.Effects)
		}
		links := result.Memory[4102]
		if links[32] != 0 || links[0] != 2 || links[64] != -1 {
			t.Fatalf("optimization %d next links = [%v %v %v], want [2 0 -1] by entity 0..2", optimization, links[0], links[32], links[64])
		}
		if links[33] != -1 || links[1] != 1 || links[65] != 0 {
			t.Fatalf("optimization %d previous links = [%v %v %v], want [1 -1 0] by entity 0..2", optimization, links[1], links[33], links[65])
		}
	}
}

func TestMemoryStackAndStreamBuiltins(t *testing.T) {
	machine := simexec.NewMachine()
	mustBuiltin := func(name string, args ...float64) float64 {
		t.Helper()
		value, handled, err := machine.Builtin(resource.RuntimeFunction(name), args...)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if !handled {
			t.Fatalf("%s was not handled", name)
		}
		return value
	}

	mustBuiltin("Set", 1, 4, 10)
	mustBuiltin("Set", 2, 0, 1)
	mustBuiltin("Set", 2, 1, 4)
	if value := mustBuiltin("GetPointed", 2, 0, 0); value != 10 {
		t.Fatalf("GetPointed = %v, want 10", value)
	}
	if value := mustBuiltin("IncrementPostPointed", 2, 0, 0); value != 10 {
		t.Fatalf("IncrementPostPointed = %v, want 10", value)
	}
	mustBuiltin("Copy", 1, 4, 1, 5, 1)
	if value := mustBuiltin("Get", 1, 5); value != 11 {
		t.Fatalf("copied value = %v, want 11", value)
	}

	mustBuiltin("StackInit")
	mustBuiltin("StackPush", 3)
	mustBuiltin("StackEnter", 2)
	mustBuiltin("StackSetFrame", 0, 7)
	if value := mustBuiltin("StackGetFrame", 0); value != 7 {
		t.Fatalf("StackGetFrame = %v, want 7", value)
	}
	mustBuiltin("StackLeave")
	if value := mustBuiltin("StackPop"); value != 3 {
		t.Fatalf("StackPop = %v, want 3", value)
	}

	mustBuiltin("StreamSet", 1, 0, 2)
	mustBuiltin("StreamSet", 1, 2, 6)
	if value := mustBuiltin("StreamGetValue", 1, 1); value != 4 {
		t.Fatalf("interpolated stream value = %v, want 4", value)
	}
	if value := mustBuiltin("StreamGetPreviousKey", 1, 1); value != 0 {
		t.Fatalf("previous stream key = %v, want 0", value)
	}
	if value := mustBuiltin("StreamGetNextKey", 1, 1); value != 2 {
		t.Fatalf("next stream key = %v, want 2", value)
	}
}

func TestSimulatorValidationAndJSNumbers(t *testing.T) {
	if err := simexec.ValidateStreams(map[int][]simexec.StreamEntry{1: {{Key: 1}, {Key: 1}}}); err == nil {
		t.Fatal("duplicate stream key was accepted")
	}
	if err := simexec.ValidateStreams(map[int][]simexec.StreamEntry{1: {{Key: math.NaN()}}}); err == nil {
		t.Fatal("non-finite stream key was accepted")
	}
	machine := simexec.NewMachine()
	if _, _, err := machine.Builtin(resource.RuntimeFunction("Get"), 1, -1); err == nil {
		t.Fatal("negative memory index was accepted")
	}
	if value, handled, err := machine.Builtin(resource.RuntimeFunction("Round"), -0.5); err != nil || !handled || !math.Signbit(value) || value != 0 {
		t.Fatalf("Round(-0.5) = %v, handled=%v, err=%v", value, handled, err)
	}
	if value, handled, err := machine.Builtin(resource.RuntimeFunction("Divide"), 1, 0); err != nil || !handled || !math.IsInf(value, 1) {
		t.Fatalf("Divide(1, 0) = %v, handled=%v, err=%v", value, handled, err)
	}
}

func TestExecutionErrorsAreStructured(t *testing.T) {
	assertKind := func(err error, kind ExecutionErrorKind) *ExecutionError {
		t.Helper()
		var executionErr *ExecutionError
		if !errors.As(err, &executionErr) {
			t.Fatalf("error %v is not an ExecutionError", err)
		}
		if executionErr.Kind != kind {
			t.Fatalf("error kind = %q, want %q: %v", executionErr.Kind, kind, err)
		}
		return executionErr
	}
	var engine *Engine
	_, err := engine.Run(Request{})
	assertKind(err, ExecutionErrorInvalidRequest)

	_, err = simexec.Execute(nil, 0, simexec.Request{})
	if executionErr := assertKind(err, ExecutionErrorInvalidNode); executionErr.NodeIndex != 0 {
		t.Fatalf("invalid node index = %d, want 0", executionErr.NodeIndex)
	}

	nodes := []resource.EngineDataNode{
		resource.EngineDataFunctionNode{Func: resource.RuntimeFunctionIf, Args: []int{1}},
		resource.EngineDataValueNode{Value: 1},
	}
	_, err = simexec.Execute(nodes, 0, simexec.Request{})
	if executionErr := assertKind(err, ExecutionErrorInvalidArity); executionErr.Function != resource.RuntimeFunctionIf {
		t.Fatalf("arity function = %q, want If", executionErr.Function)
	}

	nodes = []resource.EngineDataNode{
		resource.EngineDataFunctionNode{Func: resource.RuntimeFunctionGet, Args: []int{1, 2}},
		resource.EngineDataValueNode{Value: 1},
		resource.EngineDataValueNode{Value: -1},
	}
	_, err = simexec.Execute(nodes, 0, simexec.Request{})
	if executionErr := assertKind(err, ExecutionErrorInvalidArgument); executionErr.ArgumentIndex != 1 {
		t.Fatalf("invalid argument index = %d, want 1", executionErr.ArgumentIndex)
	}

	nodes = []resource.EngineDataNode{
		resource.EngineDataFunctionNode{Func: resource.RuntimeFunctionWhile, Args: []int{1, 2}},
		resource.EngineDataValueNode{Value: 1},
		resource.EngineDataValueNode{},
	}
	_, err = simexec.Execute(nodes, 0, simexec.Request{StepLimit: 3})
	assertKind(err, ExecutionErrorStepLimit)

	var nilFunctionNode *resource.EngineDataFunctionNode
	_, err = simexec.Execute([]resource.EngineDataNode{nilFunctionNode}, 0, simexec.Request{})
	assertKind(err, ExecutionErrorInvalidNode)
}

func equalMemory(left, right map[int][]float64) bool {
	for block, leftValues := range left {
		if block == 10000 {
			continue
		}
		rightValues, ok := right[block]
		if !ok || len(leftValues) != len(rightValues) {
			return false
		}
		for index, leftValue := range leftValues {
			if !equalNumber(leftValue, rightValues[index]) {
				return false
			}
		}
	}
	for block := range right {
		if block == 10000 {
			continue
		}
		if _, ok := left[block]; !ok {
			return false
		}
	}
	return true
}

func equalNumber(left, right float64) bool {
	return left == right || math.IsNaN(left) && math.IsNaN(right)
}

func FuzzMalformedEngineDataNeverPanics(f *testing.F) {
	f.Add([]byte{0, 1, 2, 3, 4})
	f.Add([]byte{255, 0, 7, 1})
	functions := []resource.RuntimeFunction{
		resource.RuntimeFunctionIf,
		resource.RuntimeFunctionSet,
		resource.RuntimeFunctionStreamSet,
		resource.RuntimeFunctionJumpLoop,
		resource.RuntimeFunction("UnknownRuntimeFunction"),
	}
	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) == 0 {
			return
		}
		nodes := make([]resource.EngineDataNode, min(len(data), 32))
		for index := range nodes {
			value := data[index]
			if value&1 == 0 {
				nodes[index] = resource.EngineDataValueNode{Value: float64(int8(value))}
				continue
			}
			arguments := make([]int, int(value>>5))
			for argument := range arguments {
				arguments[argument] = int(data[(index+argument+1)%len(data)]) - 64
			}
			nodes[index] = resource.EngineDataFunctionNode{Func: functions[int(value)%len(functions)], Args: arguments}
		}
		_, _ = simexec.Execute(nodes, int(data[0])%len(nodes), simexec.Request{StepLimit: 64})
	})
}
