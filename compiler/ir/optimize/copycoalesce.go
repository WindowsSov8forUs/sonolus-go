package optimize

import "github.com/WindowsSov8forUs/sonolus-go/compiler/ir"

// CopyCoalesce merges temp-block copies introduced by FromSSA's phi resolution.
// It scans the CFG for Set(t1, Get(t2)) where t1 and t2 are both size-1 temps,
// then replaces all uses of t1 with t2 and drops the copy. This is a simplified
// port of sonolus.py's copy_coalesce.CopyCoalesce: the full version requires
// LivenessAnalysis for interference checking, but post-FromSSA copies are on
// edge blocks where source and target cannot be simultaneously live, making the
// simplification sound for the SSA pipeline.
type CopyCoalesce struct{}

func (CopyCoalesce) Name() string { return "CopyCoalesce" }

func (CopyCoalesce) Run(entry *ir.BasicBlock) *ir.BasicBlock {
	blocks := ir.Preorder(entry)

	// Collect all copies: Set(t1_block, Get(t2_block)).
	type copyPair struct {
		dst, src *ir.TempBlock
		block    *ir.BasicBlock
		stmtIdx  int
	}
	var copies []copyPair
	for _, b := range blocks {
		for i, s := range b.Statements {
			set, ok := s.(ir.Set)
			if !ok {
				continue
			}
			dst, ok := tempOf(set.Place)
			if !ok || dst.Size != 1 {
				continue
			}
			get, ok := set.Value.(ir.Get)
			if !ok {
				continue
			}
			src, ok := tempOf(get.Place)
			if !ok || src.Size != 1 {
				continue
			}
			if dst == src {
				continue
			}
			copies = append(copies, copyPair{dst, src, b, i})
		}
	}

	if len(copies) == 0 {
		return entry
	}

	// Build coalescing map: temp → canonical temp (choose the smaller one
	// alphabetically for determinism).
	coalesce := map[*ir.TempBlock]*ir.TempBlock{}
	var find func(t *ir.TempBlock) *ir.TempBlock
	find = func(t *ir.TempBlock) *ir.TempBlock {
		if c, ok := coalesce[t]; ok && c != t {
			coalesce[t] = find(c)
			return coalesce[t]
		}
		return t
	}
	union := func(a, b *ir.TempBlock) {
		ra, rb := find(a), find(b)
		if ra == rb {
			return
		}
		if ra.Name <= rb.Name {
			coalesce[rb] = ra
		} else {
			coalesce[ra] = rb
		}
	}
	for _, cp := range copies {
		union(cp.dst, cp.src)
	}

	canon := func(t *ir.TempBlock) *ir.TempBlock { return find(t) }

	// Rewrite: replace each temp with its canonical representative, drop copies
	// where dst and src coalesce to the same temp.
	for _, b := range blocks {
		filtered := make([]ir.Node, 0, len(b.Statements))
		for i, s := range b.Statements {
			// If this is a coalesced copy, drop it.
			isCopy := false
			for _, cp := range copies {
				if cp.block == b && cp.stmtIdx == i {
					if canon(cp.dst) == canon(cp.src) {
						isCopy = true
					}
					break
				}
			}
			if isCopy {
				continue
			}
			filtered = append(filtered, rewriteNode(s, canon))
		}
		b.Statements = filtered
		b.Test = rewriteNode(b.Test, canon)
	}
	return entry
}

func rewriteNode(n ir.Node, canon func(*ir.TempBlock) *ir.TempBlock) ir.Node {
	switch t := n.(type) {
	case ir.Const, ir.SSAPlace, nil:
		return n
	case *ir.TempBlock:
		return canon(t)
	case ir.Instr:
		args := make([]ir.Node, len(t.Args))
		for i, a := range t.Args {
			args[i] = rewriteNode(a, canon)
		}
		return ir.Instr{Op: t.Op, Args: args, Pure: t.Pure}
	case ir.Get:
		return ir.Get{Place: rewritePlace(t.Place, canon)}
	case ir.Set:
		return ir.Set{Place: rewritePlace(t.Place, canon), Value: rewriteNode(t.Value, canon)}
	case ir.BlockPlace:
		return rewritePlace(t, canon)
	default:
		return n
	}
}

func rewritePlace(p ir.Place, canon func(*ir.TempBlock) *ir.TempBlock) ir.Place {
	bp, ok := p.(ir.BlockPlace)
	if !ok {
		return p
	}
	return ir.BlockPlace{Block: rewriteNode(bp.Block, canon), Index: rewriteNode(bp.Index, canon), Offset: bp.Offset}
}
