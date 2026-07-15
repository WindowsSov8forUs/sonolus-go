package optimize

import (
	"fmt"
	"math"
	"math/rand"
	"reflect"
	"testing"

	"github.com/WindowsSov8forUs/sonolus-core-go/core/resource"
	"github.com/WindowsSov8forUs/sonolus-go/v2/internal/compiler/ir"
	"github.com/WindowsSov8forUs/sonolus-go/v2/internal/compiler/mode"
)

type irExecution struct {
	value   []float64
	memory  map[string]map[int]float64
	effects []resource.RuntimeFunction
}

func executeTestIR(function *ir.Function, input float64) (irExecution, error) {
	result := irExecution{memory: map[string]map[int]float64{
		"LevelMemory": {0: input},
		"EngineRom":   {0: input*2 + 1},
	}}
	locals := map[[2]int]float64{}
	ssa := map[int]float64{}
	blockID, predecessor := function.Entry, -1
	for step := 0; step < 10_000; step++ {
		if blockID < 0 || blockID >= len(function.Blocks) || function.Blocks[blockID] == nil {
			return result, fmt.Errorf("invalid block %d", blockID)
		}
		block := function.Blocks[blockID]
		for _, phi := range block.Phis {
			matched := false
			for _, argument := range phi.Args {
				if argument.Predecessor == predecessor {
					ssa[phi.Target.ID] = ssa[argument.Value.ID]
					matched = true
					break
				}
			}
			if !matched {
				ssa[phi.Target.ID] = locals[[2]int{phi.Local.ID, phi.Local.Offset}]
			}
		}
		var eval func(ir.Expr) (float64, error)
		var address func(ir.Place) (string, int, error)
		address = func(place ir.Place) (string, int, error) {
			switch value := place.(type) {
			case ir.LocalPlace:
				if function.Allocated {
					return "TemporaryMemory", value.Offset, nil
				}
				return fmt.Sprintf("local:%d", value.ID), value.Offset, nil
			case ir.IndexedLocalPlace:
				index, err := eval(value.Index)
				if err != nil || index != math.Trunc(index) || index < 0 || index >= float64(value.Length) {
					return "", 0, fmt.Errorf("invalid indexed local %g", index)
				}
				return fmt.Sprintf("local:%d", value.ID), value.Base + int(index)*value.Stride + value.Offset, nil
			case ir.MemoryPlace:
				index, err := eval(value.Index)
				if err != nil || index != math.Trunc(index) {
					return "", 0, fmt.Errorf("invalid memory index %g", index)
				}
				return value.Storage, int(index)*max(1, value.Stride) + value.Offset, nil
			case ir.SSAPlace:
				return "ssa", value.ID, nil
			default:
				return "", 0, fmt.Errorf("unsupported place %T", place)
			}
		}
		eval = func(expression ir.Expr) (float64, error) {
			switch value := expression.(type) {
			case ir.Const:
				return value.Value, nil
			case ir.Load:
				storage, index, err := address(value.Place)
				if err != nil {
					return 0, err
				}
				if storage == "ssa" {
					return ssa[index], nil
				}
				if len(storage) >= 6 && storage[:6] == "local:" {
					var id int
					_, _ = fmt.Sscanf(storage, "local:%d", &id)
					return locals[[2]int{id, index}], nil
				}
				return result.memory[storage][index], nil
			case ir.RuntimeCall:
				arguments := make([]float64, len(value.Args))
				for index := range value.Args {
					var err error
					arguments[index], err = eval(value.Args[index])
					if err != nil {
						return 0, err
					}
				}
				if !value.Pure {
					result.effects = append(result.effects, value.Function)
				}
				return testRuntime(value.Function, arguments)
			default:
				return 0, fmt.Errorf("unsupported expression %T", expression)
			}
		}
		store := func(place ir.Place, expression ir.Expr) error {
			value, err := eval(expression)
			if err != nil {
				return err
			}
			storage, index, err := address(place)
			if err != nil {
				return err
			}
			switch {
			case storage == "ssa":
				ssa[index] = value
			case len(storage) >= 6 && storage[:6] == "local:":
				var id int
				_, _ = fmt.Sscanf(storage, "local:%d", &id)
				locals[[2]int{id, index}] = value
			default:
				if result.memory[storage] == nil {
					result.memory[storage] = map[int]float64{}
				}
				result.memory[storage][index] = value
			}
			return nil
		}
		for _, instruction := range block.Instructions {
			switch value := instruction.(type) {
			case ir.Store:
				if err := store(value.Place, value.Value); err != nil {
					return result, err
				}
			case ir.Eval:
				if _, err := eval(value.Value); err != nil {
					return result, err
				}
			}
		}
		predecessor = blockID
		switch terminator := block.Terminator.(type) {
		case ir.Jump:
			blockID = terminator.Target
		case ir.Branch:
			condition, err := eval(terminator.Condition)
			if err != nil {
				return result, err
			}
			if condition != 0 {
				blockID = terminator.True
			} else {
				blockID = terminator.False
			}
		case ir.Switch:
			value, err := eval(terminator.Value)
			if err != nil {
				return result, err
			}
			blockID = terminator.Default
			for _, item := range terminator.Cases {
				if item.Value == value {
					blockID = item.Target
					break
				}
			}
		case ir.Return:
			result.value = make([]float64, len(terminator.Value.Slots))
			for index := range terminator.Value.Slots {
				value, err := eval(terminator.Value.Slots[index])
				if err != nil {
					return result, err
				}
				result.value[index] = value
			}
			delete(result.memory, "TemporaryMemory")
			return result, nil
		case ir.Unreachable:
			return result, fmt.Errorf("reached unreachable block %d", blockID)
		default:
			return result, fmt.Errorf("unsupported terminator %T", block.Terminator)
		}
	}
	return result, fmt.Errorf("IR step limit exceeded")
}

