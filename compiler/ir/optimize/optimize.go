// Package optimize holds CFG optimization passes, ported from sonolus.py's
// sonolus/backend/optimize. Passes operate in place on an ir.BasicBlock CFG and
// return the (possibly new) entry block.
package optimize

import (
	"fmt"

	"github.com/WindowsSov8forUs/sonolus-go/compiler/ir"
)

// inlineCostThreshold is the maximum total cost of an expression we are willing to
// recompute in order to avoid storing it in a temporary. Used by CSE and LICM.
const inlineCostThreshold = 4

// Analysis names a CFG analysis that passes may depend on.
// Port of sonolus.py's pass analysis system.
type Analysis string

const (
	AnalysisDominance Analysis = "Dominance"
	AnalysisSSA       Analysis = "SSA"
)

// Pass is a single CFG transformation.
type Pass interface {
	Name() string
	Run(gen *ir.IDGen, entry *ir.BasicBlock) *ir.BasicBlock
}

// ManagedPass extends Pass with analysis dependency declarations.
// Passes that implement this interface can be validated by VerifyPasses.
type ManagedPass interface {
	Pass
	Requires() []Analysis
	Preserves() []Analysis
	Destroys() []Analysis
}

// RunPasses runs passes in order, threading the entry block through each.
func RunPasses(gen *ir.IDGen, entry *ir.BasicBlock, passes ...Pass) *ir.BasicBlock {
	for _, p := range passes {
		entry = p.Run(gen, entry)
	}
	return entry
}

// VerifyPasses checks that the given pass sequence is valid without running it.
// Useful for testing the Standard pipeline.
func VerifyPasses(passes ...Pass) error {
	available := map[Analysis]bool{}

	for i, p := range passes {
		if mp, ok := p.(ManagedPass); ok {
			for _, req := range mp.Requires() {
				if !available[req] {
					return fmt.Errorf("pass %d (%s): required analysis %q not available", i, p.Name(), req)
				}
			}
		}
		if mp, ok := p.(ManagedPass); ok {
			for _, d := range mp.Destroys() {
				delete(available, d)
			}
			for _, pr := range mp.Preserves() {
				available[pr] = true
			}
		}
	}
	return nil
}

// --- edge-set helpers (Incoming/Outgoing are treated as sets keyed by pointer) ---

// removeEdge filters out a specific edge from a slice (identity comparison).
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

// exprCost returns the total tree cost of an IR node: 1 for leaf nodes, 1 + sum
// of arg costs for instruction nodes. Used by CSE and LICM to decide whether to
// recompute or cache a value.
func exprCost(n ir.Node) int {
	if instr, ok := n.(ir.Instr); ok {
		s := 1
		for _, a := range instr.Args {
			s += exprCost(a)
		}
		return s
	}
	return 1
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

// Loop holds a discovered natural loop: its header, all back-edge latches,
// and the body blocks (including header and latches).
type Loop struct {
	Header  *ir.BasicBlock
	Latches []*ir.BasicBlock
	Body    map[*ir.BasicBlock]bool
}

// computeLoopBody returns the set of blocks belonging to a natural loop identified
// by header and latch. Blocks are discovered by traversing incoming edges backward
// from the latch, stopping at the header.
func computeLoopBody(header, latch *ir.BasicBlock) map[*ir.BasicBlock]bool {
	body := map[*ir.BasicBlock]bool{header: true}
	if latch == header {
		return body
	}
	body[latch] = true
	worklist := []*ir.BasicBlock{latch}
	for len(worklist) > 0 {
		b := worklist[len(worklist)-1]
		worklist = worklist[:len(worklist)-1]
		for _, e := range b.Incoming {
			if !body[e.Src] {
				body[e.Src] = true
				worklist = append(worklist, e.Src)
			}
		}
	}
	return body
}

// FindLoops discovers all natural loops in the CFG via dominance back-edges.
// It is used by both LICM and InlineVars to avoid duplicating loop discovery.
func FindLoops(blocks []*ir.BasicBlock, dom *Dominance) []Loop {
	latchesByHeader := map[*ir.BasicBlock][]*ir.BasicBlock{}
	for _, b := range blocks {
		for _, e := range b.Outgoing {
			if dominates(dom, e.Dst, e.Src) {
				latchesByHeader[e.Dst] = append(latchesByHeader[e.Dst], e.Src)
			}
		}
	}
	var loops []Loop
	for header, latches := range latchesByHeader {
		body := map[*ir.BasicBlock]bool{}
		for _, latch := range latches {
			for b := range computeLoopBody(header, latch) {
				body[b] = true
			}
		}
		loops = append(loops, Loop{Header: header, Latches: latches, Body: body})
	}
	return loops
}
