package optimize

import (
	"fmt"
	"math"
	"reflect"
	"strings"
	"testing"

	"github.com/WindowsSov8forUs/sonolus-core-go/core/resource"
	"github.com/WindowsSov8forUs/sonolus-go/v2/internal/compiler/backend"
	"github.com/WindowsSov8forUs/sonolus-go/v2/internal/compiler/frontend"
	"github.com/WindowsSov8forUs/sonolus-go/v2/internal/compiler/ir"
	"github.com/WindowsSov8forUs/sonolus-go/v2/internal/compiler/mode"
	"github.com/WindowsSov8forUs/sonolus-go/v2/internal/compiler/source"
	"github.com/WindowsSov8forUs/sonolus-go/v2/internal/simexec"
)

func TestInlineVarsPreservesLocalWritesAndControlFlow(t *testing.T) {
	local := ir.LocalPlace{ID: 0, Name: "value"}
	load := ir.Load{Place: local}
	log := func(value ir.Expr) ir.Instruction {
		return ir.Eval{Value: ir.RuntimeCall{Function: resource.RuntimeFunctionDebugLog, Args: []ir.Expr{value}, Result: ir.Type{}}}
	}
	store := ir.Store{Place: local, Value: ir.Const{Value: 7}}
	done := ir.Return{Value: ir.Value{Type: ir.Type{}}}
	tests := []struct {
		name   string
		blocks []*ir.Block
		want   []float64
		inline bool
	}{
		{"indexed-write", []*ir.Block{{ID: 0, Instructions: []ir.Instruction{
			ir.Store{Place: local, Value: ir.Const{}},
			ir.Store{Place: ir.IndexedLocalPlace{ID: 0, Length: 1, Stride: 1, Index: ir.Const{}}, Value: ir.Const{Value: 7}},
			log(load),
		}, Terminator: done}}, []float64{7}, false},
		{"skipped-definition", []*ir.Block{
			{ID: 0, Terminator: ir.Branch{Condition: ir.Const{}, True: 1, False: 2}},
			{ID: 1, Instructions: []ir.Instruction{store}, Terminator: ir.Jump{Target: 2}},
			{ID: 2, Instructions: []ir.Instruction{log(load)}, Terminator: done},
		}, []float64{0}, false},
		{"read-before-write", []*ir.Block{{ID: 0, Instructions: []ir.Instruction{log(load), store}, Terminator: done}}, []float64{0}, false},
		{"same-block", []*ir.Block{{ID: 0, Instructions: []ir.Instruction{store, log(load)}, Terminator: done}}, []float64{7}, true},
		{"dominating-block", []*ir.Block{
			{ID: 0, Instructions: []ir.Instruction{store}, Terminator: ir.Jump{Target: 1}},
			{ID: 1, Instructions: []ir.Instruction{log(load)}, Terminator: done},
		}, []float64{7}, true},
		{"self-read", []*ir.Block{{ID: 0, Instructions: []ir.Instruction{
			ir.Store{Place: local, Value: ir.RuntimeCall{Function: resource.RuntimeFunctionAdd, Args: []ir.Expr{load, ir.Const{Value: 1}}, Result: ir.Type{Slots: 1}, Pure: true}},
			log(load),
		}, Terminator: done}}, []float64{1}, false},
	}
	for _, test := range tests {
		for _, aggressive := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/aggressive=%t", test.name, aggressive), func(t *testing.T) {
				function := CloneFunction(&ir.Function{Name: test.name, Locals: []ir.Type{{Slots: 1}}, Blocks: test.blocks})
				if err := (InlineVars{Aggressive: aggressive}).Run(Context{Mode: mode.ModePlay, Callback: "preprocess"}, function); err != nil {
					t.Fatal(err)
				}
				if got := executeCallValueCheckpoint(t, function); !reflect.DeepEqual(got, test.want) {
					t.Fatalf("want %v, got %v", test.want, got)
				}
				for _, block := range function.Blocks {
					for _, instruction := range block.Instructions {
						if eval, ok := instruction.(ir.Eval); ok {
							_, constant := eval.Value.(ir.RuntimeCall).Args[0].(ir.Const)
							if constant != test.inline {
								t.Fatalf("log constant=%t, want %t", constant, test.inline)
							}
						}
					}
				}
			})
		}
	}
}

func TestInlineVarsResolvesSSAAliasChainsOnPhiEdges(t *testing.T) {
	number := ir.Type{Slots: 1}
	load := func(id int) ir.Expr { return ir.Load{Place: ir.SSAPlace{ID: id}} }
	function := &ir.Function{Name: "aliases", Locals: []ir.Type{number}, Blocks: []*ir.Block{
		{ID: 0, Instructions: []ir.Instruction{
			ir.Store{Place: ir.LocalPlace{ID: 0}, Value: ir.Const{Value: 7}},
			ir.Store{Place: ir.SSAPlace{ID: 0}, Value: ir.Load{Place: ir.LocalPlace{ID: 0}}},
			ir.Store{Place: ir.SSAPlace{ID: 1}, Value: load(0)},
			ir.Store{Place: ir.SSAPlace{ID: 2}, Value: load(1)},
			ir.Store{Place: ir.LocalPlace{ID: 0}, Value: ir.Const{Value: 99}},
		}, Terminator: ir.Jump{Target: 1}},
		{ID: 1, Phis: []ir.Phi{{Target: ir.SSAPlace{ID: 3}, Args: []ir.PhiArg{
			{Predecessor: 0, Value: ir.SSAPlace{ID: 2}}, {Predecessor: 1, Value: ir.SSAPlace{ID: 5}},
		}}}, Instructions: []ir.Instruction{
			ir.Eval{Value: ir.RuntimeCall{Function: resource.RuntimeFunctionDebugLog, Args: []ir.Expr{load(3)}}},
			ir.Store{Place: ir.SSAPlace{ID: 4}, Value: ir.RuntimeCall{Function: resource.RuntimeFunctionAdd, Args: []ir.Expr{load(3), ir.Const{Value: 1}}, Result: number, Pure: true}},
			ir.Store{Place: ir.SSAPlace{ID: 5}, Value: load(4)},
		}, Terminator: ir.Branch{Condition: ir.RuntimeCall{Function: resource.RuntimeFunctionLess, Args: []ir.Expr{load(4), ir.Const{Value: 9}}, Result: number, Pure: true}, True: 1, False: 2}},
		{ID: 2, Terminator: ir.Return{}},
	}}
	for _, aggressive := range []bool{false, true} {
		t.Run(fmt.Sprintf("aggressive=%t", aggressive), func(t *testing.T) {
			candidate := CloneFunction(function)
			if err := (InlineVars{Aggressive: aggressive}).Run(Context{}, candidate); err != nil {
				t.Fatal(err)
			}
			if err := ir.Validate(candidate); err != nil {
				t.Fatal(err)
			}
			args := candidate.Blocks[1].Phis[0].Args
			if args[0].Value.ID != 0 || args[1].Value.ID != 4 {
				t.Fatalf("unresolved Phi aliases: %+v", args)
			}
			if got := executeCallValueCheckpoint(t, candidate); !reflect.DeepEqual(got, []float64{7, 8}) {
				t.Fatalf("snapshot or loop value changed: %v", got)
			}
		})
	}
}

func TestInlineVarsCountsAddressReads(t *testing.T) {
	index := ir.LocalPlace{ID: 0}
	arguments := make([]ir.Expr, 10)
	for i := range arguments {
		arguments[i] = ir.Const{}
	}
	arguments[0] = ir.Const{Value: 1}
	// A duplicated expression must cost more than storing it once and
	// reading it twice, so omitting the address use changes this decision.
	value := ir.RuntimeCall{Function: resource.RuntimeFunctionAdd, Args: arguments, Result: numberType, Pure: true}
	for _, destination := range []ir.Place{
		ir.IndexedLocalPlace{ID: 1, Index: ir.Load{Place: index}, Length: 2, Stride: 1},
		ir.MemoryPlace{Storage: "LevelMemory", Index: ir.Load{Place: index}, Stride: 1, Write: true},
	} {
		for _, aggressive := range []bool{false, true} {
			t.Run(fmt.Sprintf("%T/aggressive=%t", destination, aggressive), func(t *testing.T) {
				function := &ir.Function{Locals: []ir.Type{{Slots: 1}, {Slots: 2}}, Blocks: []*ir.Block{{
					ID: 0, Instructions: []ir.Instruction{
						ir.Store{Place: index, Value: value},
						ir.Store{Place: destination, Value: ir.Const{Value: 7}},
						ir.Eval{Value: ir.RuntimeCall{Function: resource.RuntimeFunctionDebugLog, Args: []ir.Expr{ir.Load{Place: index}}, Result: ir.Type{}}},
					}, Terminator: ir.Return{Value: ir.Value{Type: ir.Type{}}},
				}}}
				if err := (InlineVars{Aggressive: aggressive}).Run(Context{Mode: mode.ModePlay, Callback: "preprocess"}, function); err != nil {
					t.Fatal(err)
				}
				argument := function.Blocks[0].Instructions[2].(ir.Eval).Value.(ir.RuntimeCall).Args[0]
				if _, inlined := argument.(ir.RuntimeCall); inlined != aggressive {
					t.Fatalf("two uses including the store address: inlined=%t, aggressive=%t", inlined, aggressive)
				}
				if got := executeCallValueCheckpoint(t, function); !reflect.DeepEqual(got, []float64{1}) {
					t.Fatalf("logs=%v", got)
				}
			})
		}
	}
}

