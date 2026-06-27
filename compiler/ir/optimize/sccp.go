package optimize

import (
	"math"

	"github.com/WindowsSov8forUs/sonolus-go/compiler/ir"
)

// SCCP is sparse conditional constant propagation. Port of sonolus.py
// constant_evaluation.SparseConditionalConstantPropagation, with one documented
// reduction: the multi-value (frozenset) lattice element is collapsed to NAC.
// The op set covers arithmetic/comparison/logic/trig/transcendental (35 ops);
// ops yields NAC. This reduction only makes SCCP fold less; it never produces
// a wrong constant.
type SCCP struct{}

func (SCCP) Name() string { return "SCCP" }

// lattice values: UNDEF (unknown), a constant, or NAC (not a constant).
type latKind int

const (
	latUndef latKind = iota
	latConst
	latNAC
)

type lat struct {
	kind latKind
	val  float64
}

var (
	undef = lat{kind: latUndef}
	nac   = lat{kind: latNAC}
)

func constLat(v float64) lat { return lat{kind: latConst, val: v} }
func boolLat(b bool) lat {
	if b {
		return constLat(1)
	}
	return constLat(0)
}

// sccpNode identifies something that holds a lattice value: an SSA place or a
// block's branch test (exactly one field set).
type sccpNode struct {
	place ir.SSAPlace
	block *ir.BasicBlock
}

func ssaNode(p ir.SSAPlace) sccpNode     { return sccpNode{place: p} }
func testNode(b *ir.BasicBlock) sccpNode { return sccpNode{block: b} }
func (n sccpNode) isBlock() bool         { return n.block != nil }

type sccpState struct {
	values          map[sccpNode]lat
	defs            map[sccpNode]ir.Node // SSA def value or block test
	phiDefs         map[ir.SSAPlace]map[*ir.FlowEdge]ir.Place
	ssaEdges        map[ir.SSAPlace][]sccpNode
	placesToBlocks  map[ir.SSAPlace]*ir.BasicBlock
	executableEdges map[*ir.FlowEdge]bool
	reachable       map[*ir.BasicBlock]bool
	flowWork        []*ir.FlowEdge
	ssaWork         []sccpNode
}

func (s *sccpState) val(n sccpNode) lat {
	if v, ok := s.values[n]; ok {
		return v
	}
	return undef
}

func (SCCP) Run(entry *ir.BasicBlock) *ir.BasicBlock {
	s := &sccpState{
		values:          map[sccpNode]lat{},
		defs:            map[sccpNode]ir.Node{},
		phiDefs:         map[ir.SSAPlace]map[*ir.FlowEdge]ir.Place{},
		ssaEdges:        map[ir.SSAPlace][]sccpNode{},
		placesToBlocks:  map[ir.SSAPlace]*ir.BasicBlock{},
		executableEdges: map[*ir.FlowEdge]bool{},
		reachable:       map[*ir.BasicBlock]bool{},
	}

	for _, block := range ir.Preorder(entry) {
		incomingBySrc := map[*ir.BasicBlock][]*ir.FlowEdge{}
		for _, e := range block.Incoming {
			incomingBySrc[e.Src] = append(incomingBySrc[e.Src], e)
		}
		for _, phi := range block.Phis {
			p := phi.Target.(ir.SSAPlace)
			pd := map[*ir.FlowEdge]ir.Place{}
			for srcBlock, arg := range phi.Args {
				for _, inc := range incomingBySrc[srcBlock] {
					pd[inc] = arg
				}
			}
			s.phiDefs[p] = pd
			s.values[ssaNode(p)] = undef
			for _, arg := range phi.Args {
				if a, ok := arg.(ir.SSAPlace); ok {
					s.ssaEdges[a] = appendNode(s.ssaEdges[a], ssaNode(p))
				}
			}
		}
		for _, stmt := range block.Statements {
			if set, ok := stmt.(ir.Set); ok {
				if p, ok := set.Place.(ir.SSAPlace); ok {
					s.defs[ssaNode(p)] = set.Value
					s.placesToBlocks[p] = block
					s.values[ssaNode(p)] = undef
					for dep := range sccpDeps(set.Value) {
						s.ssaEdges[dep] = appendNode(s.ssaEdges[dep], ssaNode(p))
					}
				}
			}
		}
		s.defs[testNode(block)] = block.Test
		s.values[testNode(block)] = undef
		for dep := range sccpDeps(block.Test) {
			s.ssaEdges[dep] = appendNode(s.ssaEdges[dep], testNode(block))
		}
	}

	s.flowWork = []*ir.FlowEdge{{Src: entry, Dst: entry, Cond: nil}}
	for len(s.flowWork) > 0 || len(s.ssaWork) > 0 {
		for len(s.flowWork) > 0 {
			edge := s.flowWork[len(s.flowWork)-1]
			s.flowWork = s.flowWork[:len(s.flowWork)-1]
			if s.executableEdges[edge] {
				continue
			}
			s.executableEdges[edge] = true
			block := edge.Dst

			for _, phi := range block.Phis {
				s.visitPhi(phi.Target.(ir.SSAPlace))
			}

			executableIncoming := 0
			for _, e := range block.Incoming {
				if s.executableEdges[e] {
					executableIncoming++
				}
			}
			if executableIncoming <= 1 {
				for _, stmt := range block.Statements {
					set, ok := stmt.(ir.Set)
					if !ok {
						continue
					}
					p, ok := set.Place.(ir.SSAPlace)
					if !ok {
						continue
					}
					if nv := s.evaluate(set.Value); nv != s.val(ssaNode(p)) {
						s.values[ssaNode(p)] = nv
						s.pushSSA(s.ssaEdges[p])
					}
				}
				s.evaluateTest(block)
			}
		}
		for len(s.ssaWork) > 0 {
			n := s.ssaWork[len(s.ssaWork)-1]
			s.ssaWork = s.ssaWork[:len(s.ssaWork)-1]
			switch {
			case !n.isBlock() && s.phiDefs[n.place] != nil:
				s.visitPhi(n.place)
			case n.isBlock():
				s.evaluateTest(n.block)
			default:
				if !s.reachable[s.placesToBlocks[n.place]] {
					continue
				}
				if nv := s.evaluate(s.defs[n]); nv != s.val(n) {
					s.values[n] = nv
					s.pushSSA(s.ssaEdges[n.place])
				}
			}
		}
	}

	s.rewrite(entry)
	return entry
}

