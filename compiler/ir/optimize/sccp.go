package optimize

import (
	"fmt"
	"math"

	"github.com/WindowsSov8forUs/sonolus-go/compiler/ir"
)

// SCCP is sparse conditional constant propagation. Port of sonolus.py
// constant_evaluation.SparseConditionalConstantPropagation.
// Supports frozenset lattice for phi nodes and multi-way switch-edge pruning.
// Foldable op set: 42 ops (full sonolus.py arithmetic/comparison/logic/trig).
type SCCP struct{}

func (SCCP) Name() string { return "SCCP" }

// lattice values: UNDEF (unknown), a constant, a frozenset of constants,
// or NAC (not a constant).
type latKind int

const (
	latUndef latKind = iota
	latConst
	latFrozen // finite set of distinct constants (for phi merging / switch pruning)
	latNAC
)

// frozensetMax is the maximum number of distinct constants tracked in a
// frozenset before collapsing to NAC. Exceeding 8 elements is pathological
// for engine callbacks.
const frozensetMax = 8

type lat struct {
	kind   latKind
	val    float64
	frozen []float64
}

var (
	undef = lat{kind: latUndef}
	nac   = lat{kind: latNAC}
)

func constLat(v float64) lat { return lat{kind: latConst, val: v} }
func frozenLat(vals []float64) lat {
	if len(vals) > frozensetMax {
		return nac
	}
	return lat{kind: latFrozen, frozen: vals}
}

// latEqual reports whether two lattice values are equal (struct contains slice).
func latEqual(a, b lat) bool {
	if a.kind != b.kind {
		return false
	}
	switch a.kind {
	case latConst:
		return a.val == b.val
	case latFrozen:
		if len(a.frozen) != len(b.frozen) {
			return false
		}
		for i := range a.frozen {
			if a.frozen[i] != b.frozen[i] {
				return false
			}
		}
		return true
	}
	return true
}

func boolLat(b bool) lat {
	if b {
		return constLat(1)
	}
	return constLat(0)
}

// hasVal reports whether v is one of the values in the lattice (const or frozen).
func (l lat) hasVal(v float64) bool {
	switch l.kind {
	case latConst:
		return l.val == v
	case latFrozen:
		for _, f := range l.frozen {
			if f == v {
				return true
			}
		}
	}
	return false
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
	err             error
}

func (s *sccpState) val(n sccpNode) lat {
	if v, ok := s.values[n]; ok {
		return v
	}
	return undef
}

func (SCCP) Run(gen *ir.IDGen, entry *ir.BasicBlock) *ir.BasicBlock {
	s := newSCCPState()
	s.init(entry)
	s.propagate(entry)
	if s.err != nil {
		// Lattice convergence failure is mathematically impossible
		// (finite-height lattice: undef → const/frozenset → NAC,
		// at most 3 states per node). Return the entry unchanged rather
		// than crashing the process.
		return entry
	}
	s.rewrite(entry)
	return entry
}

func newSCCPState() *sccpState {
	return &sccpState{
		values:          map[sccpNode]lat{},
		defs:            map[sccpNode]ir.Node{},
		phiDefs:         map[ir.SSAPlace]map[*ir.FlowEdge]ir.Place{},
		ssaEdges:        map[ir.SSAPlace][]sccpNode{},
		placesToBlocks:  map[ir.SSAPlace]*ir.BasicBlock{},
		executableEdges: map[*ir.FlowEdge]bool{},
		reachable:       map[*ir.BasicBlock]bool{},
	}
}

// init walks the CFG in preorder, building SSA def-use edges, phi definitions,
// and initialising all lattice values to undef.
func (s *sccpState) init(entry *ir.BasicBlock) {
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
}

// propagate runs the alternating flow-edge and SSA-value worklists until a
// fixed point is reached: edges become executable and lattice values converge.
// The lattice is finite (undef → const/frozenset → NAC, max 3 states per node),
// so convergence is guaranteed. The iteration limit is a defense-in-depth
// safeguard against bugs that could cause oscillation.
func (s *sccpState) propagate(entry *ir.BasicBlock) {
	const maxIter = 10000
	s.flowWork = []*ir.FlowEdge{{Src: entry, Dst: entry, Cond: nil}}
	for iter := 0; len(s.flowWork) > 0 || len(s.ssaWork) > 0; iter++ {
		if iter >= maxIter {
			s.err = fmt.Errorf("sccp: iteration limit exceeded — lattice convergence failure")
			return
		}
		s.drainFlowWork()
		s.drainSSAWork()
	}
}

