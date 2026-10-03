package compiler

import (
	"encoding/binary"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"testing"

	"github.com/WindowsSov8forUs/sonolus-core-go/core/resource"
	"github.com/WindowsSov8forUs/sonolus-go/v2/internal/compiler/backend"
	"github.com/WindowsSov8forUs/sonolus-go/v2/internal/compiler/frontend"
	"github.com/WindowsSov8forUs/sonolus-go/v2/internal/compiler/ir"
	"github.com/WindowsSov8forUs/sonolus-go/v2/internal/compiler/optimize"
	"github.com/WindowsSov8forUs/sonolus-go/v2/internal/compiler/testdata/freezeaddress/model"
	"github.com/WindowsSov8forUs/sonolus-go/v2/internal/simexec"
	"math"

	"github.com/WindowsSov8forUs/sonolus-go/v2/internal/compiler/mode"
	"strings"
)

var updateReferenceGolden = flag.Bool("update-reference", false, "update checked-in compiler reference golden")

func TestLargeContainerSortPreservesStableOrder(t *testing.T) {
	temporary := make([]float64, 4096)
	for i := range temporary {
		temporary[i] = float64(i + 17)
	}
	for _, options := range []Options{
		{Optimization: optimize.LevelMinimal},
		{Optimization: optimize.LevelFast},
		{Optimization: optimize.LevelStandard},
		{Optimization: optimize.LevelStandard, RuntimeChecks: RuntimeChecksNotify},
	} {
		name := options.Optimization.String()
		if options.RuntimeChecks == RuntimeChecksNotify {
			name += "-notify"
		}
		t.Run(name, func(t *testing.T) {
			artifacts, err := NewCompiler(options, "./testdata/callvalues").Compile(mode.ModePlay, mode.ModeWatch)
			if err != nil {
				t.Fatal(err)
			}
			for _, m := range []mode.Mode{mode.ModePlay, mode.ModeWatch} {
				var nodes []resource.EngineDataNode
				root := -1
				if m == mode.ModePlay {
					nodes = artifacts.Play.Nodes
					for _, a := range artifacts.Play.Archetypes {
						if a.Name == "LargeSort" {
							root = a.Preprocess.Index
						}
					}
				} else {
					nodes = artifacts.Watch.Nodes
					for _, a := range artifacts.Watch.Archetypes {
						if a.Name == "LargeSort" {
							root = a.Preprocess.Index
						}
					}
				}
				if root < 0 {
					t.Fatal("missing LargeSort")
				}
				for _, count := range []int{0, 1, 2, 3, 16, 17, 31, 64, 129, 257} {
					for pattern := range 5 {
						for descending := range 2 {
							for persistent := range 2 {
								t.Run(fmt.Sprintf("%s/n%d/p%d/d%d/storage%d", m, count, pattern, descending, persistent), func(t *testing.T) {
									want := make([][2]int, count)
									for i := range want {
										key := (i*37 + 11) % 17
										switch pattern {
										case 0:
											key = count - i
										case 1:
											key = i
										case 2:
											key = i % 5
										case 4:
											key = 1
										}
										want[i] = [2]int{key, i}
									}
									slices.SortStableFunc(want, func(a, b [2]int) int {
										if descending != 0 {
											return b[0] - a[0]
										}
										return a[0] - b[0]
									})
									result, err := simexec.Execute(nodes, root, simexec.Request{Memory: map[int][]float64{4001: {float64(count), float64(pattern), float64(descending), float64(persistent)}, 10000: temporary}, StepLimit: 10_000_000})
									if err != nil {
										t.Fatal(err)
									}
									if len(result.Effects) != 2*count+1 {
										t.Fatalf("effects: got %d, want %d", len(result.Effects), 2*count+1)
									}
									for i, item := range want {
										for j, value := range item {
											effect := result.Effects[2*i+j]
											if effect.Function != resource.RuntimeFunctionDebugLog || len(effect.Arguments) != 1 || effect.Arguments[0] != float64(value) {
												t.Fatalf("item %d slot %d: got %v want %d", i, j, effect, value)
											}
										}
									}
									comparisons := result.Effects[2*count].Arguments[0]
									if count == 257 && pattern == 0 && descending == 0 && persistent == 0 {
										t.Logf("comparisons=%g runtime node steps=%d", comparisons, result.Steps)
									}
									bound := 8 * float64(count) * math.Ceil(math.Log2(float64(max(2, count))))
									if comparisons > bound {
										t.Fatalf("comparisons %g exceed n log n envelope %g", comparisons, bound)
									}
								})
							}
						}
					}
				}
			}
		})
	}
}

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

