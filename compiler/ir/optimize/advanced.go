package optimize

import (
	"sort"

	"github.com/WindowsSov8forUs/sonolus-go/compiler/ir"
)

// LivenessResult holds per-block live-in/live-out and per-statement live-after
// sets. Statement keys are ir.Instr.ID values (monotonic, comparable).
type LivenessResult struct {
	LiveIn  map[*ir.BasicBlock]map[*ir.TempBlock]bool
	LiveOut map[*ir.BasicBlock]map[*ir.TempBlock]bool
	Live    map[int]map[*ir.TempBlock]bool // live-after for each statement (by Instr.ID)
	Defs    map[int]map[*ir.TempBlock]bool // temps defined by each statement
	Uses    map[int]map[*ir.TempBlock]bool // temps used by each statement (precomputed)
}

func analyzeLiveness(entry *ir.BasicBlock) *LivenessResult {
	blocks := ir.Preorder(entry)
	res := &LivenessResult{
		LiveIn:  map[*ir.BasicBlock]map[*ir.TempBlock]bool{},
		LiveOut: map[*ir.BasicBlock]map[*ir.TempBlock]bool{},
		Live:    map[int]map[*ir.TempBlock]bool{},
		Defs:    map[int]map[*ir.TempBlock]bool{},
		Uses:    map[int]map[*ir.TempBlock]bool{},
	}

	// Track which statements are the first write to a multi-slot temp (array init).
	arrayInit := map[int]map[*ir.TempBlock]bool{}
	for _, b := range blocks {
		for _, s := range b.Statements {
			id := stmtID(s)
			res.Defs[id] = defsOfNode(s)
			res.Uses[id] = usesOfNode(s)
			res.Live[id] = map[*ir.TempBlock]bool{}
		}
	}

	// Preprocess arrays: forward dataflow to find first-def sites per temp.
	// Port of sonolus.py liveness.preprocess_arrays.
	preprocessArrays(entry, blocks, arrayInit)

	exits := findExits(entry)
	if len(exits) == 0 {
		return res
	}

	for changed := true; changed; {
		changed = false
		for i := len(blocks) - 1; i >= 0; i-- {
			b := blocks[i]
			lo := map[*ir.TempBlock]bool{}
			for _, e := range b.Outgoing {
				if li, ok := res.LiveIn[e.Dst]; ok {
					for t := range li {
						lo[t] = true
					}
				}
			}
			// Remove multi-slot temps that have been defined in successors
			// (array_defs_out tracking — Python live filtering at process_block:74-78).
			res.LiveOut[b] = lo

			cur := copyTempSet(lo)
			for j := len(b.Statements) - 1; j >= 0; j-- {
				s := b.Statements[j]
				id := stmtID(s)
				// Record per-statement live-after.
				for t := range cur {
					res.Live[id][t] = true
				}
				for d := range res.Defs[id] {
					delete(cur, d)
				}
				// When initializing a multi-slot temp, kill it from the live set
				// so that initialization writes don't extend the live range upward.
				for t := range arrayInit[id] {
					delete(cur, t)
				}
				for u := range res.Uses[id] {
					cur[u] = true
				}
			}

			old := res.LiveIn[b]
			if !tempSetEq(old, cur) {
				res.LiveIn[b] = cur
				changed = true
			}
		}
	}
	return res
}

// preprocessArrays performs a forward dataflow pass to mark which Set
// statements are the first write to a multi-slot TempBlock. This enables
// per-element liveness for arrays: only reads after the first elemental
// write keep the array alive.
func preprocessArrays(entry *ir.BasicBlock, blocks []*ir.BasicBlock, arrayInit map[int]map[*ir.TempBlock]bool) {
	type blockArrays struct {
		in, out map[*ir.TempBlock]bool
	}
	state := map[*ir.BasicBlock]*blockArrays{}
	for _, b := range blocks {
		state[b] = &blockArrays{
			in:  map[*ir.TempBlock]bool{},
			out: map[*ir.TempBlock]bool{},
		}
	}

	visited := map[*ir.BasicBlock]bool{}
	queue := []*ir.BasicBlock{entry}
	for len(queue) > 0 {
		b := queue[len(queue)-1]
		queue = queue[:len(queue)-1]
		isFirst := !visited[b]
		visited[b] = true

		// Merge all incoming array-def sets.
		arrDefs := map[*ir.TempBlock]bool{}
		for _, e := range b.Incoming {
			if in, ok := state[e.Src]; ok {
				for t := range in.out {
					arrDefs[t] = true
				}
			}
		}
		// Also start from block's own input.
		for t := range state[b].in {
			arrDefs[t] = true
		}

		for _, s := range b.Statements {
			set, ok := s.(ir.Set)
			if !ok {
				continue
			}
			tb := multiSlotTemp(set.Place)
			if tb == nil {
				continue
			}
			id := stmtID(s)
			if !arrDefs[tb] {
				// First write to this multi-slot temp along this path.
				if arrayInit[id] == nil {
					arrayInit[id] = map[*ir.TempBlock]bool{}
				}
				arrayInit[id][tb] = true
				arrDefs[tb] = true
			}

		}

		old := state[b].out
		if isFirst || !tempSetEq(old, arrDefs) {
			state[b].out = arrDefs
			for _, e := range b.Outgoing {
				// Propagate to successors.
				if dst, ok := state[e.Dst]; ok {
					for t := range arrDefs {
						dst.in[t] = true
					}
				}
				queue = append(queue, e.Dst)
			}
		}
	}
}