func TestInlineVarsPreservesMutableTerminatorEvaluation(t *testing.T) {
	for _, tc := range []struct {
		name   string
		want   float64
		inline bool
	}{{"pure", 3, true}, {"effectful-operand", 11, false}, {"intervening-write", 2, false}, {"cheap-repeated", 6, true}} {
		t.Run(tc.name, func(t *testing.T) {
			builder := ir.NewBuilder(tc.name, numberType)
			entry := builder.NewBlock()
			_ = builder.SetEntry(entry)
			_ = builder.SetCurrent(entry)
			memory, err := builder.Memory("LevelMemory", ir.Const{}, 1, 0, true, true)
			if err != nil {
				t.Fatal(err)
			}
			local := builder.NewLocal("value", numberType)
			add := func(a, b ir.Expr) ir.Expr {
				return builder.RuntimeCall(resource.RuntimeFunctionAdd, []ir.Expr{a, b}, numberType, true, ir.SourcePos{})
			}
			var value ir.Expr = ir.Load{Place: memory}
			if tc.name == "cheap-repeated" {
				value = add(ir.Const{Value: 1}, ir.Const{Value: 2})
			}
			if err := builder.Store(ir.Places(local), ir.Value{Type: numberType, Slots: []ir.Expr{value}}, ir.SourcePos{}); err != nil {
				t.Fatal(err)
			}
			result := add(local.Slots[0], ir.Const{Value: 1})
			switch tc.name {
			case "effectful-operand":
				write := builder.RuntimeCall(resource.RuntimeFunctionSet, []ir.Expr{ir.Const{Value: 2000}, ir.Const{}, ir.Const{Value: 9}}, numberType, false, ir.SourcePos{})
				result = add(write, local.Slots[0])
			case "intervening-write":
				if err := builder.Store([]ir.Place{memory}, ir.Value{Type: numberType, Slots: []ir.Expr{ir.Const{Value: 9}}}, ir.SourcePos{}); err != nil {
					t.Fatal(err)
				}
				result = local.Slots[0]
			case "cheap-repeated":
				result = add(local.Slots[0], local.Slots[0])
			}
			_ = builder.Return(ir.Value{Type: numberType, Slots: []ir.Expr{result}})
			fn, err := builder.Finish()
			if err != nil {
				t.Fatal(err)
			}
			if err := (InlineVars{}).Run(Context{Mode: mode.ModePlay, Callback: "preprocess"}, fn); err != nil {
				t.Fatal(err)
			}
			uses := newBitSet(1)
			localUsesExprBitSet(fn.Blocks[0].Terminator.(ir.Return).Value.Slots[0], uses)
			if uses.has(0) == tc.inline {
				t.Fatalf("inline=%t want=%t", !uses.has(0), tc.inline)
			}
			if err := allocateLocals(fn, false, false); err != nil {
				t.Fatal(err)
			}
			tree, _, err := parityBackendTree(fn)
			if err != nil {
				t.Fatal(err)
			}
			nodes, root, err := parseCanonicalTree(tree)
			if err != nil {
				t.Fatal(err)
			}
			got, err := simexec.Execute(nodes, root, simexec.Request{Memory: map[int][]float64{2000: {2}}})
			if err != nil {
				t.Fatal(err)
			}
			if got.Value != tc.want {
				t.Fatalf("value=%g want=%g", got.Value, tc.want)
			}
		})
	}
}

func TestInlineVarsExpandsNestedDefinitionsWithinBudget(t *testing.T) {
	for _, repeated := range []bool{false, true} {
		t.Run(fmt.Sprintf("repeated=%t", repeated), func(t *testing.T) {
			number := ir.Type{Slots: 1}
			arguments := make([]ir.Expr, 10)
			for i := range arguments {
				arguments[i] = ir.Const{Value: 1}
			}
			var result ir.Expr = ir.Load{Place: ir.SSAPlace{ID: 1}}
			if repeated {
				result = ir.RuntimeCall{Function: resource.RuntimeFunctionAdd, Args: []ir.Expr{result, result}, Result: number, Pure: true}
			}
			function := &ir.Function{Name: "nested-inlining", Blocks: []*ir.Block{{ID: 0, Instructions: []ir.Instruction{
				ir.Store{Place: ir.SSAPlace{ID: 0}, Value: ir.RuntimeCall{Function: resource.RuntimeFunctionAdd, Args: arguments, Result: number, Pure: true}},
				ir.Store{Place: ir.SSAPlace{ID: 1}, Value: ir.RuntimeCall{Function: resource.RuntimeFunctionMultiply, Args: []ir.Expr{ir.Load{Place: ir.SSAPlace{ID: 0}}, ir.Const{Value: 2}}, Result: number, Pure: true}},
				ir.Eval{Value: ir.RuntimeCall{Function: resource.RuntimeFunctionDebugLog, Args: []ir.Expr{result}}},
			}, Terminator: ir.Return{}}}}
			if err := (InlineVars{}).Run(Context{}, function); err != nil {
				t.Fatal(err)
			}
			retained := false
			walkExpr(function.Blocks[0].Instructions[2].(ir.Eval).Value, func(expr ir.Expr) {
				if _, ok := expr.(ir.Load); ok {
					retained = true
				}
			})
			if retained != repeated {
				t.Fatalf("retained temporary=%t, repeated=%t", retained, repeated)
			}
			want := float64(20)
			if repeated {
				want = 40
			}
			if got := executeCallValueCheckpoint(t, function); !reflect.DeepEqual(got, []float64{want}) {
				t.Fatalf("nested result=%v, want %g", got, want)
			}
		})
	}
}

func TestInlineVarsKeepsWorkOutsideLoops(t *testing.T) {
	builder := ir.NewBuilder("loop-inlining", ir.Type{})
	entry, loop, exit := builder.NewBlock(), builder.NewBlock(), builder.NewBlock()
	_ = builder.SetEntry(entry)
	_ = builder.SetCurrent(entry)
	rom, err := builder.Memory("EngineRom", ir.Const{}, 1, 0, true, false)
	if err != nil {
		t.Fatal(err)
	}
	counter, err := builder.Memory("LevelMemory", ir.Const{}, 1, 0, true, true)
	if err != nil {
		t.Fatal(err)
	}
	value := builder.NewLocal("invariant", numberType)
	add := func(a, b ir.Expr) ir.Expr {
		return builder.RuntimeCall(resource.RuntimeFunctionAdd, []ir.Expr{a, b}, numberType, true, ir.SourcePos{})
	}
	store := func(place ir.Place, expr ir.Expr) {
		if err := builder.Store([]ir.Place{place}, ir.Value{Type: numberType, Slots: []ir.Expr{expr}}, ir.SourcePos{}); err != nil {
			t.Fatal(err)
		}
	}
	store(ir.Places(value)[0], add(ir.Load{Place: rom}, ir.Const{Value: 1}))
	store(counter, ir.Const{})
	_ = builder.Jump(loop)
	_ = builder.SetCurrent(loop)
	_ = builder.Eval(builder.RuntimeCall(resource.RuntimeFunctionDebugLog, value.Slots, ir.Type{}, false, ir.SourcePos{}))
	store(counter, add(ir.Load{Place: counter}, ir.Const{Value: 1}))
	condition := builder.RuntimeCall(resource.RuntimeFunctionLess, []ir.Expr{ir.Load{Place: counter}, ir.Const{Value: 16}}, numberType, true, ir.SourcePos{})
	_ = builder.Branch(condition, loop, exit)
	_ = builder.SetCurrent(exit)
	_ = builder.Return(ir.Value{Type: ir.Type{}})
	input, err := builder.Finish()
	if err != nil {
		t.Fatal(err)
	}
	steps := make([]int, 2)
	for i, aggressive := range []bool{false, true} {
		optimizer := &Optimizer{level: LevelStandard, passes: []Pass{InlineVars{Aggressive: aggressive}, DeadCodeElimination{}, AllocateBasic{}}}
		fn, err := optimizer.Optimize(Context{Mode: mode.ModePlay, Callback: "preprocess"}, input)
		if err != nil {
			t.Fatal(err)
		}
		argument := fn.Blocks[loop.ID].Instructions[0].(ir.Eval).Value.(ir.RuntimeCall).Args[0]
		if _, inlined := argument.(ir.RuntimeCall); inlined != aggressive {
			t.Fatalf("aggressive=%t inlined=%t", aggressive, inlined)
		}
		tree, _, err := parityBackendTree(fn)
		if err != nil {
			t.Fatal(err)
		}
		nodes, root, err := parseCanonicalTree(tree)
		if err != nil {
			t.Fatal(err)
		}
		result, err := simexec.Execute(nodes, root, simexec.Request{ROM: []byte{0, 0, 0xc0, 0x40}})
		if err != nil {
			t.Fatal(err)
		}
		if len(result.Effects) != 16 || result.Memory[2000][0] != 16 {
			t.Fatalf("loop result=%+v", result)
		}
		for _, effect := range result.Effects {
			if !reflect.DeepEqual(effect.Arguments, []float64{7}) {
				t.Fatalf("effect=%+v", effect)
			}
		}
		steps[i] = result.Steps
	}
	if steps[0] >= steps[1] {
		t.Fatalf("normal inline costs %d steps; aggressive costs %d", steps[0], steps[1])
	}
}

func TestIndexedLocalInitializationUsedByStoreAddress(t *testing.T) {
	index := ir.IndexedLocalPlace{ID: 0, Index: ir.Const{}, Length: 1, Stride: 1}
	for _, destination := range []ir.Place{
		ir.IndexedLocalPlace{ID: 1, Index: ir.Load{Place: index}, Length: 2, Stride: 1},
		ir.MemoryPlace{Storage: "LevelMemory", Index: ir.Load{Place: index}, Stride: 1, Write: true},
	} {
		t.Run(fmt.Sprintf("%T", destination), func(t *testing.T) {
			function := &ir.Function{Locals: []ir.Type{{Slots: 1}, {Slots: 2}}, Blocks: []*ir.Block{{
				ID: 0, Instructions: []ir.Instruction{
					ir.Store{Place: ir.LocalPlace{ID: 0}, Value: ir.Const{Value: 1}},
					ir.Store{Place: destination, Value: ir.Const{Value: 7}},
				}, Terminator: ir.Return{Value: ir.Value{Type: ir.Type{}}},
			}}}
			for _, pass := range []Pass{DeadCodeElimination{}, AdvancedDeadCodeElimination{}} {
				candidate := CloneFunction(function)
				if err := pass.Run(Context{}, candidate); err != nil {
					t.Fatal(err)
				}
				if len(candidate.Blocks[0].Instructions) != 2 {
					t.Fatalf("%s removed the nested index initialization", pass.Name())
				}
			}
			if !indexedLocalIDs(function)[0] {
				t.Fatal("nested indexed read missing from local inventory")
			}
		})
	}
}

// Execute a checkpoint through the real backend. SSA destruction and minimal
// allocation only touch a clone, so they cannot affect the pipeline under test.
func executeCallValueCheckpoint(t *testing.T, function *ir.Function) []float64 {
	t.Helper()
	return executeCheckpointInput(t, function, 1)
}

func executeCheckpointInput(t *testing.T, function *ir.Function, input float64) []float64 {
	t.Helper()
	return executeCheckpointRequest(t, function, simexec.Request{Memory: map[int][]float64{4001: {input}}, StepLimit: 100000})
}

func executeCheckpointRequest(t *testing.T, function *ir.Function, request simexec.Request) []float64 {
	t.Helper()
	final := CloneFunction(function)
	if !final.Allocated {
		if err := (FromSSA{}).Run(Context{}, final); err != nil {
			t.Fatal(err)
		}
		var err error
		final, err = NewOptimizer(LevelMinimal).Optimize(Context{Mode: mode.ModePlay, Callback: "preprocess"}, final)
		if err != nil {
			t.Fatal(err)
		}
	}
	artifacts, err := backend.Compile(&frontend.Project{Modes: map[mode.Mode]*frontend.ModeDeclarations{
		mode.ModePlay: {Mode: mode.ModePlay, Archetypes: []*frontend.ArchetypeDeclaration{{
			Name: "Checkpoint", Callbacks: []*frontend.CallbackDeclaration{{Name: "preprocess", IR: final}},
		}}},
	}})
	if err != nil {
		t.Fatal(err)
	}
	result, err := simexec.Execute(artifacts.Play.Nodes, artifacts.Play.Archetypes[0].Preprocess.Index, request)
	if err != nil {
		t.Fatal(err)
	}
	var logs []float64
	for _, effect := range result.Effects {
		if effect.Function == resource.RuntimeFunctionDebugLog {
			logs = append(logs, effect.Arguments...)
		}
	}
	return logs
}

