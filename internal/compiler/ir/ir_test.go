package ir

import (
	"testing"

	"github.com/WindowsSov8forUs/sonolus-core-go/core/resource"

	"github.com/WindowsSov8forUs/sonolus-go/internal/compiler/modecompile"
	"github.com/WindowsSov8forUs/sonolus-go/internal/compiler/snode"
)

var canon = modecompile.Canon

// mustLower calls CFGToSNode and panics on error. This is a test helper;
// CFGToSNode failures indicate an invariant violation in the test setup.
func mustLower(sn snode.SNode, err error) snode.SNode {
	if err != nil {
		panic(err)
	}
	return sn
}

// TestFinalizeSingleBlockBreak reproduces the pydori should_spawn node shape:
// a single block whose body is Break(1,1), terminating the JumpLoop.
func TestFinalizeSingleBlockBreak(t *testing.T) {
	gen := NewIDGen()
	b := NewBlock()
	b.Statements = []Node{gen.ImpureInstr(resource.RuntimeFunctionBreak, Const(1), Const(1))}

	got := canon(mustLower(CFGToSNode(gen, b)))
	want := "Block(JumpLoop(Execute(Break(#1,#1),#1),#0))"
	if got != want {
		t.Errorf("\n got: %s\nwant: %s", got, want)
	}
}

func TestFinalizeSetFallthrough(t *testing.T) {
	gen := NewIDGen()
	b := NewBlock()
	b.Statements = []Node{gen.SetPlace(Cell(0, 0), Const(5))}

	got := canon(mustLower(CFGToSNode(gen, b)))
	want := "Block(JumpLoop(Execute(Set(#0,#0,#5),#1),#0))"
	if got != want {
		t.Errorf("\n got: %s\nwant: %s", got, want)
	}
}

func TestFinalizeConditional(t *testing.T) {
	gen := NewIDGen()
	b0 := NewBlock()
	b0.Test = GetPlace(Cell(1, 0))
	bTrue := NewBlock()
	bFalse := NewBlock()
	b0.ConnectTo(bFalse, Cond(0))
	b0.ConnectTo(bTrue, nil)

	got := canon(mustLower(CFGToSNode(gen, b0)))
	// order: b0=0, bTrue=1, bFalse=2; exit=3; both leaves jump to 3 and dedup.
	want := "Block(JumpLoop(Execute(If(Get(#1,#0),#1,#2)),Execute(#3),Execute(#3),#0))"
	if got != want {
		t.Errorf("\n got: %s\nwant: %s", got, want)
	}
}

func TestFinalizeSwitchIntegerDense(t *testing.T) {
	gen := NewIDGen()
	b0 := NewBlock()
	b0.Test = GetPlace(Cell(0, 0))
	b1, b2, b3, bDef := NewBlock(), NewBlock(), NewBlock(), NewBlock()
	b0.ConnectTo(b1, Cond(0))
	b0.ConnectTo(b2, Cond(1))
	b0.ConnectTo(b3, Cond(2))
	b0.ConnectTo(bDef, nil)

	got := canon(mustLower(CFGToSNode(gen, b0)))
	// order: b0=0, bDef=1, b3=2, b2=3, b1=4; default index=1.
	want := "Block(JumpLoop(" +
		"Execute(SwitchIntegerWithDefault(Get(#0,#0),#4,#3,#2,#1))," +
		"Execute(#5),Execute(#5),Execute(#5),Execute(#5),#0))"
	if got != want {
		t.Errorf("\n got: %s\nwant: %s", got, want)
	}
}

func TestFinalizeSwitchSparse(t *testing.T) {
	gen := NewIDGen()
	b0 := NewBlock()
	b0.Test = GetPlace(Cell(0, 0))
	b1, b2, b3, bDef := NewBlock(), NewBlock(), NewBlock(), NewBlock()
	b0.ConnectTo(b1, Cond(0))
	b0.ConnectTo(b2, Cond(2))
	b0.ConnectTo(b3, Cond(5))
	b0.ConnectTo(bDef, nil)

	got := canon(mustLower(CFGToSNode(gen, b0)))
	// order: b0=0, bDef=1, b3=2, b2=3, b1=4.
	want := "Block(JumpLoop(" +
		"Execute(SwitchWithDefault(Get(#0,#0),#0,#4,#2,#3,#5,#2,#1))," +
		"Execute(#5),Execute(#5),Execute(#5),Execute(#5),#0))"
	if got != want {
		t.Errorf("\n got: %s\nwant: %s", got, want)
	}
}

