package optimize

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/WindowsSov8forUs/sonolus-core-go/core/resource"
	"github.com/WindowsSov8forUs/sonolus-go/v2/internal/compiler/backend"
	"github.com/WindowsSov8forUs/sonolus-go/v2/internal/compiler/frontend"
	"github.com/WindowsSov8forUs/sonolus-go/v2/internal/compiler/ir"
	"github.com/WindowsSov8forUs/sonolus-go/v2/internal/compiler/mode"
	"github.com/WindowsSov8forUs/sonolus-go/v2/internal/simexec"
)

type pythonPassGolden struct {
	SchemaVersion int                           `json:"schemaVersion"`
	PythonCommit  string                        `json:"pythonCommit"`
	SSACases      map[string]string             `json:"ssaCases"`
	SCCPCases     map[string]string             `json:"sccpCases"`
	FromSSACases  map[string]string             `json:"fromSSACases"`
	PipelineCases map[string]pythonPipelineCase `json:"pipelineCases"`
}

type pythonDifference struct {
	Case, Checkpoint, Path, Go, Python, Reason string
}

func pythonDifferenceReason(difference pythonDifference) string {
	if difference.Checkpoint == "standard" {
		switch difference.Case {
		case "allocation_4095", "allocation_4096":
			return "Go removes the final temporary copy after allocation; the returned semantic-memory value and 4096-slot boundary are identical"
		case "linear_constant", "diamond_constant":
			return "Go folds constant control flow before backend emission; the fixed execution matrix verifies the same result and effects"
		case "diamond_memory", "switch_chain":
			return "Go materializes the runtime discriminant and orders equivalent CFG exits differently; the fixed execution matrix covers every branch"
		case "loop_memory", "readonly_cse_loop":
			return "Go emits an explicit loop state machine while Py retains a compact loop CFG; fixed inputs compare return value, semantic memory, and effects"
		case "impure_effect":
			return "Go emits an explicit callback break after the impure effect; effect order and callback result are identical"
		}
	}
	if strings.Contains(difference.Path, "/version") {
		return "Go and Py assign normalized SSA versions in different definition traversal order; Phi/data dependencies and final semantics are compared independently"
	}
	if strings.Contains(difference.Path, "/terminator/edges/length") || strings.Contains(difference.Path, "/terminator/value/kind") {
		return "Go folds the loop entry condition one cleanup checkpoint earlier; subsequent checkpoints and fixed-input execution verify the same reachable edges"
	}
	if strings.Contains(difference.Path, "/terminator/value/arguments/0/kind") {
		return "Go keeps the loop increment expression inline in the condition while Py loads an equivalent SSA temporary; later allocation and execution verify equivalence"
	}
	if strings.Contains(difference.Path, "/blocks/length") {
		return "Go and Py use different critical-edge and parallel-copy block shapes during FromSSA; Allocate and final execution verify equivalence"
	}
	if strings.Contains(difference.Path, "/phis/length") || strings.Contains(difference.Path, "/instructions/length") {
		return "Go retains a different set of explicit loop/copy temporaries at this checkpoint; final allocation and semantic execution verify equivalent observable state"
	}
	if strings.Contains(difference.Path, "/place/kind") || strings.Contains(difference.Path, "/place/storage") {
		return "Go promotes or materializes this scalar through a temporary where Py accesses semantic memory directly; final allocation and execution verify equivalence"
	}
	if difference.Case == "switch_chain" {
		return "Go and Py normalize equivalent switch case block ordering differently; all switch inputs are compared by final execution"
	}
	return "The normalized CFG leaf differs while the following checkpoints and fixed execution matrix verify identical observable semantics"
}

type pipelineFixture struct {
	SchemaVersion int                   `json:"schemaVersion"`
	Cases         []pipelineFixtureCase `json:"cases"`
}

type pipelineFixtureCase struct {
	Name                string                 `json:"name"`
	Locals              []pipelineFixtureLocal `json:"locals"`
	Inputs              []float64              `json:"inputs"`
	ExpectAllocateError bool                   `json:"expectAllocateError"`
	Blocks              []struct {
		Statements [][]json.RawMessage `json:"statements"`
		Terminator []json.RawMessage   `json:"terminator"`
	} `json:"blocks"`
}

type pipelineFixtureLocal struct {
	Name string
	Size int
}

func (local *pipelineFixtureLocal) UnmarshalJSON(data []byte) error {
	var name string
	if err := json.Unmarshal(data, &name); err == nil {
		local.Name, local.Size = name, 1
		return nil
	}
	var value struct {
		Name string `json:"name"`
		Size int    `json:"size"`
	}
	if err := json.Unmarshal(data, &value); err != nil {
		return err
	}
	if value.Name == "" || value.Size <= 0 {
		return fmt.Errorf("invalid pipeline local %q with size %d", value.Name, value.Size)
	}
	local.Name, local.Size = value.Name, value.Size
	return nil
}

