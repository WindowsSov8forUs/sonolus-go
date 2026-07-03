package optimize

import "github.com/WindowsSov8forUs/sonolus-go/internal/compiler/ir"

// CoalesceFlow simplifies control flow: it skips over empty pass-through blocks,
// removes duplicate edges to a default target, and merges linear block chains.
// Port of sonolus.py simplify.CoalesceFlow (phi handling omitted).
type CoalesceFlow struct{}

func (CoalesceFlow) Name() string { return "CoalesceFlow" }

func (CoalesceFlow) Run(gen *ir.IDGen, entry *ir.BasicBlock) *ir.BasicBlock {
	queue := []*ir.BasicBlock{entry}
	processed := map[*ir.BasicBlock]bool{}

	for len(queue) > 0 {
		block := queue[len(queue)-1]
		queue = queue[:len(queue)-1]
		if processed[block] {
			continue
		}
		processed[block] = true

		// Skip over empty single-exit pass-through blocks.
		for _, edge := range block.Outgoing {
			for {
				dst := edge.Dst
				if len(dst.Statements) > 0 || len(dst.Outgoing) != 1 || dst == block || dst == entry {
					break
				}
				nextDst := dst.Outgoing[0].Dst

				dst.Incoming = removeEdge(dst.Incoming, edge)
				if len(dst.Incoming) == 0 {
					for _, de := range dst.Outgoing {
						de.Dst.Incoming = removeEdge(de.Dst.Incoming, de)
					}
					processed[dst] = true
				}
				edge.Dst = nextDst
				nextDst.Incoming = addEdge(nextDst.Incoming, edge)
				if dst == edge.Dst {
					break
				}
			}
		}

		// Remove conditional edges that duplicate the default target.
		var defaultEdge *ir.FlowEdge
		for _, e := range block.Outgoing {
			if e.Cond == nil {
				defaultEdge = e
				break
			}
		}
		if defaultEdge != nil {
			for _, e := range append([]*ir.FlowEdge{}, block.Outgoing...) {
				if e == defaultEdge {
					continue
				}
				if e.Dst == defaultEdge.Dst {
					block.Outgoing = removeEdge(block.Outgoing, e)
					e.Dst.Incoming = removeEdge(e.Dst.Incoming, e)
				}
			}
		}

		if len(block.Outgoing) != 1 {
			for _, e := range block.Outgoing {
				queue = append(queue, e.Dst)
			}
			continue
		}

		nextBlock := block.Outgoing[0].Dst
		if nextBlock == block || nextBlock == entry {
			continue
		}

		if len(nextBlock.Incoming) != 1 {
			queue = append(queue, nextBlock)
			// If this block is empty, bypass it entirely.
			if len(block.Statements) == 0 {
				for _, e := range block.Incoming {
					e.Dst = nextBlock
					nextBlock.Incoming = addEdge(nextBlock.Incoming, e)
				}
				for _, e := range block.Outgoing {
					nextBlock.Incoming = removeEdge(nextBlock.Incoming, e)
				}
				if block == entry {
					entry = nextBlock
				}
			}
			continue
		}

		// nextBlock has a single predecessor (this block): merge it in.
		block.Statements = append(block.Statements, nextBlock.Statements...)
		block.Test = nextBlock.Test
		block.Outgoing = nextBlock.Outgoing
		for _, e := range block.Outgoing {
			e.Src = block
		}
		processed[nextBlock] = true
		for _, e := range block.Outgoing {
			queue = append(queue, e.Dst)
		}
		delete(processed, block)
		queue = append(queue, block)
	}

	return entry
}

// Requires: none (runs early, before dominance/SSA).
func (CoalesceFlow) Requires() []Analysis  { return nil }
func (CoalesceFlow) Preserves() []Analysis { return nil }
func (CoalesceFlow) Destroys() []Analysis  { return []Analysis{AnalysisDominance} }