func TestCallValuesPreserveGoSemantics(t *testing.T) {
	combined := []float64{2, 8, 8, 8, 8, 8, 8, 8, 8, 4, 4, 2, 1, 8, 10, 10, 10, 10, 10, 324, 324, 324, 10}
	elements := make([]float64, 0, 96)
	for range 2 {
		for i := range 24 {
			elements = append(elements, float64(i+1), 1)
		}
	}
	wants := map[string][]float64{
		"Identity":  {2, 2, 1, 2, 9, 2, 3},
		"Parameter": {324}, "Return": {324}, "Direct": {324}, "Skipped": {1},
		"Scalars":    append(append([]float64(nil), combined[:14]...), 8, 8, 8),
		"Combined20": combined[:20], "Combined23": combined, "Elements": elements,
	}
	for _, checks := range []RuntimeChecks{RuntimeChecksNone, RuntimeChecksTerminate} {
		for _, level := range []optimize.Level{optimize.LevelMinimal, optimize.LevelFast, optimize.LevelStandard} {
			t.Run(fmt.Sprintf("checks%d/%s", checks, level), func(t *testing.T) {
				artifacts, err := NewCompiler(Options{Optimization: level, RuntimeChecks: checks}, "./testdata/callvalues").Compile(mode.ModePlay, mode.ModeWatch)
				if err != nil {
					t.Fatal(err)
				}
				for _, current := range []mode.Mode{mode.ModePlay, mode.ModeWatch} {
					var nodes []resource.EngineDataNode
					roots := map[string]int{}
					if current == mode.ModePlay {
						nodes = artifacts.Play.Nodes
						for _, archetype := range artifacts.Play.Archetypes {
							roots[string(archetype.Name)] = archetype.Preprocess.Index
						}
					} else {
						nodes = artifacts.Watch.Nodes
						for _, archetype := range artifacts.Watch.Archetypes {
							roots[string(archetype.Name)] = archetype.Preprocess.Index
						}
					}
					for _, name := range []string{"Parameter", "Return", "Direct", "Skipped", "Scalars", "Combined20", "Combined23", "Elements", "Identity"} {
						t.Run(string(current)+"/"+name, func(t *testing.T) {
							root, ok := roots[name]
							if !ok {
								t.Fatalf("missing archetype %s", name)
							}
							result, err := simexec.Execute(nodes, root, simexec.Request{Memory: map[int][]float64{4001: {1}}, StepLimit: 100000})
							if err != nil {
								t.Fatal(err)
							}
							var got []float64
							for _, effect := range result.Effects {
								if effect.Function == resource.RuntimeFunctionDebugLog {
									got = append(got, effect.Arguments...)
								}
							}
							if !reflect.DeepEqual(got, wants[name]) {
								t.Fatalf("Go values: want %v, got %v", wants[name], got)
							}
							if len(result.Effects) != len(got) {
								t.Fatalf("unexpected effects: %v", result.Effects)
							}
						})
					}
				}
			})
		}
	}
}

