package optimize

import "github.com/WindowsSov8forUs/sonolus-go/compiler/ir"

// DeadCodeElimination removes stores to temp blocks (and SSA places) whose value
// is never used, and drops self-copies, while preserving side effects. Port of
// sonolus.py dead_code.DeadCodeElimination (SSA/phi/array handling omitted; our
// definable places are size-1 TempBlocks).
type DeadCodeElimination struct{}

func (DeadCodeElimination) Name() string { return "DeadCodeElimination" }

func (DeadCodeElimination) Run(entry *ir.BasicBlock) *ir.BasicBlock {
	blocks := orderedBlocks(entry)

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

// tempOf returns the size-1 temp block a place writes to, if any.
func tempOf(p ir.Place) (*ir.TempBlock, bool) {
	bp, ok := p.(ir.BlockPlace)
	if !ok {
		return nil, false
	}
	tb, ok := bp.Block.(*ir.TempBlock)
	if !ok || tb.Size != 1 {
		return nil, false
	}
	return tb, true
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

// updateUses adds the temp blocks read by n to uses.
func updateUses(n ir.Node, uses map[*ir.TempBlock]bool) {
	switch t := n.(type) {
	case nil, ir.Const, ir.SSAPlace:
	case *ir.TempBlock:
		uses[t] = true
	case ir.Instr:
		for _, a := range t.Args {
			updateUses(a, uses)
		}
	case ir.Get:
		updateUses(t.Place, uses)
	case ir.Set:
		if bp, ok := t.Place.(ir.BlockPlace); ok {
			if _, isTemp := bp.Block.(*ir.TempBlock); !isTemp {
				updateUses(bp.Block, uses)
			}
			updateUses(bp.Index, uses)
		}
		updateUses(t.Value, uses)
	case ir.BlockPlace:
		updateUses(t.Block, uses)
		updateUses(t.Index, uses)
	}
}

func placeEqual(a, b ir.Place) bool {
	pa, oka := a.(ir.BlockPlace)
	pb, okb := b.(ir.BlockPlace)
	if !oka || !okb {
		return a == b
	}
	return nodeEqual(pa.Block, pb.Block) && nodeEqual(pa.Index, pb.Index) && pa.Offset == pb.Offset
}

func nodeEqual(a, b ir.Node) bool {
	switch ta := a.(type) {
	case ir.Const:
		tb, ok := b.(ir.Const)
		return ok && ta == tb
	case *ir.TempBlock:
		tb, ok := b.(*ir.TempBlock)
		return ok && ta == tb
	case ir.Get:
		tb, ok := b.(ir.Get)
		return ok && placeEqual(ta.Place, tb.Place)
	case ir.BlockPlace:
		tb, ok := b.(ir.BlockPlace)
		return ok && placeEqual(ta, tb)
	case nil:
		return b == nil
	default:
		return false
	}
}

// orderedBlocks returns reachable blocks. Order does not affect DCE (it gathers
// uses/defs over the whole CFG).
func orderedBlocks(entry *ir.BasicBlock) []*ir.BasicBlock {
	out := make([]*ir.BasicBlock, 0)
	visited := map[*ir.BasicBlock]bool{}
	stack := []*ir.BasicBlock{entry}
	for len(stack) > 0 {
		b := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if visited[b] {
			continue
		}
		visited[b] = true
		out = append(out, b)
		for _, e := range b.Outgoing {
			stack = append(stack, e.Dst)
		}
	}
	return out
}