// multiSlotTemp returns the TempBlock if p is a BlockPlace with a multi-slot
// temp block (size > 1). Returns nil for single-slot or non-temp places.
func multiSlotTemp(p ir.Place) *ir.TempBlock {
	bp, ok := p.(ir.BlockPlace)
	if !ok {
		return nil
	}
	tb, ok := bp.Block.(*ir.TempBlock)
	if !ok || tb.Size <= 1 {
		return nil
	}
	return tb
}

// stmtID returns the monotonic ID of a statement for liveness tracking.
// Nodes without an ID (e.g. Const, Get, BlockPlace) return 0 and share
// liveness slot 0. This is safe because such nodes are never defs that
// liveness needs to distinguish — only Instr and Set carry DSets.
func stmtID(s ir.Node) int {
	if instr, ok := s.(ir.Instr); ok {
		return instr.ID
	}
	if set, ok := s.(ir.Set); ok {
		return set.ID
	}
	return 0
}

// AdvancedDCE uses LivenessAnalysis to remove stores to temps that are never
// read after the store point.
type AdvancedDCE struct{}

func (AdvancedDCE) Name() string { return "AdvancedDCE" }

func (AdvancedDCE) Run(gen *ir.IDGen, entry *ir.BasicBlock) *ir.BasicBlock {
	res := analyzeLiveness(entry)
	for _, b := range ir.Preorder(entry) {
		filtered := make([]ir.Node, 0, len(b.Statements))
		for _, s := range b.Statements {
			if set, ok := s.(ir.Set); ok {
				isDead := true
				for d := range res.Defs[stmtID(s)] {
					if res.Live[stmtID(s)][d] {
						isDead = false
						break
					}
				}
				if isDead && len(res.Defs[stmtID(s)]) > 0 {
					if instr, ok2 := set.Value.(ir.Instr); ok2 && ir.SideEffects(instr.Op) {
						filtered = append(filtered, instr)
					}
					continue
				}
			}
			filtered = append(filtered, s)
		}
		b.Statements = filtered
	}
	return entry
}

// AllocateLive assigns TempBlocks to minimal concrete slots using live-interval
// packing. Non-overlapping live ranges reuse the same slot.
// It is called directly (not via RunPasses) as the final allocation step.
type AllocateLive struct {
	BlockID int
}

// Run performs the live-interval allocation and returns entry unchanged.
func (a AllocateLive) Run(gen *ir.IDGen, entry *ir.BasicBlock) *ir.BasicBlock {
	res := analyzeLiveness(entry)
	blocks := ir.Preorder(entry)
	blk := a.BlockID
	if blk == 0 {
		blk = ir.DefaultTempMemoryBlock
	}

	// Build live intervals.
	idx := map[*ir.BasicBlock]int{}
	for i, b := range blocks {
		idx[b] = i
	}

	type interval struct{ first, last int }
	ranges := map[*ir.TempBlock]interval{}
	for _, b := range blocks {
		for t := range res.LiveIn[b] {
			r := ranges[t]
			r.first = idx[b]
			r.last = idx[b]
			ranges[t] = r
		}
		for _, s := range b.Statements {
			for t := range usesOfNode(s) {
				r := ranges[t]
				r.last = idx[b]
				ranges[t] = r
			}
		}
	}

	// Linear-scan allocation.
	type slot struct {
		tb   *ir.TempBlock
		last int
	}
	var slots []slot
	for t, r := range ranges {
		slots = append(slots, slot{t, r.last})
	}
	// Sort by size descending, then first-use ascending (sonolus.py strategy).
	sort.Slice(slots, func(i, j int) bool {
		ai, aj := slots[i].tb.Size, slots[j].tb.Size
		if ai != aj {
			return ai > aj
		}
		return ranges[slots[i].tb].first < ranges[slots[j].tb].first
	})

	slotEnd := []int{}
	slotMap := map[*ir.TempBlock]int{}
	for _, s := range slots {
		r := ranges[s.tb]
		sz := s.tb.Size
		assigned := false
		// Find a run of sz contiguous free slots.
		for si := 0; si+sz <= len(slotEnd); si++ {
			ok := true
			for k := 0; k < sz; k++ {
				if slotEnd[si+k] >= r.first {
					ok = false
					break
				}
			}
			if ok {
				for k := 0; k < sz; k++ {
					slotEnd[si+k] = r.last
				}
				slotMap[s.tb] = si
				assigned = true
				break
			}
		}
		if !assigned {
			start := len(slotEnd)
			for k := 0; k < sz; k++ {
				slotEnd = append(slotEnd, r.last)
			}
			slotMap[s.tb] = start
		}
	}

	// Rewrite temps to concrete cells.
	for _, b := range blocks {
		for i, s := range b.Statements {
			b.Statements[i] = rewriteTemp(s, blk, slotMap)
		}
		b.Test = rewriteTemp(b.Test, blk, slotMap)
	}
	return entry
}