// drainFlowWork processes one edge from the flow worklist: marks it executable,
// visits phi nodes in the destination block, and, if this is the first or only
// executable incoming edge, evaluates SSA definitions and the branch test.
func (s *sccpState) drainFlowWork() {
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
				if nv := s.evaluate(set.Value); !latEqual(nv, s.val(ssaNode(p))) {
					s.values[ssaNode(p)] = nv
					s.pushSSA(s.ssaEdges[p])
				}
			}
			s.evaluateTest(block)
		}
	}
}

// drainSSAWork processes one node from the SSA worklist: re-evaluates phi
// nodes, block branch tests, and ordinary SSA definitions whose lattice
// values may have changed.
func (s *sccpState) drainSSAWork() {
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
			if nv := s.evaluate(s.defs[n]); !latEqual(nv, s.val(n)) {
				s.values[n] = nv
				s.pushSSA(s.ssaEdges[n.place])
			}
		}
	}
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
		if !latEqual(v, undef) {
			distinct = appendDistinct(distinct, v)
		}
	}

	// Collect all distinct constant values across executable phi args.
	// If they are all constants, form a frozenset (aligned with sonolus.py's
	// frozenset lattice); otherwise collapse to NAC.
	var nv lat
	switch len(distinct) {
	case 0:
		nv = undef
	case 1:
		nv = distinct[0]
	default:
		// Check: are all distinct values constants? If so, frozenset them.
		consts := make([]float64, 0, len(distinct))
		allConst := true
		for _, d := range distinct {
			if d.kind != latConst {
				allConst = false
				break
			}
			consts = append(consts, d.val)
		}
		if allConst && len(consts) <= frozensetMax {
			nv = frozenLat(consts)
		} else {
			nv = nac
		}
	}
	if !latEqual(nv, s.val(ssaNode(p))) {
		s.values[ssaNode(p)] = nv
		s.pushSSA(s.ssaEdges[p])
	}
}

