package optimize

import (
	"fmt"
	"sort"

	"github.com/WindowsSov8forUs/sonolus-go/v2/internal/compiler/ir"
)

type ToSSA struct{}

func (ToSSA) Name() string          { return "ToSSA" }
func (ToSSA) Requires() []Analysis  { return []Analysis{AnalysisDominance} }
func (ToSSA) Preserves() []Analysis { return nil }
func (ToSSA) Destroys() []Analysis  { return []Analysis{AnalysisDominance, AnalysisLiveness} }
func (ToSSA) Run(context Context, function *ir.Function) error {
	if function.Allocated {
		return fmt.Errorf("cannot enter SSA after local allocation")
	}
	eligible := eligibleScalarLocals(function)
	dom := dominanceFor(context, function)
	type key struct{ id, offset int }
	defs := map[key]map[int]bool{}
	for _, block := range function.Blocks {
		for _, instruction := range block.Instructions {
			if store, ok := instruction.(ir.Store); ok {
				if p, ok := store.Place.(ir.LocalPlace); ok && eligible[p.ID] {
					k := key{p.ID, p.Offset}
					if defs[k] == nil {
						defs[k] = map[int]bool{}
					}
					defs[k][block.ID] = true
				}
			}
		}
	}
	keys := make([]key, 0, len(defs))
	for k := range defs {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].id != keys[j].id {
			return keys[i].id < keys[j].id
		}
		return keys[i].offset < keys[j].offset
	})
	for _, k := range keys {
		placed := map[int]bool{}
		work := make([]int, 0, len(defs[k]))
		for b := range defs[k] {
			work = append(work, b)
		}
		sort.Ints(work)
		for len(work) > 0 {
			b := work[0]
			work = work[1:]
			fs := make([]int, 0, len(dom.Frontier[b]))
			for f := range dom.Frontier[b] {
				fs = append(fs, f)
			}
			sort.Ints(fs)
			for _, f := range fs {
				if placed[f] {
					continue
				}
				placed[f] = true
				function.Blocks[f].Phis = append(function.Blocks[f].Phis, ir.Phi{Local: ir.LocalPlace{ID: k.id, Offset: k.offset}})
				if !defs[k][f] {
					work = append(work, f)
				}
			}
		}
	}
	next := 0
	stacks := map[key][]ir.SSAPlace{}
	newValue := func(k key) ir.SSAPlace {
		next++
		return ir.SSAPlace{ID: next, Name: fmt.Sprintf("local.%d.%d", k.id, k.offset)}
	}
	var renameExpr func(ir.Expr) ir.Expr
	renamePlace := func(place ir.Place) ir.Place { return place }
	renameExpr = func(expr ir.Expr) ir.Expr {
		switch v := expr.(type) {
		case ir.Load:
			if p, ok := v.Place.(ir.LocalPlace); ok && eligible[p.ID] {
				s := stacks[key{p.ID, p.Offset}]
				if len(s) > 0 {
					v.Place = s[len(s)-1]
				}
			}
			v.Place = rewritePlace(v.Place, renameExpr)
			return v
		case ir.RuntimeCall:
			for i, a := range v.Args {
				v.Args[i] = renameExpr(a)
			}
			return v
		default:
			return expr
		}
	}
	_ = renamePlace
	var rename func(int)
	rename = func(id int) {
		block := function.Blocks[id]
		var pushed []key
		for i := range block.Phis {
			k := key{block.Phis[i].Local.ID, block.Phis[i].Local.Offset}
			v := newValue(k)
			block.Phis[i].Target = v
			stacks[k] = append(stacks[k], v)
			pushed = append(pushed, k)
		}
		for i, instruction := range block.Instructions {
			switch v := instruction.(type) {
			case ir.Store:
				v.Value = renameExpr(v.Value)
				v.Place = rewritePlace(v.Place, renameExpr)
				if p, ok := v.Place.(ir.LocalPlace); ok && eligible[p.ID] {
					k := key{p.ID, p.Offset}
					ssa := newValue(k)
					v.Place = ssa
					stacks[k] = append(stacks[k], ssa)
					pushed = append(pushed, k)
				}
				block.Instructions[i] = v
			case ir.Eval:
				v.Value = renameExpr(v.Value)
				block.Instructions[i] = v
			}
		}
		switch v := block.Terminator.(type) {
		case ir.Branch:
			v.Condition = renameExpr(v.Condition)
			block.Terminator = v
		case ir.Switch:
			v.Value = renameExpr(v.Value)
			block.Terminator = v
		case ir.Return:
			for i, e := range v.Value.Slots {
				v.Value.Slots[i] = renameExpr(e)
			}
			block.Terminator = v
		}
		forEachTerminatorTarget(block.Terminator, func(succ int) {
			for i := range function.Blocks[succ].Phis {
				k := key{function.Blocks[succ].Phis[i].Local.ID, function.Blocks[succ].Phis[i].Local.Offset}
				s := stacks[k]
				if len(s) > 0 {
					function.Blocks[succ].Phis[i].Args = append(function.Blocks[succ].Phis[i].Args, ir.PhiArg{Predecessor: id, Value: s[len(s)-1]})
				}
			}
		})
		for _, child := range dom.Children[id] {
			rename(child)
		}
		for i := len(pushed) - 1; i >= 0; i-- {
			k := pushed[i]
			stacks[k] = stacks[k][:len(stacks[k])-1]
		}
	}
	rename(function.Entry)
	for _, block := range function.Blocks {
		for i := range block.Phis {
			sort.Slice(block.Phis[i].Args, func(a, b int) bool {
				return block.Phis[i].Args[a].Predecessor < block.Phis[i].Args[b].Predecessor
			})
		}
	}
	return nil
}