func TestSwitchRewritePreservesIncomingValues(t *testing.T) {
	load := func(id int) ir.Expr { return ir.Load{Place: ir.SSAPlace{ID: id}} }
	log := func(value ir.Expr) ir.Instruction {
		return ir.Eval{Value: ir.RuntimeCall{Function: resource.RuntimeFunctionDebugLog, Args: []ir.Expr{value}}}
	}
	for _, duplicate := range []bool{false, true} {
		for _, sharedDefault := range []bool{false, true} {
			for _, input := range []float64{0, 1, 2, 3} {
				t.Run(fmt.Sprintf("duplicate=%t/default=%t/input=%g", duplicate, sharedDefault, input), func(t *testing.T) {
					equal := func(value float64) ir.Expr {
						return ir.RuntimeCall{Function: resource.RuntimeFunctionEqual, Args: []ir.Expr{ir.RuntimeCall{Function: resource.RuntimeFunctionGet, Args: []ir.Expr{ir.Const{Value: 4001}, ir.Const{}}, Pure: true, Result: numberType}, ir.Const{Value: value}}, Pure: true, Result: numberType}
					}
					middle := 2.0
					if duplicate {
						middle = 1
					}
					fn := &ir.Function{Locals: []ir.Type{numberType, numberType}, Blocks: []*ir.Block{
						{ID: 0, Instructions: []ir.Instruction{
							ir.Store{Place: ir.SSAPlace{ID: 0}, Value: ir.Const{Value: 10}},
							ir.Store{Place: ir.SSAPlace{ID: 1}, Value: ir.Const{Value: 20}},
							ir.Store{Place: ir.SSAPlace{ID: 2}, Value: ir.Const{Value: 30}},
						}, Terminator: ir.Branch{Condition: equal(1), True: 3, False: 1}},
						{ID: 1, Terminator: ir.Branch{Condition: equal(middle), True: 3, False: 2}},
						{ID: 2, Terminator: ir.Branch{Condition: equal(3), True: 3, False: 4}},
						{ID: 3, Phis: []ir.Phi{
							{Target: ir.SSAPlace{ID: 3}, Local: ir.LocalPlace{ID: 0}, Args: []ir.PhiArg{{Predecessor: 0, Value: ir.SSAPlace{ID: 0}}, {Predecessor: 1, Value: ir.SSAPlace{ID: 1}}, {Predecessor: 2, Value: ir.SSAPlace{ID: 2}}}},
							{Target: ir.SSAPlace{ID: 4}, Local: ir.LocalPlace{ID: 1}, Args: []ir.PhiArg{{Predecessor: 0, Value: ir.SSAPlace{ID: 2}}, {Predecessor: 1, Value: ir.SSAPlace{ID: 0}}, {Predecessor: 2, Value: ir.SSAPlace{ID: 1}}}},
						}, Instructions: []ir.Instruction{log(load(3)), log(load(4))}, Terminator: ir.Return{}},
						{ID: 4, Instructions: []ir.Instruction{log(ir.Const{Value: 99})}, Terminator: ir.Return{}},
					}}
					if sharedDefault {
						fn.Blocks[2].Terminator = ir.Branch{Condition: equal(3), True: 3, False: 3}
					}
					if err := ir.Validate(fn); err != nil {
						t.Fatal(err)
					}
					want := executeCheckpointInput(t, fn, input)
					if err := (RewriteToSwitch{}).Run(Context{}, fn); err != nil {
						t.Fatal(err)
					}
					if err := ir.Validate(fn); err != nil {
						t.Fatal(err)
					}
					if _, ok := fn.Blocks[0].Terminator.(ir.Switch); !ok {
						t.Fatal("chain was not rewritten")
					}
					if got := executeCheckpointInput(t, fn, input); !reflect.DeepEqual(got, want) {
						t.Fatalf("got %v, want %v", got, want)
					}
				})
			}
		}
	}
}

func TestOptimizerControlFixture(t *testing.T) {
	packages, err := source.LoadMode(mode.ModePlay, "../testdata/optimizercontrol")
	if err != nil {
		t.Fatal(err)
	}
	parser := frontend.NewParser()
	if err := parser.Parse(mode.ModePlay, packages[0]); err != nil {
		t.Fatal(err)
	}
	project, err := parser.GetProject()
	if err != nil {
		t.Fatal(err)
	}
	for _, archetype := range project.Modes[mode.ModePlay].Archetypes {
		inputs := []float64{1.0 / 540, 1}
		if archetype.Name == "Split" {
			inputs = []float64{1000000}
		}
		for _, input := range inputs {
			t.Run(fmt.Sprintf("%s/%g", archetype.Name, input), func(t *testing.T) {
				original := archetype.Callbacks[0].IR
				want := executeCheckpointInput(t, original, input)
				if archetype.Name == "Gauge" {
					ids := []float64{5556, 5557, 5558, 5562, 5563, 5564, 5565, 5566, 5567, 5571, 5572, 5573}
					if len(want) != 85 || want[0] != 12 {
						t.Fatalf("gauge: %v", want)
					}
					for i, id := range ids {
						if want[1+i*7] != id {
							t.Fatalf("sprite %d: %g, want %g", i, want[1+i*7], id)
						}
					}
				} else if !reflect.DeepEqual(want, []float64{1000000, 0, 1000000, 0}) {
					t.Fatalf("binary64 split: %v", want)
				}
				check := func(label string, fn *ir.Function) {
					t.Helper()
					got := executeCheckpointInput(t, fn, input)
					if len(got) != len(want) {
						t.Fatalf("%s: got %v, want %v", label, got, want)
					}
					for i := range got {
						if math.Abs(got[i]-want[i]) > 1e-12 {
							t.Fatalf("%s observation %d: %.17g != %.17g", label, i, got[i], want[i])
						}
					}
				}
				for _, level := range []Level{LevelMinimal, LevelFast, LevelStandard} {
					fn, err := NewOptimizer(level).Optimize(Context{Mode: mode.ModePlay, Callback: "preprocess"}, original)
					if err != nil {
						t.Fatal(err)
					}
					check(fmt.Sprint(level), fn)
				}
				fn := CloneFunction(original)
				for index, pass := range NewOptimizer(LevelStandard).passes {
					if err := pass.Run(Context{}, fn); err != nil {
						t.Fatalf("%s: %v", pass.Name(), err)
					}
					if err := ir.Validate(fn); err != nil {
						t.Fatalf("%s: %v", pass.Name(), err)
					}
					check(fmt.Sprintf("%d/%s", index+1, pass.Name()), fn)
				}
			})
		}
	}
}

func TestSwitchRewriteLoopAndEffects(t *testing.T) {
	load := func(id int) ir.Expr { return ir.Load{Place: ir.SSAPlace{ID: id}} }
	equal := func(value ir.Expr, n float64) ir.Expr {
		return ir.RuntimeCall{Function: resource.RuntimeFunctionEqual, Args: []ir.Expr{value, ir.Const{Value: n}}, Pure: true, Result: numberType}
	}
	fn := &ir.Function{Locals: []ir.Type{numberType}, Blocks: []*ir.Block{
		{ID: 0, Instructions: []ir.Instruction{
			ir.Store{Place: ir.SSAPlace{ID: 0}, Value: ir.Const{Value: 2}},
			ir.Store{Place: ir.SSAPlace{ID: 1}, Value: ir.Const{Value: 1}},
			ir.Store{Place: ir.SSAPlace{ID: 2}, Value: ir.Const{}},
		}, Terminator: ir.Jump{Target: 1}},
		{ID: 1, Phis: []ir.Phi{{Target: ir.SSAPlace{ID: 3}, Local: ir.LocalPlace{ID: 0}, Args: []ir.PhiArg{
			{Predecessor: 0, Value: ir.SSAPlace{ID: 0}}, {Predecessor: 1, Value: ir.SSAPlace{ID: 2}}, {Predecessor: 2, Value: ir.SSAPlace{ID: 1}},
		}}}, Terminator: ir.Branch{Condition: equal(load(3), 1), True: 1, False: 2}},
		{ID: 2, Terminator: ir.Branch{Condition: equal(load(3), 2), True: 1, False: 3}},
		{ID: 3, Instructions: []ir.Instruction{ir.Eval{Value: ir.RuntimeCall{Function: resource.RuntimeFunctionDebugLog, Args: []ir.Expr{load(3)}}}}, Terminator: ir.Return{}},
	}}
	if err := ir.Validate(fn); err != nil {
		t.Fatal(err)
	}
	if err := (RewriteToSwitch{}).Run(Context{}, fn); err != nil {
		t.Fatal(err)
	}
	if err := ir.Validate(fn); err != nil {
		t.Fatal(err)
	}
	if got := executeCallValueCheckpoint(t, fn); !reflect.DeepEqual(got, []float64{0}) {
		t.Fatalf("loop result %v", got)
	}

	// A pure Equal wrapper must not hide the effectful discriminant. Both
	// comparisons execute DebugLog; replacing them by one switch would lose one.
	effect := ir.RuntimeCall{Function: resource.RuntimeFunctionDebugLog, Args: []ir.Expr{ir.Const{Value: 9}}, Result: numberType}
	fn = &ir.Function{Blocks: []*ir.Block{
		{ID: 0, Terminator: ir.Branch{Condition: equal(effect, 1), True: 2, False: 1}},
		{ID: 1, Terminator: ir.Branch{Condition: equal(effect, 2), True: 2, False: 2}},
		{ID: 2, Terminator: ir.Return{}},
	}}
	if err := (RewriteToSwitch{}).Run(Context{}, fn); err != nil {
		t.Fatal(err)
	}
	if _, ok := fn.Blocks[0].Terminator.(ir.Branch); !ok {
		t.Fatal("effectful comparisons were merged")
	}
	if got := executeCallValueCheckpoint(t, fn); !reflect.DeepEqual(got, []float64{9, 9}) {
		t.Fatalf("effects %v", got)
	}
}

