package optimize

import (
	"hash/fnv"

	"github.com/WindowsSov8forUs/sonolus-go/compiler/ir"
)

// LICM hoists loop-invariant expressions out of loop bodies into pre-headers.
// Loop detection uses dominance-tree back-edges (FindLoops in optimize.go):
// an edge B→H is a back-edge when H dominates B. The loop body is the set of
// blocks reachable backward from the latch, stopped at the header.
//
// IMPORTANT: LICM copies loop-invariant expressions into pre-header blocks
// but does NOT rewrite uses inside the loop body. A subsequent CSE pass
// (CommonSubexpressionElimination) deduplicates the hoisted copy against the
// original, effectively rewiring loop-body reads to the pre-header value.
// This coupling matches the original sonolus.py design (sonolus/backend/
// optimize/licm.py:31-33). The Standard pipeline runs LICM immediately
// before CSE to satisfy this invariant.
//
// Port of sonolus.py licm.LoopInvariantCodeMotion.
type LICM struct {
	Oracle BlockOracle
}

func (LICM) Name() string { return "LICM" }

func (l LICM) Run(gen *ir.IDGen, entry *ir.BasicBlock) *ir.BasicBlock {
	return l.RunWithDom(gen, entry, &DominanceCache{})
}

func (l LICM) RunWithDom(gen *ir.IDGen, entry *ir.BasicBlock, dc *DominanceCache) *ir.BasicBlock {
	dom := dc.Get(entry)
	blocks := ir.ReversePostorder(entry)

	loops := FindLoops(blocks, dom)

	nextID := 0
	for _, lp := range loops {
		if lp.Header == entry {
			continue
		}
		nextID = licmProcessLoop(lp, dom, nextID, l.Oracle)
	}
	return entry
}

func licmProcessLoop(lp Loop, dom *Dominance, nextID int, oracle BlockOracle) int {
	var preheader *ir.BasicBlock
	var nonBack []*ir.FlowEdge
	for _, e := range lp.Header.Incoming {
		if !lp.Body[e.Src] {
			nonBack = append(nonBack, e)
		}
	}
	if len(nonBack) == 0 {
		return nextID
	}
	if len(nonBack) == 1 && len(nonBack[0].Src.Statements) == 0 &&
		len(nonBack[0].Src.Outgoing) == 1 && len(nonBack[0].Src.Phis) == 0 {
		preheader = nonBack[0].Src
	} else {
		preheader = ir.NewBlock()
		preheader.ConnectTo(lp.Header, nil)
		for _, e := range nonBack {
			e.Dst = preheader
			preheader.Incoming = append(preheader.Incoming, e)
			lp.Header.Incoming = removeEdge(lp.Header.Incoming, e)
		}
		lp.Header.Incoming = append(lp.Header.Incoming, &ir.FlowEdge{Src: preheader, Dst: lp.Header, Cond: nil})

		// Migrate phi nodes: values arriving from blocks outside the loop body
		// must be rewired to the newly created preheader. This mirrors the logic
		// in sonolus.py's _get_or_create_preheader (licm.py:115-124).
		for _, phi := range lp.Header.Phis {
			preheaderValues := map[*ir.BasicBlock]ir.Place{}
			for src := range phi.Args {
				if !lp.Body[src] {
					preheaderValues[src] = phi.Args[src]
					delete(phi.Args, src)
				}
			}
			if len(preheaderValues) == 1 {
				for _, v := range preheaderValues {
					phi.Args[preheader] = v
				}
			} else if len(preheaderValues) > 1 {
				// Multiple non-loop predecessors each contribute a distinct SSA
				// value. Hoist the merge into the preheader via a phi there, then
				// feed the result to the header.
				preheader.Phis = append(preheader.Phis, &ir.Phi{
					Var:    phi.Var,
					Target: phi.Target,
					Args:   preheaderValues,
				})
				phi.Args[preheader] = phi.Target
			}
		}
	}

	defs := licmDefsInLoop(lp.Body)
	var guaranteed []*ir.BasicBlock
	for b := range lp.Body {
		ok := true
		for _, latch := range lp.Latches {
			if !dominates(dom, b, latch) {
				ok = false
				break
			}
		}
		if ok {
			guaranteed = append(guaranteed, b)
		}
	}

	hoisted := map[cseKeyType]bool{}
	for _, b := range guaranteed {
		for _, s := range b.Statements {
			nextID = licmHoistExpr(s, preheader, defs, nextID, hoisted, oracle)
		}
		nextID = licmHoistExpr(b.Test, preheader, defs, nextID, hoisted, oracle)
	}
	return nextID
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

func licmHoistExpr(n ir.Node, preheader *ir.BasicBlock, defs map[ir.SSAPlace]bool, nextID int, hoisted map[cseKeyType]bool, oracle BlockOracle) int {
	instr, ok := n.(ir.Instr)
	if !ok || !ir.Pure(instr.Op) || ir.SideEffects(instr.Op) {
		return nextID
	}
	h := fnv.New128a()
	k := cseKey(instr, h)
	if hoisted[k] {
		return nextID
	}
	if !licmIsInvariant(instr, defs, oracle) {
		return nextID
	}
	if exprCost(instr) < inlineCostThreshold {
		return nextID
	}
	hoisted[k] = true
	p := ir.SSAPlace{Name: "licm", Num: nextID}
	nextID++
	preheader.Statements = append(preheader.Statements, ir.Set{Place: p, Value: instr})
	return nextID
}

func licmIsInvariant(instr ir.Instr, defs map[ir.SSAPlace]bool, oracle BlockOracle) bool {
	for _, a := range instr.Args {
		if !licmArgInvariant(a, defs, oracle) {
			return false
		}
	}
	return true
}

func licmArgInvariant(n ir.Node, defs map[ir.SSAPlace]bool, oracle BlockOracle) bool {
	switch t := n.(type) {
	case ir.Const:
		return true
	case ir.SSAPlace:
		return !defs[t]
	case ir.Instr:
		for _, a := range t.Args {
			if !licmArgInvariant(a, defs, oracle) {
				return false
			}
		}
		return true
	case ir.Get:
		if p, ok := t.Place.(ir.SSAPlace); ok {
			return !defs[p]
		}
		// A concrete-block load is invariant if the block oracle says the block
		// is not writable (or is runtime-constant). Both conditions hold for
		// ROM, shared memory, and other read-only blocks.
		if bp, ok := t.Place.(ir.BlockPlace); ok {
			if c, ok := bp.Block.(ir.Const); ok {
				blockID := int(float64(c))
				if !oracle.Writable(blockID, "") || oracle.RuntimeConstant(blockID) {
					return licmArgInvariant(bp.Index, defs, oracle)
				}
			}
		}
		return false
	default:
		return false
	}
}

// Requires implements ManagedPass — LICM uses dominance via DominanceCache (RunWithDom).
func (LICM) Requires() []Analysis { return nil }

// Preserves implements ManagedPass.
func (LICM) Preserves() []Analysis { return nil }

// Destroys implements ManagedPass.
func (LICM) Destroys() []Analysis { return nil }
