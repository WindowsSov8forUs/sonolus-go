// Package optimize holds CFG optimization passes, ported from sonolus.py's
// sonolus/backend/optimize. Passes operate in place on an ir.BasicBlock CFG and
// return the (possibly new) entry block.
//
// The current passes are pre-SSA and phi-free; the phi handling in the
// reference passes is intentionally omitted until SSA is implemented.
package optimize

import "github.com/WindowsSov8forUs/sonolus-go/compiler/ir"

// Pass is a single CFG transformation.
type Pass interface {
	Name() string
	Run(entry *ir.BasicBlock) *ir.BasicBlock
}

// RunPasses runs passes in order, threading the entry block through each.
func RunPasses(entry *ir.BasicBlock, passes ...Pass) *ir.BasicBlock {
	for _, p := range passes {
		entry = p.Run(entry)
	}
	return entry
}

// --- edge-set helpers (Incoming/Outgoing are treated as sets keyed by pointer) ---

func removeEdge(edges []*ir.FlowEdge, e *ir.FlowEdge) []*ir.FlowEdge {
	out := make([]*ir.FlowEdge, 0, len(edges))
	for _, x := range edges {
		if x != e {
			out = append(out, x)
		}
	}
	return out
}

func addEdge(edges []*ir.FlowEdge, e *ir.FlowEdge) []*ir.FlowEdge {
	for _, x := range edges {
		if x == e {
			return edges
		}
	}
	return append(edges, e)
}

// reachable returns the set of blocks reachable from entry via outgoing edges.
func reachable(entry *ir.BasicBlock) map[*ir.BasicBlock]bool {
	seen := map[*ir.BasicBlock]bool{}
	stack := []*ir.BasicBlock{entry}
	for len(stack) > 0 {
		b := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if seen[b] {
			continue
		}
		seen[b] = true
		for _, e := range b.Outgoing {
			stack = append(stack, e.Dst)
		}
	}
	return seen
}