func TestCallValuePipelineCheckpoints(t *testing.T) {
	packages, err := source.LoadMode(mode.ModePlay, "../testdata/callvalues")
	if err != nil {
		t.Fatal(err)
	}
	parser := frontend.NewParser()
	if err := parser.Parse(mode.ModePlay, packages[0]); err != nil {
		t.Fatal(err)
	}
	project, err := parser.GetProject()
	if err != nil {
		t.Fatal(err)
	}
	selected := map[string]struct {
		index int
		want  float64
	}{
		"Parameter": {0, 324}, "Return": {0, 324}, "Direct": {0, 324},
		"Combined20": {19, 324}, "Combined23": {19, 324},
		"Skipped": {0, 1}, "Scalars": {3, 8},
	}
	for _, archetype := range project.Modes[mode.ModePlay].Archetypes {
		observation, ok := selected[archetype.Name]
		if !ok {
			continue
		}
		t.Run(archetype.Name, func(t *testing.T) {
			function := CloneFunction(archetype.Callbacks[0].IR)
			check := func(label string) {
				t.Helper()
				logs := executeCallValueCheckpoint(t, function)
				if observation.index >= len(logs) {
					t.Fatalf("%s: missing observation %d in %v", label, observation.index, logs)
				}
				if got := logs[observation.index]; got != observation.want {
					t.Fatalf("%s: want %g, got %g", label, observation.want, got)
				}
			}
			if archetype.Name == "Skipped" {
				for _, block := range function.Blocks {
					for index, instruction := range block.Instructions {
						if store, ok := instruction.(ir.Store); ok {
							if place, ok := store.Place.(ir.LocalPlace); ok && place.Name == "value" {
								t.Logf("frontend parameter local %d: block %d instruction %d value=%s", place.ID, block.ID, index, exprKey(store.Value))
							}
						}
					}
				}
			}
			for id, typ := range function.Locals {
				if typ.Slots != 48 {
					continue
				}
				fixed, dynamic := 0, 0
				for _, block := range function.Blocks {
					for _, instruction := range block.Instructions {
						if store, ok := instruction.(ir.Store); ok {
							switch place := store.Place.(type) {
							case ir.LocalPlace:
								if place.ID == id && place.Offset == 0 {
									fixed++
								}
							case ir.IndexedLocalPlace:
								if place.ID == id {
									dynamic++
								}
							}
						}
					}
				}
				t.Logf("frontend local %d: slot-0 definitions=%d, indexed writes=%d", id, fixed, dynamic)
			}
			check("frontend checkpoint")
			context := Context{Mode: mode.ModePlay, Callback: "preprocess", analyses: newAnalysisManager()}
			for index, pass := range NewOptimizer(LevelStandard).passes {
				managed := pass.(ManagedPass)
				for _, analysis := range managed.Requires() {
					if err := context.analyses.ensure(analysis, function); err != nil {
						t.Fatal(err)
					}
				}
				if err := pass.Run(context, function); err != nil {
					t.Fatal(err)
				}
				if err := ir.Validate(function); err != nil {
					t.Fatal(err)
				}
				check(fmt.Sprintf("checkpoint after pass %d %s", index+1, pass.Name()))
				context.analyses.invalidateExcept(managed.Preserves())
				for _, analysis := range managed.Destroys() {
					delete(context.analyses.values, analysis)
				}
			}
		})
	}
}

func TestFrozenAddressFrontendCheckpoint(t *testing.T) {
	packages, err := source.LoadMode(mode.ModePlay, "../testdata/freezeaddress")
	if err != nil {
		t.Fatal(err)
	}
	parser := frontend.NewParser()
	if err := parser.Parse(mode.ModePlay, packages[0]); err != nil {
		t.Fatal(err)
	}
	project, err := parser.GetProject()
	if err != nil {
		t.Fatal(err)
	}
	for _, archetype := range project.Modes[mode.ModePlay].Archetypes {
		if archetype.Name != "Memory" {
			continue
		}
		function := archetype.Callbacks[0].IR
		got := executeCallValueCheckpoint(t, function)
		want := []float64{21, 22, 23, 21, 22, 23, 21, 22, 23, 21, 22, 23, 21, 22, 23, 61, 62, 63}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("frontend checkpoint: want %v, got %v", want, got)
		}
		return
	}
	t.Fatal("missing Memory callback")
}

func allocatedFunction(slots int) *ir.Function {
	return &ir.Function{
		Name: "allocation", Entry: 0, Result: ir.Type{},
		Locals: []ir.Type{{Name: "local", Slots: slots}},
		Blocks: []*ir.Block{{ID: 0, Terminator: ir.Return{Value: ir.Value{Type: ir.Type{}}}}},
	}
}

func TestSCCPTracksExecutablePhiEdges(t *testing.T) {
	number := ir.Type{Name: "number", Slots: 1}
	ssa := func(id int) ir.SSAPlace { return ir.SSAPlace{ID: id} }
	fn := &ir.Function{Name: "sccp", Entry: 0, Result: number, Blocks: []*ir.Block{
		{ID: 0, Instructions: []ir.Instruction{ir.Store{Place: ssa(1), Value: ir.Const{Value: 1}}}, Terminator: ir.Branch{Condition: ir.Load{Place: ssa(1)}, True: 1, False: 2}},
		{ID: 1, Instructions: []ir.Instruction{ir.Store{Place: ssa(2), Value: ir.Const{Value: 2}}}, Terminator: ir.Jump{Target: 3}},
		{ID: 2, Instructions: []ir.Instruction{ir.Store{Place: ssa(3), Value: ir.Const{Value: 3}}}, Terminator: ir.Jump{Target: 3}},
		{ID: 3, Phis: []ir.Phi{{Target: ssa(4), Args: []ir.PhiArg{{Predecessor: 1, Value: ssa(2)}, {Predecessor: 2, Value: ssa(3)}}}}, Terminator: ir.Return{Value: ir.Value{Type: number, Slots: []ir.Expr{ir.Load{Place: ssa(4)}}}}},
	}}
	if err := (SparseConditionalConstantPropagation{}).Run(Context{}, fn); err != nil {
		t.Fatal(err)
	}
	if err := (RemoveUnreachable{}).Run(Context{}, fn); err != nil {
		t.Fatal(err)
	}
	ret := fn.Blocks[len(fn.Blocks)-1].Terminator.(ir.Return)
	if value, ok := ret.Value.Slots[0].(ir.Const); !ok || value.Value != 2 {
		t.Fatalf("return = %#v", ret.Value.Slots)
	}
}

func TestDeadCodeEliminationPreservesIndexedLocalInitialization(t *testing.T) {
	makeFunction := func() *ir.Function {
		return &ir.Function{
			Name:   "indexed-initialization",
			Entry:  0,
			Locals: []ir.Type{{Name: "values", Slots: 2}, {Name: "index", Slots: 1}},
			Blocks: []*ir.Block{{
				ID: 0,
				Instructions: []ir.Instruction{
					ir.Store{Place: ir.LocalPlace{ID: 0, Offset: 0}, Value: ir.Const{Value: 3}},
					ir.Store{Place: ir.LocalPlace{ID: 0, Offset: 1}, Value: ir.Const{Value: 5}},
					ir.Eval{Value: ir.RuntimeCall{
						Function: resource.RuntimeFunctionDebugLog,
						Args: []ir.Expr{ir.Load{Place: ir.IndexedLocalPlace{
							ID: 0, Length: 2, Stride: 1, Index: ir.Load{Place: ir.LocalPlace{ID: 1}},
						}}},
						Result: ir.Type{},
						Pure:   false,
					}},
				},
				Terminator: ir.Return{Value: ir.Value{Type: ir.Type{}}},
			}},
		}
	}
	for _, pass := range []Pass{DeadCodeElimination{}, AdvancedDeadCodeElimination{}} {
		function := makeFunction()
		if err := pass.Run(Context{}, function); err != nil {
			t.Fatalf("%s: %v", pass.Name(), err)
		}
		if got := len(function.Blocks[0].Instructions); got != 3 {
			t.Fatalf("%s retained %d instructions, want 3", pass.Name(), got)
		}
	}
}

func TestDeadCodeEliminationPreservesDynamicStoreAddress(t *testing.T) {
	index := ir.LocalPlace{ID: 0, Name: "entity"}
	function := &ir.Function{
		Name: "dynamic-store-address", Entry: 0, Result: ir.Type{},
		Locals: []ir.Type{{Name: "entity", Slots: 1}},
		Blocks: []*ir.Block{{
			ID: 0,
			Instructions: []ir.Instruction{
				ir.Store{Place: index, Value: ir.Const{Value: 2}},
				ir.Store{Place: ir.MemoryPlace{Storage: "EntitySharedMemoryArray", Index: ir.Load{Place: index}, Stride: 32, Write: true}, Value: ir.Const{Value: -1}},
			},
			Terminator: ir.Return{Value: ir.Value{Type: ir.Type{}}},
		}},
	}
	if err := (DeadCodeElimination{}).Run(Context{}, function); err != nil {
		t.Fatal(err)
	}
	if got := len(function.Blocks[0].Instructions); got != 2 {
		t.Fatalf("DCE retained %d instructions, want the address definition and semantic store", got)
	}
}

func TestSCCPDoesNotFoldNonFiniteSensitiveRuntimeCalls(t *testing.T) {
	number := ir.Type{Name: "number", Slots: 1}
	call := ir.RuntimeCall{Function: resource.RuntimeFunctionDivide, Args: []ir.Expr{ir.Const{}, ir.Const{}}, Result: number, Pure: true}
	fn := &ir.Function{Name: "nan", Entry: 0, Result: number, Blocks: []*ir.Block{{ID: 0, Terminator: ir.Return{Value: ir.Value{Type: number, Slots: []ir.Expr{call}}}}}}
	if err := (SparseConditionalConstantPropagation{}).Run(Context{}, fn); err != nil {
		t.Fatal(err)
	}
	if _, ok := fn.Blocks[0].Terminator.(ir.Return).Value.Slots[0].(ir.RuntimeCall); !ok {
		t.Fatalf("non-finite call folded: %#v", fn.Blocks[0].Terminator)
	}
	if _, ok := evaluateRuntime(resource.RuntimeFunctionAdd, []float64{math.Inf(1), 1}); ok {
		t.Fatal("non-finite input was accepted")
	}
}

func TestSCCPUsesJavaScriptRoundAndModuloSemantics(t *testing.T) {
	for _, test := range []struct {
		function resource.RuntimeFunction
		args     []float64
		want     float64
	}{
		{resource.RuntimeFunctionRound, []float64{2.5}, 3},
		{resource.RuntimeFunctionRound, []float64{3.5}, 4},
		{resource.RuntimeFunctionRound, []float64{-1.5}, -1},
		{resource.RuntimeFunctionMod, []float64{-3, 2}, 1},
		{resource.RuntimeFunctionRem, []float64{-3, 2}, -1},
	} {
		got, ok := evaluateRuntime(test.function, test.args)
		if !ok || got != test.want {
			t.Fatalf("%s%v = %v, %t; want %v", test.function, test.args, got, ok, test.want)
		}
	}
}

func TestAllocateBasicTemporaryMemoryBoundary(t *testing.T) {
	for _, test := range []struct {
		slots   int
		wantErr bool
	}{{0, false}, {4095, false}, {4096, false}, {4097, true}} {
		fn := allocatedFunction(test.slots)
		err := (AllocateBasic{}).Run(Context{}, fn)
		if (err != nil) != test.wantErr {
			t.Fatalf("slots %d error = %v", test.slots, err)
		}
		if err == nil && !fn.Allocated {
			t.Fatalf("slots %d not marked allocated", test.slots)
		}
		if err != nil && (!strings.Contains(err.Error(), "4097") || !strings.Contains(err.Error(), "4096")) {
			t.Fatalf("boundary error = %v", err)
		}
	}
}