func testRuntime(function resource.RuntimeFunction, arguments []float64) (float64, error) {
	boolean := func(value bool) float64 {
		if value {
			return 1
		}
		return 0
	}
	switch function {
	case resource.RuntimeFunctionAdd:
		result := 0.0
		for _, value := range arguments {
			result += value
		}
		return result, nil
	case resource.RuntimeFunctionSubtract:
		return arguments[0] - arguments[1], nil
	case resource.RuntimeFunctionMultiply:
		return arguments[0] * arguments[1], nil
	case resource.RuntimeFunctionDivide:
		return arguments[0] / arguments[1], nil
	case resource.RuntimeFunctionEqual:
		return boolean(arguments[0] == arguments[1]), nil
	case resource.RuntimeFunctionGreater:
		return boolean(arguments[0] > arguments[1]), nil
	case resource.RuntimeFunctionLess:
		return boolean(arguments[0] < arguments[1]), nil
	case resource.RuntimeFunctionDebugLog:
		return 0, nil
	default:
		return 0, fmt.Errorf("unsupported test runtime %s", function)
	}
}

func TestStandardPassesPreservePipelineFixtureSemantics(t *testing.T) {
	builders, matrix := pipelineFixtureBuilders(t)
	for _, caseName := range sortedKeys(builders) {
		for _, input := range matrix[caseName] {
			function := builders[caseName]()
			want, err := executeTestIR(function, input)
			if err != nil {
				t.Fatalf("%s input %g baseline: %v", caseName, input, err)
			}
			context := Context{Mode: mode.ModePlay, Callback: "updateParallel", analyses: newAnalysisManager()}
			for _, pass := range NewOptimizer(LevelStandard).passes {
				managed := pass.(ManagedPass)
				for _, required := range managed.Requires() {
					if err := context.analyses.ensure(required, function); err != nil {
						t.Fatal(err)
					}
				}
				if err := pass.Run(context, function); err != nil {
					t.Fatalf("%s input %g %s: %v", caseName, input, pass.Name(), err)
				}
				if err := ir.Validate(function); err != nil {
					t.Fatalf("%s input %g %s validation: %v", caseName, input, pass.Name(), err)
				}
				got, err := executeTestIR(function, input)
				if err != nil {
					t.Fatalf("%s input %g %s execution: %v", caseName, input, pass.Name(), err)
				}
				if !reflect.DeepEqual(got.value, want.value) || !reflect.DeepEqual(got.memory, want.memory) || !reflect.DeepEqual(got.effects, want.effects) {
					t.Fatalf("%s input %g %s changed semantics: want=%+v got=%+v", caseName, input, pass.Name(), want, got)
				}
				context.analyses.invalidateExcept(managed.Preserves())
				for _, destroyed := range managed.Destroys() {
					delete(context.analyses.values, destroyed)
				}
			}
		}
	}
}

func TestStandardPassesPreserveGeneratedCFGCorpus(t *testing.T) {
	random := rand.New(rand.NewSource(0x534f4e4f4c5553))
	inputs := []float64{-3, 0, 2, 9}
	for caseIndex := 0; caseIndex < 48; caseIndex++ {
		seed := generatedIRSeed{
			threshold:  float64(random.Intn(9) - 4),
			left:       float64(random.Intn(7) + 1),
			right:      float64(random.Intn(7) + 1),
			iterations: random.Intn(4) + 1,
			operation:  random.Intn(3),
		}
		for _, input := range inputs {
			for _, generated := range []struct {
				name  string
				build func(generatedIRSeed) *ir.Function
			}{{"branch", buildGeneratedIR}, {"switch-multi-latch", buildGeneratedSwitchIR}} {
				name := fmt.Sprintf("generated-%s-%02d-input-%g", generated.name, caseIndex, input)
				t.Run(name, func(t *testing.T) {
					assertStandardPassSemantics(t, generated.build(seed), input)
				})
			}
		}
	}
}

