package optimize

import (
	"github.com/WindowsSov8forUs/sonolus-go/compiler/ir"
)

// AllocateBasic assigns TempBlock offsets sequentially in CFG preorder without
// liveness analysis. It mirrors sonolus.py's allocate.AllocateBasic: each
// TempBlock encountered gets a contiguous offset, and TempBlock references in
// BlockPlace.Block are rewritten to the concrete block ID.
//
// AllocateBasic is faster than AllocateLive (no dataflow analysis, no interval
// packing) but consumes more temporary memory since non-overlapping lifetimes
// are not reused. It is appropriate for MINIMAL and FAST compilation levels
// where compile speed matters more than output compactness.
type AllocateBasic struct {
	BlockID int // temporary memory block ID (defaults to DefaultTempMemoryBlock)
}

func (AllocateBasic) Name() string { return "AllocateBasic" }

func (a AllocateBasic) Run(gen *ir.IDGen, entry *ir.BasicBlock) *ir.BasicBlock {
	_ = gen
	blk := a.BlockID
	if blk == 0 {
		blk = ir.DefaultTempMemoryBlock
	}

	// Sequential allocation: assign offsets as we encounter TempBlocks in
	// CFG preorder. Non-overlapping lifetimes are not reused — each TempBlock
	// gets a fresh offset.
	offsets := map[*ir.TempBlock]int{}
	next := 0

	rewrite := func(tb *ir.TempBlock) int {
		if o, ok := offsets[tb]; ok {
			return o
		}
		o := next
		next += tb.Size
		if next < 1 {
			next = 1
		}
		offsets[tb] = o
		return o
	}

	for _, b := range ir.Preorder(entry) {
		for i, s := range b.Statements {
			b.Statements[i] = rewriteTempBlockNode(s, blk, rewrite)
		}
		if b.Test != nil {
			b.Test = rewriteTempBlockNode(b.Test, blk, rewrite)
		}
	}
	return entry
}

// rewriteTempBlockNode replaces every BlockPlace whose Block is a *TempBlock
// with a concrete BlockPlace referencing the allocated block ID and offset.
// It recurses into Instr args, Get/Set places, and nested BlockPlaces.
func rewriteTempBlockNode(n ir.Node, blk int, offsets func(*ir.TempBlock) int) ir.Node {
	switch v := n.(type) {
	case ir.Instr:
		args := make([]ir.Node, len(v.Args))
		for i, a := range v.Args {
			args[i] = rewriteTempBlockNode(a, blk, offsets)
		}
		return ir.Instr{Op: v.Op, Args: args, ID: v.ID}
	case ir.Get:
		return ir.Get{Place: rewriteTempBlockPlace(v.Place, blk, offsets)}
	case ir.Set:
		return ir.Set{
			Place: rewriteTempBlockPlace(v.Place, blk, offsets),
			Value: rewriteTempBlockNode(v.Value, blk, offsets),
		}
	case ir.BlockPlace:
		b := rewriteTempBlockNode(v.Block, blk, offsets)
		idx := rewriteTempBlockNode(v.Index, blk, offsets)
		return ir.BlockPlace{Block: b, Index: idx, Offset: v.Offset}
	}
	return n
}

func rewriteTempBlockPlace(p ir.Place, blk int, offsets func(*ir.TempBlock) int) ir.Place {
	bp, ok := p.(ir.BlockPlace)
	if !ok {
		return p
	}
	// Check if the Block field is a TempBlock — if so, rewrite it.
	if tb, ok := bp.Block.(*ir.TempBlock); ok {
		return ir.BlockPlace{Block: ir.Const(blk), Index: ir.Const(offsets(tb)), Offset: 0}
	}
	// Otherwise recurse into nested BlockPlaces.
	return ir.BlockPlace{
		Block:  rewriteTempBlockNode(bp.Block, blk, offsets),
		Index:  rewriteTempBlockNode(bp.Index, blk, offsets),
		Offset: bp.Offset,
	}
}