func TestTryAllocateBasicFallsBackToLivenessReuse(t *testing.T) {
	locals := make([]ir.Type, 5000)
	instructions := make([]ir.Instruction, len(locals))
	for i := range locals {
		locals[i] = ir.Type{Name: "scalar", Slots: 1}
		instructions[i] = ir.Store{Place: ir.LocalPlace{ID: i}, Value: ir.Const{Value: float64(i)}}
	}
	fn := &ir.Function{
		Name: "fast", Entry: 0, Result: ir.Type{},
		Locals: locals,
		Blocks: []*ir.Block{{ID: 0, Instructions: instructions, Terminator: ir.Return{Value: ir.Value{Type: ir.Type{}}}}},
	}
	if err := (TryAllocateBasic{}).Run(Context{}, fn); err != nil {
		t.Fatal(err)
	}
	if len(fn.Locals) != 1 || fn.Locals[0].Slots != 1 {
		t.Fatalf("physical locals = %#v", fn.Locals)
	}
}

func TestAllocateRetriesAfterConservativeInterferenceExhaustsSlots(t *testing.T) {
	const count = TemporaryMemorySlots + 1
	locals := make([]ir.Type, count)
	instructions := make([]ir.Instruction, count)
	for index := range locals {
		locals[index] = ir.Type{Name: "scalar", Slots: 1}
		instructions[index] = ir.Store{Place: ir.LocalPlace{ID: index}, Value: ir.Const{Value: float64(index)}}
	}
	function := &ir.Function{
		Name: "conservative", Entry: 0, Result: ir.Type{}, Locals: locals,
		Blocks: []*ir.Block{{ID: 0, Instructions: instructions, Terminator: ir.Return{Value: ir.Value{Type: ir.Type{}}}}},
	}
	conservative := newInterferenceGraph(count)
	all := newBitSet(count)
	for index := range count {
		all.set(index)
	}
	addClique(conservative, all)
	context := Context{analyses: &analysisManager{values: map[Analysis]any{AnalysisLiveness: conservative}}}
	if err := (Allocate{}).Run(context, function); err != nil {
		t.Fatal(err)
	}
	if len(function.Locals) != 1 || function.Locals[0].Slots != 1 {
		t.Fatalf("physical locals = %#v", function.Locals)
	}
	t.Run("dead-stores-within-limit", func(t *testing.T) {
		builder := ir.NewBuilder("dead-stores", ir.Type{})
		entry := builder.NewBlock()
		_ = builder.SetEntry(entry)
		_ = builder.SetCurrent(entry)
		var live ir.Value
		for index := range 3 {
			local := builder.NewLocal(fmt.Sprintf("value%d", index), numberType)
			if index == 0 {
				live = local
			}
			if err := builder.Store(ir.Places(local), ir.Value{Type: numberType, Slots: []ir.Expr{ir.Const{Value: float64(index + 1)}}}, ir.SourcePos{}); err != nil {
				t.Fatal(err)
			}
		}
		_ = builder.Eval(builder.RuntimeCall(resource.RuntimeFunctionDebugLog, live.Slots, ir.Type{}, false, ir.SourcePos{}))
		_ = builder.Return(ir.Value{Type: ir.Type{}})
		input, err := builder.Finish()
		if err != nil {
			t.Fatal(err)
		}
		// Populate the cached graph before removing dead writes. Allocation
		// must use the new lifetimes even when the old graph fits in 4096.
		optimizer := &Optimizer{level: LevelStandard, passes: []Pass{CopyCoalesce{}, AdvancedDeadCodeElimination{}, Allocate{}}}
		result, err := optimizer.Optimize(Context{Mode: mode.ModePlay, Callback: "preprocess"}, input)
		if err != nil {
			t.Fatal(err)
		}
		if result.Locals[0].Slots != 1 {
			t.Fatalf("stale interference retained %d slots", result.Locals[0].Slots)
		}
	})
}

func TestDeadCodeEliminationPropagatesDemandAcrossCycles(t *testing.T) {
	for _, observed := range []bool{false, true} {
		t.Run(fmt.Sprintf("observed=%t", observed), func(t *testing.T) {
			builder := ir.NewBuilder("demand-cycle", ir.Type{})
			entry, exit := builder.NewBlock(), builder.NewBlock()
			_ = builder.SetEntry(entry)
			_ = builder.SetCurrent(entry)
			const count = 32
			values := make([]ir.Value, count)
			blocks := make([]*ir.Block, count)
			for i := range count {
				values[i] = builder.NewLocal(fmt.Sprintf("value%d", i), numberType)
				blocks[i] = builder.NewBlock()
			}
			counter := builder.NewLocal("counter", numberType)
			_ = builder.Store(ir.Places(counter), ir.ZeroValue(numberType), ir.SourcePos{})
			_ = builder.Store(ir.Places(values[count-1]), ir.ZeroValue(numberType), ir.SourcePos{})
			_ = builder.Jump(blocks[0])
			for i, block := range blocks {
				_ = builder.SetCurrent(block)
				value := values[(i+count-1)%count]
				if i == 0 {
					value = ir.Value{Type: numberType, Slots: []ir.Expr{builder.RuntimeCall(resource.RuntimeFunctionAdd, []ir.Expr{value.Slots[0], ir.Const{Value: 1}}, numberType, true, ir.SourcePos{})}}
				}
				_ = builder.Store(ir.Places(values[i]), value, ir.SourcePos{})
				if i+1 < count {
					_ = builder.Jump(blocks[i+1])
					continue
				}
				if observed {
					_ = builder.Eval(builder.RuntimeCall(resource.RuntimeFunctionDebugLog, values[i].Slots, ir.Type{}, false, ir.SourcePos{}))
				}
				increment := builder.RuntimeCall(resource.RuntimeFunctionAdd, []ir.Expr{counter.Slots[0], ir.Const{Value: 1}}, numberType, true, ir.SourcePos{})
				_ = builder.Store(ir.Places(counter), ir.Value{Type: numberType, Slots: []ir.Expr{increment}}, ir.SourcePos{})
				condition := builder.RuntimeCall(resource.RuntimeFunctionLess, []ir.Expr{counter.Slots[0], ir.Const{Value: 3}}, numberType, true, ir.SourcePos{})
				_ = builder.Branch(condition, blocks[0], exit)
			}
			_ = builder.SetCurrent(exit)
			_ = builder.Eval(builder.RuntimeCall(resource.RuntimeFunctionDebugLog, counter.Slots, ir.Type{}, false, ir.SourcePos{}))
			_ = builder.Return(ir.Value{Type: ir.Type{}})
			function, err := builder.Finish()
			if err != nil {
				t.Fatal(err)
			}
			t.Run("ssa", func(t *testing.T) {
				ssa := CloneFunction(function)
				context := Context{Mode: mode.ModePlay, Callback: "preprocess"}
				for _, pass := range []Pass{ToSSA{}, DeadCodeElimination{}} {
					if err := pass.Run(context, ssa); err != nil {
						t.Fatal(err)
					}
					if err := ir.Validate(ssa); err != nil {
						t.Fatal(err)
					}
				}
				if !observed {
					for _, block := range ssa.Blocks {
						for _, phi := range block.Phis {
							if phi.Target.ID < count {
								t.Fatal("unobserved Phi cycle retained")
							}
						}
					}
				}
				want := []float64{3}
				if observed {
					want = []float64{1, 2, 3, 3}
				}
				if got := executeCallValueCheckpoint(t, ssa); !reflect.DeepEqual(got, want) {
					t.Fatalf("SSA final node logs = %v, want %v", got, want)
				}
			})
			if err := (AdvancedDeadCodeElimination{}).Run(Context{}, function); err != nil {
				t.Fatal(err)
			}
			if !observed {
				for _, block := range function.Blocks {
					for _, instruction := range block.Instructions {
						if store, ok := instruction.(ir.Store); ok && store.Place.(ir.LocalPlace).ID < count {
							t.Fatal("unobserved value cycle retained after one pass")
						}
					}
				}
			}
			want := []float64{3}
			if observed {
				want = []float64{1, 2, 3, 3}
			}
			if got := executeCallValueCheckpoint(t, function); !reflect.DeepEqual(got, want) {
				t.Fatalf("final node logs = %v, want %v", got, want)
			}
		})
	}
}

func TestAllocationRewritesDynamicLocalBaseAndIndex(t *testing.T) {
	fn := &ir.Function{
		Name: "indexed", Entry: 0, Result: ir.Type{},
		Locals: []ir.Type{{Name: "array", Slots: 4}, {Name: "index", Slots: 1}},
		Blocks: []*ir.Block{{ID: 0, Instructions: []ir.Instruction{ir.Store{
			Place: ir.IndexedLocalPlace{ID: 0, Base: 1, Length: 3, Stride: 1, Index: ir.Load{Place: ir.LocalPlace{ID: 1}}},
			Value: ir.Const{Value: 7},
		}}, Terminator: ir.Return{Value: ir.Value{Type: ir.Type{}}}}},
	}
	if err := (AllocateBasic{}).Run(Context{}, fn); err != nil {
		t.Fatal(err)
	}
	store := fn.Blocks[0].Instructions[0].(ir.Store)
	place := store.Place.(ir.IndexedLocalPlace)
	index := place.Index.(ir.Load).Place.(ir.LocalPlace)
	if place.ID != 0 || place.Base != 1 || index.ID != 0 || index.Offset != 4 {
		t.Fatalf("rewritten place = %#v index = %#v", place, index)
	}
}