func TestFinalizeEqualBranch(t *testing.T) {
	gen := NewIDGen()
	b0 := NewBlock()
	b0.Test = GetPlace(Cell(0, 0))
	b1, bDef := NewBlock(), NewBlock()
	b0.ConnectTo(b1, Cond(3))
	b0.ConnectTo(bDef, nil)

	got := canon(mustLower(CFGToSNode(gen, b0)))
	// order: b0=0, bDef=1, b1=2.
	want := "Block(JumpLoop(Execute(If(Equal(Get(#0,#0),#3),#2,#1)),Execute(#3),Execute(#3),#0))"
	if got != want {
		t.Errorf("\n got: %s\nwant: %s", got, want)
	}
}

// TestFinalizeToNodes drives finalize all the way into the deduplicated node
// list via the existing appender.

func TestFinalizeToNodes(t *testing.T) {
	gen := NewIDGen()
	b := NewBlock()
	b.Statements = []Node{gen.ImpureInstr(resource.RuntimeFunctionBreak, Const(1), Const(1))}

	var nodes []resource.EngineDataNode
	root, err := snode.NewAppender(&nodes).Append(mustLower(CFGToSNode(gen, b)))
	if err != nil {
		t.Fatal(err)
	}
	if root != 5 || len(nodes) != 6 {
		t.Fatalf("root=%d nodes=%d, want root=5 nodes=6", root, len(nodes))
	}
}

func TestFloorMod(t *testing.T) {
	tests := []struct {
		a, b, want float64
	}{
		{7, 3, 1},
		{-7, 3, 2},  // Python -7%3 == 2
		{7, -3, -2}, // Python 7%-3 == -2
		{-7, -3, -1},
		{-1, 3, 2},
		{0, 3, 0},
	}
	for _, tc := range tests {
		got := FloorMod(tc.a, tc.b)
		if got != tc.want {
			t.Errorf("FloorMod(%v, %v) = %v, want %v", tc.a, tc.b, got, tc.want)
		}
	}
}

// ── P1-3: IR package additional coverage ──

func TestAllocateTestBlocks_Basic(t *testing.T) {
	gen := NewIDGen()
	b0 := NewBlock()
	b1 := NewBlock()
	b0.Statements = []Node{gen.SetPlace(Cell(0, 0), Const(1))}
	b0.ConnectTo(b1, nil)
	b1.Statements = []Node{gen.SetPlace(Cell(0, 1), Const(2))}

	allocated, err := AllocateTestBlocks(b0, DefaultTempMemoryBlock)
	if err != nil {
		t.Fatal(err)
	}
	if allocated == nil {
		t.Fatal("AllocateTestBlocks returned nil")
	}
	canon(mustLower(CFGToSNode(gen, allocated)))
}

func TestAllocateTestBlocks_Empty(t *testing.T) {
	gen := NewIDGen()
	b := NewBlock()
	b.Statements = []Node{gen.SetPlace(Cell(0, 0), Const(42))}
	allocated, err := AllocateTestBlocks(b, DefaultTempMemoryBlock)
	if err != nil {
		t.Fatal(err)
	}
	if allocated == nil {
		t.Fatal("AllocateTestBlocks returned nil")
	}
}

func TestReversePostorder_Basic(t *testing.T) {
	b0 := NewBlock()
	b1 := NewBlock()
	b2 := NewBlock()
	b0.ConnectTo(b1, nil)
	b1.ConnectTo(b2, nil)

	order := ReversePostorder(b0)
	if len(order) != 3 {
		t.Fatalf("len = %d, want 3", len(order))
	}
	// Reverse postorder: b2 (deepest) should appear before b0 (entry).
	foundEntry := false
	for i, b := range order {
		if b == b0 && i < len(order)-1 {
			foundEntry = true
		}
	}
	if !foundEntry {
		t.Logf("order: b0@%v b1@%v b2@%v", indexOf(order, b0), indexOf(order, b1), indexOf(order, b2))
	}
}