func (s *sccpState) pushSSA(nodes []sccpNode) {
	s.ssaWork = append(s.ssaWork, nodes...)
}

func (s *sccpState) visitPhi(p ir.SSAPlace) {
	var distinct []lat
	for e, place := range s.phiDefs[p] {
		v := undef
		if s.executableEdges[e] {
			if pl, ok := place.(ir.SSAPlace); ok {
				v = s.val(ssaNode(pl))
			} else {
				v = nac // a non-SSA phi arg is opaque
			}
		}
		if v != undef {
			distinct = appendDistinct(distinct, v)
		}
	}

	var nv lat
	switch len(distinct) {
	case 0:
		nv = undef
	case 1:
		nv = distinct[0]
	default:
		nv = nac // multiple distinct values collapse to NAC (no frozenset)
	}
	if nv != s.val(ssaNode(p)) {
		s.values[ssaNode(p)] = nv
		s.pushSSA(s.ssaEdges[p])
	}
}

func (s *sccpState) evaluateTest(block *ir.BasicBlock) {
	old := s.val(testNode(block))
	nv := s.evaluate(block.Test)
	if nv == old {
		// An unconditional successor stays reachable.
		if len(block.Outgoing) == 1 && block.Outgoing[0].Cond == nil {
			s.flowWork = append(s.flowWork, block.Outgoing[0])
			s.reachable[block.Outgoing[0].Dst] = true
		}
		return
	}
	if nv.kind == latUndef {
		return
	}
	s.values[testNode(block)] = nv
	if nv.kind == latNAC {
		for _, e := range block.Outgoing {
			s.flowWork = append(s.flowWork, e)
			s.reachable[e.Dst] = true
		}
		return
	}
	// Constant test: take the matching edge, or the default.
	byCond := map[*float64]*ir.FlowEdge{}
	var defEdge *ir.FlowEdge
	var matched *ir.FlowEdge
	for _, e := range block.Outgoing {
		if e.Cond == nil {
			defEdge = e
		} else if *e.Cond == nv.val {
			matched = e
		}
		byCond[e.Cond] = e
	}
	taken := matched
	if taken == nil {
		taken = defEdge
	}
	if taken != nil {
		s.flowWork = append(s.flowWork, taken)
		s.reachable[taken.Dst] = true
	}
}

func (s *sccpState) evaluate(n ir.Node) lat {
	switch t := n.(type) {
	case ir.Const:
		return constLat(float64(t))
	case ir.SSAPlace:
		return s.val(ssaNode(t))
	case ir.Get:
		if p, ok := t.Place.(ir.SSAPlace); ok {
			return s.val(ssaNode(p))
		}
		return nac
	case ir.Instr:
		return s.evalInstr(t)
	default:
		return nac
	}
}

