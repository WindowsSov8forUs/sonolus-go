package optimize

import (
	"math"
	"strings"
	"testing"

	"github.com/WindowsSov8forUs/sonolus-core-go/core/resource"
	"github.com/WindowsSov8forUs/sonolus-go/v2/internal/compiler/ir"
	"github.com/WindowsSov8forUs/sonolus-go/v2/internal/compiler/mode"
)

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