func loadPipelineFixture(t *testing.T) pipelineFixture {
	t.Helper()
	data, err := os.ReadFile("../testdata/optimize/pipeline_fixture.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixture pipelineFixture
	if err := json.Unmarshal(data, &fixture); err != nil {
		t.Fatal(err)
	}
	if fixture.SchemaVersion != 2 {
		t.Fatalf("pipeline fixture schema = %d, want 2", fixture.SchemaVersion)
	}
	return fixture
}

func pipelineFixtureBuilders(t *testing.T) (map[string]func() *ir.Function, map[string][]float64) {
	t.Helper()
	builders := map[string]func() *ir.Function{}
	inputs := map[string][]float64{}
	for _, raw := range loadPipelineFixture(t).Cases {
		item := raw
		builders[item.Name] = func() *ir.Function { return buildPipelineFixture(item) }
		inputs[item.Name] = append([]float64(nil), item.Inputs...)
	}
	return builders, inputs
}

func buildPipelineFixture(item pipelineFixtureCase) *ir.Function {
	locals := make([]ir.Type, len(item.Locals))
	places := map[string]ir.LocalPlace{}
	for index, local := range item.Locals {
		locals[index], places[local.Name] = ir.Type{Name: local.Name, Slots: local.Size}, pipelineLocal(local.Name, index)
	}
	var expression func(json.RawMessage) ir.Expr
	expression = func(raw json.RawMessage) ir.Expr {
		var parts []json.RawMessage
		if err := json.Unmarshal(raw, &parts); err != nil {
			panic(err)
		}
		var operation string
		if err := json.Unmarshal(parts[0], &operation); err != nil {
			panic(err)
		}
		switch operation {
		case "const":
			var value float64
			if err := json.Unmarshal(parts[1], &value); err != nil {
				panic(err)
			}
			return ir.Const{Value: value}
		case "local":
			var name string
			if err := json.Unmarshal(parts[1], &name); err != nil {
				panic(err)
			}
			place := places[name]
			if len(parts) == 3 {
				if err := json.Unmarshal(parts[2], &place.Offset); err != nil {
					panic(err)
				}
			}
			return ir.Load{Place: place}
		case "memory":
			var index int
			if err := json.Unmarshal(parts[1], &index); err != nil {
				panic(err)
			}
			return ir.Load{Place: pipelineMemory(index)}
		case "memoryAt":
			var storage string
			var index int
			if err := json.Unmarshal(parts[1], &storage); err != nil {
				panic(err)
			}
			if err := json.Unmarshal(parts[2], &index); err != nil {
				panic(err)
			}
			return ir.Load{Place: ir.MemoryPlace{Storage: storage, Index: ir.Const{Value: float64(index)}, Read: true, Write: storage != "EngineRom"}}
		case "call":
			var name string
			if err := json.Unmarshal(parts[1], &name); err != nil {
				panic(err)
			}
			arguments := make([]ir.Expr, len(parts)-2)
			for index, argument := range parts[2:] {
				arguments[index] = expression(argument)
			}
			return pipelineCall(resource.RuntimeFunction(name), arguments...)
		case "effectCall":
			var name string
			if err := json.Unmarshal(parts[1], &name); err != nil {
				panic(err)
			}
			arguments := make([]ir.Expr, len(parts)-2)
			for index, argument := range parts[2:] {
				arguments[index] = expression(argument)
			}
			return ir.RuntimeCall{Function: resource.RuntimeFunction(name), Args: arguments, Result: ir.Type{}, Pure: false}
		default:
			panic("unknown pipeline expression " + operation)
		}
	}
	blocks := make([]*ir.Block, len(item.Blocks))
	for index := range blocks {
		blocks[index] = &ir.Block{ID: index}
	}
	for index, spec := range item.Blocks {
		for _, raw := range spec.Statements {
			var operation string
			if err := json.Unmarshal(raw[0], &operation); err != nil {
				panic(err)
			}
			switch operation {
			case "setLocal":
				var name string
				if err := json.Unmarshal(raw[1], &name); err != nil {
					panic(err)
				}
				place := places[name]
				valueIndex := 2
				if len(raw) == 4 {
					if err := json.Unmarshal(raw[2], &place.Offset); err != nil {
						panic(err)
					}
					valueIndex = 3
				}
				blocks[index].Instructions = append(blocks[index].Instructions, ir.Store{Place: place, Value: expression(raw[valueIndex])})
			case "setMemory":
				var memoryIndex int
				if err := json.Unmarshal(raw[1], &memoryIndex); err != nil {
					panic(err)
				}
				blocks[index].Instructions = append(blocks[index].Instructions, pipelineStore(memoryIndex, expression(raw[2])))
			case "eval":
				blocks[index].Instructions = append(blocks[index].Instructions, ir.Eval{Value: expression(raw[1]).(ir.RuntimeCall)})
			default:
				panic("unknown pipeline statement " + operation)
			}
		}
		var terminator string
		if err := json.Unmarshal(spec.Terminator[0], &terminator); err != nil {
			panic(err)
		}
		switch terminator {
		case "return":
			blocks[index].Terminator = emptyReturn()
		case "jump":
			var target int
			if err := json.Unmarshal(spec.Terminator[1], &target); err != nil {
				panic(err)
			}
			blocks[index].Terminator = ir.Jump{Target: target}
		case "branch":
			var whenTrue, whenFalse int
			if err := json.Unmarshal(spec.Terminator[2], &whenTrue); err != nil {
				panic(err)
			}
			if err := json.Unmarshal(spec.Terminator[3], &whenFalse); err != nil {
				panic(err)
			}
			blocks[index].Terminator = ir.Branch{Condition: expression(spec.Terminator[1]), True: whenTrue, False: whenFalse}
		default:
			panic("unknown pipeline terminator " + terminator)
		}
	}
	return &ir.Function{Name: item.Name, Entry: 0, Locals: locals, Blocks: blocks}
}

type pythonPipelineCase struct {
	Checkpoints           map[string]string          `json:"checkpoints"`
	StructuredCheckpoints map[string]json.RawMessage `json:"structuredCheckpoints"`
	NodeCount             int                        `json:"nodeCount"`
	Nodes                 string                     `json:"nodes"`
	AllocateError         string                     `json:"allocateError"`
}

func loadPythonPassGolden(t *testing.T) pythonPassGolden {
	t.Helper()
	data, err := os.ReadFile("../testdata/optimize/py_pass_golden.json")
	if err != nil {
		t.Fatal(err)
	}
	var golden pythonPassGolden
	if err := json.Unmarshal(data, &golden); err != nil {
		t.Fatal(err)
	}
	if golden.SchemaVersion != 4 {
		t.Fatalf("Python pass golden schema = %d, want 4", golden.SchemaVersion)
	}
	const pythonCommit = "1040bc0dcc116efdbca05f144edec302e839bcd3"
	if golden.PythonCommit != pythonCommit {
		t.Fatalf("Python pass golden commit = %q, want %q", golden.PythonCommit, pythonCommit)
	}
	return golden
}

func loadPythonDifferenceAllowlist(t *testing.T) []pythonDifference {
	t.Helper()
	data, err := os.ReadFile("../testdata/optimize/py_pass_allowlist.json")
	if err != nil {
		t.Fatal(err)
	}
	var differences []pythonDifference
	if err := json.Unmarshal(data, &differences); err != nil {
		t.Fatal(err)
	}
	for index, difference := range differences {
		if difference.Case == "" || difference.Checkpoint == "" || difference.Path == "" || difference.Reason == "" {
			t.Fatalf("allowlist entry %d is incomplete: %#v", index, difference)
		}
	}
	return differences
}

func TestPinnedPythonStandardPipelineSnapshotIsComplete(t *testing.T) {
	golden := loadPythonPassGolden(t)
	wantCases := make([]string, len(loadPipelineFixture(t).Cases))
	for index, item := range loadPipelineFixture(t).Cases {
		wantCases[index] = item.Name
	}
	sort.Strings(wantCases)
	wantCheckpoints := []string{"allocate", "firstSCCPCleanup", "fromSSA", "secondSCCP", "toSSA"}
	if got := sortedKeys(golden.PipelineCases); !equalStrings(got, wantCases) {
		t.Fatalf("pipeline cases = %v, want %v", got, wantCases)
	}
	for _, name := range wantCases {
		item := golden.PipelineCases[name]
		fixtureCase := pipelineFixtureCaseByName(t, name)
		expectedCheckpoints := wantCheckpoints
		if fixtureCase.ExpectAllocateError {
			expectedCheckpoints = []string{"firstSCCPCleanup", "fromSSA", "secondSCCP", "toSSA"}
		}
		if got := sortedKeys(item.Checkpoints); !equalStrings(got, expectedCheckpoints) {
			t.Errorf("%s checkpoints = %v, want %v", name, got, wantCheckpoints)
		}
		for _, checkpoint := range expectedCheckpoints {
			if item.Checkpoints[checkpoint] == "" {
				t.Errorf("%s/%s snapshot is empty", name, checkpoint)
			}
			var structured struct {
				Entry  int               `json:"entry"`
				Blocks []json.RawMessage `json:"blocks"`
			}
			if err := json.Unmarshal(item.StructuredCheckpoints[checkpoint], &structured); err != nil || len(structured.Blocks) == 0 {
				t.Errorf("%s/%s structured snapshot is invalid: blocks=%d err=%v", name, checkpoint, len(structured.Blocks), err)
			}
		}
		if fixtureCase.ExpectAllocateError {
			if item.AllocateError != "temporary-memory-overflow" {
				t.Errorf("%s allocation error = %q", name, item.AllocateError)
			}
			continue
		}
		if item.NodeCount <= 0 || item.Nodes == "" {
			t.Errorf("%s final Standard output is incomplete: count=%d nodes=%q", name, item.NodeCount, item.Nodes)
		}
		if allocated := item.Checkpoints["allocate"]; strings.Contains(allocated, "phi(") || strings.Contains(allocated, "S(") || strings.Contains(allocated, "T(") {
			t.Errorf("%s allocation snapshot retained virtual state: %s", name, allocated)
		}
	}
}

func pipelineFixtureCaseByName(t *testing.T, name string) pipelineFixtureCase {
	t.Helper()
	for _, item := range loadPipelineFixture(t).Cases {
		if item.Name == name {
			return item
		}
	}
	t.Fatalf("pipeline fixture case %q is missing", name)
	return pipelineFixtureCase{}
}

func TestPinnedPythonStandardPipelineCheckpoints(t *testing.T) {
	golden := loadPythonPassGolden(t)
	allowlist := loadPythonDifferenceAllowlist(t)
	used := make([]bool, len(allowlist))
	var unknown []pythonDifference
	var current []pythonDifference
	checkpoints := []struct {
		name  string
		count int
	}{
		{"toSSA", 5}, {"firstSCCPCleanup", 9}, {"secondSCCP", 15}, {"fromSSA", 31}, {"allocate", 38},
	}
	builders, _ := pipelineFixtureBuilders(t)
	for _, caseName := range sortedKeys(builders) {
		if pipelineFixtureCaseByName(t, caseName).ExpectAllocateError {
			continue
		}
		for _, checkpoint := range checkpoints {
			function := builders[caseName]()
			if err := runStandardPrefix(function, checkpoint.count); err != nil {
				t.Fatalf("%s/%s: %v", caseName, checkpoint.name, err)
			}
			got, err := parityPipelineStructured(function)
			if err != nil {
				t.Fatalf("%s/%s structured snapshot: %v", caseName, checkpoint.name, err)
			}
			path := fmt.Sprintf("/pipelineCases/%s/structuredCheckpoints/%s", caseName, checkpoint.name)
			for _, difference := range structuredJSONDifferences(caseName, checkpoint.name, path, got, golden.PipelineCases[caseName].StructuredCheckpoints[checkpoint.name]) {
				matched := -1
				for index, allowed := range allowlist {
					if allowed.Case == difference.Case && allowed.Checkpoint == difference.Checkpoint && allowed.Path == difference.Path && allowed.Go == difference.Go && allowed.Python == difference.Python {
						matched = index
						break
					}
				}
				if matched < 0 {
					difference.Reason = pythonDifferenceReason(difference)
					unknown = append(unknown, difference)
				} else {
					used[matched] = true
					if os.Getenv("SONOLUS_UPDATE_PY_ALLOWLIST") == "1" {
						difference.Reason = pythonDifferenceReason(difference)
					} else {
						difference.Reason = allowlist[matched].Reason
					}
				}
				current = append(current, difference)
			}
		}
	}
	if os.Getenv("SONOLUS_UPDATE_PY_ALLOWLIST") == "1" {
		kept := make([]pythonDifference, 0, len(allowlist)+len(current))
		for _, difference := range allowlist {
			if difference.Checkpoint == "standard" {
				kept = append(kept, difference)
			}
		}
		kept = append(kept, current...)
		data, err := json.MarshalIndent(kept, "", "  ")
		if err != nil {
			t.Fatal(err)
		}
		data = append(data, '\n')
		if err := os.WriteFile("../testdata/optimize/py_pass_allowlist.json", data, 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	for _, difference := range unknown {
		t.Errorf("unknown Go/Python difference at %s\nGo:     %s\nPython: %s", difference.Path, difference.Go, difference.Python)
	}
	for index, wasUsed := range used {
		if allowlist[index].Checkpoint != "standard" && !wasUsed {
			t.Errorf("unused Py difference allowlist entry %d: %#v", index, allowlist[index])
		}
	}
}

func parityPipelineStructured(function *ir.Function) (json.RawMessage, error) {
	order := pipelineReversePostorder(function)
	blockIDs := make(map[int]int, len(order))
	for index, id := range order {
		blockIDs[id] = index
	}
	ssaNames := map[int]string{}
	localNames := map[int]string{}
	versions := map[string]int{}
	baseName := func(name string) string {
		if strings.HasPrefix(name, "local.") {
			parts := strings.Split(name, ".")
			if len(parts) >= 3 {
				var id int
				if _, err := fmt.Sscanf(parts[1], "%d", &id); err == nil && id >= 0 && id < len(function.Locals) && function.Locals[id].Name != "" {
					return function.Locals[id].Name
				}
			}
		}
		if name == "" {
			return "tmp"
		}
		return name
	}
	named := func(items map[int]string, id int, name string) string {
		if existing := items[id]; existing != "" {
			return existing
		}
		base := baseName(name)
		versions[base]++
		items[id] = fmt.Sprintf("%s.%d", base, versions[base])
		return items[id]
	}
	var place func(ir.Place) any
	var expression func(ir.Expr) any
	place = func(raw ir.Place) any {
		switch value := raw.(type) {
		case ir.SSAPlace:
			name := named(ssaNames, value.ID, value.Name)
			separator := strings.LastIndexByte(name, '.')
			version, _ := strconv.Atoi(name[separator+1:])
			return map[string]any{"kind": "ssa", "name": name[:separator], "version": version}
		case ir.LocalPlace:
			if function.Allocated {
				return map[string]any{"kind": "memory", "storage": "10000", "index": 0, "offset": value.Offset}
			}
			return map[string]any{"kind": "local", "name": named(localNames, value.ID, value.Name), "index": 0, "offset": value.Offset}
		case ir.IndexedLocalPlace:
			return map[string]any{"kind": "local", "name": named(localNames, value.ID, value.Name), "index": expression(value.Index), "offset": value.Offset}
		case ir.MemoryPlace:
			storage := value.Storage
			if storage == "LevelMemory" {
				storage = "2000"
			} else if storage == "EngineRom" {
				storage = "3000"
			} else if storage == "TemporaryMemory" {
				storage = "10000"
			}
			index := expression(value.Index)
			if constant, ok := index.(map[string]any); ok && constant["kind"] == "const" {
				index = constant["value"]
			}
			return map[string]any{"kind": "memory", "storage": storage, "index": index, "offset": value.Offset}
		default:
			return map[string]any{"kind": fmt.Sprintf("%T", raw)}
		}
	}
	expression = func(raw ir.Expr) any {
		switch value := raw.(type) {
		case ir.Const:
			return map[string]any{"kind": "const", "value": value.Value}
		case ir.Load:
			return map[string]any{"kind": "load", "place": place(value.Place)}
		case ir.RuntimeCall:
			arguments := make([]any, len(value.Args))
			for index, argument := range value.Args {
				arguments[index] = expression(argument)
			}
			return map[string]any{"kind": "call", "function": string(value.Function), "arguments": arguments}
		default:
			return map[string]any{"kind": fmt.Sprintf("%T", raw)}
		}
	}
	blocks := make([]any, 0, len(order))
	for normalized, id := range order {
		block := function.Blocks[id]
		phis := make([]any, len(block.Phis))
		for index, phi := range block.Phis {
			arguments := append([]ir.PhiArg(nil), phi.Args...)
			sort.Slice(arguments, func(i, j int) bool { return blockIDs[arguments[i].Predecessor] < blockIDs[arguments[j].Predecessor] })
			values := make([]any, len(arguments))
			for argumentIndex, argument := range arguments {
				values[argumentIndex] = map[string]any{"predecessor": blockIDs[argument.Predecessor], "value": place(argument.Value)}
			}
			phis[index] = map[string]any{"target": place(phi.Target), "arguments": values}
		}
		instructions := make([]any, len(block.Instructions))
		for index, instruction := range block.Instructions {
			switch value := instruction.(type) {
			case ir.Store:
				instructions[index] = map[string]any{"kind": "store", "place": place(value.Place), "value": expression(value.Value)}
			case ir.Eval:
				instructions[index] = map[string]any{"kind": "eval", "value": expression(value.Value)}
			default:
				instructions[index] = map[string]any{"kind": fmt.Sprintf("%T", instruction)}
			}
		}
		terminator := map[string]any{"kind": "return"}
		edges := []any{}
		switch value := block.Terminator.(type) {
		case ir.Jump:
			terminator = map[string]any{"kind": "switch", "value": expression(ir.Const{}), "edges": []any{map[string]any{"value": nil, "target": blockIDs[value.Target]}}}
		case ir.Branch:
			terminator = map[string]any{"kind": "switch", "value": expression(value.Condition), "edges": []any{
				map[string]any{"value": expression(ir.Const{}), "target": blockIDs[value.False]},
				map[string]any{"value": nil, "target": blockIDs[value.True]},
			}}
		case ir.Switch:
			for _, item := range value.Cases {
				edges = append(edges, map[string]any{"value": expression(ir.Const{Value: item.Value}), "target": blockIDs[item.Target]})
			}
			sort.Slice(edges, func(i, j int) bool {
				return edges[i].(map[string]any)["value"].(map[string]any)["value"].(float64) < edges[j].(map[string]any)["value"].(map[string]any)["value"].(float64)
			})
			edges = append(edges, map[string]any{"value": nil, "target": blockIDs[value.Default]})
			terminator = map[string]any{"kind": "switch", "value": expression(value.Value), "edges": edges}
		}
		blocks = append(blocks, map[string]any{"id": normalized, "phis": phis, "instructions": instructions, "terminator": terminator})
	}
	data, err := json.Marshal(map[string]any{"entry": 0, "blocks": blocks})
	return data, err
}

func structuredJSONDifferences(caseName, checkpoint, path string, goRaw, pythonRaw json.RawMessage) []pythonDifference {
	var goValue, pythonValue any
	if err := json.Unmarshal(goRaw, &goValue); err != nil {
		return []pythonDifference{{Case: caseName, Checkpoint: checkpoint, Path: path, Go: "<invalid: " + err.Error() + ">", Python: string(pythonRaw)}}
	}
	if err := json.Unmarshal(pythonRaw, &pythonValue); err != nil {
		return []pythonDifference{{Case: caseName, Checkpoint: checkpoint, Path: path, Go: string(goRaw), Python: "<invalid: " + err.Error() + ">"}}
	}
	var differences []pythonDifference
	type missingJSON struct{}
	var compare func(string, any, any)
	encode := func(value any) string {
		if _, missing := value.(missingJSON); missing {
			return `"<missing>"`
		}
		data, _ := json.Marshal(value)
		return string(data)
	}
	escape := func(value string) string { return strings.ReplaceAll(strings.ReplaceAll(value, "~", "~0"), "/", "~1") }
	compare = func(current string, goValue, pythonValue any) {
		_, goMissing := goValue.(missingJSON)
		_, pythonMissing := pythonValue.(missingJSON)
		goObject, goIsObject := goValue.(map[string]any)
		pythonObject, pythonIsObject := pythonValue.(map[string]any)
		if goMissing && pythonIsObject {
			differences = append(differences, pythonDifference{Case: caseName, Checkpoint: checkpoint, Path: current, Go: encode(goValue), Python: `"<object>"`})
			return
		}
		if pythonMissing && goIsObject {
			differences = append(differences, pythonDifference{Case: caseName, Checkpoint: checkpoint, Path: current, Go: `"<object>"`, Python: encode(pythonValue)})
			return
		}
		if goIsObject && pythonIsObject {
			goKind, goHasKind := goObject["kind"]
			pythonKind, pythonHasKind := pythonObject["kind"]
			if goHasKind && pythonHasKind && !reflect.DeepEqual(goKind, pythonKind) {
				differences = append(differences, pythonDifference{Case: caseName, Checkpoint: checkpoint, Path: current + "/kind", Go: encode(goKind), Python: encode(pythonKind)})
				return
			}
			keys := map[string]bool{}
			for key := range goObject {
				keys[key] = true
			}
			for key := range pythonObject {
				keys[key] = true
			}
			ordered := sortedKeys(keys)
			for _, key := range ordered {
				left, leftExists := goObject[key]
				right, rightExists := pythonObject[key]
				if !leftExists {
					left = missingJSON{}
				}
				if !rightExists {
					right = missingJSON{}
				}
				compare(current+"/"+escape(key), left, right)
			}
			return
		}
		goArray, goIsArray := goValue.([]any)
		pythonArray, pythonIsArray := pythonValue.([]any)
		if goMissing && pythonIsArray {
			differences = append(differences, pythonDifference{Case: caseName, Checkpoint: checkpoint, Path: current, Go: encode(goValue), Python: fmt.Sprintf(`"<array:%d>"`, len(pythonArray))})
			return
		}
		if pythonMissing && goIsArray {
			differences = append(differences, pythonDifference{Case: caseName, Checkpoint: checkpoint, Path: current, Go: fmt.Sprintf(`"<array:%d>"`, len(goArray)), Python: encode(pythonValue)})
			return
		}
		if goIsArray && pythonIsArray {
			if len(goArray) != len(pythonArray) {
				differences = append(differences, pythonDifference{Case: caseName, Checkpoint: checkpoint, Path: current + "/length", Go: strconv.Itoa(len(goArray)), Python: strconv.Itoa(len(pythonArray))})
				return
			}
			for index := range goArray {
				compare(current+"/"+strconv.Itoa(index), goArray[index], pythonArray[index])
			}
			return
		}
		if !reflect.DeepEqual(goValue, pythonValue) {
			differences = append(differences, pythonDifference{Case: caseName, Checkpoint: checkpoint, Path: current, Go: encode(goValue), Python: encode(pythonValue)})
		}
	}
	compare(path, goValue, pythonValue)
	return differences
}

func TestStructuredJSONDifferencesStopAtStructuralDivergence(t *testing.T) {
	differences := structuredJSONDifferences("case", "checkpoint", "/root", json.RawMessage(`{"items":[{"kind":"load","place":{"kind":"local"}}]}`), json.RawMessage(`{"items":[{"kind":"const","value":1},{"kind":"const","value":2}]}`))
	if len(differences) != 1 || differences[0].Path != "/root/items/length" {
		t.Fatalf("array divergence = %#v", differences)
	}
	differences = structuredJSONDifferences("case", "checkpoint", "/root", json.RawMessage(`{"kind":"load","place":{"kind":"local"}}`), json.RawMessage(`{"kind":"const","value":1}`))
	if len(differences) != 1 || differences[0].Path != "/root/kind" {
		t.Fatalf("kind divergence = %#v", differences)
	}
	differences = structuredJSONDifferences("case", "checkpoint", "/root", json.RawMessage(`{}`), json.RawMessage(`{"value":{"kind":"const","value":1}}`))
	if len(differences) != 1 || differences[0].Path != "/root/value" || differences[0].Python != `"<object>"` {
		t.Fatalf("missing subtree divergence = %#v", differences)
	}
}

func TestPinnedPythonFinalEngineDataTrees(t *testing.T) {
	golden := loadPythonPassGolden(t)
	allowlist := loadPythonDifferenceAllowlist(t)
	used := make([]bool, len(allowlist))
	var unknown []pythonDifference
	var current []pythonDifference
	builders, _ := pipelineFixtureBuilders(t)
	for _, caseName := range sortedKeys(golden.PipelineCases) {
		if pipelineFixtureCaseByName(t, caseName).ExpectAllocateError {
			continue
		}
		builder := builders[caseName]
		function := builder()
		if err := runStandardPrefix(function, len(NewOptimizer(LevelStandard).passes)); err != nil {
			t.Fatalf("%s: %v", caseName, err)
		}
		got, count, err := parityBackendTree(function)
		if err != nil {
			t.Fatalf("%s: %v", caseName, err)
		}
		want := golden.PipelineCases[caseName]
		if got == want.Nodes && count == want.NodeCount {
			continue
		}
		path := fmt.Sprintf("/pipelineCases/%s/nodes", caseName)
		matched := false
		for index, difference := range allowlist {
			if difference.Case == caseName && difference.Checkpoint == "standard" && difference.Path == path && difference.Go == got && difference.Python == want.Nodes {
				used[index], matched = true, true
				break
			}
		}
		if !matched {
			difference := pythonDifference{Case: caseName, Checkpoint: "standard", Path: path, Go: got, Python: want.Nodes}
			difference.Reason = pythonDifferenceReason(difference)
			unknown = append(unknown, difference)
			current = append(current, difference)
		} else {
			for index, difference := range allowlist {
				if used[index] && difference.Case == caseName && difference.Checkpoint == "standard" && difference.Path == path {
					if os.Getenv("SONOLUS_UPDATE_PY_ALLOWLIST") == "1" {
						difference.Reason = pythonDifferenceReason(difference)
					}
					current = append(current, difference)
					break
				}
			}
		}
	}
	if os.Getenv("SONOLUS_UPDATE_PY_ALLOWLIST") == "1" {
		kept := make([]pythonDifference, 0, len(allowlist)+len(current))
		for _, difference := range allowlist {
			if difference.Checkpoint != "standard" {
				kept = append(kept, difference)
			}
		}
		kept = append(kept, current...)
		data, err := json.MarshalIndent(kept, "", "  ")
		if err != nil {
			t.Fatal(err)
		}
		data = append(data, '\n')
		if err := os.WriteFile("../testdata/optimize/py_pass_allowlist.json", data, 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	for _, difference := range unknown {
		t.Errorf("unknown final EngineData difference at %s\nGo:     %s\nPython: %s", difference.Path, difference.Go, difference.Python)
	}
	for index, difference := range allowlist {
		if difference.Checkpoint == "standard" && !used[index] {
			t.Errorf("unused final EngineData allowlist entry %d: %#v", index, difference)
		}
	}
}

func TestPinnedPythonAllocationOverflowBoundary(t *testing.T) {
	golden := loadPythonPassGolden(t)
	builders, _ := pipelineFixtureBuilders(t)
	for _, item := range loadPipelineFixture(t).Cases {
		if !item.ExpectAllocateError {
			continue
		}
		function := builders[item.Name]()
		err := runStandardPrefix(function, 38)
		if err == nil || !strings.Contains(err.Error(), "4096") {
			t.Fatalf("%s Go allocation error = %v", item.Name, err)
		}
		if golden.PipelineCases[item.Name].AllocateError != "temporary-memory-overflow" {
			t.Fatalf("%s Python allocation error = %q", item.Name, golden.PipelineCases[item.Name].AllocateError)
		}
	}
}

func TestPinnedPythonFinalEngineDataSemantics(t *testing.T) {
	golden := loadPythonPassGolden(t)
	builders, matrix := pipelineFixtureBuilders(t)
	for _, caseName := range sortedKeys(builders) {
		if pipelineFixtureCaseByName(t, caseName).ExpectAllocateError {
			continue
		}
		function := builders[caseName]()
		if err := runStandardPrefix(function, len(NewOptimizer(LevelStandard).passes)); err != nil {
			t.Fatal(err)
		}
		goTree, _, err := parityBackendTree(function)
		if err != nil {
			t.Fatal(err)
		}
		goNodes, goRoot, err := parseCanonicalTree(goTree)
		if err != nil {
			t.Fatal(err)
		}
		pythonNodes, pythonRoot, err := parseCanonicalTree(golden.PipelineCases[caseName].Nodes)
		if err != nil {
			t.Fatal(err)
		}
		for _, input := range matrix[caseName] {
			request := simexec.Request{Memory: map[int][]float64{2000: {input}}}
			goResult, goErr := simexec.Execute(goNodes, goRoot, request)
			pythonResult, pythonErr := simexec.Execute(pythonNodes, pythonRoot, request)
			if goErr != nil || pythonErr != nil {
				t.Fatalf("%s input %g: Go err=%v Python err=%v", caseName, input, goErr, pythonErr)
			}
			if !reflect.DeepEqual(goResult.Memory[2000], pythonResult.Memory[2000]) || !reflect.DeepEqual(goResult.Effects, pythonResult.Effects) {
				t.Fatalf("%s input %g semantic mismatch:\nGo: %+v\nPython: %+v", caseName, input, goResult, pythonResult)
			}
		}
	}
}

func parseCanonicalTree(source string) ([]resource.EngineDataNode, int, error) {
	position := 0
	nodes := []resource.EngineDataNode{}
	var parse func() (int, error)
	parse = func() (int, error) {
		if position >= len(source) {
			return 0, fmt.Errorf("unexpected end of node tree")
		}
		if source[position] == '#' {
			position++
			start := position
			for position < len(source) && source[position] != ',' && source[position] != ')' {
				position++
			}
			value, err := strconv.ParseFloat(source[start:position], 64)
			if err != nil {
				return 0, err
			}
			index := len(nodes)
			nodes = append(nodes, resource.EngineDataValueNode{Value: value})
			return index, nil
		}
		start := position
		for position < len(source) && source[position] != '(' {
			position++
		}
		if position >= len(source) {
			return 0, fmt.Errorf("missing argument list at %d", start)
		}
		function := resource.RuntimeFunction(source[start:position])
		position++
		arguments := []int{}
		if position < len(source) && source[position] != ')' {
			for {
				argument, err := parse()
				if err != nil {
					return 0, err
				}
				arguments = append(arguments, argument)
				if position >= len(source) {
					return 0, fmt.Errorf("unterminated %s", function)
				}
				if source[position] == ')' {
					break
				}
				if source[position] != ',' {
					return 0, fmt.Errorf("invalid separator at %d", position)
				}
				position++
			}
		}
		if position >= len(source) || source[position] != ')' {
			return 0, fmt.Errorf("unterminated %s", function)
		}
		position++
		index := len(nodes)
		nodes = append(nodes, resource.EngineDataFunctionNode{Func: function, Args: arguments})
		return index, nil
	}
	root, err := parse()
	if err != nil {
		return nil, 0, err
	}
	if position != len(source) {
		return nil, 0, fmt.Errorf("trailing node data at %d", position)
	}
	return nodes, root, nil
}

func parityBackendTree(function *ir.Function) (string, int, error) {
	callback := &frontend.CallbackDeclaration{Name: "preprocess", IR: function}
	project := &frontend.Project{Modes: map[mode.Mode]*frontend.ModeDeclarations{mode.ModePlay: {Mode: mode.ModePlay, Archetypes: []*frontend.ArchetypeDeclaration{{Name: "Parity", Callbacks: []*frontend.CallbackDeclaration{callback}}}}}}
	artifacts, err := backend.Compile(project)
	if err != nil {
		return "", 0, err
	}
	root := artifacts.Play.Archetypes[0].Preprocess.Index
	count := 0
	var render func(int) (string, error)
	render = func(index int) (string, error) {
		if index < 0 || index >= len(artifacts.Play.Nodes) {
			return "", fmt.Errorf("node %d out of range", index)
		}
		count++
		switch node := artifacts.Play.Nodes[index].(type) {
		case resource.EngineDataValueNode:
			if node.Value == math.Trunc(node.Value) {
				return fmt.Sprintf("#%.0f", node.Value), nil
			}
			return fmt.Sprintf("#%g", node.Value), nil
		case resource.EngineDataFunctionNode:
			arguments := make([]string, len(node.Args))
			for i, arg := range node.Args {
				value, renderErr := render(arg)
				if renderErr != nil {
					return "", renderErr
				}
				arguments[i] = value
			}
			return string(node.Func) + "(" + strings.Join(arguments, ",") + ")", nil
		default:
			return "", fmt.Errorf("unsupported node %T", node)
		}
	}
	value, err := render(root)
	return value, count, err
}

func runStandardPrefix(function *ir.Function, count int) error {
	optimizer := NewOptimizer(LevelStandard)
	context := Context{Mode: "play", Callback: "updateParallel", analyses: newAnalysisManager()}
	for _, pass := range optimizer.passes[:count] {
		if managed, ok := pass.(ManagedPass); ok {
			for _, required := range managed.Requires() {
				if err := context.analyses.ensure(required, function); err != nil {
					return err
				}
			}
		}
		if err := pass.Run(context, function); err != nil {
			return fmt.Errorf("%s: %w", pass.Name(), err)
		}
		if err := ir.Validate(function); err != nil {
			return fmt.Errorf("%s validation: %w", pass.Name(), err)
		}
		if managed, ok := pass.(ManagedPass); ok {
			context.analyses.invalidateExcept(managed.Preserves())
			for _, destroyed := range managed.Destroys() {
				delete(context.analyses.values, destroyed)
			}
		} else {
			context.analyses.invalidateExcept(nil)
		}
	}
	return nil
}

func pipelineLocal(name string, id int) ir.LocalPlace { return ir.LocalPlace{ID: id, Name: name} }
func pipelineLocalType(name string) ir.Type           { return ir.Type{Name: name, Slots: 1} }
func pipelineMemory(index int) ir.MemoryPlace {
	return ir.MemoryPlace{Storage: "LevelMemory", Index: ir.Const{Value: float64(index)}, Read: true, Write: true}
}
func pipelineStore(index int, value ir.Expr) ir.Store {
	return ir.Store{Place: pipelineMemory(index), Value: value}
}
func pipelineCall(function resource.RuntimeFunction, arguments ...ir.Expr) ir.RuntimeCall {
	return ir.RuntimeCall{Function: function, Args: arguments, Result: parityNumber(), Pure: true}
}
func emptyReturn() ir.Return { return ir.Return{Value: ir.Value{Type: ir.Type{}}} }

func parityPipelineLinear() *ir.Function {
	x := pipelineLocal("x", 0)
	return &ir.Function{Name: "linear_constant", Entry: 0, Locals: []ir.Type{pipelineLocalType("x")}, Blocks: []*ir.Block{{ID: 0, Instructions: []ir.Instruction{ir.Store{Place: x, Value: ir.Const{Value: 3}}, pipelineStore(0, ir.Load{Place: x})}, Terminator: emptyReturn()}}}
}

func parityPipelineDiamondConstant() *ir.Function {
	x := pipelineLocal("x", 0)
	return &ir.Function{Name: "diamond_constant", Entry: 0, Locals: []ir.Type{pipelineLocalType("x")}, Blocks: []*ir.Block{
		{ID: 0, Instructions: []ir.Instruction{ir.Store{Place: x, Value: ir.Const{Value: 5}}}, Terminator: ir.Branch{Condition: pipelineCall(resource.RuntimeFunctionGreater, ir.Load{Place: x}, ir.Const{Value: 3}), True: 1, False: 2}},
		{ID: 1, Instructions: []ir.Instruction{pipelineStore(0, ir.Const{Value: 1})}, Terminator: ir.Jump{Target: 3}},
		{ID: 2, Instructions: []ir.Instruction{pipelineStore(0, ir.Const{Value: 2})}, Terminator: ir.Jump{Target: 3}},
		{ID: 3, Terminator: emptyReturn()},
	}}
}

func parityPipelineDiamondMemory() *ir.Function {
	x := pipelineLocal("x", 0)
	return &ir.Function{Name: "diamond_memory", Entry: 0, Locals: []ir.Type{pipelineLocalType("x")}, Blocks: []*ir.Block{
		{ID: 0, Instructions: []ir.Instruction{ir.Store{Place: x, Value: ir.Load{Place: pipelineMemory(0)}}}, Terminator: ir.Branch{Condition: pipelineCall(resource.RuntimeFunctionGreater, ir.Load{Place: x}, ir.Const{Value: 5}), True: 1, False: 2}},
		{ID: 1, Instructions: []ir.Instruction{pipelineStore(1, ir.Const{Value: 1})}, Terminator: ir.Jump{Target: 3}},
		{ID: 2, Instructions: []ir.Instruction{pipelineStore(1, ir.Const{Value: 2})}, Terminator: ir.Jump{Target: 3}},
		{ID: 3, Terminator: emptyReturn()},
	}}
}

func parityPipelineLoop() *ir.Function {
	sum, index, value := pipelineLocal("sum", 0), pipelineLocal("i", 1), pipelineLocal("v", 2)
	return &ir.Function{Name: "loop_memory", Entry: 0, Locals: []ir.Type{pipelineLocalType("sum"), pipelineLocalType("i"), pipelineLocalType("v")}, Blocks: []*ir.Block{
		{ID: 0, Instructions: []ir.Instruction{ir.Store{Place: sum, Value: ir.Const{}}, ir.Store{Place: index, Value: ir.Const{}}}, Terminator: ir.Jump{Target: 1}},
		{ID: 1, Terminator: ir.Branch{Condition: pipelineCall(resource.RuntimeFunctionLess, ir.Load{Place: index}, ir.Const{Value: 10}), True: 2, False: 3}},
		{ID: 2, Instructions: []ir.Instruction{ir.Store{Place: value, Value: ir.Load{Place: pipelineMemory(0)}}, ir.Store{Place: sum, Value: pipelineCall(resource.RuntimeFunctionAdd, ir.Load{Place: sum}, ir.Load{Place: value})}, ir.Store{Place: index, Value: pipelineCall(resource.RuntimeFunctionAdd, ir.Load{Place: index}, ir.Const{Value: 1})}}, Terminator: ir.Jump{Target: 1}},
		{ID: 3, Instructions: []ir.Instruction{pipelineStore(1, ir.Load{Place: sum})}, Terminator: emptyReturn()},
	}}
}

func parityPipelineSwitch() *ir.Function {
	x := pipelineLocal("x", 0)
	return &ir.Function{Name: "switch_chain", Entry: 0, Locals: []ir.Type{pipelineLocalType("x")}, Blocks: []*ir.Block{
		{ID: 0, Instructions: []ir.Instruction{ir.Store{Place: x, Value: ir.Load{Place: pipelineMemory(0)}}}, Terminator: ir.Branch{Condition: pipelineCall(resource.RuntimeFunctionEqual, ir.Load{Place: x}, ir.Const{Value: 1}), True: 1, False: 2}},
		{ID: 1, Instructions: []ir.Instruction{pipelineStore(1, ir.Const{Value: 10})}, Terminator: ir.Jump{Target: 5}},
		{ID: 2, Terminator: ir.Branch{Condition: pipelineCall(resource.RuntimeFunctionEqual, ir.Load{Place: x}, ir.Const{Value: 2}), True: 3, False: 4}},
		{ID: 3, Instructions: []ir.Instruction{pipelineStore(1, ir.Const{Value: 20})}, Terminator: ir.Jump{Target: 5}},
		{ID: 4, Instructions: []ir.Instruction{pipelineStore(1, ir.Const{Value: 30})}, Terminator: ir.Jump{Target: 5}},
		{ID: 5, Terminator: emptyReturn()},
	}}
}

func parityPipelineString(function *ir.Function) string {
	order := pipelineReversePostorder(function)
	blockIDs := make(map[int]int, len(order))
	for index, id := range order {
		blockIDs[id] = index
	}
	ssaNames := map[int]string{}
	localNames := map[int]string{}
	versions := map[string]int{}
	baseName := func(name string) string {
		if strings.HasPrefix(name, "local.") {
			parts := strings.Split(name, ".")
			if len(parts) >= 3 {
				var id int
				if _, err := fmt.Sscanf(parts[1], "%d", &id); err == nil && id >= 0 && id < len(function.Locals) {
					if candidate := function.Locals[id].Name; candidate != "" {
						return candidate
					}
				}
			}
		}
		if name == "" {
			return "tmp"
		}
		return name
	}
	named := func(items map[int]string, id int, name string) string {
		if existing := items[id]; existing != "" {
			return existing
		}
		base := baseName(name)
		versions[base]++
		items[id] = fmt.Sprintf("%s.%d", base, versions[base])
		return items[id]
	}
	var place func(ir.Place) string
	var expression func(ir.Expr) string
	place = func(raw ir.Place) string {
		switch value := raw.(type) {
		case ir.SSAPlace:
			return "S(" + named(ssaNames, value.ID, value.Name) + ")"
		case ir.LocalPlace:
			if function.Allocated {
				return fmt.Sprintf("M(10000,0,%d)", value.Offset)
			}
			return fmt.Sprintf("T(%s,%d)", named(localNames, value.ID, value.Name), value.Offset)
		case ir.IndexedLocalPlace:
			return fmt.Sprintf("T(%s,%s,%d)", named(localNames, value.ID, value.Name), expression(value.Index), value.Offset)
		case ir.MemoryPlace:
			block := value.Storage
			if block == "LevelMemory" {
				block = "2000"
			}
			index := strings.TrimPrefix(expression(value.Index), "#")
			return fmt.Sprintf("M(%s,%s,%d)", block, index, value.Offset)
		default:
			return fmt.Sprintf("%T", raw)
		}
	}
	expression = func(raw ir.Expr) string {
		switch value := raw.(type) {
		case ir.Const:
			if value.Value == math.Trunc(value.Value) {
				return fmt.Sprintf("#%.0f", value.Value)
			}
			return fmt.Sprintf("#%g", value.Value)
		case ir.Load:
			return "G(" + place(value.Place) + ")"
		case ir.RuntimeCall:
			arguments := make([]string, len(value.Args))
			for index, argument := range value.Args {
				arguments[index] = expression(argument)
			}
			return string(value.Function) + "[" + strings.Join(arguments, ",") + "]"
		default:
			return fmt.Sprintf("%T", raw)
		}
	}
	blocks := make([]string, 0, len(order))
	for normalized, id := range order {
		block := function.Blocks[id]
		parts := []string{}
		for _, phi := range block.Phis {
			arguments := append([]ir.PhiArg(nil), phi.Args...)
			sort.Slice(arguments, func(i, j int) bool { return blockIDs[arguments[i].Predecessor] < blockIDs[arguments[j].Predecessor] })
			values := make([]string, len(arguments))
			for index, argument := range arguments {
				values[index] = fmt.Sprintf("P%d:%s", blockIDs[argument.Predecessor], place(argument.Value))
			}
			parts = append(parts, place(phi.Target)+"=phi("+strings.Join(values, ",")+")")
		}
		for _, instruction := range block.Instructions {
			switch value := instruction.(type) {
			case ir.Store:
				parts = append(parts, place(value.Place)+"="+expression(value.Value))
			case ir.Eval:
				parts = append(parts, expression(value.Value))
			}
		}
		var edges []string
		switch value := block.Terminator.(type) {
		case ir.Jump:
			edges = []string{"default:B" + itoa(blockIDs[value.Target])}
		case ir.Branch:
			parts = append(parts, "?"+expression(value.Condition))
			edges = []string{"#0:B" + itoa(blockIDs[value.False]), "default:B" + itoa(blockIDs[value.True])}
		case ir.Switch:
			parts = append(parts, "?"+expression(value.Value))
			for _, item := range value.Cases {
				edges = append(edges, fmt.Sprintf("#%g:B%d", item.Value, blockIDs[item.Target]))
			}
			sort.Strings(edges)
			edges = append(edges, "default:B"+itoa(blockIDs[value.Default]))
		}
		blocks = append(blocks, fmt.Sprintf("B%d{%s}[%s]", normalized, strings.Join(parts, ";"), strings.Join(edges, ",")))
	}
	return strings.Join(blocks, "")
}

func pipelineReversePostorder(function *ir.Function) []int {
	seen := map[int]bool{}
	postorder := []int{}
	var visit func(int)
	visit = func(id int) {
		if id < 0 || id >= len(function.Blocks) || seen[id] {
			return
		}
		seen[id] = true
		switch terminator := function.Blocks[id].Terminator.(type) {
		case ir.Jump:
			visit(terminator.Target)
		case ir.Branch:
			visit(terminator.False)
			visit(terminator.True)
		case ir.Switch:
			visit(terminator.Default)
			for index := len(terminator.Cases) - 1; index >= 0; index-- {
				visit(terminator.Cases[index].Target)
			}
		}
		postorder = append(postorder, id)
	}
	visit(function.Entry)
	for left, right := 0, len(postorder)-1; left < right; left, right = left+1, right-1 {
		postorder[left], postorder[right] = postorder[right], postorder[left]
	}
	return postorder
}

func sortedKeys[V any](items map[string]V) []string {
	result := make([]string, 0, len(items))
	for key := range items {
		result = append(result, key)
	}
	sort.Strings(result)
	return result
}

func equalStrings(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}

func parityNumber() ir.Type { return ir.Type{Name: "number", Slots: 1} }

func parityMemory(index int) ir.MemoryPlace {
	return ir.MemoryPlace{Storage: "0", Index: ir.Const{Value: float64(index)}, Read: true, Write: true}
}

func parityDiamond() *ir.Function {
	number := parityNumber()
	x := ir.LocalPlace{ID: 0, Name: "x"}
	return &ir.Function{Name: "diamond", Entry: 0, Result: ir.Type{}, Locals: []ir.Type{number}, Blocks: []*ir.Block{
		{ID: 0, Terminator: ir.Branch{Condition: ir.Load{Place: parityMemory(0)}, True: 1, False: 2}},
		{ID: 1, Instructions: []ir.Instruction{ir.Store{Place: x, Value: ir.Const{Value: 1}}}, Terminator: ir.Jump{Target: 3}},
		{ID: 2, Instructions: []ir.Instruction{ir.Store{Place: x, Value: ir.Const{Value: 2}}}, Terminator: ir.Jump{Target: 3}},
		{ID: 3, Instructions: []ir.Instruction{ir.Store{Place: parityMemory(1), Value: ir.Load{Place: x}}}, Terminator: ir.Return{Value: ir.Value{Type: ir.Type{}}}},
	}}
}

func parityLoop() *ir.Function {
	number := parityNumber()
	i := ir.LocalPlace{ID: 0, Name: "i"}
	return &ir.Function{Name: "loop", Entry: 0, Result: ir.Type{}, Locals: []ir.Type{number}, Blocks: []*ir.Block{
		{ID: 0, Instructions: []ir.Instruction{ir.Store{Place: i, Value: ir.Const{}}}, Terminator: ir.Jump{Target: 1}},
		{ID: 1, Terminator: ir.Branch{Condition: ir.Load{Place: parityMemory(0)}, True: 2, False: 3}},
		{ID: 2, Instructions: []ir.Instruction{ir.Store{Place: i, Value: ir.RuntimeCall{Function: resource.RuntimeFunctionAdd, Args: []ir.Expr{ir.Load{Place: i}, ir.Const{Value: 1}}, Result: number, Pure: true}}}, Terminator: ir.Jump{Target: 1}},
		{ID: 3, Instructions: []ir.Instruction{ir.Store{Place: parityMemory(1), Value: ir.Load{Place: i}}}, Terminator: ir.Return{Value: ir.Value{Type: ir.Type{}}}},
	}}
}

func TestToSSAMatchesPinnedPythonSnapshots(t *testing.T) {
	golden := loadPythonPassGolden(t)
	for name, build := range map[string]func() *ir.Function{"diamond": parityDiamond, "loop": parityLoop} {
		t.Run(name, func(t *testing.T) {
			function := build()
			if err := (ToSSA{}).Run(Context{}, function); err != nil {
				t.Fatal(err)
			}
			if got, want := paritySSAString(function), golden.SSACases[name]; got != want {
				t.Fatalf("Go/Python ToSSA mismatch\nGo:     %s\nPython: %s", got, want)
			}
		})
	}
}

func TestSCCPMatchesPinnedPythonSnapshots(t *testing.T) {
	golden := loadPythonPassGolden(t)
	allowedEquivalent := map[string]struct {
		goSnapshot string
		reason     string
	}{
		"const_fold": {
			goSnapshot: "B0{x.1=#5;y.1=#8;M(0,0)=#8}",
			reason:     "Go substitutes the proven constant into the SSA definition itself; Python keeps the folded Add definition while substituting all uses",
		},
	}
	for name, function := range map[string]*ir.Function{
		"const_fold": paritySCCPConstFold(),
		"phi_const":  paritySCCPPhiConst(),
	} {
		t.Run(name, func(t *testing.T) {
			if err := (SparseConditionalConstantPropagation{}).Run(Context{}, function); err != nil {
				t.Fatal(err)
			}
			if got, want := paritySSAString(function), golden.SCCPCases[name]; got != want {
				allowed, ok := allowedEquivalent[name]
				if !ok || got != allowed.goSnapshot {
					t.Fatalf("Go/Python SCCP mismatch\nGo:     %s\nPython: %s", got, want)
				}
				t.Logf("allowed equivalent SCCP structure: %s", allowed.reason)
			}
		})
	}
}

func paritySCCPConstFold() *ir.Function {
	number := parityNumber()
	x, y := ir.SSAPlace{ID: 1, Name: "x"}, ir.SSAPlace{ID: 2, Name: "y"}
	return &ir.Function{Name: "const-fold", Entry: 0, Result: ir.Type{}, Blocks: []*ir.Block{{ID: 0, Instructions: []ir.Instruction{
		ir.Store{Place: x, Value: ir.Const{Value: 5}},
		ir.Store{Place: y, Value: ir.RuntimeCall{Function: resource.RuntimeFunctionAdd, Args: []ir.Expr{ir.Load{Place: x}, ir.Const{Value: 3}}, Result: number, Pure: true}},
		ir.Store{Place: parityMemory(0), Value: ir.Load{Place: y}},
	}, Terminator: ir.Return{Value: ir.Value{Type: ir.Type{}}}}}}
}

func paritySCCPPhiConst() *ir.Function {
	x1, x2, x3 := ir.SSAPlace{ID: 1, Name: "x"}, ir.SSAPlace{ID: 2, Name: "x"}, ir.SSAPlace{ID: 3, Name: "x"}
	return &ir.Function{Name: "phi-const", Entry: 0, Result: ir.Type{}, Blocks: []*ir.Block{
		{ID: 0, Terminator: ir.Branch{Condition: ir.Load{Place: parityMemory(0)}, True: 1, False: 2}},
		{ID: 1, Instructions: []ir.Instruction{ir.Store{Place: x1, Value: ir.Const{Value: 7}}}, Terminator: ir.Jump{Target: 3}},
		{ID: 2, Instructions: []ir.Instruction{ir.Store{Place: x2, Value: ir.Const{Value: 7}}}, Terminator: ir.Jump{Target: 3}},
		{ID: 3, Phis: []ir.Phi{{Target: x3, Args: []ir.PhiArg{{Predecessor: 1, Value: x1}, {Predecessor: 2, Value: x2}}}}, Instructions: []ir.Instruction{ir.Store{Place: parityMemory(1), Value: ir.Load{Place: x3}}}, Terminator: ir.Return{Value: ir.Value{Type: ir.Type{}}}},
	}}
}

func paritySSAString(function *ir.Function) string {
	versions := map[int]string{}
	next := map[string]int{}
	name := func(place ir.SSAPlace) string {
		if existing := versions[place.ID]; existing != "" {
			return existing
		}
		base := place.Name
		if strings.HasPrefix(base, "local.0.") {
			base = "x"
			if function.Name == "loop" {
				base = "i"
			}
		}
		next[base]++
		versions[place.ID] = fmt.Sprintf("%s.%d", base, next[base])
		return versions[place.ID]
	}
	var placeString func(ir.Place) string
	var exprString func(ir.Expr) string
	placeString = func(place ir.Place) string {
		switch value := place.(type) {
		case ir.SSAPlace:
			return name(value)
		case ir.LocalPlace:
			return value.Name
		case ir.MemoryPlace:
			if index, ok := value.Index.(ir.Const); ok {
				return fmt.Sprintf("M(%s,%g)", value.Storage, index.Value)
			}
			return fmt.Sprintf("M(%s,%s)", value.Storage, exprString(value.Index))
		default:
			return fmt.Sprintf("%T", place)
		}
	}
	exprString = func(expression ir.Expr) string {
		switch value := expression.(type) {
		case ir.Const:
			return fmt.Sprintf("#%g", value.Value)
		case ir.Load:
			return "G(" + placeString(value.Place) + ")"
		case ir.RuntimeCall:
			args := make([]string, len(value.Args))
			for i, argument := range value.Args {
				args[i] = exprString(argument)
			}
			return string(value.Function) + "[" + strings.Join(args, ",") + "]"
		default:
			return fmt.Sprintf("%T", expression)
		}
	}
	blocks := make([]string, len(function.Blocks))
	for _, block := range function.Blocks {
		parts := []string{}
		for _, phi := range block.Phis {
			target := name(phi.Target)
			args := append([]ir.PhiArg(nil), phi.Args...)
			sort.Slice(args, func(i, j int) bool { return args[i].Predecessor < args[j].Predecessor })
			values := make([]string, len(args))
			for i, argument := range args {
				values[i] = fmt.Sprintf("P%d:%s", argument.Predecessor, name(argument.Value))
			}
			parts = append(parts, target+"=phi("+strings.Join(values, ",")+")")
		}
		for _, instruction := range block.Instructions {
			switch value := instruction.(type) {
			case ir.Store:
				parts = append(parts, placeString(value.Place)+"="+exprString(value.Value))
			case ir.Eval:
				parts = append(parts, exprString(value.Value))
			}
		}
		if branch, ok := block.Terminator.(ir.Branch); ok {
			parts = append(parts, "?"+exprString(branch.Condition))
		}
		blocks[block.ID] = fmt.Sprintf("B%d{%s}", block.ID, strings.Join(parts, ";"))
	}
	return strings.Join(blocks, "")
}

func TestFromSSAAndAllocationMatchPinnedPythonContracts(t *testing.T) {
	golden := loadPythonPassGolden(t)
	if golden.FromSSACases["diamond"] == "" || golden.FromSSACases["loop"] == "" {
		t.Fatal("pinned Python golden is missing FromSSA checkpoints")
	}
	for name, build := range map[string]func() *ir.Function{"diamond": parityDiamond, "loop": parityLoop} {
		t.Run(name, func(t *testing.T) {
			function := build()
			if err := (ToSSA{}).Run(Context{}, function); err != nil {
				t.Fatal(err)
			}
			if err := (FromSSA{}).Run(Context{}, function); err != nil {
				t.Fatal(err)
			}
			if err := (Allocate{}).Run(Context{}, function); err != nil {
				t.Fatal(err)
			}
			if err := ir.ValidateFinal(function); err != nil {
				t.Fatal(err)
			}
			for _, block := range function.Blocks {
				if len(block.Phis) != 0 {
					t.Fatalf("block %d retained Phi nodes", block.ID)
				}
			}
			if len(function.Locals) > 1 || (len(function.Locals) == 1 && function.Locals[0].Slots > 4096) {
				t.Fatalf("invalid allocated Temporary Memory layout: %#v", function.Locals)
			}
		})
	}
}
