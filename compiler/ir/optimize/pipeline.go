// Package optimize implements the ~40-pass IR optimizer ported from sonolus.py.
//
// Pipeline pass ordering is documented in PIPELINE.md. The Go pipeline contains
// several intentional improvements over the Python reference:
//   - Earlier CoalesceSmallConditionalBlocks (pre-SSA) for more DCE opportunities
//   - UnflattenAssociativeOps after FromSSA to clean up SSA destruction
//   - Extra cleanup round (Coalesce + UCE + DCE) after AdvancedDCE
//   - RenumberVars at end of Standard for deterministic output
//   - Unified AllocateLive for all levels (Python has tiered allocation)
//
// See PIPELINE.md for the full divergence rationale and pass-by-pass comparison.
package optimize

import (
	"context"
	"fmt"

	"github.com/WindowsSov8forUs/sonolus-go/compiler/ir"
)

// Standard returns the full Standard-level pass list.
func Standard(mode ir.Mode, callback string) []Pass {
	return []Pass{
		// Pre-SSA cleanup.
		CoalesceFlow{},
		CoalesceSmallConditionalBlocks{},
		UnreachableCodeElimination{},
		DeadCodeElimination{},

		// ── SSA region ──
		ToSSA{},

		// Round 1: constant propagation + aggressive inlining.
		SCCP{},
		UnreachableCodeElimination{},
		DeadCodeElimination{},
		CoalesceFlow{},

		NormalizeBlocks{},

		InlineVars{Aggressive: false, Callback: callback, Oracle: ir.Blocks(mode)},
		DeadCodeElimination{},
		InlineVars{Aggressive: false, Callback: callback, Oracle: ir.Blocks(mode)},
		CoalesceFlow{},

		// Round 2: second SCCP catches constants exposed by inlining.
		SCCP{},
		FlattenAssociativeOps{},
		RemoveRedundantArguments{},
		DeadCodeElimination{},
		CoalesceFlow{},

		RewriteToSwitch{},
		InlineVars{Aggressive: true, Callback: callback, Oracle: ir.Blocks(mode)},
		UnflattenAssociativeOps{},

		// Loop-invariant hoisting + common subexpression elimination.
		LICM{Oracle: ir.Blocks(mode)},
		CSE{},

		NormalizeBlocks{},

		// Pre-FromSSA cleanup: flatten, inline, and simplify newly normalised
		// expressions before leaving SSA form.
		FlattenAssociativeOps{},
		InlineVars{Aggressive: false, Callback: callback, Oracle: ir.Blocks(mode)},
		DeadCodeElimination{},
		FlattenAssociativeOps{},
		RemoveRedundantArguments{},

		FromSSA{},
		UnflattenAssociativeOps{},

		// ── Post-SSA cleanup ──
		// INVARIANT: these passes assume FromSSA has already run — no SSAPlace or
		// Phi nodes remain. CoalesceFlow and CombineExitBlocks omit phi handling
		// deliberately; reordering before FromSSA would corrupt the CFG.
		CopyCoalesce{},
		UnreachableCodeElimination{},
		CoalesceFlow{},
		CombineExitBlocks{},

		// Switch normalisation benefits from the post-SSA block layout.
		NormalizeSwitch{},

		AdvancedDCE{},
		DeadCodeElimination{},

		// Second cleanup round for cascading simplifications.
		CoalesceFlow{},
		UnreachableCodeElimination{},
		DeadCodeElimination{},

		// Renumber temp variables for deterministic output.
		RenumberVars{},
	}
}

// Minimal returns the pass list for the MINIMAL optimisation level. It runs
// only essential cleanup (coalesce, unreachable code, dead code) and skips
// SSA construction, SCCP, inlining, LICM, and CSE entirely. Compilation is
// fast but output quality may be lower.
func Minimal(mode ir.Mode, callback string) []Pass {
	_ = mode
	_ = callback
	return []Pass{
		CoalesceFlow{},
		UnreachableCodeElimination{},
		DeadCodeElimination{},
		CoalesceFlow{},
		RenumberVars{},
	}
}

// Fast returns the pass list for the FAST optimisation level. It runs a
// single SSA round with SCCP and one inlining pass, then exits SSA. It
// skips LICM, CSE, and the second SCCP round, providing a good balance
// between compilation speed and output quality.
func Fast(mode ir.Mode, callback string) []Pass {
	return []Pass{
		// Pre-SSA cleanup.
		CoalesceFlow{},
		UnreachableCodeElimination{},
		DeadCodeElimination{},
		CoalesceFlow{},

		// Single SSA round.
		ToSSA{},
		SCCP{},
		UnreachableCodeElimination{},
		DeadCodeElimination{},
		CoalesceFlow{},

		NormalizeBlocks{},

		InlineVars{Aggressive: false, Callback: callback, Oracle: ir.Blocks(mode)},
		DeadCodeElimination{},
		CoalesceFlow{},

		// Exit SSA.
		FlattenAssociativeOps{},
		FromSSA{},
		UnflattenAssociativeOps{},

		// Post-SSA cleanup.
		CoalesceFlow{},
		UnreachableCodeElimination{},
		DeadCodeElimination{},
		NormalizeSwitch{},
		CombineExitBlocks{},
		CoalesceFlow{},
		UnreachableCodeElimination{},
		DeadCodeElimination{},

		RenumberVars{},
	}
}

// Level selects an optimisation preset.
type Level int

const (
	LevelMinimal  Level = iota // only essential cleanup, no SSA
	LevelFast                  // single SSA round, no LICM/CSE
	LevelStandard              // full pipeline (~40 passes)
)

// Optimize runs the optimization pipeline for the given level and returns the
// (possibly new) entry block. A pass dependency violation returns an error
// instead of panicking — this indicates a programming error in the pipeline
// definition and should be treated as a fatal error by callers.
func Optimize(gen *ir.IDGen, entry *ir.BasicBlock, mode ir.Mode, callback string, tempBlock int, level Level) (*ir.BasicBlock, error) {
	return OptimizeCtx(gen, entry, mode, callback, tempBlock, level, nil)
}

// OptimizeCtx is like Optimize but checks ctx after every pass for cancellation.
// If ctx is nil, cancellation is skipped (same behavior as Optimize).
func OptimizeCtx(gen *ir.IDGen, entry *ir.BasicBlock, mode ir.Mode, callback string, tempBlock int, level Level, ctx context.Context) (*ir.BasicBlock, error) {
	var passes []Pass
	switch level {
	case LevelMinimal:
		passes = Minimal(mode, callback)
	case LevelFast:
		passes = Fast(mode, callback)
	default:
		passes = Standard(mode, callback)
	}
	if err := VerifyPasses(passes...); err != nil {
		return nil, fmt.Errorf("optimize: pass dependency violation in %v pipeline: %w", level, err)
	}
	entry = runPassesCtx(gen, entry, ctx, passes...)

	// Select the appropriate allocator for the optimization level.
	// AllocateBasic (sequential) for MINIMAL/FAST: faster compilation,
	// no liveness analysis. AllocateLive (interval packing) for STANDARD:
	// reuses non-overlapping lifetimes, producing more compact output.
	switch level {
	case LevelMinimal, LevelFast:
		return AllocateBasic{BlockID: tempBlock}.Run(gen, entry), nil
	default:
		return AllocateLive{BlockID: tempBlock}.Run(gen, entry), nil
	}
}