func TestFrozenFieldAddressesPreserveGoSemantics(t *testing.T) {
	want := []float64{21, 22, 23, 21, 22, 23, 21, 22, 23, 21, 22, 23, 21, 22, 23, 61, 62, 63}
	var host model.State
	host.Init()
	host.Exercise()
	hostValues := host.Values()
	if !reflect.DeepEqual(hostValues[:], want) {
		t.Fatalf("host Go: want %v, got %v", want, hostValues)
	}
	type addressCase struct {
		name        string
		input, want []float64
	}
	cases := []addressCase{}
	for _, name := range []string{"Memory", "Local", "Static", "ReadMemory"} {
		cases = append(cases, addressCase{name, []float64{1}, want})
	}
	host.Init()
	observed := host.Observe()
	if !reflect.DeepEqual(observed[:], want) {
		t.Fatal("host Observe", observed)
	}
	host.Init()
	direct := host.ReadDirect()
	if direct != [3]float64{21, 22, 23} {
		t.Fatal("host ReadDirect", direct)
	}
	cases = append(cases, addressCase{"ReadDirect", nil, []float64{21, 22, 23}})
	initial := []float64{11, 12, 13, 21, 22, 23, 31, 32, 33, 41, 42, 43, 51, 52, 53, 61, 62, 63}
	once := []float64{11, 12, 13, 21, 22, 23, 21, 22, 23, 11, 12, 13, 21, 22, 23, 61, 62, 63}
	host.Init()
	host.Once()
	hostValues = host.Values()
	if !reflect.DeepEqual(hostValues[:], once) {
		t.Fatal("host Once", hostValues)
	}
	cases = append(cases, addressCase{"Once", nil, once})
	for _, enter := range []int{0, 1} {
		branchWant := initial
		if enter != 0 {
			branchWant = want
		}
		host.Init()
		host.Branch(enter != 0)
		hostValues = host.Values()
		if !reflect.DeepEqual(hostValues[:], branchWant) {
			t.Fatal("host Branch", hostValues)
		}
		cases = append(cases, addressCase{"Branch", []float64{float64(enter)}, branchWant})
		for _, index := range []int{0, 1} {
			aliasWant := append(append([]float64{}, initial...), initial...)
			copy(aliasWant[18:24], []float64{101, 102, 103, 111, 112, 113})
			base := index * 18
			aliasWant[base+2] = 72
			if enter != 0 {
				left := []float64{111, 112, 114}
				if index == 1 {
					left = []float64{21, 22, 24}
				}
				copy(aliasWant[base:base+3], left)
			}
			copy(aliasWant[base+6:base+9], []float64{81, 72, 35})
			aliasWant = append(aliasWant, float64(1-index))
			var pair model.Pair
			pair.Init()
			alias := pair.Alias(index, enter != 0)
			if !reflect.DeepEqual(alias[:], aliasWant) {
				t.Fatal("host Alias", index, enter, alias)
			}
			for _, name := range []string{"MemoryAlias", "LocalAlias"} {
				cases = append(cases, addressCase{name, []float64{float64(index), float64(enter)}, aliasWant})
			}
		}
	}
	conversion := model.Conversion(1.75)
	if conversion != [2]float64{1.75, 1} {
		t.Fatal("host Conversion", conversion)
	}
	cases = append(cases, addressCase{"Conversion", []float64{1.75}, []float64{1.75, 1}})
	for _, checks := range []RuntimeChecks{RuntimeChecksNone, RuntimeChecksTerminate} {
		for _, level := range []optimize.Level{optimize.LevelMinimal, optimize.LevelFast, optimize.LevelStandard} {
			t.Run(fmt.Sprintf("checks%d/%s", checks, level), func(t *testing.T) {
				artifacts, err := NewCompiler(Options{Optimization: level, RuntimeChecks: checks}, "./testdata/freezeaddress").Compile(mode.ModePlay, mode.ModeWatch)
				if err != nil {
					t.Fatal(err)
				}
				for _, current := range []mode.Mode{mode.ModePlay, mode.ModeWatch} {
					var nodes []resource.EngineDataNode
					roots := map[string]int{}
					if current == mode.ModePlay {
						nodes = artifacts.Play.Nodes
						for _, a := range artifacts.Play.Archetypes {
							roots[string(a.Name)] = a.Preprocess.Index
						}
					} else {
						nodes = artifacts.Watch.Nodes
						for _, a := range artifacts.Watch.Archetypes {
							roots[string(a.Name)] = a.Preprocess.Index
						}
					}
					for _, tc := range cases {
						t.Run(fmt.Sprintf("%s/%s/%v", current, tc.name, tc.input), func(t *testing.T) {
							root, ok := roots[tc.name]
							if !ok {
								t.Fatal("missing archetype", tc.name)
							}
							result, err := simexec.Execute(nodes, root, simexec.Request{Memory: map[int][]float64{4001: tc.input}, StepLimit: 100000})
							if err != nil {
								t.Fatal(err)
							}
							var got []float64
							for _, effect := range result.Effects {
								if effect.Function == resource.RuntimeFunctionDebugLog {
									got = append(got, effect.Arguments...)
								}
							}
							if !reflect.DeepEqual(got, tc.want) {
								t.Fatalf("Go values: want %v, got %v", tc.want, got)
							}
						})
					}
				}
			})
		}
	}
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

func TestOptimizationTierCapacityAndRuntime(t *testing.T) {
	levels := []optimize.Level{optimize.LevelMinimal, optimize.LevelFast, optimize.LevelStandard}
	t.Run("capacity", func(t *testing.T) {
		scalarEvents := make([]int, 0, 8194)
		for id := 1; id <= 4097; id++ {
			scalarEvents = append(scalarEvents, id, -id)
		}
		for _, tc := range []struct {
			name       string
			width      int
			events     []int  // positive stores, negative reads; IDs are one-based
			wantSlots  [3]int // -1 requires rejection; -2 permits conservative rejection or improved allocation
			initialize bool
		}{
			{"exact-limit", 2048, []int{1, 2, -1, -2}, [3]int{4096, 4096, 4096}, false},
			{"scalar-reuse", 1, scalarEvents, [3]int{-1, 1, 1}, false},
			{"aggregate-reuse-single-store", 2048, []int{1, -1, 2, -2, 3, -3}, [3]int{-1, 2048, 2048}, false},
			{"aggregate-chain", 2048, []int{1, 2, -1, 3, -2, -3}, [3]int{-1, 4096, 4096}, false},
			{"live-overflow", 2049, []int{1, 2, -1, -2}, [3]int{-1, -1, -1}, false},
			{"initialized-aggregate-reuse", 2048, []int{1, -1, 2, -2, 3, -3}, [3]int{-1, 2048, 2048}, true},
			{"initialized-aggregate-live-overflow", 2049, []int{1, 2, -1, -2}, [3]int{-1, -1, -1}, true},
		} {
			t.Run(tc.name, func(t *testing.T) {
				builder := ir.NewBuilder(tc.name, ir.Type{})
				entry := builder.NewBlock()
				_ = builder.SetEntry(entry)
				_ = builder.SetCurrent(entry)
				input, err := builder.Memory("memory", ir.Const{}, 1, 0, true, false)
				if err != nil {
					t.Fatal(err)
				}
				places := map[int]ir.Place{}
				var want []float64
				for _, event := range tc.events {
					if event > 0 {
						local := builder.NewLocal(fmt.Sprintf("array%d", event), ir.Type{Name: "array", Slots: tc.width})
						if tc.initialize {
							values := make([]ir.Expr, tc.width)
							for index := range values {
								values[index] = ir.Const{Value: float64(event * 10)}
							}
							if err := builder.Store(ir.Places(local), ir.Value{Type: local.Type, Slots: values}, ir.SourcePos{}); err != nil {
								t.Fatal(err)
							}
						}
						var place ir.Place = local.Slots[0].(ir.Load).Place
						if tc.width > 1 {
							place, err = builder.IndexedLocal(place.(ir.LocalPlace), ir.Load{Place: input}, tc.width, 1, 0)
							if err != nil {
								t.Fatal(err)
							}
						}
						places[event] = place
						if err := builder.Store([]ir.Place{place}, ir.Value{Type: ir.Type{Name: "number", Slots: 1}, Slots: []ir.Expr{ir.Const{Value: float64(event * 10)}}}, ir.SourcePos{}); err != nil {
							t.Fatal(err)
						}
					} else {
						want = append(want, float64(-event*10))
						if err := builder.Eval(builder.RuntimeCall(resource.RuntimeFunctionDebugLog, []ir.Expr{ir.Load{Place: places[-event]}}, ir.Type{}, false, ir.SourcePos{})); err != nil {
							t.Fatal(err)
						}
					}
				}
				_ = builder.Return(ir.Value{Type: ir.Type{}})
				fn, err := builder.Finish()
				if err != nil {
					t.Fatal(err)
				}
				for index, level := range levels {
					t.Run(level.String(), func(t *testing.T) {
						optimized, err := optimize.NewOptimizer(level).Optimize(optimize.Context{Mode: mode.ModePlay, Callback: "preprocess"}, fn)
						if tc.wantSlots[index] == -1 || tc.wantSlots[index] == -2 && err != nil {
							if err == nil || !strings.Contains(err.Error(), "4096") {
								t.Fatalf("expected capacity rejection, got %v", err)
							}
							t.Logf("capacity rejection: %v", err)
							return
						}
						if err != nil {
							t.Fatal(err)
						}
						slots := 0
						for _, local := range optimized.Locals {
							slots += local.Slots
						}
						if slots > optimize.TemporaryMemorySlots || tc.wantSlots[index] >= 0 && slots != tc.wantSlots[index] {
							t.Fatalf("slots=%d want=%d", slots, tc.wantSlots[index])
						}
						project := &frontend.Project{Modes: map[mode.Mode]*frontend.ModeDeclarations{mode.ModePlay: {Mode: mode.ModePlay, Archetypes: []*frontend.ArchetypeDeclaration{{Name: "Capacity", Callbacks: []*frontend.CallbackDeclaration{{Name: "preprocess", IR: optimized}}}}}}}
						artifacts, err := backend.Compile(project)
						if err != nil {
							t.Fatal(err)
						}
						seeds := []float64{0}
						if tc.width > 1 {
							seeds = append(seeds, 1, float64(tc.width-1))
						}
						for _, seed := range seeds {
							result, err := simexec.Execute(artifacts.Play.Nodes, artifacts.Play.Archetypes[0].Preprocess.Index, simexec.Request{Memory: map[int][]float64{4000: {seed}}, StepLimit: 100000})
							if err != nil {
								t.Fatal(err)
							}
							var got []float64
							for _, effect := range result.Effects {
								got = append(got, effect.Arguments...)
							}
							if !reflect.DeepEqual(got, want) {
								t.Fatalf("seed=%g got=%v want=%v", seed, got, want)
							}
							t.Logf("slots=%d nodes=%d seed=%g steps=%d", slots, len(artifacts.Play.Nodes), seed, result.Steps)
						}
					})
				}
			})
		}
	})
	t.Run("dynamic-loop", func(t *testing.T) {
		for _, level := range levels {
			t.Run(level.String(), func(t *testing.T) {
				artifacts, err := NewCompiler(Options{Optimization: level}, "./testdata/fuzzsemantics").Compile(mode.ModePlay)
				if err != nil {
					t.Fatal(err)
				}
				for _, n := range []int{-1, 0, 1, 8, 32, 127} {
					seed := float64(n)
					result, err := simexec.Execute(artifacts.Play.Nodes, artifacts.Play.Archetypes[0].Preprocess.Index, simexec.Request{DefaultMemory: &seed, StepLimit: 100000})
					if err != nil {
						t.Fatal(err)
					}
					want := 9.0
					if n > 0 {
						want = 11 + float64(n*(n-1))/2
					}
					if len(result.Effects) != 1 || result.Effects[0].Function != resource.RuntimeFunctionDebugLog || !reflect.DeepEqual(result.Effects[0].Arguments, []float64{want}) {
						t.Fatalf("n=%d effects=%v want=%g", n, result.Effects, want)
					}
					// The pinned Py pipeline executes this same input CFG in 4566
					// steps. Guard this measured loop regression, not a universal
					// ordering of the three optimization levels.
					if level == optimize.LevelStandard && n == 127 && result.Steps > 4566 {
						t.Fatalf("loop cost regressed: %d steps exceeds pinned Py budget 4566", result.Steps)
					}
					t.Logf("n=%d nodes=%d steps=%d", n, len(artifacts.Play.Nodes), result.Steps)
				}
			})
		}
	})
}

func BenchmarkCompileAll(b *testing.B) {
	corpora := []struct {
		name    string
		pattern string
		modes   []mode.Mode
	}{
		{name: "reference", pattern: "./testdata/reference", modes: orderedModes},
		{name: "callvalues-playwatch", pattern: "./testdata/callvalues", modes: []mode.Mode{mode.ModePlay, mode.ModeWatch}},
		{name: "godori", pattern: "../../godori", modes: orderedModes},
	}
	levels := []optimize.Level{optimize.LevelMinimal, optimize.LevelFast, optimize.LevelStandard}
	for _, corpus := range corpora {
		for _, level := range levels {
			b.Run(corpus.name+"/"+level.String(), func(b *testing.B) {
				var nodes int
				b.ResetTimer()
				for range b.N {
					artifacts, err := NewCompiler(Options{Optimization: level}, corpus.pattern).Compile(corpus.modes...)
					if err != nil {
						b.Fatal(err)
					}
					nodes = len(artifacts.Play.Nodes) + len(artifacts.Watch.Nodes)
					if artifacts.Preview != nil {
						nodes += len(artifacts.Preview.Nodes)
					}
					if artifacts.Tutorial != nil {
						nodes += len(artifacts.Tutorial.Nodes)
					}
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
			for _, level := range []optimize.Level{optimize.LevelMinimal, optimize.LevelFast, optimize.LevelStandard} {
				b.Run("optimize/"+level.String(), func(b *testing.B) {
					optimizer := optimize.NewOptimizer(level)
					for range b.N {
						result, optimizeErr := optimizeProject(optimizer, project)
						if optimizeErr != nil {
							b.Fatal(optimizeErr)
						}
						peak := 0
						for _, declarations := range result.Modes {
							callbacks := append([]*frontend.CallbackDeclaration(nil), declarations.Globals...)
							for _, archetype := range declarations.Archetypes {
								callbacks = append(callbacks, archetype.Callbacks...)
							}
							for _, callback := range callbacks {
								slots := 0
								for _, local := range callback.IR.Locals {
									slots += local.Slots
								}
								peak = max(peak, slots)
							}
						}
						b.ReportMetric(float64(peak), "peak-slots/op")
					}
				})
			}
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
