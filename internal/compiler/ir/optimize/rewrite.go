package optimize

import (
	"github.com/WindowsSov8forUs/sonolus-go/internal/compiler/ir"
)

// RewriteToSwitch converts if-else chains comparing against constants into switch
// statements. Phase 1 swaps Equal(const, a) tests to make the const the edge
// condition. Phase 2 merges chained if-else-if blocks that share the same test
// expression and have empty successor blocks.
type RewriteToSwitch struct{}

func (RewriteToSwitch) Name() string { return "RewriteToSwitch" }

func (RewriteToSwitch) Requires() []Analysis  { return nil }
func (RewriteToSwitch) Preserves() []Analysis { return nil }
func (RewriteToSwitch) Destroys() []Analysis  { return []Analysis{AnalysisDominance} }

func (RewriteToSwitch) Run(gen *ir.IDGen, entry *ir.BasicBlock) *ir.BasicBlock {
	rewriteIfsToSwitch(entry)
	rewriteCombineBlocks(entry)
	rewriteRemoveUnreachable(entry)
	return entry
}

func rewriteIfsToSwitch(entry *ir.BasicBlock) {
	for _, b := range ir.Preorder(entry) {
		if len(b.Outgoing) != 2 {
			continue
		}
		// Must have exactly None+0 edges.
		hasNone := false
		hasZero := false
		for _, e := range b.Outgoing {
			if e.Cond == nil {
				hasNone = true
			} else if e.Cond != nil && *e.Cond == 0 {
				hasZero = true
			}
		}
		if !hasNone || !hasZero {
			continue
		}
		instr, ok := b.Test.(ir.Instr)
		if !ok || instr.Op != opEqual || len(instr.Args) != 2 {
			continue
		}
		var constVal *float64
		var other ir.Node
		if c, ok := instr.Args[0].(ir.Const); ok {
			v := float64(c)
			constVal = &v
			other = instr.Args[1]
		} else if c, ok := instr.Args[1].(ir.Const); ok {
			v := float64(c)
			constVal = &v
			other = instr.Args[0]
		} else {
			continue
		}
		// Swap: const becomes the edge condition, other becomes the test.
		b.Test = other
		for _, e := range b.Outgoing {
			if e.Cond == nil {
				e.Cond = constVal
			} else {
				e.Cond = nil
			}
		}
	}
}

func rewriteCombineBlocks(entry *ir.BasicBlock) {
	queue := []*ir.BasicBlock{entry}
	processed := map[*ir.BasicBlock]bool{}

	for len(queue) > 0 {
		block := queue[len(queue)-1]
		queue = queue[:len(queue)-1]
		if processed[block] {
			continue
		}
		processed[block] = true
		for _, e := range block.Outgoing {
			queue = append(queue, e.Dst)
		}

		// Find the default (nil) edge.
		var defaultEdge *ir.FlowEdge
		for _, e := range block.Outgoing {
			if e.Cond == nil {
				defaultEdge = e
				break
			}
		}
		if defaultEdge == nil {
			continue
		}
		nextBlock := defaultEdge.Dst
		// Skip if next has multiple incoming, statements, or is the same block.
		if len(nextBlock.Incoming) > 1 || len(nextBlock.Statements) > 0 || block == nextBlock || nextBlock == entry {
			continue
		}
		// The test expression must be structurally identical.
		if !nodeEqual(block.Test, nextBlock.Test) {
			continue
		}
		// Merge: graft next's outgoing edges onto block.
		block.Outgoing = removeEdge(block.Outgoing, defaultEdge)
		nextBlock.Incoming = removeEdge(nextBlock.Incoming, defaultEdge)

		for _, e := range nextBlock.Outgoing {
			if hasCond(block.Outgoing, e.Cond) {
				e.Dst.Incoming = removeEdge(e.Dst.Incoming, e)
				continue
			}
			e.Src = block
			block.Outgoing = append(block.Outgoing, e)
		}
		processed[nextBlock] = true
		delete(processed, block)
		queue = append(queue, block)
	}
}

func hasCond(edges []*ir.FlowEdge, cond *float64) bool {
	for _, e := range edges {
		if cond == nil && e.Cond == nil {
			return true
		}
		if cond != nil && e.Cond != nil && *cond == *e.Cond {
			return true
		}
	}
	return false
}

func rewriteRemoveUnreachable(entry *ir.BasicBlock) {
	r := reachable(entry)
	for _, b := range ir.Preorder(entry) {
		b.Incoming = filterIncoming(b.Incoming, r)
	}
}

func filterIncoming(edges []*ir.FlowEdge, reachable map[*ir.BasicBlock]bool) []*ir.FlowEdge {
	var out []*ir.FlowEdge
	for _, e := range edges {
		if reachable[e.Src] {
			out = append(out, e)
		}
	}
	return out
}
