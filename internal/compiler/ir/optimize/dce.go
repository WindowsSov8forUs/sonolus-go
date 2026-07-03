package optimize

import "github.com/WindowsSov8forUs/sonolus-go/internal/compiler/ir"

// DeadCodeElimination removes stores to temp blocks (and SSA places) whose value
// is never used, and drops self-copies, while preserving side effects. Port of
// sonolus.py dead_code.DeadCodeElimination (SSA/phi/array handling omitted; our
// definable places are size-1 TempBlocks).
type DeadCodeElimination struct{}

func (DeadCodeElimination) Name() string { return "DeadCodeElimination" }

func (DeadCodeElimination) Run(gen *ir.IDGen, entry *ir.BasicBlock) *ir.BasicBlock {
	blocks := ir.Preorder(entry)

	uses := map[*ir.TempBlock]bool{}
	defs := map[*ir.TempBlock][]ir.Set{}

	// Collect uses and definitions.
	for _, b := range blocks {
		for _, s := range b.Statements {
			handleStatement(s, uses, defs)
		}
		updateUses(b.Test, uses)
	}

	// Mark-and-sweep: a definition's value-uses become live once its target is.
	queue := make([]*ir.TempBlock, 0, len(uses))
	for u := range uses {
		queue = append(queue, u)
	}
	for len(queue) > 0 {
		val := queue[len(queue)-1]
		queue = queue[:len(queue)-1]
		for _, st := range defs[val] {
			stUses := map[*ir.TempBlock]bool{}
			updateUses(st, stUses)
			for u := range stUses {
				if !uses[u] {
					uses[u] = true
					queue = append(queue, u)
				}
			}
		}
	}

	// Sweep: drop dead stores, keeping side-effecting values.
	for _, b := range blocks {
		live := make([]ir.Node, 0, len(b.Statements))
		for _, s := range b.Statements {
			set, ok := s.(ir.Set)
			if !ok {
				live = append(live, s)
				continue
			}
			if isLiveStore(set, uses) {
				live = append(live, s)
			} else if instr, ok := set.Value.(ir.Instr); ok && ir.SideEffects(instr.Op) {
				live = append(live, instr)
			}
		}
		b.Statements = live
	}
	return entry
}

// blockTemp extracts the size-1 *TempBlock backing a BlockPlace, if any.
// Shared by tempOf (DCE) and stmtDef (SSA construction) to avoid duplicating
// the type-assert-and-size-check pattern.
func blockTemp(bp ir.BlockPlace) (*ir.TempBlock, bool) {
	tb, ok := bp.Block.(*ir.TempBlock)
	if !ok || tb.Size != 1 {
		return nil, false
	}
	return tb, true
}

// tempOf returns the size-1 temp block a place writes to, if any.
func tempOf(p ir.Place) (*ir.TempBlock, bool) {
	bp, ok := p.(ir.BlockPlace)
	if !ok {
		return nil, false
	}
	return blockTemp(bp)
}

func isLiveStore(set ir.Set, uses map[*ir.TempBlock]bool) bool {
	// A store to an unused temp is dead.
	if tb, ok := tempOf(set.Place); ok && !uses[tb] {
		return false
	}
	// A self-copy (x <- x) is dead.
	if g, ok := set.Value.(ir.Get); ok && placeEqual(set.Place, g.Place) {
		return false
	}
	return true
}

func handleStatement(s ir.Node, uses map[*ir.TempBlock]bool, defs map[*ir.TempBlock][]ir.Set) {
	set, ok := s.(ir.Set)
	if !ok {
		updateUses(s, uses)
		return
	}
	if tb, ok := tempOf(set.Place); ok {
		defs[tb] = append(defs[tb], set)
		// A side-effecting value is always evaluated, so its uses are live.
		if instr, ok := set.Value.(ir.Instr); ok && ir.SideEffects(instr.Op) {
			updateUses(set.Value, uses)
		}
		return
	}
	// A store to concrete memory is always live: record uses of place and value.
	updateUses(set, uses)
}

// updateUses adds the temp blocks read by n to uses. Uses ir.Walk for
// read-only traversal of the node tree.
func updateUses(n ir.Node, uses map[*ir.TempBlock]bool) {
	ir.Walk(n, func(node ir.Node) {
		if tb, ok := node.(*ir.TempBlock); ok {
			uses[tb] = true
		}
	})
}

func (DeadCodeElimination) Requires() []Analysis  { return nil }
func (DeadCodeElimination) Preserves() []Analysis { return nil }
func (DeadCodeElimination) Destroys() []Analysis  { return nil }
