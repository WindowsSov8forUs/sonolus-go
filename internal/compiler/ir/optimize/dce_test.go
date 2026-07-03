package optimize

import (
	"encoding/json"
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/WindowsSov8forUs/sonolus-core-go/core/resource"

	"github.com/WindowsSov8forUs/sonolus-go/internal/compiler/ir"
)

// --- DCE: IR-level canon, matching testdata/harness.py ---

func ircanonVal(n ir.Node) string {
	switch t := n.(type) {
	case ir.Const:
		return strconv.Itoa(int(t))
	case ir.Instr:
		ps := make([]string, len(t.Args))
		for i, a := range t.Args {
			ps[i] = ircanonVal(a)
		}
		return string(t.Op) + "[" + strings.Join(ps, ",") + "]"
	case ir.Get:
		return "G(" + ircanonPlace(t.Place) + ")"
	case ir.BlockPlace:
		return ircanonPlace(t)
	default:
		return "?"
	}
}

func ircanonPlace(p ir.Place) string {
	bp := p.(ir.BlockPlace)
	if tb, ok := bp.Block.(*ir.TempBlock); ok {
		return "T(" + tb.Name + ")"
	}
	return "M(" + ircanonVal(bp.Block) + "," + ircanonVal(bp.Index) + ")"
}

func ircanonStmt(s ir.Node) string {
	if set, ok := s.(ir.Set); ok {
		return ircanonPlace(set.Place) + "=" + ircanonVal(set.Value)
	}
	return ircanonVal(s)
}

func ircanonBlock(b *ir.BasicBlock) string {
	ps := make([]string, len(b.Statements))
	for i, s := range b.Statements {
		ps[i] = ircanonStmt(s)
	}
	return strings.Join(ps, ";")
}

func tcell(t *ir.TempBlock) ir.BlockPlace { return ir.TempCell(t) }
func mget(b, i int) ir.Get                { return ir.GetPlace(ir.Cell(b, i)) }

func dceBuilders() map[string]func() *ir.BasicBlock {
	return map[string]func() *ir.BasicBlock{
		"dead_store": func() *ir.BasicBlock {
			a, b := ir.NewTemp("a"), ir.NewTemp("b")
			e := ir.NewBlock()
			e.Statements = []ir.Node{
				testGen.SetPlace(tcell(a), ir.Const(5)),
				testGen.SetPlace(tcell(b), mget(0, 0)),
				testGen.SetPlace(ir.Cell(0, 1), ir.GetPlace(tcell(b))),
			}
			return e
		},
		"self_copy": func() *ir.BasicBlock {
			a := ir.NewTemp("a")
			e := ir.NewBlock()
			e.Statements = []ir.Node{
				testGen.SetPlace(tcell(a), mget(0, 0)),
				testGen.SetPlace(tcell(a), ir.GetPlace(tcell(a))),
				testGen.SetPlace(ir.Cell(0, 1), ir.GetPlace(tcell(a))),
			}
			return e
		},
		"side_effect": func() *ir.BasicBlock {
			a := ir.NewTemp("a")
			e := ir.NewBlock()
			e.Statements = []ir.Node{
				testGen.SetPlace(tcell(a), testGen.ImpureInstr(resource.RuntimeFunctionDraw, ir.Const(1), ir.Const(2))),
			}
			return e
		},
		"transitive": func() *ir.BasicBlock {
			a, b := ir.NewTemp("a"), ir.NewTemp("b")
			e := ir.NewBlock()
			e.Statements = []ir.Node{
				testGen.SetPlace(tcell(a), mget(0, 0)),
				testGen.SetPlace(tcell(b), testGen.PureInstr(resource.RuntimeFunctionAdd, ir.GetPlace(tcell(a)), ir.Const(1))),
				testGen.SetPlace(ir.Cell(0, 1), ir.GetPlace(tcell(b))),
			}
			return e
		},
	}
}

func TestDCEGolden(t *testing.T) {
	data, err := os.ReadFile("testdata/optimize_golden.json")
	if err != nil {
		t.Fatal(err)
	}
	var gold struct {
		DCECases map[string]string `json:"dceCases"`
	}
	if err := json.Unmarshal(data, &gold); err != nil {
		t.Fatal(err)
	}

	for name, build := range dceBuilders() {
		want, ok := gold.DCECases[name]
		if !ok {
			t.Fatalf("no DCE golden for %q", name)
		}
		t.Run(name, func(t *testing.T) {
			got := ircanonBlock(DeadCodeElimination{}.Run(testGen, build()))
			if got != want {
				t.Errorf("\n got: %s\nwant: %s", got, want)
			}
		})
	}
}
