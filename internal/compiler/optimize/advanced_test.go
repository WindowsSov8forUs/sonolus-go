package optimize

import (
	"math"
	"strings"
	"testing"

	"github.com/WindowsSov8forUs/sonolus-core-go/core/resource"
	"github.com/WindowsSov8forUs/sonolus-go/internal/compiler/ir"
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

func TestSCCPUsesSonolusRoundAndModuloSemantics(t *testing.T) {
	for _, test := range []struct {
		function resource.RuntimeFunction
		args     []float64
		want     float64
	}{
		{resource.RuntimeFunctionRound, []float64{2.5}, 2},
		{resource.RuntimeFunctionRound, []float64{3.5}, 4},
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
