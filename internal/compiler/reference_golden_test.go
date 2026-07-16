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
	path := filepath.Join("backend", "testdata", "reference.golden.json")
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
