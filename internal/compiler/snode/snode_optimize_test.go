package snode

import (
	"testing"
)

// --- Unit tests for peephole optimization rules ---

func TestPeepholeAddIdentity(t *testing.T) {
	// Add(x, 0) → x
	n := Peephole(Func{Op: OpAdd, Args: []SNode{Value(42), Value(0)}})
	if got, _ := asValue(n); got != 42 {
		t.Errorf("Add(42, 0) = %v, want 42", n)
	}
}

func TestPeepholeSubIdentity(t *testing.T) {
	// Sub(x, 0) → x
	n := Peephole(Func{Op: OpSubtract, Args: []SNode{Value(42), Value(0)}})
	if got, _ := asValue(n); got != 42 {
		t.Errorf("Sub(42, 0) = %v, want 42", n)
	}
}

func TestPeepholeMulIdentity(t *testing.T) {
	// Mul(x, 1) → x
	n := Peephole(Func{Op: OpMultiply, Args: []SNode{Value(42), Value(1)}})
	if got, _ := asValue(n); got != 42 {
		t.Errorf("Mul(42, 1) = %v, want 42", n)
	}
}

func TestPeepholeDivIdentity(t *testing.T) {
	// Div(x, 1) → x
	n := Peephole(Func{Op: OpDivide, Args: []SNode{Value(42), Value(1)}})
	if got, _ := asValue(n); got != 42 {
		t.Errorf("Div(42, 1) = %v, want 42", n)
	}
}

func TestPeepholeAddConstantFold(t *testing.T) {
	// Add(1, 2) → 3
	n := Peephole(Func{Op: OpAdd, Args: []SNode{Value(1), Value(2)}})
	if got, _ := asValue(n); got != 3 {
		t.Errorf("Add(1, 2) = %v, want 3", n)
	}
}

func TestPeepholeMulConstantFold(t *testing.T) {
	// Mul(3, 4) → 12
	n := Peephole(Func{Op: OpMultiply, Args: []SNode{Value(3), Value(4)}})
	if got, _ := asValue(n); got != 12 {
		t.Errorf("Mul(3, 4) = %v, want 12", n)
	}
}

func TestPeepholeIfZeroToAnd(t *testing.T) {
	// If(a, b, 0) → And(a, b)
	a := Func{Op: OpGet, Args: []SNode{Value(0), Value(0)}}
	b := Func{Op: OpGet, Args: []SNode{Value(0), Value(1)}}
	n := Peephole(Func{Op: OpIf, Args: []SNode{a, b, Value(0)}})
	f, ok := n.(Func)
	if !ok || f.Op != OpAnd {
		t.Errorf("If(a,b,0) should become And(a,b), got %s", canonSNode(n))
	}
}

func TestPeepholeIfWithNonZeroElse(t *testing.T) {
	// If(a, b, 1) stays as If — not simplified
	a := Func{Op: OpGet, Args: []SNode{Value(0), Value(0)}}
	b := Func{Op: OpGet, Args: []SNode{Value(0), Value(1)}}
	n := Peephole(Func{Op: OpIf, Args: []SNode{a, b, Value(1)}})
	f, ok := n.(Func)
	if !ok || f.Op != OpIf {
		t.Errorf("If(a,b,1) should stay as If, got %s", canonSNode(n))
	}
}

func TestPeepholeNestedAddFlatten(t *testing.T) {
	// Add(Add(1, 2), 3) → 6 (flatten + constant fold)
	n := Peephole(Func{Op: OpAdd, Args: []SNode{
		Func{Op: OpAdd, Args: []SNode{Value(1), Value(2)}},
		Value(3),
	}})
	if got, _ := asValue(n); got != 6 {
		t.Errorf("Add(Add(1,2), 3) = %v, want 6", n)
	}
}

func TestPeepholeMulZeroAnnihilation(t *testing.T) {
	// Mul(0, dynamic) → Execute(dynamic, 0) preserving side effects
	d := Func{Op: OpGet, Args: []SNode{Value(0), Value(0)}}
	n := Peephole(Func{Op: OpMultiply, Args: []SNode{Value(0), d}})
	f, ok := n.(Func)
	if !ok || f.Op != OpExecute {
		t.Errorf("Mul(0, dynamic) should produce Execute, got %s", canonSNode(n))
	}
}

func TestPeepholeGetShiftedPattern(t *testing.T) {
	// Get(id, Add(x, Mul(y, sh))) → GetShifted(id, x, y, sh)
	// Use dynamic values (Get nodes) so they aren't constant-folded before the pattern match.
	x := Func{Op: OpGet, Args: []SNode{Value(0), Value(0)}}
	y := Func{Op: OpGet, Args: []SNode{Value(0), Value(1)}}
	sh := Func{Op: OpGet, Args: []SNode{Value(0), Value(2)}}
	n := Peephole(Func{Op: OpGet, Args: []SNode{
		Value(0),
		Func{Op: OpAdd, Args: []SNode{
			x,
			Func{Op: OpMultiply, Args: []SNode{y, sh}},
		}},
	}})
	f, ok := n.(Func)
	if !ok || f.Op != OpGetShifted {
		t.Errorf("Get(Add+Mul) should produce GetShifted, got %s", canonSNode(n))
	}
}

func TestPeepholeValueIdentity(t *testing.T) {
	v := Value(3.14)
	if Peephole(v) != v {
		t.Error("Peephole(Value) should return identity")
	}
}

func TestPeepholeSwitchNormalize(t *testing.T) {
	// SwitchWithDefault(disc, 0, body0, 2, body1, 4, body2, 0) →
	// SwitchInteger((disc-0)/2, body0, body1, body2)
	s := Func{Op: OpSwitchWithDefault, Args: []SNode{
		Func{Op: OpGet, Args: []SNode{Value(0), Value(1)}},
		Value(0), Func{Op: OpSet, Args: []SNode{Value(0), Value(0), Value(1)}},
		Value(2), Func{Op: OpSet, Args: []SNode{Value(0), Value(0), Value(2)}},
		Value(4), Func{Op: OpSet, Args: []SNode{Value(0), Value(0), Value(3)}},
		Value(0), // default case = 0
	}}
	n := Peephole(s)
	f, ok := n.(Func)
	if !ok {
		t.Fatalf("expected Func, got %T", n)
	}
	if f.Op != OpSwitchInteger {
		t.Errorf("expected SwitchInteger, got %s", f.Op)
	}
}

func TestPeepholeWhileBodySimplify(t *testing.T) {
	// While(cond, Execute(a, 0)) → While(cond, a)
	n := Peephole(Func{Op: OpWhile, Args: []SNode{
		Func{Op: OpGet, Args: []SNode{Value(0), Value(0)}},
		Func{Op: OpExecute, Args: []SNode{
			Func{Op: OpSet, Args: []SNode{Value(0), Value(0), Value(1)}},
			Value(0),
		}},
	}})
	f, ok := n.(Func)
	if !ok || f.Op != OpWhile {
		t.Fatal("expected While to survive optimization")
	}
}
