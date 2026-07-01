package optimize

import "github.com/WindowsSov8forUs/sonolus-go/compiler/ir"

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
