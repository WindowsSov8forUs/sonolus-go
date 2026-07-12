package optimize

import (
	"reflect"
	"strings"
	"sync"
	"testing"

	"github.com/WindowsSov8forUs/sonolus-core-go/core/resource"
	"github.com/WindowsSov8forUs/sonolus-go/internal/newcompiler/ir"
	"github.com/WindowsSov8forUs/sonolus-go/internal/newcompiler/mode"
)

var (
	voidType   = ir.Type{Name: "void"}
	numberType = ir.Type{Name: "number", Slots: 1}
)

func function(blocks ...*ir.Block) *ir.Function {
	return &ir.Function{Name: "callback", Result: voidType, Blocks: blocks, Entry: 0}
}

func returnVoid(id int) *ir.Block {
	return &ir.Block{ID: id, Terminator: ir.Return{Value: ir.Value{Type: voidType}}}
}

func TestOptimizerRejectsInvalidInputAndLevels(t *testing.T) {
	context := Context{Mode: mode.ModePlay, Callback: "update"}
	if _, err := NewOptimizer(LevelMinimal).Optimize(context, nil); err == nil || !strings.Contains(err.Error(), "play/update") || !strings.Contains(err.Error(), "function is nil") {
		t.Fatalf("invalid input error = %v", err)
	}
	for _, level := range []Level{LevelFast, LevelStandard, Level(99)} {
		if _, err := NewOptimizer(level).Optimize(context, function(returnVoid(0))); err == nil || !strings.Contains(err.Error(), "only minimal") {
			t.Fatalf("level %d error = %v", level, err)
		}
	}
}

type invalidPass struct{}

func (invalidPass) Name() string { return "Invalid" }
func (invalidPass) Run(_ Context, function *ir.Function) error {
	function.Blocks[0].Terminator = nil
	return nil
}

func TestOptimizerReportsPassProducingInvalidIR(t *testing.T) {
	optimizer := &Optimizer{level: LevelMinimal, passes: []Pass{invalidPass{}}}
	_, err := optimizer.Optimize(Context{Mode: mode.ModeTutorial, Callback: "update"}, function(returnVoid(0)))
	if err == nil || !strings.Contains(err.Error(), "pass Invalid produced invalid IR") || !strings.Contains(err.Error(), "tutorial/update") {
		t.Fatalf("error = %v", err)
	}
}

func TestCloneFunctionIsDeep(t *testing.T) {
	index := ir.RuntimeCall{Function: resource.RuntimeFunctionAdd, Args: []ir.Expr{ir.Const{Value: 1}, ir.Const{Value: 2}}, Result: numberType, Pure: true}
	typ := ir.Type{Name: "record", Slots: 1, Fields: []ir.Field{{Name: "value", Type: numberType}}}
	call := ir.RuntimeCall{Function: resource.RuntimeFunctionAdd, Args: []ir.Expr{ir.Load{Place: ir.IndexedLocalPlace{ID: 0, Index: index, Length: 1, Stride: 1}}}, Result: numberType, Pure: true}
	input := &ir.Function{Name: "clone", Result: typ, Entry: 0, Locals: []ir.Type{typ}, Blocks: []*ir.Block{{ID: 0, Terminator: ir.Return{Value: ir.Value{Type: typ, Slots: []ir.Expr{call}}}}}}
	cloned := CloneFunction(input)
	cloned.Result.Fields[0].Name = "changed"
	cloned.Locals[0].Fields[0].Name = "changed"
	clonedCall := cloned.Blocks[0].Terminator.(ir.Return).Value.Slots[0].(ir.RuntimeCall)
	clonedCall.Args[0].(ir.Load).Place.(ir.IndexedLocalPlace).Index.(ir.RuntimeCall).Args[0] = ir.Const{Value: 9}
	if input.Result.Fields[0].Name != "value" || input.Locals[0].Fields[0].Name != "value" {
		t.Fatal("type layout was shared with clone")
	}
	original := input.Blocks[0].Terminator.(ir.Return).Value.Slots[0].(ir.RuntimeCall)
	if original.Args[0].(ir.Load).Place.(ir.IndexedLocalPlace).Index.(ir.RuntimeCall).Args[0].(ir.Const).Value != 1 {
		t.Fatal("nested expression slices were shared with clone")
	}
}

func TestMinimalFoldsControlAndRemovesUnreachable(t *testing.T) {
	input := function(
		&ir.Block{ID: 0, Terminator: ir.Branch{Condition: ir.Const{Value: 1}, True: 1, False: 2}},
		returnVoid(1),
		returnVoid(2),
	)
	result, err := NewOptimizer(0).Optimize(Context{Mode: mode.ModePlay, Callback: "update"}, input)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Blocks) != 1 || result.Entry != 0 {
		t.Fatalf("blocks = %#v", result.Blocks)
	}
	if _, ok := result.Blocks[0].Terminator.(ir.Return); !ok {
		t.Fatalf("terminator = %T", result.Blocks[0].Terminator)
	}
	if _, ok := input.Blocks[0].Terminator.(ir.Branch); !ok || len(input.Blocks) != 3 {
		t.Fatal("optimizer modified input")
	}
}

