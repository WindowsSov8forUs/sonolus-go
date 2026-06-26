package optimize

import "github.com/WindowsSov8forUs/sonolus-go/compiler/ir"

// CoalesceSmallConditionalBlocks merges blocks with 1 outgoing edge whose target
// has <= 1 statement. This collapses trivial passthroughs produced by frontend
// constructs (switch cases, if/else empty branches) without the full complexity
// of CoalesceFlow (which needs phi handling).
type CoalesceSmallConditionalBlocks struct{}

func (CoalesceSmallConditionalBlocks) Name() string { return "CoalesceSmallCond" }

func (CoalesceSmallConditionalBlocks) Run(entry *ir.BasicBlock) *ir.BasicBlock {
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
			nextBlock.Incoming = removeEdgeItem(nextBlock.Incoming, nextEdge)
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
	for _, b := range allBlocks(entry) {
		for _, e := range b.Incoming {
			if !reachable[e.Src] {
				b.Incoming = removeEdgeItem(b.Incoming, e)
			}
		}
	}
	return entry
}

// RemoveRedundantArguments strips identity arguments from pure operations:
// Add(a,0) → a, Multiply(a,1) → a, Divide(a,1) → a, Add() → 0, Multiply() → 1.
// Port of sonolus.py simplify.RemoveRedundantArguments.
type RemoveRedundantArguments struct{}

func (RemoveRedundantArguments) Name() string { return "TrimArgs" }

func (RemoveRedundantArguments) Run(entry *ir.BasicBlock) *ir.BasicBlock {
	for _, b := range ir.Preorder(entry) {
		for i, s := range b.Statements {
			b.Statements[i] = trimArgs(s)
		}
	}
	return entry
}

func trimArgs(n ir.Node) ir.Node {
	instr, ok := n.(ir.Instr)
	if !ok || !ir.Pure(instr.Op) {
		return n
	}
	args := instr.Args
	op := instr.Op

	// Remove identity elements: Add(a,0)→a, Sub(a,0)→a, Mul(a,1)→a, Div(a,1)→a
	switch op {
	case "Add", "Subtract":
		args = filterConst(args, 0)
	case "Multiply", "Divide":
		args = filterConst(args, 1)
	}
	// Fold empty op lists.
	switch op {
	case "Add", "Subtract":
		if len(args) == 0 {
			return ir.Const(0)
		}
	case "Multiply", "Divide":
		if len(args) == 0 {
			return ir.Const(1)
		}
	}
	if len(args) == len(instr.Args) && sameArgs(args, instr.Args) {
		return n
	}
	if len(args) == 1 {
		return args[0]
	}
	return ir.Instr{Op: op, Args: args, Pure: true}
}

func filterConst(args []ir.Node, drop float64) []ir.Node {
	var out []ir.Node
	for _, a := range args {
		if c, ok := a.(ir.Const); ok && float64(c) == drop {
			continue
		}
		out = append(out, a)
	}
	return out
}

func sameArgs(a, b []ir.Node) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func removeEdgeItem(edges []*ir.FlowEdge, target *ir.FlowEdge) []*ir.FlowEdge {
	out := make([]*ir.FlowEdge, 0, len(edges))
	for _, e := range edges {
		if e != target {
			out = append(out, e)
		}
	}
	return out
}

func allBlocks(entry *ir.BasicBlock) []*ir.BasicBlock {
	var out []*ir.BasicBlock
	seen := map[*ir.BasicBlock]bool{}
	stack := []*ir.BasicBlock{entry}
	for len(stack) > 0 {
		b := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if seen[b] {
			continue
		}
		seen[b] = true
		out = append(out, b)
		for _, e := range b.Outgoing {
			stack = append(stack, e.Dst)
		}
	}
	return out
}
