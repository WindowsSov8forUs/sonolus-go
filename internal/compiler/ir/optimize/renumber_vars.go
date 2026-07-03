package optimize

import "github.com/WindowsSov8forUs/sonolus-go/internal/compiler/ir"

// RenumberVars reassigns sequential names to TempBlocks in preorder so the
// output is deterministic across runs. Port of sonolus.py simplify.RenumberVars.
type RenumberVars struct{}

func (RenumberVars) Name() string { return "RenumberVars" }

func (RenumberVars) Run(gen *ir.IDGen, entry *ir.BasicBlock) *ir.BasicBlock {
	blocks := ir.Preorder(entry)

	// Build replacement map: old TempBlock → renumbered TempBlock.
	renumber := map[*ir.TempBlock]*ir.TempBlock{}
	counter := 0
	nextName := func() string {
		n := ""
		c := counter
		for {
			n = string(rune('0'+c%10)) + n
			c /= 10
			if c == 0 {
				break
			}
		}
		counter++
		return n
	}

	// First pass: collect temps via Walk (read-only; avoids allocations from Map).
	collect := func(n ir.Node) {
		if tb, ok := n.(*ir.TempBlock); ok {
			if _, seen := renumber[tb]; !seen {
				renumber[tb] = &ir.TempBlock{Name: nextName(), Size: tb.Size}
			}
		}
	}
	for _, b := range blocks {
		for _, s := range b.Statements {
			ir.Walk(s, collect)
		}
		if b.Test != nil {
			ir.Walk(b.Test, collect)
		}
	}
	if len(renumber) == 0 {
		return entry
	}

	// Second pass: rewrite all TempBlock references.
	remap := func(n ir.Node) ir.Node {
		if tb, ok := n.(*ir.TempBlock); ok {
			if r, ok := renumber[tb]; ok {
				return r
			}
		}
		return n
	}
	for _, b := range blocks {
		for si, s := range b.Statements {
			b.Statements[si] = ir.Map(s, remap)
		}
		if b.Test != nil {
			b.Test = ir.Map(b.Test, remap)
		}
	}
	for _, b := range blocks {
		for _, phi := range b.Phis {
			if newVar, ok := renumber[phi.Var]; ok {
				phi.Var = newVar
			}
			for k, v := range phi.Args {
				phi.Args[k] = ir.Map(v, remap).(ir.Place)
			}
		}
	}
	return entry
}
