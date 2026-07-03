// Package optimize holds CFG optimization passes, ported from sonolus.py's
// sonolus/backend/optimize. Passes operate in place on an ir.BasicBlock CFG and
// return the (possibly new) entry block.
package optimize

import (
	"context"
	"sort"
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

// PassWithDom is an optional interface for passes that benefit from a cached
// dominance tree. Passes that modify CFG structure should call
// dom.Invalidate() to force recomputation on the next access.
type PassWithDom interface {
	Pass
	RunWithDom(gen *ir.IDGen, entry *ir.BasicBlock, dom *DominanceCache) *ir.BasicBlock
}

// ManagedPass extends Pass with analysis dependency declarations.
// Passes that implement this interface can be validated by VerifyPasses.
type ManagedPass interface {
	Pass
	Requires() []Analysis
	Preserves() []Analysis
	Destroys() []Analysis
}

// BlockOracle answers queries about Sonolus memory blocks that the optimizer
// needs. Implementations are provided by the IR layer (ir.BlockSet) and can be
// mocked for testing.
type BlockOracle interface {
	Writable(block int, callback string) bool
	RuntimeConstant(block int) bool
}

// RunPasses runs passes in order, threading the entry block through each.
// Passes that implement PassWithDom receive a cached dominance tree that is
// shared across the pipeline, avoiding redundant O(N²) recomputation.
func RunPasses(gen *ir.IDGen, entry *ir.BasicBlock, passes ...Pass) *ir.BasicBlock {
	return RunPassesCtx(gen, entry, nil, passes...)
}

// RunPassesCtx is like RunPasses but checks ctx after every pass. If ctx is
// nil or ctx.Done() is nil, cancellation is skipped entirely (zero overhead).
func RunPassesCtx(gen *ir.IDGen, entry *ir.BasicBlock, ctx context.Context, passes ...Pass) *ir.BasicBlock {
	dom := &DominanceCache{}
	for _, p := range passes {
		if pd, ok := p.(PassWithDom); ok {
			entry = pd.RunWithDom(gen, entry, dom)
		} else {
			entry = p.Run(gen, entry)
		}
		if ctx != nil {
			select {
			case <-ctx.Done():
				return entry
			default:
			}
		}
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
// Returns the original slice unchanged when the edge is not found (zero-allocation
// fast path matching the addEdge pattern).
func removeEdge(edges []*ir.FlowEdge, e *ir.FlowEdge) []*ir.FlowEdge {
	for i, x := range edges {
		if x == e {
			out := make([]*ir.FlowEdge, len(edges)-1)
			copy(out, edges[:i])
			copy(out[i:], edges[i+1:])
			return out
		}
	}
	return edges
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

// transformBlocks applies a node transformer to every statement and test in
// every block of the CFG (preorder traversal).
func transformBlocks(entry *ir.BasicBlock, xform func(ir.Node) ir.Node) {
	for _, b := range ir.Preorder(entry) {
		for i, s := range b.Statements {
			b.Statements[i] = xform(s)
		}
		b.Test = xform(b.Test)
	}
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
	// Sort by header preorder number descending, matching sonolus.py licm.py:45.
	// Dominance.Num is assigned during ReversePostorder walk and is deterministic.
	sort.Slice(loops, func(i, j int) bool {
		return dom.Num[loops[i].Header] > dom.Num[loops[j].Header]
	})
	return loops
}