func TestSingleStoreAggregatePreservesIndexSnapshots(t *testing.T) {
	for _, name := range []string{"same-index", "log", "nested-log-write", "changed-local-index", "changed-memory-index", "different-index", "cross-block", "read-before-store"} {
		for _, fast := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/fast=%t", name, fast), func(t *testing.T) {
				builder := ir.NewBuilder(name, voidType)
				entry := builder.NewBlock()
				_ = builder.SetEntry(entry)
				_ = builder.SetCurrent(entry)
				store := func(place ir.Place, value ir.Expr) {
					_ = builder.Store([]ir.Place{place}, ir.Value{Type: numberType, Slots: []ir.Expr{value}}, ir.SourcePos{})
				}
				log := func(place ir.Place) {
					_ = builder.Eval(builder.RuntimeCall(resource.RuntimeFunctionDebugLog, []ir.Expr{ir.Load{Place: place}}, voidType, false, ir.SourcePos{}))
				}
				typ := ir.Type{Slots: 2}
				a, b := builder.NewLocal("a", typ), builder.NewLocal("b", typ)
				memory, err := builder.Memory("memory", ir.Const{}, 1, 0, true, true)
				if err != nil {
					t.Fatal(err)
				}
				var index ir.Expr = ir.Load{Place: memory}
				var indexLocal ir.Place
				if name == "changed-local-index" {
					local := builder.NewLocal("index", numberType)
					index, indexLocal = local.Slots[0], ir.Places(local)[0]
					store(indexLocal, ir.Const{})
				}
				store(ir.Places(a)[0], ir.Const{Value: 7})
				store(ir.Places(a)[1], ir.Const{Value: 8})
				log(ir.Places(a)[1])
				address, err := builder.IndexedLocal(ir.Places(b)[0].(ir.LocalPlace), index, 2, 1, 0)
				if err != nil {
					t.Fatal(err)
				}
				want := []float64{8, 99}
				if name == "read-before-store" {
					log(address)
					want = []float64{8, 0, 99}
				}
				store(address, ir.Const{Value: 99})
				switch name {
				case "log":
					log(memory)
					want = []float64{8, 0, 99}
				case "nested-log-write":
					write := builder.RuntimeCall(resource.RuntimeFunctionSet, []ir.Expr{ir.Const{Value: 4000}, ir.Const{}, ir.Const{Value: 1}}, numberType, false, ir.SourcePos{})
					_ = builder.Eval(builder.RuntimeCall(resource.RuntimeFunctionDebugLog, []ir.Expr{write}, voidType, false, ir.SourcePos{}))
					want = []float64{8, 1, 0}
				case "changed-local-index":
					store(indexLocal, ir.Const{Value: 1})
					want = []float64{8, 0}
				case "changed-memory-index":
					store(memory, ir.Const{Value: 1})
					want = []float64{8, 0}
				case "different-index":
					address.Index = ir.Const{Value: 1}
					want = []float64{8, 0}
				case "cross-block":
					next := builder.NewBlock()
					_ = builder.Jump(next)
					_ = builder.SetCurrent(next)
				}
				log(address)
				_ = builder.Return(ir.Value{Type: voidType})
				function, err := builder.Finish()
				if err != nil {
					t.Fatal(err)
				}
				if err := allocateLocals(function, true, fast); err != nil {
					t.Fatal(err)
				}
				if (name == "same-index" || name == "log") && function.Locals[0].Slots != 2 {
					t.Fatalf("single-store aggregate failed to reuse: %d slots", function.Locals[0].Slots)
				}
				if got := executeCallValueCheckpoint(t, function); !reflect.DeepEqual(got, want) {
					t.Fatalf("final node logs = %v, want %v", got, want)
				}
			})
		}
	}
}

func TestAggregateAllocationPreservesInitializationReads(t *testing.T) {
	for _, name := range []string{"complete", "partial", "duplicate-slot", "read-during-overwrite", "read-in-first-store", "indexed-write"} {
		for _, fast := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/fast=%t", name, fast), func(t *testing.T) {
				builder := ir.NewBuilder(name, ir.Type{})
				entry := builder.NewBlock()
				_ = builder.SetEntry(entry)
				_ = builder.SetCurrent(entry)
				typ := ir.Type{Name: "pair", Slots: 2}
				a, b := builder.NewLocal("a", typ), builder.NewLocal("b", typ)
				placesA, placesB := ir.Places(a), ir.Places(b)
				store := func(place ir.Place, value ir.Expr) {
					if err := builder.Store([]ir.Place{place}, ir.Value{Type: numberType, Slots: []ir.Expr{value}}, ir.SourcePos{}); err != nil {
						t.Fatal(err)
					}
				}
				log := func(place ir.Place) {
					if err := builder.Eval(builder.RuntimeCall(resource.RuntimeFunctionDebugLog, []ir.Expr{ir.Load{Place: place}}, ir.Type{}, false, ir.SourcePos{})); err != nil {
						t.Fatal(err)
					}
				}
				store(placesA[0], ir.Const{Value: 7})
				store(placesA[1], ir.Const{Value: 8})
				var want []float64
				wantSlots := 4
				switch name {
				case "complete", "partial", "duplicate-slot":
					log(placesA[1])
					store(placesB[0], ir.Const{Value: 20})
					if name == "complete" {
						store(placesB[1], ir.Const{Value: 21})
						want, wantSlots = []float64{8, 21}, 2
					} else {
						if name == "duplicate-slot" {
							store(placesB[0], ir.Const{Value: 21})
						}
						want = []float64{8, 0}
					}
					log(placesB[1])
				default:
					store(placesB[0], ir.Const{Value: 10})
					store(placesB[1], ir.Const{Value: 11})
					log(placesB[1])
					value := ir.Expr(ir.Const{Value: 20})
					if name == "read-in-first-store" {
						value = ir.Load{Place: placesA[1]}
					}
					store(placesA[0], value)
					if name == "read-during-overwrite" {
						log(placesA[1])
						want = []float64{11, 8, 21}
					} else if name == "read-in-first-store" {
						want = []float64{11, 8, 21}
					} else {
						want = []float64{11, 20, 21}
					}
					last := placesA[1]
					if name == "indexed-write" {
						var err error
						last, err = builder.IndexedLocal(placesA[0].(ir.LocalPlace), ir.Const{Value: 1}, 2, 1, 0)
						if err != nil {
							t.Fatal(err)
						}
						// A is fully overwritten after B dies in this case.
						wantSlots = 4 // Dynamic writes deliberately stay conservative.
					}
					store(last, ir.Const{Value: 21})
					if name != "read-during-overwrite" {
						log(placesA[0])
					}
					log(placesA[1])
				}
				_ = builder.Return(ir.Value{Type: ir.Type{}})
				fn, err := builder.Finish()
				if err != nil {
					t.Fatal(err)
				}
				if err := allocateLocals(fn, true, fast); err != nil {
					t.Fatal(err)
				}
				if fn.Locals[0].Slots != wantSlots {
					t.Fatalf("slots=%d want=%d", fn.Locals[0].Slots, wantSlots)
				}
				tree, _, err := parityBackendTree(fn)
				if err != nil {
					t.Fatal(err)
				}
				nodes, root, err := parseCanonicalTree(tree)
				if err != nil {
					t.Fatal(err)
				}
				result, err := simexec.Execute(nodes, root, simexec.Request{})
				if err != nil {
					t.Fatal(err)
				}
				var got []float64
				for _, effect := range result.Effects {
					got = append(got, effect.Arguments...)
				}
				if !reflect.DeepEqual(got, want) {
					t.Fatalf("effects=%v want=%v", got, want)
				}
			})
		}
	}
}

func TestFromSSAPreservesParallelPhiAssignments(t *testing.T) {
	number := ir.Type{Slots: 1}
	load := func(id int) ir.Expr { return ir.Load{Place: ir.SSAPlace{ID: id}} }
	function := &ir.Function{Name: "phi-swap", Blocks: []*ir.Block{
		{ID: 0, Instructions: []ir.Instruction{
			ir.Store{Place: ir.SSAPlace{ID: 0}, Value: ir.Const{Value: 1}},
			ir.Store{Place: ir.SSAPlace{ID: 1}, Value: ir.Const{Value: 2}},
			ir.Store{Place: ir.SSAPlace{ID: 2}, Value: ir.Const{}},
		}, Terminator: ir.Jump{Target: 1}},
		{ID: 1, Phis: []ir.Phi{
			{Target: ir.SSAPlace{ID: 3}, Args: []ir.PhiArg{{Predecessor: 0, Value: ir.SSAPlace{ID: 0}}, {Predecessor: 1, Value: ir.SSAPlace{ID: 4}}}},
			{Target: ir.SSAPlace{ID: 4}, Args: []ir.PhiArg{{Predecessor: 0, Value: ir.SSAPlace{ID: 1}}, {Predecessor: 1, Value: ir.SSAPlace{ID: 3}}}},
			{Target: ir.SSAPlace{ID: 5}, Args: []ir.PhiArg{{Predecessor: 0, Value: ir.SSAPlace{ID: 2}}, {Predecessor: 1, Value: ir.SSAPlace{ID: 6}}}},
		}, Instructions: []ir.Instruction{
			ir.Eval{Value: ir.RuntimeCall{Function: resource.RuntimeFunctionDebugLog, Args: []ir.Expr{load(3)}}},
			ir.Eval{Value: ir.RuntimeCall{Function: resource.RuntimeFunctionDebugLog, Args: []ir.Expr{load(4)}}},
			ir.Store{Place: ir.SSAPlace{ID: 6}, Value: ir.RuntimeCall{Function: resource.RuntimeFunctionAdd, Args: []ir.Expr{load(5), ir.Const{Value: 1}}, Result: number, Pure: true}},
		}, Terminator: ir.Branch{Condition: ir.RuntimeCall{Function: resource.RuntimeFunctionLess, Args: []ir.Expr{load(6), ir.Const{Value: 2}}, Result: number, Pure: true}, True: 1, False: 2}},
		{ID: 2, Terminator: ir.Return{}},
	}}
	if got := executeCallValueCheckpoint(t, function); !reflect.DeepEqual(got, []float64{1, 2, 2, 1}) {
		t.Fatalf("Phi assignments were not simultaneous: %v", got)
	}
	// A chain also needs parallel assignment semantics: x = next, y = old x.
	// Saving incoming values only for overwritten destinations is insufficient
	// if x is restored before y reads it.
	chain := CloneFunction(function)
	chain.Blocks[0].Instructions[0] = ir.Store{Place: ir.SSAPlace{ID: 0}, Value: ir.Const{Value: 7}}
	chain.Blocks[1].Phis[0].Args[1].Value = ir.SSAPlace{ID: 6}
	if got := executeCallValueCheckpoint(t, chain); !reflect.DeepEqual(got, []float64{7, 2, 1, 7}) {
		t.Fatalf("Phi copy chain lost its old value: %v", got)
	}
}

func TestFromSSAMaterializesConstantInputsOnSelectedEdges(t *testing.T) {
	function := &ir.Function{Name: "phi-constants", Blocks: []*ir.Block{
		{ID: 0, Instructions: []ir.Instruction{
			ir.Store{Place: ir.SSAPlace{ID: 0}, Value: ir.Const{Value: 7}},
			ir.Store{Place: ir.SSAPlace{ID: 1}, Value: ir.Const{Value: 9}},
		}, Terminator: ir.Branch{Condition: ir.Load{Place: ir.MemoryPlace{Storage: "data", Index: ir.Const{}, Read: true}}, True: 1, False: 2}},
		{ID: 1, Terminator: ir.Jump{Target: 3}},
		{ID: 2, Terminator: ir.Jump{Target: 3}},
		{ID: 3, Phis: []ir.Phi{{Target: ir.SSAPlace{ID: 2}, Args: []ir.PhiArg{
			{Predecessor: 1, Value: ir.SSAPlace{ID: 0}}, {Predecessor: 2, Value: ir.SSAPlace{ID: 1}},
		}}}, Instructions: []ir.Instruction{
			ir.Eval{Value: ir.RuntimeCall{Function: resource.RuntimeFunctionDebugLog, Args: []ir.Expr{ir.Load{Place: ir.SSAPlace{ID: 2}}}}},
		}, Terminator: ir.Return{}},
	}}
	if err := (FromSSA{}).Run(Context{}, function); err != nil {
		t.Fatal(err)
	}
	for i, want := range []float64{7, 9} {
		store := function.Blocks[i+1].Instructions[0].(ir.Store)
		if value, ok := store.Value.(ir.Const); !ok || value.Value != want {
			t.Fatalf("edge %d still copies a temporary: %#v", i+1, store)
		}
	}
	if got := executeCallValueCheckpoint(t, function); !reflect.DeepEqual(got, []float64{7}) {
		t.Fatalf("selected Phi value=%v, want 7", got)
	}
}