func buildGeneratedSwitchIR(seed generatedIRSeed) *ir.Function {
	number := ir.Type{Name: "number", Slots: 1}
	pair := ir.Type{Name: "pair", Slots: 2}
	x := ir.LocalPlace{ID: 0, Name: "x"}
	index := ir.LocalPlace{ID: 1, Name: "index"}
	sum := ir.LocalPlace{ID: 2, Name: "sum"}
	input := ir.Load{Place: ir.MemoryPlace{Storage: "LevelMemory", Index: ir.Const{}, Read: true, Write: true}}
	readonly := ir.Load{Place: ir.MemoryPlace{Storage: "EngineRom", Index: ir.Const{}, Read: true}}
	pure := func(function resource.RuntimeFunction, arguments ...ir.Expr) ir.RuntimeCall {
		return ir.RuntimeCall{Function: function, Args: arguments, Result: number, Pure: true}
	}
	return &ir.Function{Name: "generated-switch", Entry: 0, Result: pair, Locals: []ir.Type{number, number, number}, Blocks: []*ir.Block{
		{ID: 0, Instructions: []ir.Instruction{
			ir.Store{Place: index, Value: ir.Const{}},
			ir.Store{Place: sum, Value: ir.Const{}},
		}, Terminator: ir.Switch{Value: input, Cases: []ir.SwitchCase{{Value: -3, Target: 1}, {Value: 0, Target: 2}}, Default: 3}},
		{ID: 1, Instructions: []ir.Instruction{ir.Store{Place: x, Value: ir.Const{Value: seed.left}}}, Terminator: ir.Jump{Target: 4}},
		{ID: 2, Instructions: []ir.Instruction{ir.Store{Place: x, Value: ir.Const{Value: seed.right}}}, Terminator: ir.Jump{Target: 4}},
		{ID: 3, Instructions: []ir.Instruction{ir.Store{Place: x, Value: pure(resource.RuntimeFunctionAdd, input, ir.Const{Value: seed.threshold})}}, Terminator: ir.Jump{Target: 4}},
		{ID: 4, Terminator: ir.Branch{Condition: pure(resource.RuntimeFunctionLess, ir.Load{Place: index}, ir.Const{Value: float64(seed.iterations)}), True: 5, False: 8}},
		{ID: 5, Terminator: ir.Branch{Condition: pure(resource.RuntimeFunctionEqual, ir.Load{Place: index}, ir.Const{Value: 1}), True: 6, False: 7}},
		{ID: 6, Instructions: []ir.Instruction{
			ir.Store{Place: ir.MemoryPlace{Storage: "LevelMemory", Index: ir.Load{Place: x}, Read: true, Write: true}, Value: pure(resource.RuntimeFunctionAdd, ir.Load{Place: sum}, readonly)},
			ir.Store{Place: sum, Value: pure(resource.RuntimeFunctionAdd, ir.Load{Place: sum}, readonly)},
			ir.Store{Place: index, Value: pure(resource.RuntimeFunctionAdd, ir.Load{Place: index}, ir.Const{Value: 1})},
		}, Terminator: ir.Jump{Target: 4}},
		{ID: 7, Instructions: []ir.Instruction{
			ir.Store{Place: ir.MemoryPlace{Storage: "LevelMemory", Index: pure(resource.RuntimeFunctionAdd, ir.Load{Place: index}, ir.Const{Value: 1}), Read: true, Write: true}, Value: pure(resource.RuntimeFunctionAdd, ir.Load{Place: sum}, ir.Load{Place: x})},
			ir.Store{Place: sum, Value: pure(resource.RuntimeFunctionAdd, ir.Load{Place: sum}, ir.Load{Place: x})},
			ir.Store{Place: index, Value: pure(resource.RuntimeFunctionAdd, ir.Load{Place: index}, ir.Const{Value: 1})},
		}, Terminator: ir.Jump{Target: 4}},
		{ID: 8, Instructions: []ir.Instruction{ir.Eval{Value: ir.RuntimeCall{Function: resource.RuntimeFunctionDebugLog, Args: []ir.Expr{ir.Load{Place: sum}}, Result: ir.Type{}, Pure: false}}}, Terminator: ir.Return{Value: ir.Value{Type: pair, Slots: []ir.Expr{ir.Load{Place: sum}, ir.Load{Place: x}}}}},
	}}
}

type generatedIRSeed struct {
	threshold, left, right float64
	iterations             int
	operation              int
}

