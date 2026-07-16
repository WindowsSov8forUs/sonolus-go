package compiler

import (
	"encoding/binary"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/WindowsSov8forUs/sonolus-go/v2/internal/compiler/optimize"
	"math"

	"github.com/WindowsSov8forUs/sonolus-go/v2/internal/compiler/mode"
	"strings"
)

var updateReferenceGolden = flag.Bool("update-reference", false, "update checked-in compiler reference golden")

type referenceSnapshot struct {
	PythonCommit  string         `json:"pythonCommit"`
	JSCommit      string         `json:"jsCommit"`
	Configuration any            `json:"configuration"`
	ROM           []uint32       `json:"romFloat32Bits"`
	Modes         map[string]any `json:"modes"`
}

func TestReferenceEngineDataGolden(t *testing.T) {
	artifacts, err := NewCompiler(Options{Optimization: optimize.LevelMinimal}, "./testdata/reference").CompileAll()
	if err != nil {
		t.Fatal(err)
	}
	snapshot := referenceSnapshot{
		PythonCommit:  "1040bc0dcc116efdbca05f144edec302e839bcd3",
		JSCommit:      "37b0eee5aa16d1e01973d33d625d86f5ef72d268",
		Configuration: artifacts.Configuration,
		ROM:           romBits(artifacts.ROM),
		Modes:         map[string]any{},
	}
	for name, data := range map[string]any{"play": artifacts.Play, "watch": artifacts.Watch, "preview": artifacts.Preview, "tutorial": artifacts.Tutorial} {
		normalized, err := normalizeEngineData(data)
		if err != nil {
			t.Fatalf("normalize %s: %v", name, err)
		}
		snapshot.Modes[name] = normalized
	}
	actual, err := json.MarshalIndent(snapshot, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	actual = append(actual, '\n')
	path := filepath.Join("testdata", "backend", "reference.golden.json")
	if *updateReferenceGolden {
		if err := os.WriteFile(path, actual, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	expected, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(actual, expected) {
		t.Fatalf("reference golden differs; run go test ./internal/compiler -run TestReferenceEngineDataGolden -update-reference")
	}
}

func romBits(data []byte) []uint32 {
	result := make([]uint32, len(data)/4)
	for i := range result {
		result[i] = binary.LittleEndian.Uint32(data[i*4:])
	}
	return result
}

func normalizeEngineData(data any) (map[string]any, error) {
	raw, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}
	var value map[string]any
	if err := json.Unmarshal(raw, &value); err != nil {
		return nil, err
	}
	nodes, _ := value["nodes"].([]any)
	expand := func(index int) (any, error) { return expandNode(nodes, index, map[int]bool{}) }
	callbacks := map[string]bool{
		"preprocess": true, "spawnOrder": true, "shouldSpawn": true, "initialize": true,
		"updateSequential": true, "touch": true, "updateParallel": true, "terminate": true,
		"spawnTime": true, "despawnTime": true, "render": true,
	}
	if archetypes, ok := value["archetypes"].([]any); ok {
		for _, rawArchetype := range archetypes {
			archetype := rawArchetype.(map[string]any)
			for name := range callbacks {
				callback, ok := archetype[name].(map[string]any)
				if !ok {
					continue
				}
				index := int(callback["index"].(float64))
				tree, err := expand(index)
				if err != nil {
					return nil, fmt.Errorf("%s callback: %w", name, err)
				}
				delete(callback, "index")
				callback["tree"] = tree
			}
		}
	}
	for _, name := range []string{"updateSpawn", "preprocess", "navigate", "update"} {
		index, ok := value[name].(float64)
		if !ok {
			continue
		}
		tree, err := expand(int(index))
		if err != nil {
			return nil, fmt.Errorf("global %s: %w", name, err)
		}
		value[name] = tree
	}
	delete(value, "nodes")
	return value, nil
}

func expandNode(nodes []any, index int, visiting map[int]bool) (any, error) {
	if index < 0 || index >= len(nodes) {
		return nil, fmt.Errorf("node index %d outside [0,%d)", index, len(nodes))
	}
	if visiting[index] {
		return nil, fmt.Errorf("node cycle at %d", index)
	}
	visiting[index] = true
	defer delete(visiting, index)
	node := nodes[index].(map[string]any)
	if value, ok := node["value"]; ok {
		return map[string]any{"value": value}, nil
	}
	function, ok := node["func"]
	if !ok {
		return nil, fmt.Errorf("node %d has no value or func", index)
	}
	args := node["args"].([]any)
	expanded := make([]any, len(args))
	for i, raw := range args {
		child, err := expandNode(nodes, int(raw.(float64)), visiting)
		if err != nil {
			return nil, err
		}
		expanded[i] = child
	}
	return map[string]any{"func": function, "args": expanded}, nil
}

type executableMode struct {
	nodes []map[string]any
	roots map[string]int
}

type executionResult struct {
	Value   uint64
	Memory  map[string]uint64
	Effects []string
}

func TestOptimizationLevelsPreserveReferenceCallbackSemantics(t *testing.T) {
	levels := []optimize.Level{optimize.LevelMinimal, optimize.LevelFast, optimize.LevelStandard}
	compiled := map[optimize.Level]map[string]executableMode{}
	for _, level := range levels {
		artifacts, err := NewCompiler(Options{Optimization: level}, "./testdata/reference").CompileAll()
		if err != nil {
			t.Fatalf("compile %s: %v", level, err)
		}
		compiled[level] = map[string]executableMode{}
		for name, data := range map[string]any{"play": artifacts.Play, "watch": artifacts.Watch, "preview": artifacts.Preview, "tutorial": artifacts.Tutorial} {
			mode, err := decodeExecutableMode(data)
			if err != nil {
				t.Fatalf("decode %s/%s: %v", level, name, err)
			}
			compiled[level][name] = mode
		}
	}
	for _, modeName := range []string{"play", "watch", "preview", "tutorial"} {
		baseline := compiled[optimize.LevelMinimal][modeName]
		for _, level := range levels[1:] {
			candidate := compiled[level][modeName]
			labels := map[string]bool{}
			for label := range baseline.roots {
				labels[label] = true
			}
			for label := range candidate.roots {
				labels[label] = true
			}
			for label := range labels {
				for _, seed := range []float64{-1, 0, 1, 2.5} {
					want, err := executeOptionalRoot(baseline, label, seed)
					if err != nil {
						t.Fatalf("minimal/%s/%s seed %v: %v", modeName, label, seed, err)
					}
					got, err := executeOptionalRoot(candidate, label, seed)
					if err != nil {
						t.Fatalf("%s/%s/%s seed %v: %v", level, modeName, label, seed, err)
					}
					if !reflect.DeepEqual(got, want) {
						t.Fatalf("%s/%s/%s seed %v changed semantics\nminimal: %#v\n%s: %#v", level, modeName, label, seed, want, level, got)
					}
				}
			}
		}
	}
}

func executeOptionalRoot(mode executableMode, label string, seed float64) (executionResult, error) {
	if root, ok := mode.roots[label]; ok {
		return executeRoot(mode, root, seed)
	}
	value := float64(0)
	if len(label) >= len("shouldSpawn") && label[len(label)-len("shouldSpawn"):] == "shouldSpawn" {
		value = 1
	}
	return executionResult{Value: math.Float64bits(value), Memory: map[string]uint64{}}, nil
}

func decodeExecutableMode(data any) (executableMode, error) {
	raw, err := json.Marshal(data)
	if err != nil {
		return executableMode{}, err
	}
	var value map[string]any
	if err := json.Unmarshal(raw, &value); err != nil {
		return executableMode{}, err
	}
	rawNodes, _ := value["nodes"].([]any)
	nodes := make([]map[string]any, len(rawNodes))
	for i, node := range rawNodes {
		nodes[i] = node.(map[string]any)
	}
	roots := map[string]int{}
	if archetypes, ok := value["archetypes"].([]any); ok {
		for archetypeIndex, rawArchetype := range archetypes {
			archetype := rawArchetype.(map[string]any)
			for _, name := range []string{"preprocess", "spawnOrder", "shouldSpawn", "initialize", "updateSequential", "touch", "updateParallel", "terminate", "spawnTime", "despawnTime", "render"} {
				callback, ok := archetype[name].(map[string]any)
				if !ok {
					continue
				}
				index, ok := callback["index"].(float64)
				if ok {
					roots[fmt.Sprintf("archetype[%d].%s", archetypeIndex, name)] = int(index)
				}
			}
		}
	}
	for _, name := range []string{"updateSpawn", "preprocess", "navigate", "update"} {
		if index, ok := value[name].(float64); ok && int(index) >= 0 && int(index) < len(nodes) {
			roots["global."+name] = int(index)
		}
	}
	return executableMode{nodes: nodes, roots: roots}, nil
}

type runtimeExecutor struct {
	mode    executableMode
	seed    float64
	memory  map[string]float64
	effects []string
	steps   int
}

type breakSignal struct {
	depth int
	value float64
}

func executeRoot(mode executableMode, root int, seed float64) (executionResult, error) {
	executor := &runtimeExecutor{mode: mode, seed: seed, memory: map[string]float64{}}
	value, signal, err := executor.eval(root)
	if err != nil {
		return executionResult{}, err
	}
	if signal != nil {
		return executionResult{}, fmt.Errorf("uncaught break depth %d", signal.depth)
	}
	memory := make(map[string]uint64, len(executor.memory))
	for key, value := range executor.memory {
		if strings.HasPrefix(key, fmt.Sprintf("%x:", math.Float64bits(10000))) {
			continue
		}
		memory[key] = math.Float64bits(value)
	}
	return executionResult{Value: math.Float64bits(value), Memory: memory, Effects: executor.effects}, nil
}

func (e *runtimeExecutor) eval(index int) (float64, *breakSignal, error) {
	e.steps++
	if e.steps > 10000 {
		return 0, nil, fmt.Errorf("execution step limit exceeded")
	}
	if index < 0 || index >= len(e.mode.nodes) {
		return 0, nil, fmt.Errorf("node index %d out of range", index)
	}
	node := e.mode.nodes[index]
	if value, ok := node["value"].(float64); ok {
		return value, nil, nil
	}
	function := node["func"].(string)
	rawArgs := node["args"].([]any)
	argIndex := func(i int) int { return int(rawArgs[i].(float64)) }
	evalArg := func(i int) (float64, *breakSignal, error) { return e.eval(argIndex(i)) }
	evalAll := func() ([]float64, *breakSignal, error) {
		values := make([]float64, len(rawArgs))
		for i := range rawArgs {
			value, signal, err := evalArg(i)
			if err != nil || signal != nil {
				return nil, signal, err
			}
			values[i] = value
		}
		return values, nil, nil
	}
	switch function {
	case "Execute":
		values, signal, err := evalAll()
		if err != nil || signal != nil || len(values) == 0 {
			return 0, signal, err
		}
		return values[len(values)-1], nil, nil
	case "If":
		condition, signal, err := evalArg(0)
		if err != nil || signal != nil {
			return 0, signal, err
		}
		if condition != 0 {
			return evalArg(1)
		}
		return evalArg(2)
	case "And":
		for i := range rawArgs {
			value, signal, err := evalArg(i)
			if err != nil || signal != nil {
				return 0, signal, err
			}
			if value == 0 {
				return 0, nil, nil
			}
		}
		return 1, nil, nil
	case "Block":
		value, signal, err := evalArg(0)
		if signal != nil {
			if signal.depth == 1 {
				return signal.value, nil, err
			}
			signal.depth--
		}
		return value, signal, err
	case "JumpLoop":
		next := 0
		for next >= 0 && next < len(rawArgs) {
			value, signal, err := evalArg(next)
			if err != nil || signal != nil {
				return 0, signal, err
			}
			next = int(value)
		}
		return float64(next), nil, nil
	case "Break":
		depth, signal, err := evalArg(0)
		if err != nil || signal != nil {
			return 0, signal, err
		}
		value, signal, err := evalArg(1)
		if err != nil || signal != nil {
			return 0, signal, err
		}
		return 0, &breakSignal{depth: int(depth), value: value}, nil
	case "Get":
		args, signal, err := evalAll()
		if err != nil || signal != nil {
			return 0, signal, err
		}
		key := memoryKey(args[0], args[1])
		if value, ok := e.memory[key]; ok {
			return value, nil, nil
		}
		return e.initialMemory(args[0]), nil, nil
	case "GetShifted":
		args, signal, err := evalAll()
		if err != nil || signal != nil {
			return 0, signal, err
		}
		key := memoryKey(args[0], args[1]+args[2]*args[3])
		if value, ok := e.memory[key]; ok {
			return value, nil, nil
		}
		return e.initialMemory(args[0]), nil, nil
	case "Set", "SetAdd", "SetSubtract", "SetMultiply", "SetDivide", "SetMod", "SetRem", "SetPower":
		args, signal, err := evalAll()
		if err != nil || signal != nil {
			return 0, signal, err
		}
		key := memoryKey(args[0], args[1])
		value := args[2]
		if function != "Set" {
			current, ok := e.memory[key]
			if !ok {
				current = e.initialMemory(args[0])
			}
			switch function {
			case "SetAdd":
				value = current + value
			case "SetSubtract":
				value = current - value
			case "SetMultiply":
				value = current * value
			case "SetDivide":
				value = current / value
			case "SetMod":
				value = current - math.Floor(current/value)*value
			case "SetRem":
				value = math.Mod(current, value)
			case "SetPower":
				value = math.Pow(current, value)
			}
		}
		e.memory[key] = value
		return value, nil, nil
	case "SetShifted", "SetAddShifted", "SetSubtractShifted", "SetMultiplyShifted", "SetDivideShifted", "SetModShifted", "SetRemShifted", "SetPowerShifted":
		args, signal, err := evalAll()
		if err != nil || signal != nil {
			return 0, signal, err
		}
		key := memoryKey(args[0], args[1]+args[2]*args[3])
		value := args[4]
		if function != "SetShifted" {
			current, ok := e.memory[key]
			if !ok {
				current = e.initialMemory(args[0])
			}
			switch function {
			case "SetAddShifted":
				value = current + value
			case "SetSubtractShifted":
				value = current - value
			case "SetMultiplyShifted":
				value = current * value
			case "SetDivideShifted":
				value = current / value
			case "SetModShifted":
				value = current - math.Floor(current/value)*value
			case "SetRemShifted":
				value = math.Mod(current, value)
			case "SetPowerShifted":
				value = math.Pow(current, value)
			}
		}
		e.memory[key] = value
		return value, nil, nil
	case "DebugLog", "Paint":
		args, signal, err := evalAll()
		if err != nil || signal != nil {
			return 0, signal, err
		}
		e.effects = append(e.effects, fmt.Sprintf("%s:%v", function, floatBits(args)))
		return 0, nil, nil
	case "Negate":
		value, signal, err := evalArg(0)
		return -value, signal, err
	case "Add", "Subtract", "Less", "Greater":
		args, signal, err := evalAll()
		if err != nil || signal != nil {
			return 0, signal, err
		}
		switch function {
		case "Add":
			result := 0.0
			for _, value := range args {
				result += value
			}
			return result, nil, nil
		case "Subtract":
			return args[0] - args[1], nil, nil
		case "Less":
			if args[0] < args[1] {
				return 1, nil, nil
			}
			return 0, nil, nil
		}
		if args[0] > args[1] {
			return 1, nil, nil
		}
		return 0, nil, nil
	default:
		return 0, nil, fmt.Errorf("interpreter does not support %s", function)
	}
}

func memoryKey(block, index float64) string {
	return fmt.Sprintf("%x:%x", math.Float64bits(block), math.Float64bits(index))
}

func (e *runtimeExecutor) initialMemory(block float64) float64 {
	if block == 10000 {
		return 0
	}
	return e.seed
}

func floatBits(values []float64) []uint64 {
	result := make([]uint64, len(values))
	for i, value := range values {
		result[i] = math.Float64bits(value)
	}
	return result
}
func BenchmarkCompileAll(b *testing.B) {
	corpora := []struct {
		name    string
		pattern string
	}{
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