func TestSSAConstructionAndCriticalEdgeDestruction(t *testing.T) {
	number := ir.Type{Name: "number", Slots: 1}
	fn := &ir.Function{
		Name: "phi", Entry: 0, Result: number, Locals: []ir.Type{number},
		Blocks: []*ir.Block{
			{ID: 0, Instructions: []ir.Instruction{ir.Store{Place: ir.LocalPlace{ID: 0}, Value: ir.Const{Value: 1}}}, Terminator: ir.Branch{Condition: ir.Load{Place: ir.LocalPlace{ID: 0}}, True: 2, False: 1}},
			{ID: 1, Instructions: []ir.Instruction{ir.Store{Place: ir.LocalPlace{ID: 0}, Value: ir.Const{Value: 2}}}, Terminator: ir.Jump{Target: 2}},
			{ID: 2, Terminator: ir.Return{Value: ir.Value{Type: number, Slots: []ir.Expr{ir.Load{Place: ir.LocalPlace{ID: 0}}}}}},
		},
	}
	if err := (ToSSA{}).Run(Context{}, fn); err != nil {
		t.Fatal(err)
	}
	if len(fn.Blocks[2].Phis) != 1 || len(fn.Blocks[2].Phis[0].Args) != 2 {
		t.Fatalf("phis = %#v", fn.Blocks[2].Phis)
	}
	if err := (FromSSA{}).Run(Context{}, fn); err != nil {
		t.Fatal(err)
	}
	if len(fn.Blocks) != 4 || len(fn.Blocks[2].Phis) != 0 {
		t.Fatalf("critical edge was not split: %#v", fn.Blocks)
	}
	branch := fn.Blocks[0].Terminator.(ir.Branch)
	if branch.True == 2 {
		t.Fatalf("critical edge still targets join: %#v", branch)
	}
	if err := ir.Validate(fn); err != nil {
		t.Fatal(err)
	}
}

func TestCoalesceFlowMaterializesSinglePredecessorPhi(t *testing.T) {
	number := ir.Type{Name: "number", Slots: 1}
	first := ir.SSAPlace{ID: 1}
	second := ir.SSAPlace{ID: 2}
	fn := &ir.Function{Name: "coalesce-phi", Entry: 0, Result: number, Locals: []ir.Type{number}, Blocks: []*ir.Block{
		{ID: 0, Instructions: []ir.Instruction{ir.Store{Place: first, Value: ir.Const{Value: 4}}}, Terminator: ir.Jump{Target: 1}},
		{ID: 1, Phis: []ir.Phi{{Target: second, Local: ir.LocalPlace{ID: 0}, Args: []ir.PhiArg{{Predecessor: 0, Value: first}}}}, Terminator: ir.Return{Value: ir.Value{Type: number, Slots: []ir.Expr{ir.Load{Place: second}}}}},
	}}
	if err := (CoalesceFlow{}).Run(Context{}, fn); err != nil {
		t.Fatal(err)
	}
	if len(fn.Blocks) != 1 || len(fn.Blocks[0].Phis) != 0 || len(fn.Blocks[0].Instructions) != 2 {
		t.Fatalf("coalesced function = %#v", fn)
	}
	if err := ir.Validate(fn); err != nil {
		t.Fatal(err)
	}
}

func TestLICMHoistsReadonlyMemoryWithInvariantIndex(t *testing.T) {
	number := ir.Type{Name: "number", Slots: 1}
	index := ir.SSAPlace{ID: 1, Name: "index"}
	value := ir.SSAPlace{ID: 2, Name: "value"}
	readonly := ir.MemoryPlace{Storage: "EngineRom", Index: ir.Load{Place: index}, Read: true, Write: false}
	function := &ir.Function{Name: "licm-readonly", Entry: 0, Result: ir.Type{}, Blocks: []*ir.Block{
		{ID: 0, Instructions: []ir.Instruction{ir.Store{Place: index, Value: ir.Const{Value: 3}}}, Terminator: ir.Jump{Target: 1}},
		{ID: 1, Terminator: ir.Branch{Condition: ir.Load{Place: parityMemory(0)}, True: 2, False: 3}},
		{ID: 2, Instructions: []ir.Instruction{ir.Store{Place: value, Value: ir.RuntimeCall{Function: resource.RuntimeFunctionAdd, Args: []ir.Expr{ir.Load{Place: readonly}, ir.Const{Value: 1}}, Result: number, Pure: true}}}, Terminator: ir.Jump{Target: 1}},
		{ID: 3, Terminator: ir.Return{Value: ir.Value{Type: ir.Type{}}}},
	}}
	if err := (LoopInvariantCodeMotion{}).Run(Context{}, function); err != nil {
		t.Fatal(err)
	}
	if len(function.Blocks[0].Instructions) != 2 || len(function.Blocks[2].Instructions) != 1 {
		t.Fatalf("readonly invariant was not hoisted: preheader=%#v body=%#v", function.Blocks[0].Instructions, function.Blocks[2].Instructions)
	}
	bodyStore := function.Blocks[2].Instructions[0].(ir.Store)
	load, ok := bodyStore.Value.(ir.Load)
	if !ok {
		t.Fatalf("loop body still recomputes invariant expression: %#v", bodyStore.Value)
	}
	if _, ok := load.Place.(ir.SSAPlace); !ok {
		t.Fatalf("loop body does not reuse hoisted SSA value: %#v", load.Place)
	}
	writableFunction := CloneFunction(function)
	writableFunction.Blocks[0].Instructions = writableFunction.Blocks[0].Instructions[:1]
	writableFunction.Blocks[2].Instructions = []ir.Instruction{ir.Store{Place: value, Value: ir.Load{Place: ir.MemoryPlace{Storage: "shared", Index: ir.Const{}, Read: true, Write: true}}}}
	if err := (LoopInvariantCodeMotion{}).Run(Context{}, writableFunction); err != nil {
		t.Fatal(err)
	}
	if len(writableFunction.Blocks[2].Instructions) != 1 {
		t.Fatal("writable memory load was hoisted")
	}
	t.Run("single-block-loop", func(t *testing.T) {
		fn := &ir.Function{Name: "self-loop", Entry: 0, Result: ir.Type{}, Blocks: []*ir.Block{
			{ID: 0, Instructions: []ir.Instruction{ir.Store{Place: index, Value: ir.Const{Value: 3}}}, Terminator: ir.Jump{Target: 1}},
			{ID: 1, Instructions: []ir.Instruction{ir.Store{Place: value, Value: ir.RuntimeCall{Function: resource.RuntimeFunctionAdd, Args: []ir.Expr{ir.Load{Place: readonly}, ir.Const{Value: 1}}, Result: number, Pure: true}}}, Terminator: ir.Branch{Condition: ir.Load{Place: parityMemory(0)}, True: 1, False: 2}},
			{ID: 2, Terminator: ir.Return{Value: ir.Value{Type: ir.Type{}}}},
		}}
		if err := (LoopInvariantCodeMotion{}).Run(Context{}, fn); err != nil {
			t.Fatal(err)
		}
		if err := ir.Validate(fn); err != nil {
			t.Fatal(err)
		}
		if len(fn.Blocks[0].Instructions) != 2 {
			t.Fatal("self-loop swallowed its preheader and prevented invariant hoisting")
		}
		if load, ok := fn.Blocks[1].Instructions[0].(ir.Store).Value.(ir.Load); !ok {
			t.Fatal("self-loop still recomputes its invariant")
		} else if _, ok := load.Place.(ir.SSAPlace); !ok {
			t.Fatal("self-loop does not reuse the hoisted value")
		}
	})
}

func TestLICMHandlesMultipleLatchesWhenTheCandidateDominatesAll(t *testing.T) {
	number := ir.Type{Name: "number", Slots: 1}
	value := ir.SSAPlace{ID: 1, Name: "value"}
	readonly := ir.MemoryPlace{Storage: "EngineRom", Index: ir.Const{Value: 3}, Read: true, Write: false}
	function := &ir.Function{Name: "licm-multiple-latches", Entry: 0, Result: ir.Type{}, Blocks: []*ir.Block{
		{ID: 0, Terminator: ir.Jump{Target: 1}},
		{ID: 1, Instructions: []ir.Instruction{ir.Store{Place: value, Value: ir.RuntimeCall{Function: resource.RuntimeFunctionAdd, Args: []ir.Expr{ir.Load{Place: readonly}, ir.Const{Value: 1}}, Result: number, Pure: true}}}, Terminator: ir.Branch{Condition: ir.Load{Place: parityMemory(0)}, True: 2, False: 4}},
		{ID: 2, Terminator: ir.Branch{Condition: ir.Load{Place: parityMemory(1)}, True: 1, False: 3}},
		{ID: 3, Terminator: ir.Jump{Target: 1}},
		{ID: 4, Terminator: emptyReturn()},
	}}
	if err := (LoopInvariantCodeMotion{}).Run(Context{}, function); err != nil {
		t.Fatal(err)
	}
	if len(function.Blocks[0].Instructions) != 1 {
		t.Fatalf("multi-latch invariant was not hoisted: %#v", function.Blocks[0].Instructions)
	}
	store := function.Blocks[1].Instructions[0].(ir.Store)
	if _, ok := store.Value.(ir.Load); !ok {
		t.Fatalf("multi-latch body still recomputes invariant: %#v", store.Value)
	}
	if err := ir.Validate(function); err != nil {
		t.Fatal(err)
	}
}