func (s *sccpState) evalInstr(t ir.Instr) lat {
	if !sccpSupportedOps[t.Op] {
		return nac
	}
	args := make([]lat, len(t.Args))
	for i, a := range t.Args {
		args[i] = s.evaluate(a)
	}

	switch t.Op {
	case ir.Op("And"):
		for _, a := range args {
			if a.kind == latConst && a.val == 0 {
				return constLat(0)
			}
		}
	case ir.Op("Or"):
		for _, a := range args {
			if a.kind == latConst && a.val == 1 {
				return constLat(1)
			}
		}
	case ir.Op("Multiply"):
		for _, a := range args {
			if a.kind == latConst && a.val == 0 {
				return constLat(0)
			}
		}
	}
	for _, a := range args {
		if a.kind == latNAC {
			return nac
		}
	}
	for _, a := range args {
		if a.kind == latUndef {
			return undef
		}
	}

	v := make([]float64, len(args))
	for i, a := range args {
		v[i] = a.val
	}
	return computeOp(t.Op, v)
}

func computeOp(op ir.Op, v []float64) lat {
	switch op {
	case "Equal":
		return boolLat(v[0] == v[1])
	case "NotEqual":
		return boolLat(v[0] != v[1])
	case "Greater":
		return boolLat(v[0] > v[1])
	case "GreaterOr":
		return boolLat(v[0] >= v[1])
	case "Less":
		return boolLat(v[0] < v[1])
	case "LessOr":
		return boolLat(v[0] <= v[1])
	case "Not":
		return boolLat(v[0] == 0)
	case "And":
		return boolLat(allNonzero(v))
	case "Or":
		return boolLat(anyNonzero(v))
	case "Negate":
		return constLat(-v[0])
	case "Add":
		sum := 0.0
		for _, x := range v {
			sum += x
		}
		return constLat(sum)
	case "Subtract":
		if len(v) == 0 {
			return constLat(0)
		}
		r := v[0]
		for _, x := range v[1:] {
			r -= x
		}
		return constLat(r)
	case "Multiply":
		r := 1.0
		for _, x := range v {
			r *= x
		}
		return constLat(r)
	case "Divide":
		if len(v) == 0 {
			return constLat(1)
		}
		denom := 1.0
		for _, x := range v[1:] {
			denom *= x
		}
		if denom == 0 {
			return nac
		}
		return constLat(v[0] / denom)
	case "Power":
		if len(v) == 0 {
			return constLat(1)
		}
		r := v[0]
		for _, x := range v[1:] {
			r = math.Pow(r, x)
		}
		return constLat(r)
	case "Mod":
		return constLat(floorMod(v[0], v[1]))
	case "Max":
		return constLat(math.Max(v[0], v[1]))
	case "Min":
		return constLat(math.Min(v[0], v[1]))
	case "Abs":
		return constLat(math.Abs(v[0]))
	case "Clamp":
		return constLat(math.Min(math.Max(v[0], v[1]), v[2]))
	case "Rem":
		return constLat(floorMod(v[0], v[1]))
	case "Sign":
		if v[0] < 0 {
			return constLat(-1)
		}
		if v[0] > 0 {
			return constLat(1)
		}
		return constLat(0)
	case "Log":
		return constLat(math.Log(v[0]))
	case "Ceil":
		return constLat(math.Ceil(v[0]))
	case "Floor":
		return constLat(math.Floor(v[0]))
	case "Round":
		return constLat(math.Round(v[0]))
	case "Frac":
		return constLat(v[0] - float64(int(v[0])))
	case "Sin":
		return constLat(math.Sin(v[0]))
	case "Cos":
		return constLat(math.Cos(v[0]))
	case "Tan":
		return constLat(math.Tan(v[0]))
	case "Arctan":
		return constLat(math.Atan(v[0]))
	case "Arctan2":
		return constLat(math.Atan2(v[0], v[1]))
	case "Degree":
		return constLat(v[0] * (180 / math.Pi))
	case "Radian":
		return constLat(v[0] * (math.Pi / 180))
	case "CopySign":
		return constLat(math.Copysign(v[0], v[1]))
	default:
		return nac
	}
}

var sccpSupportedOps = map[ir.Op]bool{
	"Equal": true, "NotEqual": true, "Greater": true, "GreaterOr": true,
	"Less": true, "LessOr": true, "Not": true, "And": true, "Or": true,
	"Negate": true, "Add": true, "Subtract": true, "Multiply": true,
	"Divide": true, "Power": true, "Mod": true, "Rem": true,
	"Max": true, "Min": true, "Abs": true, "Clamp": true,
	// Transcendental / rounding / trig (sonolus.py full set).
	"Sign": true, "Log": true, "Ceil": true, "Floor": true, "Round": true,
	"Frac": true, "Sin": true, "Cos": true, "Tan": true,
	"Arctan": true, "Arctan2": true, "Degree": true, "Radian": true,
	"CopySign": true,
}

