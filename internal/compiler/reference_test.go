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

	"github.com/WindowsSov8forUs/sonolus-core-go/core/resource"
	"github.com/WindowsSov8forUs/sonolus-go/v2/internal/compiler/backend"
	"github.com/WindowsSov8forUs/sonolus-go/v2/internal/compiler/frontend"
	"github.com/WindowsSov8forUs/sonolus-go/v2/internal/compiler/optimize"
	"github.com/WindowsSov8forUs/sonolus-go/v2/internal/simexec"
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
	nodes []resource.EngineDataNode
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
						t.Fatalf("%s/%s/%s seed %v changed semantics\\nminimal: %#v\\n%s: %#v", level, modeName, label, seed, want, level, got)
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
	if strings.HasSuffix(label, "shouldSpawn") {
		value = 1
	}
	return executionResult{Value: math.Float64bits(value), Memory: map[string]uint64{}, Effects: []string{}}, nil
}

func decodeExecutableMode(data any) (executableMode, error) {
	result := executableMode{roots: map[string]int{}}
	add := func(archetype int, name string, callbackIndex int) {
		result.roots[fmt.Sprintf("archetype[%d].%s", archetype, name)] = callbackIndex
	}
	switch value := data.(type) {
	case *resource.EnginePlayData:
		result.nodes = value.Nodes
		for index, archetype := range value.Archetypes {
			for name, callback := range map[string]*resource.EnginePlayDataArchetypeCallback{
				"preprocess": archetype.Preprocess, "spawnOrder": archetype.SpawnOrder, "shouldSpawn": archetype.ShouldSpawn,
				"initialize": archetype.Initialize, "updateSequential": archetype.UpdateSequential, "touch": archetype.Touch,
				"updateParallel": archetype.UpdateParallel, "terminate": archetype.Terminate,
			} {
				if callback != nil {
					add(index, name, callback.Index)
				}
			}
		}
	case *resource.EngineWatchData:
		result.nodes = value.Nodes
		result.roots["global.updateSpawn"] = value.UpdateSpawn
		for index, archetype := range value.Archetypes {
			for name, callback := range map[string]*resource.EngineWatchDataArchetypeCallback{
				"preprocess": archetype.Preprocess, "spawnTime": archetype.SpawnTime, "despawnTime": archetype.DespawnTime,
				"initialize": archetype.Initialize, "updateSequential": archetype.UpdateSequential,
				"updateParallel": archetype.UpdateParallel, "terminate": archetype.Terminate,
			} {
				if callback != nil {
					add(index, name, callback.Index)
				}
			}
		}
	case *resource.EnginePreviewData:
		result.nodes = value.Nodes
		for index, archetype := range value.Archetypes {
			for name, callback := range map[string]*resource.EnginePreviewDataArchetypeCallback{"preprocess": archetype.Preprocess, "render": archetype.Render} {
				if callback != nil {
					add(index, name, callback.Index)
				}
			}
		}
	case *resource.EngineTutorialData:
		result.nodes = value.Nodes
		result.roots["global.preprocess"] = value.Preprocess
		result.roots["global.navigate"] = value.Navigate
		result.roots["global.update"] = value.Update
	default:
		return executableMode{}, fmt.Errorf("unsupported executable mode %T", data)
	}
	return result, nil
}

func executeRoot(mode executableMode, root int, seed float64) (executionResult, error) {
	result, err := simexec.Execute(mode.nodes, root, simexec.Request{DefaultMemory: &seed, StepLimit: 10000})
	if err != nil {
		return executionResult{}, err
	}
	memory := map[string]uint64{}
	for block, values := range result.Memory {
		if block == 10000 || block == 3000 {
			continue
		}
		for index, value := range values {
			memory[fmt.Sprintf("%d:%d", block, index)] = math.Float64bits(value)
		}
	}
	effects := make([]string, len(result.Effects))
	for index, effect := range result.Effects {
		arguments := make([]uint64, len(effect.Arguments))
		for argument, value := range effect.Arguments {
			arguments[argument] = math.Float64bits(value)
		}
		effects[index] = fmt.Sprintf("%s:%v", effect.Function, arguments)
	}
	return executionResult{Value: math.Float64bits(result.Value), Memory: memory, Effects: effects}, nil
}

