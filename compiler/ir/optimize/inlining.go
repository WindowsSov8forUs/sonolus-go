package optimize

import "github.com/WindowsSov8forUs/sonolus-go/compiler/ir"

// BlockOracle answers questions about concrete memory blocks needed to decide
// whether a memory read may be inlined. The full model (sonolus.py blocks.py)
// is not yet ported; the conservative default treats every block as writable
// and non-constant, so memory reads are never inlined (always safe).
type BlockOracle interface {
	Writable(blockID int, callback string) bool
	RuntimeConstant(blockID int) bool
}

type conservativeOracle struct{}

func (conservativeOracle) Writable(int, string) bool { return true }
func (conservativeOracle) RuntimeConstant(int) bool  { return false }

// InlineVars inlines SSA value definitions into their uses, collapsing read-once
// temporaries and copies. Port of sonolus.py inlining.InlineVars.
type InlineVars struct {
	Aggressive bool
	Callback   string
	Oracle     BlockOracle
}

func (InlineVars) Name() string { return "InlineVars" }

func (v InlineVars) oracle() BlockOracle {
	if v.Oracle != nil {
		return v.Oracle
	}
	return conservativeOracle{}
}

func (v InlineVars) Run(entry *ir.BasicBlock) *ir.BasicBlock {
	dom := ComputeDominance(entry)
	blocks := ir.Preorder(entry)

	loopBodies := computeLoopBodies(blocks, dom)
	defBlocks := map[ir.SSAPlace]*ir.BasicBlock{}
	for _, b := range blocks {
		for _, phi := range b.Phis {
			if s, ok := phi.Target.(ir.SSAPlace); ok {
				defBlocks[s] = b
			}
		}
		for _, stmt := range b.Statements {
			if set, ok := stmt.(ir.Set); ok {
				if s, ok := set.Place.(ir.SSAPlace); ok {
					defBlocks[s] = b
				}
			}
		}
	}
	crossesLoop := func(p ir.SSAPlace, useBlock *ir.BasicBlock) bool {
		if v.Aggressive || useBlock == nil {
			return false
		}
		db := defBlocks[p]
		for _, body := range loopBodies {
			if body[useBlock] && !body[db] {
				return true
			}
		}
		return false
	}

	useCounts := map[ir.SSAPlace]int{}
	definitions := map[ir.SSAPlace]ir.Node{}
	var defOrder []ir.SSAPlace
	addDef := func(p ir.SSAPlace, val ir.Node) {
		if _, ok := definitions[p]; !ok {
			defOrder = append(defOrder, p)
		}
		definitions[p] = val
	}

	for _, b := range blocks {
		for _, stmt := range b.Statements {
			countUses(stmt, useCounts)
			if set, ok := stmt.(ir.Set); ok {
				if s, ok := set.Place.(ir.SSAPlace); ok {
					addDef(s, set.Value)
				}
			}
		}
		countUses(b.Test, useCounts)
		for _, phi := range b.Phis {
			for _, arg := range phi.Args {
				countUses(arg, useCounts)
			}
			if len(phi.Args) == 1 {
				var arg ir.Place
				for _, a := range phi.Args {
					arg = a
				}
				addDef(phi.Target.(ir.SSAPlace), ir.Get{Place: arg})
			}
		}
	}

	// A copy definition (p = Get(q)) does not really "use" q for counting.
	for _, defn := range definitions {
		if q, ok := ssaGet(defn); ok {
			useCounts[q]--
		}
	}

	// Canonicalize copy chains, then fold an inlinable inner definition in.
	canonical := map[ir.SSAPlace]ir.Node{}
	for _, p := range defOrder {
		defn := definitions[p]
		canonical[p] = defn
		for d := defn; d != nil; {
			q, ok := ssaGet(d)
			if !ok {
				break
			}
			canonical[p] = d
			d = definitions[q]
		}
		if useCounts[p] > 0 {
			if q, ok := ssaGet(canonical[p]); ok {
				useCounts[q]++
			}
		}
	}
	for _, p := range defOrder {
		q, ok := ssaGet(canonical[p])
		if !ok {
			continue
		}
		inner, has := canonical[q]
		if has && inner != nil && v.isInlinable(inner) &&
			(v.isFreeToInline(inner) || v.Aggressive ||
				(useCounts[q] <= 1 && !crossesLoop(q, defBlocks[p]))) {
			canonical[p] = inner
		}
	}

	// Fully inline each definition's inlinable uses.
	inlined := map[ir.SSAPlace]ir.Node{}
	for _, p := range defOrder {
		defn := canonical[p]
		for {
			subs := map[ir.SSAPlace]ir.Node{}
			for inP := range getInlinableUses(defn) {
				inDefn, has := canonical[inP]
				if !has || !v.isInlinable(inDefn) {
					continue
				}
				if _, isCopy := ssaGet(inDefn); isCopy || v.isFreeToInline(inDefn) || v.Aggressive ||
					(useCounts[inP] == 1 && !crossesLoop(inP, defBlocks[p])) {
					subs[inP] = inDefn
				}
			}
			if len(subs) == 0 {
				break
			}
			defn = substitute(defn, subs)
		}
		inlined[p] = defn
	}

	valid := map[ir.SSAPlace]bool{}
	for _, p := range defOrder {
		if v.isInlinable(inlined[p]) && (useCounts[p] <= 1 || v.isFreeToInline(inlined[p]) || v.Aggressive) {
			valid[p] = true
		}
	}

	for _, b := range blocks {
		all := append(append([]ir.Node{}, b.Statements...), b.Test)
		newStmts := make([]ir.Node, 0, len(all))
		for _, stmt := range all {
			if set, ok := stmt.(ir.Set); ok {
				if _, ok := set.Place.(ir.SSAPlace); ok {
					if q, ok := ssaGet(set.Value); ok {
						if repl, has := inlined[q]; has && repl != nil && v.isFreeToInline(repl) {
							newStmts = append(newStmts, ir.Set{Place: set.Place, Value: repl})
						} else {
							newStmts = append(newStmts, stmt)
						}
						continue
					}
				}
			}
			for {
				subs := map[ir.SSAPlace]ir.Node{}
				for p := range getInlinableUses(stmt) {
					if valid[p] && (v.Aggressive || v.isFreeToInline(inlined[p]) || !crossesLoop(p, b)) {
						subs[p] = inlined[p]
					}
				}
				if len(subs) > 0 {
					stmt = substitute(stmt, subs)
				} else {
					newStmts = append(newStmts, stmt)
					break
				}
			}
		}
		b.Statements = newStmts[:len(newStmts)-1]
		b.Test = newStmts[len(newStmts)-1]
	}

	return entry
}

