package optimize

import "github.com/WindowsSov8forUs/sonolus-go/compiler/ir"

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
	case opAdd:
		args = filterConst(args, 0)
	case opSubtract:
		args = filterConst(args, 0)
		// Sub(0, a) → Negate(a) — check after filterConst for n-ary case
		// (e.g. Sub(0, x, 0) flattened to 3-arg form).
		if len(args) == 2 {
			if c, ok := args[0].(ir.Const); ok && float64(c) == 0 {
				return ir.Instr{ID: gen.Next(), Op: opNegate, Args: []ir.Node{args[1]}, Pure: true}
			}
		}
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
