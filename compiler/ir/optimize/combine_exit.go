package optimize

import "github.com/WindowsSov8forUs/sonolus-go/compiler/ir"

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