func TestLICMDoesNotHoistExpressionDependingOnLoopPhi(t *testing.T) {
	number := ir.Type{Name: "number", Slots: 1}
	index := ir.SSAPlace{ID: 1, Name: "index"}
	next := ir.SSAPlace{ID: 2, Name: "next"}
	value := ir.SSAPlace{ID: 3, Name: "value"}
	readonly := ir.MemoryPlace{Storage: "EngineRom", Index: ir.Load{Place: index}, Read: true, Write: false}
	function := &ir.Function{Name: "licm-loop-phi", Entry: 0, Result: ir.Type{}, Blocks: []*ir.Block{
		{ID: 0, Terminator: ir.Jump{Target: 1}},
		{ID: 1, Phis: []ir.Phi{{Target: index, Args: []ir.PhiArg{{Predecessor: 0, Value: ir.SSAPlace{ID: 0}}, {Predecessor: 2, Value: next}}}}, Terminator: ir.Branch{Condition: ir.RuntimeCall{Function: resource.RuntimeFunctionLess, Args: []ir.Expr{ir.Load{Place: index}, ir.Const{Value: 3}}, Result: number, Pure: true}, True: 2, False: 3}},
		{ID: 2, Instructions: []ir.Instruction{
			ir.Store{Place: value, Value: ir.RuntimeCall{Function: resource.RuntimeFunctionAdd, Args: []ir.Expr{ir.Load{Place: readonly}, ir.Const{Value: 1}}, Result: number, Pure: true}},
			ir.Store{Place: next, Value: ir.RuntimeCall{Function: resource.RuntimeFunctionAdd, Args: []ir.Expr{ir.Load{Place: index}, ir.Const{Value: 1}}, Result: number, Pure: true}},
		}, Terminator: ir.Jump{Target: 1}},
		{ID: 3, Terminator: emptyReturn()},
	}}
	if err := (LoopInvariantCodeMotion{}).Run(Context{Mode: mode.ModePlay, Callback: "updateParallel"}, function); err != nil {
		t.Fatal(err)
	}
	if len(function.Blocks[0].Instructions) != 0 {
		t.Fatalf("loop-phi-dependent expression was hoisted: %#v", function.Blocks[0].Instructions)
	}
}

func TestLICMCreatesPreheaderAndMergesOutsidePhiValues(t *testing.T) {
	number := ir.Type{Name: "number", Slots: 1}
	left := ir.SSAPlace{ID: 0, Name: "left"}
	right := ir.SSAPlace{ID: 1, Name: "right"}
	current := ir.SSAPlace{ID: 2, Name: "current"}
	next := ir.SSAPlace{ID: 3, Name: "next"}
	value := ir.SSAPlace{ID: 4, Name: "value"}
	readonly := ir.MemoryPlace{Storage: "EngineRom", Index: ir.Const{}, Read: true, Write: false}
	function := &ir.Function{Name: "licm-create-preheader", Entry: 0, Result: ir.Type{}, Locals: []ir.Type{number}, Blocks: []*ir.Block{
		{ID: 0, Terminator: ir.Branch{Condition: ir.Load{Place: parityMemory(0)}, True: 1, False: 2}},
		{ID: 1, Instructions: []ir.Instruction{ir.Store{Place: left, Value: ir.Const{Value: 1}}}, Terminator: ir.Jump{Target: 3}},
		{ID: 2, Instructions: []ir.Instruction{ir.Store{Place: right, Value: ir.Const{Value: 2}}}, Terminator: ir.Jump{Target: 3}},
		{ID: 3, Phis: []ir.Phi{{Target: current, Local: ir.LocalPlace{ID: 0}, Args: []ir.PhiArg{{Predecessor: 1, Value: left}, {Predecessor: 2, Value: right}, {Predecessor: 4, Value: next}}}}, Instructions: []ir.Instruction{
			ir.Store{Place: value, Value: ir.RuntimeCall{Function: resource.RuntimeFunctionAdd, Args: []ir.Expr{ir.Load{Place: readonly}, ir.Const{Value: 1}}, Result: number, Pure: true}},
		}, Terminator: ir.Branch{Condition: ir.RuntimeCall{Function: resource.RuntimeFunctionLess, Args: []ir.Expr{ir.Load{Place: current}, ir.Const{Value: 3}}, Result: number, Pure: true}, True: 4, False: 5}},
		{ID: 4, Instructions: []ir.Instruction{ir.Store{Place: next, Value: ir.RuntimeCall{Function: resource.RuntimeFunctionAdd, Args: []ir.Expr{ir.Load{Place: current}, ir.Const{Value: 1}}, Result: number, Pure: true}}}, Terminator: ir.Jump{Target: 3}},
		{ID: 5, Terminator: emptyReturn()},
	}}
	if err := (LoopInvariantCodeMotion{}).Run(Context{Mode: mode.ModePlay, Callback: "updateParallel"}, function); err != nil {
		t.Fatal(err)
	}
	if len(function.Blocks) != 7 {
		t.Fatalf("blocks = %d, want 7", len(function.Blocks))
	}
	preheader := function.Blocks[6]
	if len(preheader.Phis) != 1 || len(preheader.Phis[0].Args) != 2 {
		t.Fatalf("preheader phis = %#v", preheader.Phis)
	}
	if len(preheader.Instructions) != 1 {
		t.Fatalf("preheader instructions = %#v", preheader.Instructions)
	}
	for _, predecessor := range []int{1, 2} {
		jump, ok := function.Blocks[predecessor].Terminator.(ir.Jump)
		if !ok || jump.Target != preheader.ID {
			t.Fatalf("block %d terminator = %#v", predecessor, function.Blocks[predecessor].Terminator)
		}
	}
	headerPhi := function.Blocks[3].Phis[0]
	if len(headerPhi.Args) != 2 || headerPhi.Args[0].Predecessor != 4 || headerPhi.Args[1].Predecessor != preheader.ID || headerPhi.Args[1].Value != preheader.Phis[0].Target {
		t.Fatalf("header phi args = %#v", headerPhi.Args)
	}
	if _, ok := function.Blocks[3].Instructions[0].(ir.Store).Value.(ir.Load); !ok {
		t.Fatalf("loop body still recomputes invariant: %#v", function.Blocks[3].Instructions[0])
	}
	if err := ir.Validate(function); err != nil {
		t.Fatal(err)
	}
}

func TestCSEExtractsNestedReadonlyAndCanonicalizesSafeCommutativeOps(t *testing.T) {
	number := ir.Type{Name: "number", Slots: 1}
	left, right := ir.SSAPlace{ID: 1, Name: "left"}, ir.SSAPlace{ID: 2, Name: "right"}
	equal := func(a, b ir.Expr) ir.Expr {
		return ir.RuntimeCall{Function: resource.RuntimeFunctionEqual, Args: []ir.Expr{a, b}, Result: number, Pure: true}
	}
	function := &ir.Function{Name: "cse-nested", Entry: 0, Result: number, Blocks: []*ir.Block{{ID: 0, Instructions: []ir.Instruction{ir.Store{Place: left, Value: ir.Const{Value: 1}}, ir.Store{Place: right, Value: ir.Const{Value: 2}}}, Terminator: ir.Return{Value: ir.Value{Type: number, Slots: []ir.Expr{ir.RuntimeCall{Function: resource.RuntimeFunctionAdd, Args: []ir.Expr{equal(ir.Load{Place: left}, ir.Load{Place: right}), equal(ir.Load{Place: right}, ir.Load{Place: left})}, Result: number, Pure: true}}}}}}}
	if err := (CommonSubexpressionElimination{}).Run(Context{}, function); err != nil {
		t.Fatal(err)
	}
	if len(function.Blocks[0].Instructions) < 3 {
		t.Fatalf("nested expression was not extracted: %#v", function.Blocks[0].Instructions)
	}
	returned := function.Blocks[0].Terminator.(ir.Return).Value.Slots[0]
	call, ok := returned.(ir.Load)
	if !ok {
		t.Fatalf("top-level CSE result = %#v, want extracted load", returned)
	}
	if _, ok := call.Place.(ir.SSAPlace); !ok {
		t.Fatalf("CSE result place = %#v", call.Place)
	}
	if err := ir.Validate(function); err != nil {
		t.Fatal(err)
	}
}

func TestCSEKeepsConstantsInsteadOfReplacingThemWithSSALoads(t *testing.T) {
	number := ir.Type{Name: "number", Slots: 1}
	constant := ir.SSAPlace{ID: 1, Name: "constant"}
	function := &ir.Function{
		Name: "cse-constant", Entry: 0, Result: number,
		Blocks: []*ir.Block{{
			ID: 0,
			Instructions: []ir.Instruction{
				ir.Store{Place: constant, Value: ir.Const{Value: 1}},
			},
			Terminator: ir.Return{Value: ir.Value{Type: number, Slots: []ir.Expr{
				ir.RuntimeCall{
					Function: resource.RuntimeFunctionAdd,
					Args:     []ir.Expr{ir.Const{Value: 2}, ir.Const{Value: 1}},
					Result:   number,
					Pure:     true,
				},
			}}},
		}},
	}
	if err := (CommonSubexpressionElimination{}).Run(Context{}, function); err != nil {
		t.Fatal(err)
	}
	store := function.Blocks[0].Instructions[0].(ir.Store)
	if _, ok := store.Value.(ir.Const); !ok {
		t.Fatalf("constant store = %#v, want constant", store.Value)
	}
	returned := function.Blocks[0].Terminator.(ir.Return).Value.Slots[0].(ir.RuntimeCall)
	if value, ok := returned.Args[1].(ir.Const); !ok || value.Value != 1 {
		t.Fatalf("nested constant = %#v, want Const(1)", returned.Args[1])
	}
}

func TestRemoveRedundantArgumentsKeepsUnaryOperations(t *testing.T) {
	number := ir.Type{Name: "number", Slots: 1}
	value := ir.LocalPlace{ID: 0}
	function := &ir.Function{
		Name:   "unary-operations",
		Entry:  0,
		Locals: []ir.Type{number},
		Result: number,
		Blocks: []*ir.Block{{
			ID: 0,
			Instructions: []ir.Instruction{
				ir.Store{Place: value, Value: ir.RuntimeCall{Function: resource.RuntimeFunctionNegate, Args: []ir.Expr{ir.Load{Place: value}}, Result: number, Pure: true}},
			},
			Terminator: ir.Return{Value: ir.Value{Type: number, Slots: []ir.Expr{
				ir.RuntimeCall{Function: resource.RuntimeFunctionAbs, Args: []ir.Expr{ir.Load{Place: value}}, Result: number, Pure: true},
			}}},
		}},
	}
	if err := (RemoveRedundantArguments{}).Run(Context{}, function); err != nil {
		t.Fatal(err)
	}
	store := function.Blocks[0].Instructions[0].(ir.Store)
	if call, ok := store.Value.(ir.RuntimeCall); !ok || call.Function != resource.RuntimeFunctionNegate {
		t.Fatalf("negate was removed: %#v", store.Value)
	}
	result := function.Blocks[0].Terminator.(ir.Return).Value.Slots[0]
	if call, ok := result.(ir.RuntimeCall); !ok || call.Function != resource.RuntimeFunctionAbs {
		t.Fatalf("abs was removed: %#v", result)
	}
}

func TestReadonlyMemoryOracleBlocksWritableFacadeStorage(t *testing.T) {
	context := Context{Mode: "play", Callback: "preprocess"}
	ui := ir.Load{Place: ir.MemoryPlace{Storage: "RuntimeUI", Index: ir.Const{}, Read: true, Write: false}}
	rom := ir.Load{Place: ir.MemoryPlace{Storage: "EngineRom", Index: ir.Const{}, Read: true, Write: false}}
	if movableExpression(context, ui) {
		t.Fatal("RuntimeUI load was considered movable across preprocess setters")
	}
	if !movableExpression(context, rom) {
		t.Fatal("EngineRom load was not considered readonly")
	}
}