func TestFoldConstantSwitchUsesFirstMatchAndDefault(t *testing.T) {
	pass := FoldConstantControl{}
	for _, test := range []struct {
		value float64
		want  int
	}{{2, 1}, {3, 3}} {
		fn := function(&ir.Block{ID: 0, Terminator: ir.Switch{Value: ir.Const{Value: test.value}, Cases: []ir.SwitchCase{{Value: 2, Target: 1}, {Value: 2, Target: 2}}, Default: 3}}, returnVoid(1), returnVoid(2), returnVoid(3))
		if err := pass.Run(Context{}, fn); err != nil {
			t.Fatal(err)
		}
		if got := fn.Blocks[0].Terminator.(ir.Jump).Target; got != test.want {
			t.Fatalf("switch %v target = %d, want %d", test.value, got, test.want)
		}
	}
}

func TestCoalesceFlowMergesLinearButPreservesLoopAndJoin(t *testing.T) {
	linear := function(&ir.Block{ID: 0, Terminator: ir.Jump{Target: 1}}, &ir.Block{ID: 1, Terminator: ir.Jump{Target: 2}}, returnVoid(2))
	if err := (CoalesceFlow{}).Run(Context{}, linear); err != nil {
		t.Fatal(err)
	}
	if len(linear.Blocks) != 1 {
		t.Fatalf("linear blocks = %d", len(linear.Blocks))
	}
	loop := function(&ir.Block{ID: 0, Terminator: ir.Jump{Target: 1}}, &ir.Block{ID: 1, Terminator: ir.Jump{Target: 1}})
	if err := (CoalesceFlow{}).Run(Context{}, loop); err != nil {
		t.Fatal(err)
	}
	if len(loop.Blocks) != 2 {
		t.Fatalf("loop blocks = %d", len(loop.Blocks))
	}
	join := function(&ir.Block{ID: 0, Terminator: ir.Branch{Condition: ir.Const{Value: 1}, True: 1, False: 2}}, &ir.Block{ID: 1, Terminator: ir.Jump{Target: 3}}, &ir.Block{ID: 2, Terminator: ir.Jump{Target: 3}}, returnVoid(3))
	if err := (CoalesceFlow{}).Run(Context{}, join); err != nil {
		t.Fatal(err)
	}
	if len(join.Blocks) != 2 {
		t.Fatalf("join blocks = %d", len(join.Blocks))
	}
	branch := join.Blocks[0].Terminator.(ir.Branch)
	if branch.True != 1 || branch.False != 1 {
		t.Fatalf("join branch = %#v", branch)
	}
}

func TestRemoveNoOpsPreservesEffects(t *testing.T) {
	local := ir.Type{Name: "local", Slots: 1}
	impure := ir.RuntimeCall{Function: resource.RuntimeFunctionDebugLog, Args: []ir.Expr{ir.Const{Value: 1}}, Result: numberType, Pure: false}
	pureVoid := ir.RuntimeCall{Function: resource.RuntimeFunctionExecute, Result: voidType, Pure: true}
	withEffect := ir.RuntimeCall{Function: resource.RuntimeFunctionExecute, Args: []ir.Expr{impure}, Result: voidType, Pure: true}
	fn := function(&ir.Block{ID: 0, Instructions: []ir.Instruction{
		ir.Store{Place: ir.LocalPlace{ID: 0}, Value: ir.Load{Place: ir.LocalPlace{ID: 0}}},
		ir.Eval{Value: pureVoid},
		ir.Eval{Value: withEffect},
		ir.Store{Place: ir.LocalPlace{ID: 0}, Value: ir.Const{Value: 2}},
	}, Terminator: ir.Return{Value: ir.Value{Type: voidType}}})
	fn.Locals = []ir.Type{local}
	if err := (RemoveNoOps{}).Run(Context{}, fn); err != nil {
		t.Fatal(err)
	}
	if len(fn.Blocks[0].Instructions) != 2 {
		t.Fatalf("instructions = %#v", fn.Blocks[0].Instructions)
	}
	if _, ok := fn.Blocks[0].Instructions[0].(ir.Eval); !ok {
		t.Fatal("effectful nested call was removed")
	}
}

func TestOptimizerIsConcurrentAndDeterministic(t *testing.T) {
	input := function(&ir.Block{ID: 0, Terminator: ir.Jump{Target: 1}}, returnVoid(1))
	optimizer := NewOptimizer(LevelMinimal)
	results := make([]*ir.Function, 16)
	errors := make([]error, len(results))
	var wg sync.WaitGroup
	for i := range results {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			results[i], errors[i] = optimizer.Optimize(Context{Mode: mode.ModeWatch, Callback: "update"}, input)
		}(i)
	}
	wg.Wait()
	for i := range results {
		if errors[i] != nil {
			t.Fatal(errors[i])
		}
		if !reflect.DeepEqual(results[0], results[i]) {
			t.Fatalf("result %d differs", i)
		}
	}
	if len(input.Blocks) != 2 {
		t.Fatal("concurrent optimization modified input")
	}
}
