package ir

import "fmt"

// DefaultTempMemoryBlock is the memory block temps are allocated into by
// default (sonolus.py play-mode TemporaryMemory).
const DefaultTempMemoryBlock = 10000

// AllocateTempBlocks resolves every TempBlock-backed place to a concrete cell in
// the given memory block, assigning each distinct temp its own slot (no reuse).
// This is the trivial allocator required before finalization; a liveness-based
// allocator with slot reuse is a later optimization.
//
// Temps are assigned slots in deterministic first-seen order over the CFG.
func AllocateTempBlocks(entry *BasicBlock, blockID int) *BasicBlock {
	a := &allocator{blockID: blockID, slot: map[*TempBlock]int{}}

	// Pass 1: assign slots in a deterministic traversal order.
	for _, b := range traverseReversePostorder(entry) {
		for _, s := range b.Statements {
			a.collect(s)
		}
		a.collect(b.Test)
	}

	// Pass 2: rewrite all nodes to use concrete cells.
	for _, b := range traverseReversePostorder(entry) {
		for i, s := range b.Statements {
			b.Statements[i] = a.rewrite(s)
		}
		b.Test = a.rewrite(b.Test)
	}
	return entry
}

type allocator struct {
	blockID int
	slot    map[*TempBlock]int
	next    int
}

func (a *allocator) slotOf(t *TempBlock) int {
	if s, ok := a.slot[t]; ok {
		return s
	}
	s := a.next
	a.next += t.Size
	a.slot[t] = s
	return s
}

// collect assigns slots to temps in first-seen order.
func (a *allocator) collect(n Node) {
	switch t := n.(type) {
	case Const, SSAPlace, nil:
	case *TempBlock:
		a.slotOf(t)
	case Instr:
		for _, arg := range t.Args {
			a.collect(arg)
		}
	case Get:
		a.collectPlace(t.Place)
	case Set:
		a.collectPlace(t.Place)
		a.collect(t.Value)
	case BlockPlace:
		a.collectPlace(t)
	}
}

func (a *allocator) collectPlace(p Place) {
	bp, ok := p.(BlockPlace)
	if !ok {
		return
	}
	a.collect(bp.Block)
	a.collect(bp.Index)
}

func (a *allocator) rewrite(n Node) Node {
	switch t := n.(type) {
	case nil:
		return nil
	case Const, SSAPlace:
		return t
	case *TempBlock:
		// A bare temp block as a node resolves to its base cell id.
		return Const(a.blockID)
	case Instr:
		args := make([]Node, len(t.Args))
		for i, arg := range t.Args {
			args[i] = a.rewrite(arg)
		}
		return Instr{Op: t.Op, Args: args, Pure: t.Pure}
	case Get:
		return Get{Place: a.rewritePlace(t.Place)}
	case Set:
		return Set{Place: a.rewritePlace(t.Place), Value: a.rewrite(t.Value)}
	case BlockPlace:
		return a.rewritePlace(t)
	default:
		panic(fmt.Sprintf("ir: allocator cannot rewrite %T", n))
	}
}

func (a *allocator) rewritePlace(p Place) Place {
	bp, ok := p.(BlockPlace)
	if !ok {
		return p
	}
	if tb, ok := bp.Block.(*TempBlock); ok {
		base := a.slotOf(tb) + bp.Offset
		// A constant index folds into the slot; a dynamic index (array access)
		// becomes an offset added at runtime.
		if bp.Index == nil {
			return BlockPlace{Block: Const(a.blockID), Index: Const(base), Offset: 0}
		}
		if c, ok := bp.Index.(Const); ok {
			return BlockPlace{Block: Const(a.blockID), Index: Const(base + int(c)), Offset: 0}
		}
		return BlockPlace{Block: Const(a.blockID), Index: a.rewrite(bp.Index), Offset: base}
	}
	return BlockPlace{Block: a.rewrite(bp.Block), Index: a.rewrite(bp.Index), Offset: bp.Offset}
}
