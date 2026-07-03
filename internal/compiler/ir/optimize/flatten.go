package optimize

import "github.com/WindowsSov8forUs/sonolus-go/internal/compiler/ir"

var flattenOps = map[ir.Op]bool{opAdd: true, opMultiply: true, opMod: true, opRem: true}

// FlattenAssociativeOps flattens nested Add/Add chains: a+(b+c) -> a+b+c.
// This lets RemoveRedundantArguments see and strip identity elements (+0, *1).
type FlattenAssociativeOps struct{}

func (FlattenAssociativeOps) Name() string { return "FlattenAssociativeOps" }

func (FlattenAssociativeOps) Run(gen *ir.IDGen, entry *ir.BasicBlock) *ir.BasicBlock {
	transformBlocks(entry, func(n ir.Node) ir.Node { return flattenStmt(n) })
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