func rewriteTemp(n ir.Node, blk int, slots map[*ir.TempBlock]int) ir.Node {
	switch t := n.(type) {
	case ir.Instr:
		args := make([]ir.Node, len(t.Args))
		for i, a := range t.Args {
			args[i] = rewriteTemp(a, blk, slots)
		}
		return ir.Instr{ID: t.ID, Op: t.Op, Args: args, Pure: t.Pure}
	case ir.Get:
		return ir.Get{Place: rewriteTempPlace(t.Place, blk, slots)}
	case ir.Set:
		return ir.Set{ID: t.ID, Place: rewriteTempPlace(t.Place, blk, slots), Value: rewriteTemp(t.Value, blk, slots)}
	case ir.BlockPlace:
		return rewriteTempPlace(t, blk, slots)
	default:
		return n
	}
}

func rewriteTempPlace(p ir.Place, blk int, slots map[*ir.TempBlock]int) ir.Place {
	bp, ok := p.(ir.BlockPlace)
	if !ok {
		return p
	}
	if tb, ok2 := bp.Block.(*ir.TempBlock); ok2 {
		s := slots[tb]
		idx := ir.Const(s)
		if c, ok3 := bp.Index.(ir.Const); ok3 {
			idx = ir.Const(s + int(c))
		}
		return ir.BlockPlace{Block: ir.Const(blk), Index: idx, Offset: 0}
	}
	return ir.BlockPlace{Block: rewriteTemp(bp.Block, blk, slots), Index: rewriteTemp(bp.Index, blk, slots), Offset: bp.Offset}
}

func defsOfNode(n ir.Node) map[*ir.TempBlock]bool {
	d := map[*ir.TempBlock]bool{}
	if set, ok := n.(ir.Set); ok {
		if bp, ok2 := set.Place.(ir.BlockPlace); ok2 {
			if tb, ok3 := bp.Block.(*ir.TempBlock); ok3 {
				d[tb] = true
			}
		}
	}
	return d
}

func usesOfNode(n ir.Node) map[*ir.TempBlock]bool {
	u := map[*ir.TempBlock]bool{}
	collectTUses(n, u)
	return u
}

func collectTUses(n ir.Node, u map[*ir.TempBlock]bool) {
	switch t := n.(type) {
	case nil, ir.Const, ir.SSAPlace:
	case *ir.TempBlock:
		u[t] = true
	case ir.Instr:
		for _, a := range t.Args {
			collectTUses(a, u)
		}
	case ir.Get:
		collectTUses(t.Place, u)
	case ir.Set:
		collectTUses(t.Place, u)
		collectTUses(t.Value, u)
	case ir.BlockPlace:
		collectTUses(t.Block, u)
		collectTUses(t.Index, u)
	}
}

func findExits(entry *ir.BasicBlock) []*ir.BasicBlock {
	var out []*ir.BasicBlock
	for _, b := range ir.Preorder(entry) {
		if len(b.Outgoing) == 0 {
			out = append(out, b)
		}
	}
	return out
}

func copyTempSet(m map[*ir.TempBlock]bool) map[*ir.TempBlock]bool {
	o := map[*ir.TempBlock]bool{}
	for k := range m {
		o[k] = true
	}
	return o
}

func tempSetEq(a, b map[*ir.TempBlock]bool) bool {
	if len(a) != len(b) {
		return false
	}
	for k := range a {
		if !b[k] {
			return false
		}
	}
	return true
}
