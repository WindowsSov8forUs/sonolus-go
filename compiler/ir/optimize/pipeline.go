package optimize

import "github.com/WindowsSov8forUs/sonolus-go/compiler/ir"

// Standard returns the ordered optimization pipeline for a callback, modeled on
// sonolus.py's STANDARD pass sequence (restricted to the passes ported so far).
//
// Ordering constraint: CoalesceFlow / UnreachableCodeElimination /
// DeadCodeElimination are phi-unaware in this port, so they run only OUTSIDE the
// SSA region (before ToSSA and after FromSSA). Only the phi-aware passes (SCCP,
// InlineVars) run between ToSSA and FromSSA.
func Standard(mode ir.Mode, callback string) []Pass {
	return []Pass{
		// Pre-SSA cleanup.
		CoalesceFlow{},
		UnreachableCodeElimination{},
		DeadCodeElimination{},

		// SSA region.
		ToSSA{},
		SCCP{},
		InlineVars{Callback: callback, Oracle: ir.Blocks(mode)},
		FromSSA{},

		// Post-SSA cleanup (removes phi copies, folds tests SCCP made constant).
		UnreachableCodeElimination{},
		CoalesceFlow{},
		DeadCodeElimination{},
	}
}

// Optimize runs the standard pipeline and resolves temp blocks to concrete cells
// in tempBlock, yielding a finalize-ready CFG.
func Optimize(entry *ir.BasicBlock, mode ir.Mode, callback string, tempBlock int) *ir.BasicBlock {
	entry = RunPasses(entry, Standard(mode, callback)...)
	return ir.AllocateTempBlocks(entry, tempBlock)
}
