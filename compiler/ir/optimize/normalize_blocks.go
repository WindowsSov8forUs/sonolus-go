package optimize

import "github.com/WindowsSov8forUs/sonolus-go/compiler/ir"

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