func eligibleScalarLocals(function *ir.Function) map[int]bool {
	result := map[int]bool{}
	for id := range function.Locals {
		result[id] = true
	}
	var inspectPlace func(ir.Place)
	inspectPlace = func(place ir.Place) {
		switch p := place.(type) {
		case ir.IndexedLocalPlace:
			delete(result, p.ID)
			inspectExprPlaces(p.Index, inspectPlace)
		case ir.MemoryPlace:
			inspectExprPlaces(p.Index, inspectPlace)
		}
	}
	for _, b := range function.Blocks {
		for _, in := range b.Instructions {
			switch v := in.(type) {
			case ir.Store:
				inspectPlace(v.Place)
				inspectExprPlaces(v.Value, inspectPlace)
			case ir.Eval:
				inspectExprPlaces(v.Value, inspectPlace)
			}
		}
	}
	return result
}
func inspectExprPlaces(expr ir.Expr, fn func(ir.Place)) {
	switch v := expr.(type) {
	case ir.Load:
		fn(v.Place)
	case ir.RuntimeCall:
		for _, a := range v.Args {
			inspectExprPlaces(a, fn)
		}
	}
}

type FromSSA struct{}

func (FromSSA) Name() string          { return "FromSSA" }
func (FromSSA) Requires() []Analysis  { return []Analysis{AnalysisSSA} }
func (FromSSA) Preserves() []Analysis { return nil }
func (FromSSA) Destroys() []Analysis {
	return []Analysis{AnalysisSSA, AnalysisDominance, AnalysisLiveness}
}
func (FromSSA) Run(_ Context, function *ir.Function) error {
	splitPhiCriticalEdges(function)
	mapping := map[int]ir.LocalPlace{}
	constants := map[int]ir.Const{}
	placeFor := func(ssa ir.SSAPlace) ir.LocalPlace {
		if p, ok := mapping[ssa.ID]; ok {
			return p
		}
		id := len(function.Locals)
		function.Locals = append(function.Locals, ir.Type{Name: ssa.Name, Slots: 1})
		p := ir.LocalPlace{ID: id, Name: ssa.Name}
		mapping[ssa.ID] = p
		return p
	}
	for _, block := range function.Blocks {
		for _, phi := range block.Phis {
			placeFor(phi.Target)
			for _, arg := range phi.Args {
				placeFor(arg.Value)
			}
		}
		for _, in := range block.Instructions {
			if s, ok := in.(ir.Store); ok {
				if p, ok := s.Place.(ir.SSAPlace); ok {
					local := placeFor(p)
					if value, ok := s.Value.(ir.Const); ok {
						constants[local.ID] = value
					}
				}
			}
		}
	}
	for _, block := range function.Blocks {
		type copyPair struct{ target, source ir.LocalPlace }
		copies := map[int][]copyPair{}
		var predecessors []int
		for _, phi := range block.Phis {
			target := placeFor(phi.Target)
			for _, arg := range phi.Args {
				source := placeFor(arg.Value)
				if target.ID == source.ID {
					continue
				}
				if _, exists := copies[arg.Predecessor]; !exists {
					predecessors = append(predecessors, arg.Predecessor)
				}
				copies[arg.Predecessor] = append(copies[arg.Predecessor], copyPair{target, source})
			}
		}
		sort.Ints(predecessors)
		for _, id := range predecessors {
			pred := function.Blocks[id]
			moves := copies[id]
			sources := map[int]bool{}
			for _, move := range moves {
				sources[move.source.ID] = true
			}
			// Phi assignments on an edge are simultaneous. Save old values
			// of destinations that are also sources, then redirect every read
			// of those values before writing any destination.
			snapshots := map[int]ir.LocalPlace{}
			for _, move := range moves {
				if !sources[move.target.ID] {
					continue
				}
				temporary := ir.LocalPlace{ID: len(function.Locals), Name: "phi.copy"}
				function.Locals = append(function.Locals, ir.Type{Name: "phi.copy", Slots: 1})
				pred.Instructions = append(pred.Instructions, ir.Store{Place: temporary, Value: ir.Load{Place: move.target}})
				snapshots[move.target.ID] = temporary
			}
			for _, move := range moves {
				if temporary, exists := snapshots[move.source.ID]; exists {
					move.source = temporary
				}
				var value ir.Expr = ir.Load{Place: move.source}
				if constant, ok := constants[move.source.ID]; ok {
					// Materialize immutable constants on the selected edge. The
					// original SSA store can then die without an entry-time copy.
					value = constant
				}
				pred.Instructions = append(pred.Instructions, ir.Store{Place: move.target, Value: value})
			}
		}
		block.Phis = nil
	}
	rewriteSSA := func(expr ir.Expr) (ir.Expr, bool) {
		if load, ok := expr.(ir.Load); ok {
			if p, ok := load.Place.(ir.SSAPlace); ok {
				load.Place = placeFor(p)
				return load, true
			}
		}
		return expr, false
	}
	for _, block := range function.Blocks {
		for i, in := range block.Instructions {
			if store, ok := in.(ir.Store); ok {
				if p, ok := store.Place.(ir.SSAPlace); ok {
					store.Place = placeFor(p)
				}
				block.Instructions[i] = store
			}
		}
	}
	rewriteFunctionExpressionsChanged(function, rewriteSSA)
	return nil
}

