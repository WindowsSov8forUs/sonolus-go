package optimize

import (
	"testing"

	"github.com/WindowsSov8forUs/sonolus-go/compiler/ir"
)

// benchEntry builds a moderate-sized CFG (a loop with arithmetic and branches)
// suitable for per-pass benchmarking.
func benchEntry(gen *ir.IDGen) *ir.BasicBlock {
	entry := ir.NewBlock()
	loop := ir.NewBlock()
	body := ir.NewBlock()
	exit := ir.NewBlock()

	entry.Statements = []ir.Node{
		ir.Set{ID: gen.Next(), Place: ir.Cell(0, 0), Value: ir.Const(0)},
		ir.Set{ID: gen.Next(), Place: ir.Cell(0, 1), Value: ir.Const(1000)},
	}
	entry.Test = ir.Const(1)
	entry.ConnectTo(loop, nil)

	loop.Test = gen.PureInstr(opLess, ir.GetPlace(ir.Cell(0, 0)), ir.GetPlace(ir.Cell(0, 1)))
	loop.ConnectTo(exit, ir.Cond(0))
	loop.ConnectTo(body, nil)

	body.Statements = []ir.Node{
		ir.Set{ID: gen.Next(), Place: ir.Cell(0, 0), Value: gen.PureInstr(
			opAdd, ir.GetPlace(ir.Cell(0, 0)), ir.Const(1),
		)},
		// Computable arithmetic to exercise constant propagation.
		ir.Set{ID: gen.Next(), Place: ir.Cell(0, 2), Value: gen.PureInstr(
			opMultiply,
			gen.PureInstr(opAdd, ir.Const(3), ir.Const(5)),
			gen.PureInstr(opSubtract, ir.Const(10), ir.Const(2)),
		)},
		// Redundant expression that CSE should dedup.
		ir.Set{ID: gen.Next(), Place: ir.Cell(0, 3), Value: gen.PureInstr(
			opMultiply,
			gen.PureInstr(opAdd, ir.Const(3), ir.Const(5)),
			gen.PureInstr(opSubtract, ir.Const(10), ir.Const(2)),
		)},
	}
	body.Test = ir.Const(1)
	body.ConnectTo(loop, nil)

	exit.Test = ir.Const(0)
	return entry
}

func BenchmarkSCCP(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		gen := ir.NewIDGen()
		entry := benchEntry(gen)
		s := SCCP{}
		s.Run(gen, entry)
	}
}

func BenchmarkCSE(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		gen := ir.NewIDGen()
		entry := benchEntry(gen)
		ToSSA{}.Run(gen, entry)
		c := CSE{}
		c.Run(gen, entry)
	}
}

func BenchmarkLICM(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		gen := ir.NewIDGen()
		entry := benchEntry(gen)
		ToSSA{}.Run(gen, entry)
		SCCP{}.Run(gen, entry)
		l := LICM{Oracle: ir.Blocks(ir.ModePlay)}
		l.Run(gen, entry)
	}
}

func BenchmarkInlining(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		gen := ir.NewIDGen()
		entry := benchEntry(gen)
		ToSSA{}.Run(gen, entry)
		i := InlineVars{Aggressive: false, Callback: "test", Oracle: ir.Blocks(ir.ModePlay)}
		i.Run(gen, entry)
	}
}
