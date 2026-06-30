package modecompile

import (
	"testing"

	"github.com/WindowsSov8forUs/sonolus-core-go/core/resource"

	"github.com/WindowsSov8forUs/sonolus-go/compiler/snode"
)

func TestCompileCallback_PureConstant(t *testing.T) {
	// A pure-constant body (snode.Value) does nothing observable — omit.
	r := CompileCallback(0, "test", snode.Value(42), nil)
	if r != nil {
		t.Errorf("pure constant should be omitted, got %+v", r)
	}
}

func TestCompileCallback_ExecuteIgnoreReturn(t *testing.T) {
	// Execute(..., 0) discards its trailing return value.
	f := snode.Func{
		Op: resource.RuntimeFunctionExecute,
		Args: []snode.SNode{snode.Value(2000), snode.Value(1), snode.Value(0)},
	}
	r := CompileCallback(0, "test", f, nil)
	if r == nil {
		t.Fatal("Execute with trailing 0 should not be omitted")
	}
	// After IgnoreReturn, the trailing 0 arg should be stripped.
	ff, ok := r.Node.(snode.Func)
	if !ok {
		t.Fatalf("expected snode.Func, got %T", r.Node)
	}
	if len(ff.Args) != 2 {
		t.Fatalf("expected 2 args after trailing-0 strip, got %d", len(ff.Args))
	}
}

func TestCompileCallback_OmitFunc(t *testing.T) {
	// When OmitFunc returns (true, true), general omission rules are skipped.
	f := snode.Func{
		Op: resource.RuntimeFunctionExecute,
		Args: []snode.SNode{snode.Value(2000), snode.Value(0)},
	}
	omitAlways := func(s snode.SNode, cb string) (omit, handled bool) {
		return true, true
	}
	r := CompileCallback(0, "test", f, omitAlways)
	if r != nil {
		t.Errorf("omitAlways should omit, got %+v", r)
	}
}

func TestAssemble_OutOfRange(t *testing.T) {
	nodes := []resource.EngineDataNode{}
	arcs := []testArch{{}}
	results := []*Result{{ArchetypeIndex: 1, Callback: "preprocess", Node: snode.Value(1)}}
	err := Assemble(&nodes, arcs, results, testSetCb)
	if err == nil {
		t.Fatal("expected error for out-of-range archetype index")
	}
}

func TestAssemble_NilResult(t *testing.T) {
	nodes := []resource.EngineDataNode{}
	arcs := []testArch{{}}
	results := []*Result{nil}
	err := Assemble(&nodes, arcs, results, testSetCb)
	if err != nil {
		t.Fatal(err)
	}
}

func TestIsConstZero(t *testing.T) {
	tests := []struct {
		s      snode.SNode
		expect bool
	}{
		{snode.Value(0), true},
		{snode.Value(1), false},
		{snode.Func{}, false}, // non-value
	}
	for _, tt := range tests {
		if got := IsConstZero(tt.s); got != tt.expect {
			t.Errorf("IsConstZero(%v) = %v, want %v", tt.s, got, tt.expect)
		}
	}
}

func TestIsConstNonZero(t *testing.T) {
	tests := []struct {
		s      snode.SNode
		expect bool
	}{
		{snode.Value(0), false},
		{snode.Value(1), true},
		{snode.Func{}, false}, // non-value
	}
	for _, tt := range tests {
		if got := IsConstNonZero(tt.s); got != tt.expect {
			t.Errorf("IsConstNonZero(%v) = %v, want %v", tt.s, got, tt.expect)
		}
	}
}

func TestIgnoreReturn(t *testing.T) {
	// Execute(a, 0) with 2 args → a
	f := snode.Func{
		Op: resource.RuntimeFunctionExecute,
		Args: []snode.SNode{snode.Value(42), snode.Value(0)},
	}
	result := ignoreReturn(f)
	if v, ok := result.(snode.Value); !ok || float64(v) != 42 {
		t.Errorf("Execute(a, 0) should return a, got %+v", result)
	}

	// Execute(a, b, 0) → Execute(a, b)
	f2 := snode.Func{
		Op: resource.RuntimeFunctionExecute,
		Args: []snode.SNode{snode.Value(1), snode.Value(2), snode.Value(0)},
	}
	result2 := ignoreReturn(f2)
	ff, ok := result2.(snode.Func)
	if !ok || len(ff.Args) != 2 {
		t.Errorf("Execute(a, b, 0) should strip last arg, got %+v", result2)
	}

	// Non-Execute function → unchanged
	f3 := snode.Func{
		Op: resource.RuntimeFunctionAdd,
		Args: []snode.SNode{snode.Value(1), snode.Value(2)},
	}
	result3 := ignoreReturn(f3)
	ff3, ok := result3.(snode.Func)
	if !ok || ff3.Op != resource.RuntimeFunctionAdd {
		t.Errorf("non-Execute should be unchanged, got %+v", result3)
	}
}

// testArch is a minimal archetype for Assemble tests.
type testArch struct{}

func testSetCb(arch *testArch, cb string, index int, order int) error { return nil }