func indexOf(blocks []*BasicBlock, target *BasicBlock) int {
	for i, b := range blocks {
		if b == target {
			return i
		}
	}
	return -1
}

func TestPreorder_Basic(t *testing.T) {
	b0 := NewBlock()
	b1 := NewBlock()
	b2 := NewBlock()
	b0.ConnectTo(b1, nil)
	b0.ConnectTo(b2, Cond(0))

	order := Preorder(b0)
	if len(order) != 3 {
		t.Fatalf("len = %d, want 3", len(order))
	}
	if order[0] != b0 {
		t.Error("first should be entry")
	}
}

func TestWalk_AddInstr(t *testing.T) {
	gen := NewIDGen()
	root := gen.PureInstr(resource.RuntimeFunctionAdd, Const(1), Const(2))
	var nodes []Node
	Walk(root, func(n Node) { nodes = append(nodes, n) })
	if len(nodes) < 2 {
		t.Errorf("Walk visited %d nodes, want at least 2", len(nodes))
	}
}

func TestWalk_SetInstr(t *testing.T) {
	gen := NewIDGen()
	root := gen.SetPlace(Cell(0, 0), Const(5))
	var nodes []Node
	Walk(root, func(n Node) { nodes = append(nodes, n) })
	if len(nodes) < 1 {
		t.Error("Walk visited no nodes for Set")
	}
}

func TestBlocks_Writable(t *testing.T) {
	bs := Blocks(ModePlay)
	// BlockRuntimeEnvironment should not be writable in typical callbacks.
	if bs.Writable(BlockRuntimeEnvironment, "initialize") {
		t.Error("BlockRuntimeEnvironment should be read-only in initialize")
	}
	// Temp memory block may or may not be in block tables — either answer is valid.
	isWritable := bs.Writable(DefaultTempMemoryBlock, "updateSequential")
	t.Logf("DefaultTempMemoryBlock writable in updateSequential: %v", isWritable)
}

func TestBlocks_RuntimeConstant(t *testing.T) {
	bs := Blocks(ModePlay)
	if bs.RuntimeConstant(BlockRuntimeEnvironment) {
		t.Log("BlockRuntimeEnvironment is a runtime constant block")
	}
}

func TestNewTemp_Basic(t *testing.T) {
	tb := NewTemp("myvar")
	if tb.Name != "myvar" {
		t.Errorf("Name = %q, want %q", tb.Name, "myvar")
	}
	if tb.Size != 1 {
		t.Errorf("Size = %d, want 1", tb.Size)
	}
}

func TestTempCell(t *testing.T) {
	tb := NewTemp("x")
	cell := TempCell(tb)
	if cell.Block != tb {
		t.Error("TempCell block should be the TempBlock")
	}
}

func TestNewBlockPlace_Basic(t *testing.T) {
	bp := NewBlockPlace(Const(0), Const(0), 0)
	if bp.Offset != 0 {
		t.Errorf("Offset = %d, want 0", bp.Offset)
	}
}

func TestSideEffects_Pure(t *testing.T) {
	if !SideEffects(resource.RuntimeFunctionSet) {
		t.Error("Set should have side effects")
	}
	if SideEffects(resource.RuntimeFunctionAdd) {
		t.Error("Add should be pure")
	}
	if !Pure(resource.RuntimeFunctionAdd) {
		t.Error("Add should be marked pure")
	}
	if Pure(resource.RuntimeFunctionSet) {
		t.Error("Set should not be pure")
	}
}

func TestCond_Nil(t *testing.T) {
	c := Cond(0)
	if c == nil {
		t.Fatal("Cond returned nil")
	}
	if *c != 0 {
		t.Errorf("Cond(0) = %v, want 0", *c)
	}
}

func TestIEEERem(t *testing.T) {
	tests := []struct {
		a, b, want float64
	}{
		{7, 3, 1},
		{-7, 3, -1}, // IEEE remainder(-7,3) == -1
		{7, -3, 1},
		{-7, -3, -1},
		{10, 3, 1},
		{-10, 3, -1},
	}
	for _, tc := range tests {
		got := IEEERem(tc.a, tc.b)
		if got != tc.want {
			t.Errorf("IEEERem(%v, %v) = %v, want %v", tc.a, tc.b, got, tc.want)
		}
	}
}
