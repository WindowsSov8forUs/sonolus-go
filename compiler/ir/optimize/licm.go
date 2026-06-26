package optimize

import "github.com/WindowsSov8forUs/sonolus-go/compiler/ir"

type LICM struct{}

type licmLoop struct {
	header  *ir.BasicBlock
	latches []*ir.BasicBlock
	body    map[*ir.BasicBlock]bool
}

func (LICM) Name() string { return "LICM" }

func (LICM) Run(entry *ir.BasicBlock) *ir.BasicBlock {
	dom := ComputeDominance(entry)
	blocks := ir.ReversePostorder(entry)

	hl := map[*ir.BasicBlock][]*ir.BasicBlock{}
	for _, b := range blocks {
		for _, e := range b.Outgoing {
			if dominates(dom, e.Dst, e.Src) {
				hl[e.Dst] = append(hl[e.Dst], e.Src)
			}
		}
	}

	var loops []licmLoop
	for header, latches := range hl {
		body := map[*ir.BasicBlock]bool{}
		for _, latch := range latches {
			for b := range licmComputeLoopBody(header, latch) {
				body[b] = true
			}
		}
		loops = append(loops, licmLoop{header, latches, body})
	}

	nextID := 0
	for _, lp := range loops {
		if lp.header == entry {
			continue
		}
		licmProcessLoop(lp, dom, &nextID)
	}
	return entry
}

func licmProcessLoop(lp licmLoop, dom *Dominance, nextID *int) {
	var preheader *ir.BasicBlock
	var nonBack []*ir.FlowEdge
	for _, e := range lp.header.Incoming {
		if !lp.body[e.Src] {
			nonBack = append(nonBack, e)
		}
	}
	if len(nonBack) == 0 {
		return
	}
	if len(nonBack) == 1 && len(nonBack[0].Src.Statements) == 0 && len(nonBack[0].Src.Outgoing) == 1 {
		preheader = nonBack[0].Src
	} else {
		preheader = ir.NewBlock()
		preheader.ConnectTo(lp.header, nil)
		for _, e := range nonBack {
			e.Dst = preheader
			preheader.Incoming = append(preheader.Incoming, e)
			lp.header.Incoming = licmRemoveEdge(lp.header.Incoming, e)
		}
		lp.header.Incoming = append(lp.header.Incoming, &ir.FlowEdge{Src: preheader, Dst: lp.header, Cond: nil})
	}

	defs := licmDefsInLoop(lp.body)
	var guaranteed []*ir.BasicBlock
	for b := range lp.body {
		ok := true
		for _, latch := range lp.latches {
			if !dominates(dom, b, latch) {
				ok = false
				break
			}
		}
		if ok {
			guaranteed = append(guaranteed, b)
		}
	}

	hoisted := map[string]bool{}
	for _, b := range guaranteed {
		for _, s := range b.Statements {
			licmHoistExpr(s, preheader, defs, nextID, hoisted)
		}
		licmHoistExpr(b.Test, preheader, defs, nextID, hoisted)
	}
}

func licmDefsInLoop(body map[*ir.BasicBlock]bool) map[ir.SSAPlace]bool {
	defs := map[ir.SSAPlace]bool{}
	for b := range body {
		for _, phi := range b.Phis {
			if p, ok := phi.Target.(ir.SSAPlace); ok {
				defs[p] = true
			}
		}
		for _, s := range b.Statements {
			if set, ok := s.(ir.Set); ok {
				if p, ok2 := set.Place.(ir.SSAPlace); ok2 {
					defs[p] = true
				}
			}
		}
	}
	return defs
}

func licmHoistExpr(n ir.Node, preheader *ir.BasicBlock, defs map[ir.SSAPlace]bool, nextID *int, hoisted map[string]bool) {
	instr, ok := n.(ir.Instr)
	if !ok || !ir.Pure(instr.Op) || ir.SideEffects(instr.Op) {
		return
	}
	k := cseKey(instr)
	if hoisted[k] {
		return
	}
	if !licmIsInvariant(instr, defs) {
		return
	}
	if licmCost(instr) < 4 {
		return
	}
	hoisted[k] = true
	p := ir.SSAPlace{Name: "licm", Num: *nextID}
	*nextID++
	preheader.Statements = append(preheader.Statements, ir.Set{Place: p, Value: instr})
}

func licmIsInvariant(instr ir.Instr, defs map[ir.SSAPlace]bool) bool {
	for _, a := range instr.Args {
		if !licmArgInvariant(a, defs) {
			return false
		}
	}
	return true
}

func licmArgInvariant(n ir.Node, defs map[ir.SSAPlace]bool) bool {
	switch t := n.(type) {
	case ir.Const:
		return true
	case ir.SSAPlace:
		return !defs[t]
	case ir.Instr:
		for _, a := range t.Args {
			if !licmArgInvariant(a, defs) {
				return false
			}
		}
		return true
	case ir.Get:
		if p, ok := t.Place.(ir.SSAPlace); ok {
			return !defs[p]
		}
		return false
	default:
		return false
	}
}

func licmCost(n ir.Node) int {
	if instr, ok := n.(ir.Instr); ok {
		s := 1
		for _, a := range instr.Args {
			s += licmCost(a)
		}
		return s
	}
	return 1
}

func licmComputeLoopBody(header, latch *ir.BasicBlock) map[*ir.BasicBlock]bool {
	body := map[*ir.BasicBlock]bool{header: true}
	if latch == header {
		return body
	}
	body[latch] = true
	work := []*ir.BasicBlock{latch}
	for len(work) > 0 {
		b := work[0]
		work = work[1:]
		for _, e := range b.Incoming {
			if !body[e.Src] {
				body[e.Src] = true
				work = append(work, e.Src)
			}
		}
	}
	return body
}

func licmRemoveEdge(edges []*ir.FlowEdge, target *ir.FlowEdge) []*ir.FlowEdge {
	out := make([]*ir.FlowEdge, 0, len(edges))
	for _, e := range edges {
		if e != target {
			out = append(out, e)
		}
	}
	return out
}