// ssaGet returns the SSA place q if n is Get(q), i.e. a copy.
func ssaGet(n ir.Node) (ir.SSAPlace, bool) {
	g, ok := n.(ir.Get)
	if !ok {
		return ir.SSAPlace{}, false
	}
	s, ok := g.Place.(ir.SSAPlace)
	return s, ok
}

func constInt(n ir.Node) (int, bool) {
	c, ok := n.(ir.Const)
	return int(c), ok
}

func isConstNode(n ir.Node) bool {
	_, ok := n.(ir.Const)
	return ok
}

// --- predicates (need callback + block oracle) ---

func (v InlineVars) isInlinable(n ir.Node) bool {
	switch t := n.(type) {
	case ir.Const:
		return true
	case ir.Instr:
		if ir.SideEffects(t.Op) || !ir.Pure(t.Op) {
			return false
		}
		for _, a := range t.Args {
			if !v.isInlinable(a) {
				return false
			}
		}
		return true
	case ir.Get:
		if _, ok := t.Place.(ir.SSAPlace); ok {
			return true
		}
		bp, ok := t.Place.(ir.BlockPlace)
		if !ok {
			return false
		}
		id, ok := constInt(bp.Block)
		if !ok {
			return false
		}
		return !v.oracle().Writable(id, v.Callback) && v.isInlinableIndex(bp.Index)
	default:
		return false
	}
}

func (v InlineVars) isInlinableIndex(idx ir.Node) bool {
	switch idx.(type) {
	case ir.SSAPlace, ir.Const:
		return true
	case ir.Instr, ir.Get:
		return v.isInlinable(idx)
	default:
		return false
	}
}

func (v InlineVars) isFreeToInline(n ir.Node) bool {
	switch t := n.(type) {
	case ir.Const:
		return true
	case ir.Instr:
		return v.isRuntimeConstant(n)
	case ir.Get:
		if _, ok := t.Place.(ir.SSAPlace); ok {
			return true
		}
		bp, ok := t.Place.(ir.BlockPlace)
		return ok && isConstNode(bp.Block) && isConstNode(bp.Index)
	default:
		return false
	}
}

func (v InlineVars) isRuntimeConstant(n ir.Node) bool {
	switch t := n.(type) {
	case ir.Const:
		return true
	case ir.Instr:
		if ir.SideEffects(t.Op) || !ir.Pure(t.Op) {
			return false
		}
		for _, a := range t.Args {
			if !v.isRuntimeConstant(a) {
				return false
			}
		}
		return true
	case ir.Get:
		bp, ok := t.Place.(ir.BlockPlace)
		if !ok {
			return false
		}
		id, ok := constInt(bp.Block)
		if !ok {
			return false
		}
		_, idxConst := bp.Index.(ir.Const)
		return !v.oracle().Writable(id, v.Callback) && v.oracle().RuntimeConstant(id) && idxConst
	default:
		return false
	}
}