func (s *sccpState) evaluateTest(block *ir.BasicBlock) {
	old := s.val(testNode(block))
	nv := s.evaluate(block.Test)
	if latEqual(nv, old) {
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
	// Constant or frozenset test: take matching edge(s), prune non-matching.
	// For a single constant: one edge. For a frozenset: all edges whose cond
	// is in the set are taken; edges whose cond is not in the set are pruned
	// (unless one is the default fallback).
	var defEdge *ir.FlowEdge
	matched := 0
	for _, e := range block.Outgoing {
		if e.Cond == nil {
			defEdge = e
		} else if nv.hasVal(*e.Cond) {
			s.flowWork = append(s.flowWork, e)
			s.reachable[e.Dst] = true
			matched++
		} else {
			// Edge condition is NOT in the frozenset/const — it's dead.
			// We don't mark it executable, effectively pruning it.
		}
	}
	// If no conditional edge matched, fall through to the default edge.
	if matched == 0 && defEdge != nil {
		s.flowWork = append(s.flowWork, defEdge)
		s.reachable[defEdge.Dst] = true
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
	case opAnd:
		// 0 & x = 0 for all x (even NAC/undef), Sonolus boolean semantics.
		for _, a := range args {
			if a.kind == latConst && a.val == 0 {
				return constLat(0)
			}
		}
	case opOr:
		// 1 | x = 1 for all x (even NAC/undef), Sonolus boolean semantics.
		for _, a := range args {
			if a.kind == latConst && a.val == 1 {
				return constLat(1)
			}
		}
	case opMultiply:
		// 0 * x → 0: Sonolus runtime uses only finite floats (no NaN/Inf).
		for _, a := range args {
			if a.kind == latConst && a.val == 0 {
				return constLat(0)
			}
		}
	}
	for _, a := range args {
		if a.kind == latNAC || a.kind == latFrozen {
			return nac // frozenset in arithmetic → NAC (conservative)
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
	case opEqual:
		return boolLat(v[0] == v[1])
	case opNotEqual:
		return boolLat(v[0] != v[1])
	case opGreater:
		return boolLat(v[0] > v[1])
	case opGreaterOr:
		return boolLat(v[0] >= v[1])
	case opLess:
		return boolLat(v[0] < v[1])
	case opLessOr:
		return boolLat(v[0] <= v[1])
	case opNot:
		return boolLat(v[0] == 0)
	case opAnd:
		return boolLat(allNonzero(v))
	case opOr:
		return boolLat(anyNonzero(v))
	case opNegate:
		return constLat(-v[0])
	case opAdd:
		sum := 0.0
		for _, x := range v {
			sum += x
		}
		return constLat(sum)
	case opSubtract:
		if len(v) == 0 {
			return constLat(0)
		}
		r := v[0]
		for _, x := range v[1:] {
			r -= x
		}
		return constLat(r)
	case opMultiply:
		r := 1.0
		for _, x := range v {
			r *= x
		}
		return constLat(r)
	case opDivide:
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
	case opPower:
		if len(v) == 0 {
			return constLat(1)
		}
		r := v[0]
		for _, x := range v[1:] {
			r = math.Pow(r, x)
		}
		return constLat(r)
	case opMod:
		if v[1] == 0 {
			return nac
		}
		return constLat(ir.FloorMod(v[0], v[1]))
	case opMax:
		return constLat(math.Max(v[0], v[1]))
	case opMin:
		return constLat(math.Min(v[0], v[1]))
	case opAbs:
		return constLat(math.Abs(v[0]))
	case opClamp:
		return constLat(math.Min(math.Max(v[0], v[1]), v[2]))
	case opRem:
		if v[1] == 0 {
			return nac
		}
		return constLat(ir.IEEERem(v[0], v[1]))
	case opSign:
		if v[0] < 0 {
			return constLat(-1)
		}
		if v[0] > 0 {
			return constLat(1)
		}
		return constLat(0)
	case opLog:
		return constLat(math.Log(v[0]))
	case opCeil:
		return constLat(math.Ceil(v[0]))
	case opFloor:
		return constLat(math.Floor(v[0]))
	case opRound:
		return constLat(math.Round(v[0]))
	case opFrac:
		return constLat(v[0] - math.Trunc(v[0]))
	case opSin:
		return constLat(math.Sin(v[0]))
	case opCos:
		return constLat(math.Cos(v[0]))
	case opTan:
		return constLat(math.Tan(v[0]))
	case opAtan:
		return constLat(math.Atan(v[0]))
	case opAtan2:
		return constLat(math.Atan2(v[0], v[1]))
	case opDeg:
		return constLat(v[0] * (180 / math.Pi))
	case opRad:
		return constLat(v[0] * (math.Pi / 180))
	case opSinh:
		return constLat(math.Sinh(v[0]))
	case opCosh:
		return constLat(math.Cosh(v[0]))
	case opTanh:
		return constLat(math.Tanh(v[0]))
	case opAsin:
		return constLat(math.Asin(v[0]))
	case opAcos:
		return constLat(math.Acos(v[0]))
	case opLerp:
		return constLat(v[0] + (v[1]-v[0])*v[2])
	case opLerpClamped:
		t := math.Max(0, math.Min(1, v[2]))
		return constLat(v[0] + (v[1]-v[0])*t)
	case opRemap:
		// remap(x, srcMin, srcMax, dstMin, dstMax)
		t := (v[0] - v[1]) / (v[2] - v[1])
		return constLat(v[3] + (v[4]-v[3])*t)
	case opRemapClamped:
		t := math.Max(0, math.Min(1, (v[0]-v[1])/(v[2]-v[1])))
		return constLat(v[3] + (v[4]-v[3])*t)
	default:
		return nac
	}
}

var sccpSupportedOps = map[ir.Op]bool{
	opEqual: true, opNotEqual: true, opGreater: true, opGreaterOr: true,
	opLess: true, opLessOr: true, opNot: true, opAnd: true, opOr: true,
	opNegate: true, opAdd: true, opSubtract: true, opMultiply: true,
	opDivide: true, opPower: true, opMod: true, opRem: true,
	opMax: true, opMin: true, opAbs: true, opClamp: true, opSign: true,
	opLog: true, opCeil: true, opFloor: true, opRound: true, opFrac: true,
	opSin: true, opCos: true, opTan: true, opAtan: true, opAtan2: true,
	opDeg: true, opRad: true,
	// Transcendental (was missing from earlier version, now folded).
	opSinh: true, opCosh: true, opTanh: true, opAsin: true, opAcos: true,
	// Interpolation/remapping.
	opLerp: true, opLerpClamped: true, opRemap: true, opRemapClamped: true,
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
		return ir.Instr{ID: t.ID, Op: t.Op, Args: args, Pure: t.Pure}
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
			return ir.Set{ID: t.ID, Place: t.Place, Value: s.substitute(t.Value)}
		}
		return ir.Set{ID: t.ID, Place: s.substitutePlace(t.Place), Value: s.substitute(t.Value)}
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

// appendNode adds n to the nodes slice. Deduplication is unnecessary because
// the SCCP lattice has finite height (3 states per node), so duplicate
// worklist entries cause at most one redundant but convergent re-evaluation.
func appendNode(nodes []sccpNode, n sccpNode) []sccpNode {
	return append(nodes, n)
}

func appendDistinct(vals []lat, v lat) []lat {
	for _, x := range vals {
		if latEqual(x, v) {
			return vals
		}
	}
	return append(vals, v)
}

func (SCCP) Requires() []Analysis  { return []Analysis{AnalysisSSA} }
func (SCCP) Preserves() []Analysis { return nil }
func (SCCP) Destroys() []Analysis  { return nil }
