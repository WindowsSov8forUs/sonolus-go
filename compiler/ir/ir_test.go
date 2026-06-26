package ir

import (
	"strings"
	"testing"

	"github.com/WindowsSov8forUs/sonolus-core-go/core/resource"

	"github.com/WindowsSov8forUs/sonolus-go/compiler/snode"
)

func canon(n snode.SNode) string {
	switch t := n.(type) {
	case snode.Value:
		return "#" + snode.FormatNumber(float64(t))
	case snode.Func:
		ps := make([]string, len(t.Args))
		for i, a := range t.Args {
			ps[i] = canon(a)
		}
		return string(t.Func) + "(" + strings.Join(ps, ",") + ")"
	}
	return "?"
}

// TestFinalizeSingleBlockBreak reproduces the pydori should_spawn node shape:
// a single block whose body is Break(1,1), terminating the JumpLoop.
func TestFinalizeSingleBlockBreak(t *testing.T) {
	b := NewBlock()
	b.Statements = []Node{ImpureInstr(resource.RuntimeFunctionBreak, Const(1), Const(1))}

	got := canon(CFGToSNode(b))
	want := "Block(JumpLoop(Execute(Break(#1,#1),#1),#0))"
	if got != want {
		t.Errorf("\n got: %s\nwant: %s", got, want)
	}
}

func TestFinalizeSetFallthrough(t *testing.T) {
	b := NewBlock()
	b.Statements = []Node{SetPlace(Cell(0, 0), Const(5))}

	got := canon(CFGToSNode(b))
	want := "Block(JumpLoop(Execute(Set(#0,#0,#5),#1),#0))"
	if got != want {
		t.Errorf("\n got: %s\nwant: %s", got, want)
	}
}

func TestFinalizeConditional(t *testing.T) {
	b0 := NewBlock()
	b0.Test = GetPlace(Cell(1, 0))
	bTrue := NewBlock()
	bFalse := NewBlock()
	b0.ConnectTo(bFalse, Cond(0))
	b0.ConnectTo(bTrue, nil)

	got := canon(CFGToSNode(b0))
	// order: b0=0, bTrue=1, bFalse=2; exit=3; both leaves jump to 3 and dedup.
	want := "Block(JumpLoop(Execute(If(Get(#1,#0),#1,#2)),Execute(#3),Execute(#3),#0))"
	if got != want {
		t.Errorf("\n got: %s\nwant: %s", got, want)
	}
}

func TestFinalizeSwitchIntegerDense(t *testing.T) {
	b0 := NewBlock()
	b0.Test = GetPlace(Cell(0, 0))
	b1, b2, b3, bDef := NewBlock(), NewBlock(), NewBlock(), NewBlock()
	b0.ConnectTo(b1, Cond(0))
	b0.ConnectTo(b2, Cond(1))
	b0.ConnectTo(b3, Cond(2))
	b0.ConnectTo(bDef, nil)

	got := canon(CFGToSNode(b0))
	// order: b0=0, bDef=1, b3=2, b2=3, b1=4; default index=1.
	want := "Block(JumpLoop(" +
		"Execute(SwitchIntegerWithDefault(Get(#0,#0),#4,#3,#2,#1))," +
		"Execute(#5),Execute(#5),Execute(#5),Execute(#5),#0))"
	if got != want {
		t.Errorf("\n got: %s\nwant: %s", got, want)
	}
}

func TestFinalizeSwitchSparse(t *testing.T) {
	b0 := NewBlock()
	b0.Test = GetPlace(Cell(0, 0))
	b1, b2, b3, bDef := NewBlock(), NewBlock(), NewBlock(), NewBlock()
	b0.ConnectTo(b1, Cond(0))
	b0.ConnectTo(b2, Cond(2))
	b0.ConnectTo(b3, Cond(5))
	b0.ConnectTo(bDef, nil)

	got := canon(CFGToSNode(b0))
	// order: b0=0, bDef=1, b3=2, b2=3, b1=4.
	want := "Block(JumpLoop(" +
		"Execute(SwitchWithDefault(Get(#0,#0),#0,#4,#2,#3,#5,#2,#1))," +
		"Execute(#5),Execute(#5),Execute(#5),Execute(#5),#0))"
	if got != want {
		t.Errorf("\n got: %s\nwant: %s", got, want)
	}
}

func TestFinalizeEqualBranch(t *testing.T) {
	b0 := NewBlock()
	b0.Test = GetPlace(Cell(0, 0))
	b1, bDef := NewBlock(), NewBlock()
	b0.ConnectTo(b1, Cond(3))
	b0.ConnectTo(bDef, nil)

	got := canon(CFGToSNode(b0))
	// order: b0=0, bDef=1, b1=2.
	want := "Block(JumpLoop(Execute(If(Equal(Get(#0,#0),#3),#2,#1)),Execute(#3),Execute(#3),#0))"
	if got != want {
		t.Errorf("\n got: %s\nwant: %s", got, want)
	}
}

// TestFinalizeToNodes drives finalize all the way into the deduplicated node
// list via the existing appender.
func TestFinalizeToNodes(t *testing.T) {
	b := NewBlock()
	b.Statements = []Node{ImpureInstr(resource.RuntimeFunctionBreak, Const(1), Const(1))}

	var nodes []resource.EngineDataNode
	root, err := snode.NewAppender(&nodes).Append(CFGToSNode(b))
	if err != nil {
		t.Fatal(err)
	}
	if root != 5 || len(nodes) != 6 {
		t.Fatalf("root=%d nodes=%d, want root=5 nodes=6", root, len(nodes))
	}
}