// --- node helpers ---

func countUses(n ir.Node, counts map[ir.SSAPlace]int) {
	switch t := n.(type) {
	case ir.Instr:
		for _, a := range t.Args {
			countUses(a, counts)
		}
	case ir.Get:
		countUses(t.Place, counts)
	case ir.Set:
		if _, ok := t.Place.(ir.SSAPlace); !ok {
			countUses(t.Place, counts)
		}
		countUses(t.Value, counts)
	case ir.SSAPlace:
		counts[t]++
	case ir.BlockPlace:
		countUses(t.Block, counts)
		countUses(t.Index, counts)
	}
}

func getInlinableUses(n ir.Node) map[ir.SSAPlace]bool {
	uses := map[ir.SSAPlace]bool{}
	collectInlinableUses(n, uses)
	return uses
}

func collectInlinableUses(n ir.Node, uses map[ir.SSAPlace]bool) {
	switch t := n.(type) {
	case ir.Instr:
		for _, a := range t.Args {
			collectInlinableUses(a, uses)
		}
	case ir.Get:
		if s, ok := t.Place.(ir.SSAPlace); ok {
			uses[s] = true
		} else if bp, ok := t.Place.(ir.BlockPlace); ok {
			collectInlinableUses(bp.Block, uses)
			collectInlinableUses(bp.Index, uses)
		}
	case ir.Set:
		if _, ok := t.Place.(ir.SSAPlace); !ok {
			collectInlinableUses(t.Place, uses)
		}
		collectInlinableUses(t.Value, uses)
	case ir.SSAPlace:
		uses[t] = true
	case ir.BlockPlace:
		collectInlinableUses(t.Block, uses)
		collectInlinableUses(t.Index, uses)
	}
}

func substitute(n ir.Node, subs map[ir.SSAPlace]ir.Node) ir.Node {
	switch t := n.(type) {
	case ir.Instr:
		args := make([]ir.Node, len(t.Args))
		for i, a := range t.Args {
			args[i] = substitute(a, subs)
		}
		return ir.Instr{Op: t.Op, Args: args, Pure: t.Pure}
	case ir.Get:
		if s, ok := t.Place.(ir.SSAPlace); ok {
			if r, has := subs[s]; has {
				return r
			}
		}
		return ir.Get{Place: substitutePlace(t.Place, subs)}
	case ir.Set:
		return ir.Set{Place: substitutePlace(t.Place, subs), Value: substitute(t.Value, subs)}
	case ir.SSAPlace:
		if r, has := subs[t]; has {
			return r
		}
		return t
	case ir.BlockPlace:
		return substitutePlace(t, subs)
	default:
		return n
	}
}

func substitutePlace(p ir.Place, subs map[ir.SSAPlace]ir.Node) ir.Place {
	switch t := p.(type) {
	case ir.SSAPlace:
		if r, has := subs[t]; has {
			if rp, ok := r.(ir.Place); ok {
				return rp
			}
		}
		return t
	case ir.BlockPlace:
		return ir.BlockPlace{Block: substitute(t.Block, subs), Index: substitute(t.Index, subs), Offset: t.Offset}
	default:
		return p
	}
}

// --- loops ---

func computeLoopBodies(blocks []*ir.BasicBlock, dom *Dominance) []map[*ir.BasicBlock]bool {
	loopsByHeader := map[*ir.BasicBlock][]*ir.BasicBlock{}
	for _, b := range blocks {
		for _, e := range b.Outgoing {
			if dominates(dom, e.Dst, e.Src) {
				loopsByHeader[e.Dst] = append(loopsByHeader[e.Dst], e.Src)
			}
		}
	}
	var bodies []map[*ir.BasicBlock]bool
	for header, latches := range loopsByHeader {
		body := map[*ir.BasicBlock]bool{}
		for _, latch := range latches {
			for b := range computeLoopBody(header, latch) {
				body[b] = true
			}
		}
		bodies = append(bodies, body)
	}
	return bodies
}

func dominates(dom *Dominance, a, b *ir.BasicBlock) bool {
	for x := b; x != nil; {
		if x == a {
			return true
		}
		idom := dom.IDom[x]
		if idom == x {
			break
		}
		x = idom
	}
	return false
}

func computeLoopBody(header, latch *ir.BasicBlock) map[*ir.BasicBlock]bool {
	body := map[*ir.BasicBlock]bool{header: true}
	if latch == header {
		return body
	}
	body[latch] = true
	worklist := []*ir.BasicBlock{latch}
	for len(worklist) > 0 {
		b := worklist[len(worklist)-1]
		worklist = worklist[:len(worklist)-1]
		for _, e := range b.Incoming {
			if !body[e.Src] {
				body[e.Src] = true
				worklist = append(worklist, e.Src)
			}
		}
	}
	return body
}
