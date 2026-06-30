package optimize

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/WindowsSov8forUs/sonolus-core-go/core/resource"

	"github.com/WindowsSov8forUs/sonolus-go/compiler/ir"
)

// --- InlineVars: SSA-form canon, matching harness.py inlineCases ---

func inlineBuilders() map[string]func() *ir.BasicBlock {
	bs := map[string]func() *ir.BasicBlock{
		"const_chain": func() *ir.BasicBlock {
			x, y := ir.NewTemp("x"), ir.NewTemp("y")
			e := ir.NewBlock()
			e.Statements = []ir.Node{
				testGen.SetPlace(ir.TempCell(x), ir.Const(5)),
				testGen.SetPlace(ir.TempCell(y), testGen.PureInstr(resource.RuntimeFunctionAdd, ir.GetPlace(ir.TempCell(x)), ir.Const(3))),
				testGen.SetPlace(ir.Cell(0, 0), ir.GetPlace(ir.TempCell(y))),
			}
			return e
		},
		"shared_const": func() *ir.BasicBlock {
			x := ir.NewTemp("x")
			e := ir.NewBlock()
			e.Statements = []ir.Node{
				testGen.SetPlace(ir.TempCell(x), ir.Const(5)),
				testGen.SetPlace(ir.Cell(0, 0), ir.GetPlace(ir.TempCell(x))),
				testGen.SetPlace(ir.Cell(0, 1), ir.GetPlace(ir.TempCell(x))),
			}
			return e
		},
	}
	bs["diamond"] = ssaBuilders()["diamond"]
	// A memory read from EngineRom (3000) inlines into a LevelMemory (2000) write
	// once the oracle knows updateParallel can't write EngineRom.
	bs["memory"] = func() *ir.BasicBlock {
		tb := ir.NewTemp("t")
		e := ir.NewBlock()
		e.Statements = []ir.Node{
			testGen.SetPlace(ir.TempCell(tb), ir.GetPlace(ir.Cell(3000, 0))),
			testGen.SetPlace(ir.Cell(2000, 1), ir.GetPlace(ir.TempCell(tb))),
		}
		return e
	}
	return bs
}

func TestInlineVarsGolden(t *testing.T) {
	data, err := os.ReadFile("testdata/optimize_golden.json")
	if err != nil {
		t.Fatal(err)
	}
	var gold struct {
		InlineCases map[string]string `json:"inlineCases"`
	}
	if err := json.Unmarshal(data, &gold); err != nil {
		t.Fatal(err)
	}
	// The memory case needs a real block oracle + callback; the rest are
	// metadata-independent.
	configs := map[string]InlineVars{
		"memory": {Callback: "updateParallel", Oracle: ir.Blocks(ir.ModePlay)},
	}
	for name, build := range inlineBuilders() {
		want, ok := gold.InlineCases[name]
		if !ok {
			t.Fatalf("no InlineVars golden for %q", name)
		}
		t.Run(name, func(t *testing.T) {
			got := ssaCanon(configs[name].Run(testGen, ToSSA{}.Run(testGen, build())))
			if got != want {
				t.Errorf("\n got: %s\nwant: %s", got, want)
			}
		})
	}
}
