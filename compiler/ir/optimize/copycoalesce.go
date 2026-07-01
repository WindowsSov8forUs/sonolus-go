package optimize

import "github.com/WindowsSov8forUs/sonolus-go/compiler/ir"

// CopyCoalesce merges temp-block copies introduced by FromSSA's phi resolution.
// It scans the CFG for Set(t1, Get(t2)) where t1 and t2 are both size-1 temps,
// then replaces all uses of t1 with t2 and drops the copy using union-find.
// For multi-predecessor blocks, copies are only coalesced when the destination
// temp has a single definition, preventing interference between simultaneously
// live temps. Port of sonolus.py copy_coalesce.CopyCoalesce.
type CopyCoalesce struct{}

func (CopyCoalesce) Name() string { return "CopyCoalesce" }

func (CopyCoalesce) Run(gen *ir.IDGen, entry *ir.BasicBlock) *ir.BasicBlock {
	blocks := ir.Preorder(entry)

	// Compute liveness once to augment the def-count guard below,
	// matching Python's liveness-based copy coalescing.
	liveness := analyzeLiveness(entry)

	// Count definitions per temp: how many Set(tb, ...) exist in the CFG.
	defCount := map[*ir.TempBlock]int{}
	for _, b := range blocks {
		for _, s := range b.Statements {
			if set, ok := s.(ir.Set); ok {
				if tb, ok := tempOf(set.Place); ok {
					defCount[tb]++
				}
			}
		}
	}

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
			// Only size-1 temps participate in SSA coalescing — larger temps
			// represent arrays/records and cannot be trivially unified (aligned
			// with ssa.go stmtDef and the sonolus.py backend's single-slot model).
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
			// Guard: for multi-predecessor blocks, only coalesce if the
			// destination temp has a single definition OR is not live
			// after the copy (no interference with other definitions).
			if len(b.Incoming) > 1 && defCount[dst] > 1 {
				liveAfter := liveness.Live[set.ID]
				if liveAfter != nil && liveAfter[dst] {
					continue
				}
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
	return ir.Map(n, func(n ir.Node) ir.Node {
		if tb, ok := n.(*ir.TempBlock); ok {
			return canon(tb)
		}
		return n
	})
}
