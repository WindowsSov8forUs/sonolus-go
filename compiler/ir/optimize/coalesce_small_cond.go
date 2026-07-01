package optimize

import "github.com/WindowsSov8forUs/sonolus-go/compiler/ir"

// CoalesceSmallConditionalBlocks merges blocks with 1 outgoing edge whose target
// has <= 1 statement. This collapses trivial passthroughs produced by frontend
// constructs (switch cases, if/else empty branches) without the full complexity
// of CoalesceFlow (which needs phi handling).
type CoalesceSmallConditionalBlocks struct{}

func (CoalesceSmallConditionalBlocks) Name() string { return "CoalesceSmallConditionalBlocks" }

func (CoalesceSmallConditionalBlocks) Run(gen *ir.IDGen, entry *ir.BasicBlock) *ir.BasicBlock {
	queue := []*ir.BasicBlock{entry}
	processed := map[*ir.BasicBlock]bool{}

	for len(queue) > 0 {
		block := queue[len(queue)-1]
		queue = queue[:len(queue)-1]
		if processed[block] {
			continue
		}
		processed[block] = true

		// Keep slurping up single-exit blocks whose target is small.
		for len(block.Outgoing) == 1 {
			nextEdge := block.Outgoing[0]
			nextBlock := nextEdge.Dst
			if len(nextBlock.Statements) > 1 {
				break
			}
			// Graft nextBlock into this one.
			nextBlock.Incoming = removeEdge(nextBlock.Incoming, nextEdge)
			block.Test = nextBlock.Test
			block.Outgoing = nextBlock.Outgoing
			block.Statements = append(block.Statements, nextBlock.Statements...)
			for _, e := range block.Outgoing {
				e.Src = block
			}
		}

		for _, e := range block.Outgoing {
			queue = append(queue, e.Dst)
		}
	}

	// Drop edges from unreachable blocks.
	reachable := map[*ir.BasicBlock]bool{entry: true}
	stack := []*ir.BasicBlock{entry}
	for len(stack) > 0 {
		b := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		for _, e := range b.Outgoing {
			if !reachable[e.Dst] {
				reachable[e.Dst] = true
				stack = append(stack, e.Dst)
			}
		}
	}
	for _, b := range ir.Preorder(entry) {
		for _, e := range b.Incoming {
			if !reachable[e.Src] {
				b.Incoming = removeEdge(b.Incoming, e)
			}
		}
	}
	return entry
}
