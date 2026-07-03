package optimize

import "github.com/WindowsSov8forUs/sonolus-go/internal/compiler/ir"

// UnreachableCodeElimination folds constant branch tests (keeping only the taken
// edge) and removes edges originating from blocks that become unreachable. Port
// of sonolus.py dead_code.UnreachableCodeElimination (phi handling omitted).
type UnreachableCodeElimination struct{}

func (UnreachableCodeElimination) Name() string { return "UnreachableCodeElimination" }

func (UnreachableCodeElimination) Requires() []Analysis  { return nil }
func (UnreachableCodeElimination) Preserves() []Analysis { return nil }
func (UnreachableCodeElimination) Destroys() []Analysis  { return []Analysis{AnalysisDominance} }

func (UnreachableCodeElimination) Run(gen *ir.IDGen, entry *ir.BasicBlock) *ir.BasicBlock {
	original := reachable(entry)

	visited := map[*ir.BasicBlock]bool{}
	worklist := []*ir.BasicBlock{entry}
	for len(worklist) > 0 {
		b := worklist[len(worklist)-1]
		worklist = worklist[:len(worklist)-1]
		if visited[b] {
			continue
		}
		visited[b] = true

		c, isConst := b.Test.(ir.Const)
		if !isConst {
			for _, e := range b.Outgoing {
				worklist = append(worklist, e.Dst)
			}
			continue
		}

		// Constant test: only the matching (or default) edge is taken.
		val := float64(c)
		b.Test = ir.Const(0)

		var taken *ir.FlowEdge
		for _, e := range b.Outgoing {
			if e.Cond != nil && *e.Cond == val {
				taken = e
				break
			}
		}
		if taken == nil {
			for _, e := range b.Outgoing {
				if e.Cond == nil {
					taken = e
					break
				}
			}
		}

		for _, e := range append([]*ir.FlowEdge{}, b.Outgoing...) {
			if e != taken {
				e.Dst.Incoming = removeEdge(e.Dst.Incoming, e)
				b.Outgoing = removeEdge(b.Outgoing, e)
			}
		}
		if taken != nil {
			taken.Cond = nil
			worklist = append(worklist, taken.Dst)
		}
	}

	// Drop edges leaving blocks that are no longer reachable.
	for b := range original {
		if !visited[b] {
			for _, e := range b.Outgoing {
				e.Dst.Incoming = removeEdge(e.Dst.Incoming, e)
			}
		}
	}

	return entry
}
