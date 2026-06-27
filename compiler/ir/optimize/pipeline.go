package optimize

import "github.com/WindowsSov8forUs/sonolus-go/compiler/ir"

func Standard(mode ir.Mode, callback string) []Pass {
	return []Pass{
		CoalesceFlow{},
		CoalesceSmallConditionalBlocks{},
		UnreachableCodeElimination{},
		DeadCodeElimination{},

		// SSA region — passes that benefit from SSA form.
		ToSSA{},
		SCCP{},
		InlineVars{Aggressive: true, Callback: callback, Oracle: ir.Blocks(mode)},
		FlattenAssociativeOps{},
		RemoveRedundantArguments{},
		RewriteToSwitch{},
		NormalizeSwitch{},
		CSE{},
		LICM{},

		// Second InlineVars round (non-aggressive, no loop-crossing hoists)
		// catches copies introduced by CSE/LICM normalization.
		InlineVars{Aggressive: false, Callback: callback, Oracle: ir.Blocks(mode)},

		FromSSA{},
		UnflattenAssociativeOps{},

		// Post-SSA cleanup — two rounds to approach fixpoint.
		CopyCoalesce{},
		UnreachableCodeElimination{},
		CoalesceFlow{},
		CombineExitBlocks{},
		AdvancedDCE{},
		DeadCodeElimination{},

		// Round 2 (cheap; catches cascading simplification after DCE).
		CoalesceFlow{},
		UnreachableCodeElimination{},
		DeadCodeElimination{},
	}
}

func Optimize(entry *ir.BasicBlock, mode ir.Mode, callback string, tempBlock int) *ir.BasicBlock {
	entry = RunPasses(entry, Standard(mode, callback)...)
	return AllocateLive{BlockID: tempBlock}.Run(entry)
}
