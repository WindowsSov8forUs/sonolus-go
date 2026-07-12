package compiler

import (
	"encoding/json"
	"fmt"
	"math"
	"reflect"
	"strings"
	"testing"

	"github.com/WindowsSov8forUs/sonolus-go/internal/compiler/optimize"
)

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
