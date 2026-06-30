package ir

import "fmt"

// DefaultTempMemoryBlock is the memory block temps are allocated into by
// default (sonolus.py play-mode TemporaryMemory).
// This is an alias for BlockTempMemory defined in blocks.go.
const DefaultTempMemoryBlock = BlockTempMemory

// allocateTempBlocks resolves every TempBlock-backed place to a concrete cell in
// the given memory block, assigning each distinct temp its own slot (no reuse).
// Temps are assigned slots in deterministic first-seen order over the CFG.
//
// This is test-only scaffolding. The production pipeline uses
// optimize.AllocateLive (linear-scan, liveness-aware) in [optimize.Standard] / advanced.go.
// Use [AllocateTestBlocks] for the exported test-only wrapper.
func allocateTempBlocks(entry *BasicBlock, blockID int) *BasicBlock {
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
			rw, err := a.rewrite(s)
			if err != nil {
				return nil
			}
			b.Statements[i] = rw
		}
		rw, err := a.rewrite(b.Test)
		if err != nil {
			return nil
		}
		b.Test = rw
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

func (a *allocator) rewrite(n Node) (Node, error) {
	switch t := n.(type) {
	case nil:
		return nil, nil
	case Const, SSAPlace:
		return t, nil
	case *TempBlock:
		// A bare temp block as a node resolves to its base cell id.
		return Const(a.blockID), nil
	case Instr:
		args := make([]Node, len(t.Args))
		for i, arg := range t.Args {
			rw, err := a.rewrite(arg)
			if err != nil {
				return nil, err
			}
			args[i] = rw
		}
		return Instr{ID: t.ID, Op: t.Op, Args: args, Pure: t.Pure}, nil
	case Get:
		rp, err := a.rewritePlace(t.Place)
		if err != nil {
			return nil, err
		}
		return Get{Place: rp}, nil
	case Set:
		rp, err := a.rewritePlace(t.Place)
		if err != nil {
			return nil, err
		}
		rv, err := a.rewrite(t.Value)
		if err != nil {
			return nil, err
		}
		return Set{ID: t.ID, Place: rp, Value: rv}, nil
	case BlockPlace:
		rp, err := a.rewritePlace(t)
		if err != nil {
			return nil, err
		}
		return rp, nil
	default:
		return nil, fmt.Errorf("ir: allocator cannot rewrite %T", n)
	}
}

func (a *allocator) rewritePlace(p Place) (Place, error) {
	bp, ok := p.(BlockPlace)
	if !ok {
		return p, nil
	}
	if tb, ok := bp.Block.(*TempBlock); ok {
		base := a.slotOf(tb) + bp.Offset
		// A constant index folds into the slot; a dynamic index (array access)
		// becomes an offset added at runtime.
		if bp.Index == nil {
			return BlockPlace{Block: Const(a.blockID), Index: Const(base), Offset: 0}, nil
		}
		if c, ok := bp.Index.(Const); ok {
			return BlockPlace{Block: Const(a.blockID), Index: Const(base + int(c)), Offset: 0}, nil
		}
		rw, err := a.rewrite(bp.Index)
		if err != nil {
			return nil, err
		}
		return BlockPlace{Block: Const(a.blockID), Index: rw, Offset: base}, nil
	}
	rwBlock, err := a.rewrite(bp.Block)
	if err != nil {
		return nil, err
	}
	rwIndex, err := a.rewrite(bp.Index)
	if err != nil {
		return nil, err
	}
	return BlockPlace{Block: rwBlock, Index: rwIndex, Offset: bp.Offset}, nil
}

// AllocateTestBlocks is the exported test-only wrapper around allocateTempBlocks.
// It assigns each distinct TempBlock a slot in the given memory block (no reuse,
// deterministic first-seen order).
//
// Production code must use optimize.AllocateLive instead. This function is
// exported solely for unit tests that need a pre-SSA, pre-optimize allocation,
// because those tests call the frontend tracer directly and the result contains
// unresolved TempBlocks.
func AllocateTestBlocks(entry *BasicBlock, blockID int) *BasicBlock {
	return allocateTempBlocks(entry, blockID)
}
