package optimize

import "github.com/WindowsSov8forUs/sonolus-go/compiler/ir"

// LivenessResult holds per-block live-in/live-out and per-statement live-after
// sets. Statement keys are ir.Instr.ID values (monotonic, comparable).
type LivenessResult struct {
	LiveIn  map[*ir.BasicBlock]map[*ir.TempBlock]bool
	LiveOut map[*ir.BasicBlock]map[*ir.TempBlock]bool
	Live    map[int]map[*ir.TempBlock]bool // live-after for each statement (by Instr.ID)
	Defs    map[int]map[*ir.TempBlock]bool // temps defined by each statement
}

// LivenessAnalysis computes live-in, live-out, and per-statement live-after
// sets via backward dataflow. Only size-1 TempBlocks are tracked.
type LivenessAnalysis struct{}

func (LivenessAnalysis) Name() string { return "LivenessAnalysis" }

func (LivenessAnalysis) Run(entry *ir.BasicBlock) *ir.BasicBlock {
	_ = analyzeLiveness(entry)
	return entry
}

func analyzeLiveness(entry *ir.BasicBlock) *LivenessResult {
	blocks := ir.Preorder(entry)
	res := &LivenessResult{
		LiveIn:  map[*ir.BasicBlock]map[*ir.TempBlock]bool{},
		LiveOut: map[*ir.BasicBlock]map[*ir.TempBlock]bool{},
		Live:    map[int]map[*ir.TempBlock]bool{},
		Defs:    map[int]map[*ir.TempBlock]bool{},
	}

	for _, b := range blocks {
		for _, s := range b.Statements {
			id := stmtID(s)
			res.Defs[id] = defsOfNode(s)
			res.Live[id] = map[*ir.TempBlock]bool{}
		}
	}

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
				for u := range usesOfNode(s) {
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

// stmtID returns the Instr.ID of a statement, or 0 for non-Instr nodes.
func stmtID(s ir.Node) int {
	if instr, ok := s.(ir.Instr); ok {
		return instr.ID
	}
	return 0
}

// AdvancedDCE uses LivenessAnalysis to remove stores to temps that are never
// read after the store point.
type AdvancedDCE struct{}

func (AdvancedDCE) Name() string { return "AdvancedDCE" }

func (AdvancedDCE) Run(entry *ir.BasicBlock) *ir.BasicBlock {
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
type AllocateLive struct {
	BlockID int
}

func (a AllocateLive) Name() string { return "AllocateLive" }

func (a AllocateLive) Run(entry *ir.BasicBlock) *ir.BasicBlock {
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
	// Sort by first block index.
	for i := 1; i < len(slots); i++ {
		for j := i; j > 0 && ranges[slots[j].tb].first < ranges[slots[j-1].tb].first; j-- {
			slots[j], slots[j-1] = slots[j-1], slots[j]
		}
	}

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
		return ir.Set{Place: rewriteTempPlace(t.Place, blk, slots), Value: rewriteTemp(t.Value, blk, slots)}
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