func BenchmarkCompileAll(b *testing.B) {
	corpora := []struct {
		name    string
		pattern string
	}{
		{name: "reference", pattern: "./testdata/reference"},
		{name: "godori", pattern: "../../godori"},
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

func BenchmarkCompilerStages(b *testing.B) {
	for _, corpus := range []struct {
		name, pattern string
	}{{"reference", "./testdata/reference"}, {"godori", "../../godori"}} {
		b.Run(corpus.name, func(b *testing.B) {
			b.Run("load", func(b *testing.B) {
				for range b.N {
					compiler := NewCompiler(Options{Optimization: optimize.LevelStandard}, corpus.pattern)
					if _, _, err := compiler.loadModes(orderedModes); err != nil {
						b.Fatal(err)
					}
				}
			})

			compiler := NewCompiler(Options{Optimization: optimize.LevelStandard}, corpus.pattern)
			loaded, _, err := compiler.loadModes(orderedModes)
			if err != nil {
				b.Fatal(err)
			}
			parse := func() *frontend.Project {
				parser := frontend.NewParser()
				for _, currentMode := range orderedModes {
					if err := parser.Parse(currentMode, loaded[currentMode]); err != nil {
						b.Fatal(err)
					}
				}
				project, parseErr := parser.GetProject()
				if parseErr != nil {
					b.Fatal(parseErr)
				}
				if diagnosticErr := frontend.ResolveDiagnostics(project); diagnosticErr != nil {
					b.Fatal(diagnosticErr)
				}
				return project
			}

			b.Run("frontend", func(b *testing.B) {
				for range b.N {
					_ = parse()
				}
			})
			project := parse()
			b.Run("optimize", func(b *testing.B) {
				optimizer := optimize.NewOptimizer(optimize.LevelStandard)
				for range b.N {
					if _, optimizeErr := optimizeProject(optimizer, project); optimizeErr != nil {
						b.Fatal(optimizeErr)
					}
				}
			})
			optimized, err := optimizeProject(optimize.NewOptimizer(optimize.LevelStandard), project)
			if err != nil {
				b.Fatal(err)
			}
			b.Run("backend", func(b *testing.B) {
				for range b.N {
					if _, backendErr := backend.Compile(optimized); backendErr != nil {
						b.Fatal(backendErr)
					}
				}
			})
		})
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

func TestTypedStreamsRoundTripThroughFinalEngineData(t *testing.T) {
	var reference map[int][]simexec.StreamEntry
	for levelIndex, level := range []optimize.Level{optimize.LevelMinimal, optimize.LevelFast, optimize.LevelStandard} {
		artifacts, err := NewCompiler(Options{Optimization: level}, "./testdata/streams").Compile(mode.ModePlay, mode.ModeWatch)
		if err != nil {
			t.Fatalf("%s: %v", level, err)
		}
		playRoot := artifacts.Play.Archetypes[0].Preprocess.Index
		playResult, err := simexec.Execute(artifacts.Play.Nodes, playRoot, simexec.Request{})
		if err != nil {
			t.Fatalf("%s play: %v", level, err)
		}
		for streamID, want := range map[int][]simexec.StreamEntry{
			1: {{Key: 1, Value: 2}}, 2: {{Key: 1, Value: 3}}, 3: {{Key: 1, Value: 4}},
			7: {{Key: -0.5, Value: 0}, {Key: 0, Value: 5}, {Key: 0.5, Value: 0}, {Key: 1, Value: 6}, {Key: 1.5, Value: 0}},
		} {
			if !reflect.DeepEqual(playResult.Streams[streamID], want) {
				t.Fatalf("%s stream %d = %+v, want %+v", level, streamID, playResult.Streams[streamID], want)
			}
		}
		watchRoot := artifacts.Watch.Archetypes[0].Preprocess.Index
		watchResult, err := simexec.Execute(artifacts.Watch.Nodes, watchRoot, simexec.Request{Streams: playResult.Streams})
		if err != nil || len(watchResult.Effects) == 0 {
			t.Fatalf("%s watch result=%+v err=%v", level, watchResult, err)
		}
		if levelIndex == 0 {
			reference = playResult.Streams
		} else if !reflect.DeepEqual(playResult.Streams, reference) {
			t.Fatalf("%s stream state differs from Minimal", level)
		}
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