func splitPhiCriticalEdges(function *ir.Function) {
	for _, block := range function.Blocks {
		if len(block.Phis) == 0 {
			continue
		}
		for _, phi := range block.Phis {
			for _, arg := range phi.Args {
				if terminatorTargetCount(function.Blocks[arg.Predecessor].Terminator) <= 1 {
					continue
				}
				splitPhiEdge(function, arg.Predecessor, block.ID)
			}
		}
	}
}

// splitPhiEdge preserves the identity of a predecessor's value on an edge.
// The caller normalizes/sorts Phi arguments after completing its CFG rewrite.
func splitPhiEdge(function *ir.Function, from, to int) int {
	id := len(function.Blocks)
	function.Blocks = append(function.Blocks, &ir.Block{ID: id, Terminator: ir.Jump{Target: to}})
	retargetEdge(function.Blocks[from], to, id)
	for i := range function.Blocks[to].Phis {
		for j := range function.Blocks[to].Phis[i].Args {
			arg := &function.Blocks[to].Phis[i].Args[j]
			if arg.Predecessor == from {
				arg.Predecessor = id
			}
		}
	}
	return id
}

func retargetEdge(block *ir.Block, from, to int) {
	switch term := block.Terminator.(type) {
	case ir.Jump:
		if term.Target == from {
			term.Target = to
		}
		block.Terminator = term
	case ir.Branch:
		if term.True == from {
			term.True = to
		}
		if term.False == from {
			term.False = to
		}
		block.Terminator = term
	case ir.Switch:
		if term.Default == from {
			term.Default = to
		}
		for i := range term.Cases {
			if term.Cases[i].Target == from {
				term.Cases[i].Target = to
			}
		}
		block.Terminator = term
	}
}
