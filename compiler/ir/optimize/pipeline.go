package optimize

import "github.com/WindowsSov8forUs/sonolus-go/compiler/ir"

func Standard(mode ir.Mode, callback string) []Pass {
	return []Pass{
		CoalesceFlow{},
		CoalesceSmallConditionalBlocks{},
		UnreachableCodeElimination{},
		DeadCodeElimination{},

		ToSSA{},
		SCCP{},
		InlineVars{Aggressive: true, Callback: callback, Oracle: ir.Blocks(mode)},
		FlattenAssociativeOps{},
		RemoveRedundantArguments{},
		RewriteToSwitch{},
		NormalizeSwitch{},
		CSE{},
		LICM{},
		FromSSA{},
		UnflattenAssociativeOps{},

		CopyCoalesce{},
		UnreachableCodeElimination{},
		CoalesceFlow{},
		CombineExitBlocks{},
		AdvancedDCE{},
		DeadCodeElimination{},
	}
}

func Optimize(entry *ir.BasicBlock, mode ir.Mode, callback string, tempBlock int) *ir.BasicBlock {
	entry = RunPasses(entry, Standard(mode, callback)...)
	return AllocateLive{BlockID: tempBlock}.Run(entry)
}