func buildGeneratedIR(seed generatedIRSeed) *ir.Function {
	number := ir.Type{Name: "number", Slots: 1}
	x := ir.LocalPlace{ID: 0, Name: "x"}
	index := ir.LocalPlace{ID: 1, Name: "index"}
	sum := ir.LocalPlace{ID: 2, Name: "sum"}
	input := ir.Load{Place: ir.MemoryPlace{Storage: "LevelMemory", Index: ir.Const{}, Read: true, Write: true}}
	pure := func(function resource.RuntimeFunction, arguments ...ir.Expr) ir.RuntimeCall {
		return ir.RuntimeCall{Function: function, Args: arguments, Result: number, Pure: true}
	}
	leftFunction := []resource.RuntimeFunction{resource.RuntimeFunctionAdd, resource.RuntimeFunctionSubtract, resource.RuntimeFunctionMultiply}[seed.operation]
	rightFunction := []resource.RuntimeFunction{resource.RuntimeFunctionSubtract, resource.RuntimeFunctionMultiply, resource.RuntimeFunctionAdd}[seed.operation]
	return &ir.Function{Name: "generated", Entry: 0, Result: number, Locals: []ir.Type{number, number, number}, Blocks: []*ir.Block{
		{ID: 0, Instructions: []ir.Instruction{
			ir.Store{Place: index, Value: ir.Const{}},
			ir.Store{Place: sum, Value: ir.Const{}},
		}, Terminator: ir.Branch{Condition: pure(resource.RuntimeFunctionGreater, input, ir.Const{Value: seed.threshold}), True: 1, False: 2}},
		{ID: 1, Instructions: []ir.Instruction{ir.Store{Place: x, Value: pure(leftFunction, input, ir.Const{Value: seed.left})}}, Terminator: ir.Jump{Target: 3}},
		{ID: 2, Instructions: []ir.Instruction{ir.Store{Place: x, Value: pure(rightFunction, input, ir.Const{Value: seed.right})}}, Terminator: ir.Jump{Target: 3}},
		{ID: 3, Terminator: ir.Branch{Condition: pure(resource.RuntimeFunctionLess, ir.Load{Place: index}, ir.Const{Value: float64(seed.iterations)}), True: 4, False: 5}},
		{ID: 4, Instructions: []ir.Instruction{
			ir.Store{Place: ir.MemoryPlace{Storage: "LevelMemory", Index: ir.Load{Place: index}, Read: true, Write: true}, Value: pure(resource.RuntimeFunctionAdd, ir.Load{Place: sum}, ir.Load{Place: x})},
			ir.Store{Place: sum, Value: pure(resource.RuntimeFunctionAdd, ir.Load{Place: sum}, ir.Load{Place: x})},
			ir.Store{Place: index, Value: pure(resource.RuntimeFunctionAdd, ir.Load{Place: index}, ir.Const{Value: 1})},
		}, Terminator: ir.Jump{Target: 3}},
		{ID: 5, Instructions: []ir.Instruction{ir.Eval{Value: ir.RuntimeCall{Function: resource.RuntimeFunctionDebugLog, Args: []ir.Expr{ir.Load{Place: sum}}, Result: ir.Type{}, Pure: false}}}, Terminator: ir.Return{Value: ir.Value{Type: number, Slots: []ir.Expr{ir.Load{Place: sum}}}}},
	}}
}

func assertStandardPassSemantics(t *testing.T, function *ir.Function, input float64) {
	t.Helper()
	if err := ir.Validate(function); err != nil {
		t.Fatalf("initial validation: %v", err)
	}
	want, err := executeTestIR(function, input)
	if err != nil {
		t.Fatalf("baseline: %v", err)
	}
	context := Context{Mode: mode.ModePlay, Callback: "updateParallel", analyses: newAnalysisManager()}
	for _, pass := range NewOptimizer(LevelStandard).passes {
		managed := pass.(ManagedPass)
		for _, required := range managed.Requires() {
			if err := context.analyses.ensure(required, function); err != nil {
				t.Fatal(err)
			}
		}
		if err := pass.Run(context, function); err != nil {
			t.Fatalf("%s: %v", pass.Name(), err)
		}
		if err := ir.Validate(function); err != nil {
			t.Fatalf("%s validation: %v", pass.Name(), err)
		}
		got, err := executeTestIR(function, input)
		if err != nil {
			t.Fatalf("%s execution: %v", pass.Name(), err)
		}
		if !reflect.DeepEqual(got.value, want.value) || !reflect.DeepEqual(got.memory, want.memory) || !reflect.DeepEqual(got.effects, want.effects) {
			t.Fatalf("%s changed semantics: want=%+v got=%+v", pass.Name(), want, got)
		}
		context.analyses.invalidateExcept(managed.Preserves())
		for _, destroyed := range managed.Destroys() {
			delete(context.analyses.values, destroyed)
		}
	}
}
