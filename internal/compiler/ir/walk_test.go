package ir

import (
	"testing"
)

func TestMapIdentity(t *testing.T) {
	root := Instr{
		ID: 1, Op: "Add", Pure: true,
		Args: []Node{
			Const(1),
			Get{Place: BlockPlace{Block: Const(100), Index: Const(5)}},
		},
	}
	result := Map(root, func(n Node) Node { return n })
	ri, ok := result.(Instr)
	if !ok {
		t.Fatalf("Map identity returned %T, want Instr", result)
	}
	if ri.ID != 1 || ri.Op != "Add" || !ri.Pure {
		t.Errorf("Instr metadata lost: %+v", ri)
	}
	if len(ri.Args) != 2 {
		t.Fatalf("Instr has %d args, want 2", len(ri.Args))
	}
	if c0, ok := ri.Args[0].(Const); !ok || c0 != 1 {
		t.Errorf("Arg 0 = %v, want Const(1)", ri.Args[0])
	}
	g, ok := ri.Args[1].(Get)
	if !ok {
		t.Fatalf("Arg 1 = %T, want Get", ri.Args[1])
	}
	bp, ok := g.Place.(BlockPlace)
	if !ok {
		t.Fatalf("Get.Place = %T, want BlockPlace", g.Place)
	}
	if c, ok := bp.Block.(Const); !ok || c != 100 {
		t.Errorf("BlockPlace.Block = %v, want Const(100)", bp.Block)
	}
	if c, ok := bp.Index.(Const); !ok || c != 5 {
		t.Errorf("BlockPlace.Index = %v, want Const(5)", bp.Index)
	}
}

func TestMapTransform(t *testing.T) {
	// Replace all Const(1) with Const(99)
	root := Instr{
		ID: 2, Op: "Mul", Pure: true,
		Args: []Node{
			Const(1),
			Const(2),
		},
	}
	result := Map(root, func(n Node) Node {
		if c, ok := n.(Const); ok && c == 1 {
			return Const(99)
		}
		return n
	})
	ri := result.(Instr)
	if c0, ok := ri.Args[0].(Const); !ok || c0 != 99 {
		t.Errorf("Arg 0 = %v, want Const(99)", ri.Args[0])
	}
	if c1, ok := ri.Args[1].(Const); !ok || c1 != 2 {
		t.Errorf("Arg 1 = %v, want Const(2)", ri.Args[1])
	}
	if ri.ID != 2 {
		t.Errorf("Instr.ID = %d, want 2", ri.ID)
	}
}

func TestMapReplaceInstr(t *testing.T) {
	// Replace the whole Instr with a Const
	root := Instr{ID: 3, Op: "Abs", Args: []Node{Const(-5)}, Pure: true}
	result := Map(root, func(n Node) Node {
		if in, ok := n.(Instr); ok && in.Op == "Abs" {
			return Const(42)
		}
		return n
	})
	if c, ok := result.(Const); !ok || c != 42 {
		t.Errorf("Map result = %v, want Const(42)", result)
	}
}

func TestMapSSAPlaceInGet(t *testing.T) {
	// Get{SSAPlace} — Map should preserve it structurally, fn can replace the Get.
	root := Get{Place: SSAPlace{Name: "tmp", Num: 3}}
	result := Map(root, func(n Node) Node {
		if g, ok := n.(Get); ok {
			if _, isSSA := g.Place.(SSAPlace); isSSA {
				return Const(7) // replace Get{SSAPlace} with Const
			}
		}
		return n
	})
	if c, ok := result.(Const); !ok || c != 7 {
		t.Errorf("Map result = %v, want Const(7)", result)
	}
}

func TestMapGetBlockPlace(t *testing.T) {
	root := Get{Place: BlockPlace{Block: Const(200), Index: Const(10)}}
	result := Map(root, func(n Node) Node {
		if c, ok := n.(Const); ok && c == 10 {
			return Const(20)
		}
		return n
	})
	g := result.(Get)
	bp := g.Place.(BlockPlace)
	if c, ok := bp.Index.(Const); !ok || c != 20 {
		t.Errorf("Index = %v, want Const(20)", bp.Index)
	}
}

func TestMapSetPreservesID(t *testing.T) {
	root := Set{
		ID:    55,
		Place: BlockPlace{Block: Const(100), Index: Const(0)},
		Value: Const(42),
	}
	result := Map(root, func(n Node) Node { return n })
	s := result.(Set)
	if s.ID != 55 {
		t.Errorf("Set.ID = %d, want 55", s.ID)
	}
}

func TestMapNil(t *testing.T) {
	// fn should be called on nil.
	called := false
	result := Map(nil, func(n Node) Node {
		called = true
		if n != nil {
			t.Errorf("fn called with %v, want nil", n)
		}
		return Const(0)
	})
	if !called {
		t.Error("fn was not called on nil")
	}
	if c, ok := result.(Const); !ok || c != 0 {
		t.Errorf("Map(nil) = %v, want Const(0)", result)
	}
}
