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

// RemoveRedundantArguments strips identity arguments from pure operations:
// Add(a,0) → a, Multiply(a,1) → a, Divide(a,1) → a, Add() → 0, Multiply() → 1.
// Port of sonolus.py simplify.RemoveRedundantArguments.
type RemoveRedundantArguments struct{}

func (RemoveRedundantArguments) Name() string { return "RemoveRedundantArguments" }

func (RemoveRedundantArguments) Run(gen *ir.IDGen, entry *ir.BasicBlock) *ir.BasicBlock {
	for _, b := range ir.Preorder(entry) {
		b.Test = trimArgs(gen, b.Test)
		for i, s := range b.Statements {
			b.Statements[i] = trimArgs(gen, s)
		}
	}
	return entry
}

func trimArgs(gen *ir.IDGen, n ir.Node) ir.Node {
	instr, ok := n.(ir.Instr)
	if !ok || !ir.Pure(instr.Op) {
		return n
	}
	args := instr.Args
	op := instr.Op

	// Remove identity elements: Add(a,0)→a, Sub(a,0)→a, Mul(a,1)→a, Div(a,1)→a
	switch op {
	case opAdd, opSubtract:
		args = filterConst(args, 0)
	case opMultiply, opDivide:
		args = filterConst(args, 1)
	}
	// Fold empty op lists.
	switch op {
	case opAdd, opSubtract:
		if len(args) == 0 {
			return ir.Const(0)
		}
	case opMultiply, opDivide:
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
	return ir.Instr{ID: gen.Next(), Op: op, Args: args, Pure: true}
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

var flattenOps = map[ir.Op]bool{opAdd: true, opMultiply: true, opMod: true, opRem: true}

// FlattenAssociativeOps flattens nested Add/Add chains: a+(b+c) -> a+b+c.
// This lets RemoveRedundantArguments see and strip identity elements (+0, *1).
type FlattenAssociativeOps struct{}

func (FlattenAssociativeOps) Name() string { return "FlattenAssociativeOps" }

func (FlattenAssociativeOps) Run(gen *ir.IDGen, entry *ir.BasicBlock) *ir.BasicBlock {
	for _, b := range ir.Preorder(entry) {
		for i, s := range b.Statements {
			b.Statements[i] = flattenStmt(s)
		}
		b.Test = flattenStmt(b.Test)
	}
	return entry
}

func flattenStmt(n ir.Node) ir.Node {
	instr, ok := n.(ir.Instr)
	if !ok || !flattenOps[instr.Op] {
		return n
	}
	args := make([]ir.Node, 0, len(instr.Args))
	for _, a := range instr.Args {
		a2 := flattenStmt(a)
		if in, ok2 := a2.(ir.Instr); ok2 && in.Op == instr.Op {
			args = append(args, in.Args...)
		} else {
			args = append(args, a2)
		}
	}
	return ir.Instr{ID: instr.ID, Op: instr.Op, Args: args, Pure: true}
}

// UnflattenAssociativeOps restores binary form: a+b+c -> ((a+b)+c). Sonolus
// opcodes are binary, so this must run before finalization.
type UnflattenAssociativeOps struct{}

func (UnflattenAssociativeOps) Name() string { return "UnflattenAssociativeOps" }

func (UnflattenAssociativeOps) Run(gen *ir.IDGen, entry *ir.BasicBlock) *ir.BasicBlock {
	for _, b := range ir.Preorder(entry) {
		for i, s := range b.Statements {
			b.Statements[i] = unflattenStmt(gen, s)
		}
		b.Test = unflattenStmt(gen, b.Test)
	}
	return entry
}

func unflattenStmt(gen *ir.IDGen, n ir.Node) ir.Node {
	instr, ok := n.(ir.Instr)
	if !ok || !flattenOps[instr.Op] || len(instr.Args) <= 2 {
		return n
	}
	args := make([]ir.Node, len(instr.Args))
	for i, a := range instr.Args {
		args[i] = unflattenStmt(gen, a)
	}
	result := args[0]
	for _, arg := range args[1:] {
		result = ir.Instr{ID: gen.Next(), Op: instr.Op, Args: []ir.Node{result, arg}, Pure: true}
	}
	return result
}

// NormalizeSwitch normalizes dense sequential cases: {100,101,102,103} becomes
// {(cond-100)}→{0,1,2,3} by transforming the test expression.
type NormalizeSwitch struct{}

func (NormalizeSwitch) Name() string { return "NormalizeSwitch" }

func (NormalizeSwitch) Run(gen *ir.IDGen, entry *ir.BasicBlock) *ir.BasicBlock {
	for _, b := range ir.Preorder(entry) {
		cases := map[float64]bool{}
		var hasNil bool
		for _, e := range b.Outgoing {
			if e.Cond == nil {
				hasNil = true
			} else {
				cases[*e.Cond] = true
			}
		}
		if len(cases) <= 2 || !hasNil {
			continue
		}
		offset, stride := normSwitchParams(cases)
		if offset == 0 && stride == 1 {
			continue
		}
		for _, e := range b.Outgoing {
			if e.Cond == nil {
				continue
			}
			v := (*e.Cond - offset) / stride
			e.Cond = &v
		}
		if offset != 0 {
			b.Test = gen.PureInstr(opSubtract, b.Test, ir.Const(offset))
		}
		if stride != 1 {
			b.Test = gen.PureInstr(opDivide, b.Test, ir.Const(stride))
		}
	}
	return entry
}

func normSwitchParams(cases map[float64]bool) (offset, stride float64) {
	// Collect and sort case values.
	vals := make([]float64, 0, len(cases))
	for v := range cases {
		vals = append(vals, v)
	}
	for i := 1; i < len(vals); i++ {
		for j := i; j > 0 && vals[j] < vals[j-1]; j-- {
			vals[j], vals[j-1] = vals[j-1], vals[j]
		}
	}
	offset = vals[0]
	stride = vals[1] - offset
	if float64(int(offset)) != offset || float64(int(stride)) != stride {
		return 0, 1
	}
	for i, v := range vals[2:] {
		if v != offset+float64(i+2)*stride {
			return 0, 1
		}
	}
	return offset, stride
}

// CombineExitBlocks merges empty exit blocks (no statements, no outgoing edges)
// into a single canonical exit, reducing block count.
type CombineExitBlocks struct{}

func (CombineExitBlocks) Name() string { return "CombineExitBlocks" }

func (CombineExitBlocks) Run(gen *ir.IDGen, entry *ir.BasicBlock) *ir.BasicBlock {
	var firstExit *ir.BasicBlock
	for _, b := range ir.Preorder(entry) {
		if len(b.Outgoing) == 0 && len(b.Statements) == 0 {
			if firstExit == nil {
				firstExit = b
			} else {
				for _, e := range append([]*ir.FlowEdge{}, b.Incoming...) {
					e.Dst = firstExit
					firstExit.Incoming = append(firstExit.Incoming, e)
					b.Incoming = removeEdge(b.Incoming, e)
				}
			}
		}
	}
	return entry
}

// NormalizeBlocks recursively normalizes the IR tree in each block: nil
// BlockPlace.Index is replaced with Const(0), and all sub-expressions are
// recursively normalized. Port of sonolus.py simplify.NormalizeBlocks.
type NormalizeBlocks struct{}

func (NormalizeBlocks) Name() string { return "NormalizeBlocks" }

func (NormalizeBlocks) Run(gen *ir.IDGen, entry *ir.BasicBlock) *ir.BasicBlock {
	norm := func(n ir.Node) ir.Node {
		if n == nil {
			return nil
		}
		if bp, ok := n.(ir.BlockPlace); ok {
			if bp.Index == nil {
				bp.Index = ir.Const(0)
			}
			return bp
		}
		return n
	}
	for _, b := range ir.Preorder(entry) {
		for si, s := range b.Statements {
			b.Statements[si] = ir.Map(s, norm)
		}
		if b.Test != nil {
			b.Test = ir.Map(b.Test, norm)
		}
	}
	return entry
}