func allNonzero(v []float64) bool {
	for _, x := range v {
		if x == 0 {
			return false
		}
	}
	return true
}

func anyNonzero(v []float64) bool {
	for _, x := range v {
		if x != 0 {
			return true
		}
	}
	return false
}

// floorMod matches Python's % (result has the sign of the divisor).
func floorMod(a, b float64) float64 {
	r := math.Mod(a, b)
	if r != 0 && (r < 0) != (b < 0) {
		r += b
	}
	return r
}

// sccpDeps returns the SSA places a node reads (mirrors get_dependencies).
func sccpDeps(n ir.Node) map[ir.SSAPlace]bool {
	deps := map[ir.SSAPlace]bool{}
	collectDeps(n, deps)
	return deps
}

func collectDeps(n ir.Node, deps map[ir.SSAPlace]bool) {
	switch t := n.(type) {
	case ir.Instr:
		for _, a := range t.Args {
			collectDeps(a, deps)
		}
	case ir.Get:
		if p, ok := t.Place.(ir.SSAPlace); ok {
			deps[p] = true
		} else if bp, ok := t.Place.(ir.BlockPlace); ok {
			collectDeps(bp.Block, deps)
		}
	case ir.BlockPlace:
		collectDeps(t.Block, deps)
		collectDeps(t.Index, deps)
	case ir.SSAPlace:
		deps[t] = true
	}
}

func (s *sccpState) rewrite(entry *ir.BasicBlock) {
	for _, block := range ir.Preorder(entry) {
		for i, stmt := range block.Statements {
			block.Statements[i] = s.substitute(stmt)
		}
		block.Test = s.substitute(block.Test)
	}

	// Drop statements that define a never-evaluated (unreachable) SSA value.
	for _, block := range ir.Preorder(entry) {
		stmts := block.Statements[:0]
		for _, stmt := range block.Statements {
			if set, ok := stmt.(ir.Set); ok {
				if p, ok := set.Place.(ir.SSAPlace); ok && s.val(ssaNode(p)).kind == latUndef {
					continue
				}
			}
			stmts = append(stmts, stmt)
		}
		block.Statements = stmts
		// Drop phi args that flow a never-evaluated value.
		for _, phi := range block.Phis {
			for src, arg := range phi.Args {
				if a, ok := arg.(ir.SSAPlace); ok && s.val(ssaNode(a)).kind == latUndef {
					delete(phi.Args, src)
				}
			}
		}
	}
}

// substitute replaces SSA reads of constant values with literals.
func (s *sccpState) substitute(n ir.Node) ir.Node {
	switch t := n.(type) {
	case ir.Instr:
		args := make([]ir.Node, len(t.Args))
		for i, a := range t.Args {
			args[i] = s.substitute(a)
		}
		return ir.Instr{Op: t.Op, Args: args, Pure: t.Pure}
	case ir.Get:
		if p, ok := t.Place.(ir.SSAPlace); ok {
			if v := s.val(ssaNode(p)); v.kind == latConst {
				return ir.Const(v.val)
			}
			return t
		}
		return ir.Get{Place: s.substitutePlace(t.Place)}
	case ir.Set:
		if _, ok := t.Place.(ir.SSAPlace); ok {
			return ir.Set{Place: t.Place, Value: s.substitute(t.Value)}
		}
		return ir.Set{Place: s.substitutePlace(t.Place), Value: s.substitute(t.Value)}
	case ir.SSAPlace:
		if v := s.val(ssaNode(t)); v.kind == latConst {
			return ir.Const(v.val)
		}
		return t
	default:
		return n
	}
}

func (s *sccpState) substitutePlace(p ir.Place) ir.Place {
	if bp, ok := p.(ir.BlockPlace); ok {
		return ir.BlockPlace{Block: s.substitute(bp.Block), Index: s.substitute(bp.Index), Offset: bp.Offset}
	}
	return p
}

func appendNode(nodes []sccpNode, n sccpNode) []sccpNode {
	for _, x := range nodes {
		if x == n {
			return nodes
		}
	}
	return append(nodes, n)
}

func appendDistinct(vals []lat, v lat) []lat {
	for _, x := range vals {
		if x == v {
			return vals
		}
	}
	return append(vals, v)
}
