package optimize

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/WindowsSov8forUs/sonolus-core-go/core/resource"

	"github.com/WindowsSov8forUs/sonolus-go/compiler/ir"
)

// --- SCCP: SSA-form canon, matching harness.py sccpCases ---

func sccpBuilders() map[string]func() *ir.BasicBlock {
	return map[string]func() *ir.BasicBlock{
		"const_fold": func() *ir.BasicBlock {
			x, y := ir.NewTemp("x"), ir.NewTemp("y")
			e := ir.NewBlock()
			e.Statements = []ir.Node{
				testGen.SetPlace(ir.TempCell(x), ir.Const(5)),
				testGen.SetPlace(ir.TempCell(y), testGen.PureInstr(resource.RuntimeFunctionAdd, ir.GetPlace(ir.TempCell(x)), ir.Const(3))),
				testGen.SetPlace(ir.Cell(0, 0), ir.GetPlace(ir.TempCell(y))),
			}
			return e
		},
		"phi_const": func() *ir.BasicBlock {
			x := ir.NewTemp("x")
			e, thenB, elseB, merge := ir.NewBlock(), ir.NewBlock(), ir.NewBlock(), ir.NewBlock()
			e.Test = mget(0, 0)
			thenB.Statements = []ir.Node{testGen.SetPlace(ir.TempCell(x), ir.Const(7))}
			elseB.Statements = []ir.Node{testGen.SetPlace(ir.TempCell(x), ir.Const(7))}
			merge.Statements = []ir.Node{testGen.SetPlace(ir.Cell(0, 1), ir.GetPlace(ir.TempCell(x)))}
			e.ConnectTo(elseB, ir.Cond(0))
			e.ConnectTo(thenB, nil)
			thenB.ConnectTo(merge, nil)
			elseB.ConnectTo(merge, nil)
			return e
		},
	}
}

func TestSCCPGolden(t *testing.T) {
	data, err := os.ReadFile("testdata/optimize_golden.json")
	if err != nil {
		t.Fatal(err)
	}
	var gold struct {
		SCCPCases map[string]string `json:"sccpCases"`
	}
	if err := json.Unmarshal(data, &gold); err != nil {
		t.Fatal(err)
	}
	for name, build := range sccpBuilders() {
		want, ok := gold.SCCPCases[name]
		if !ok {
			t.Fatalf("no SCCP golden for %q", name)
		}
		t.Run(name, func(t *testing.T) {
			got := ssaCanon(SCCP{}.Run(testGen, ToSSA{}.Run(testGen, build())))
			if got != want {
				t.Errorf("\n got: %s\nwant: %s", got, want)
			}
		})
	}
}
