package optimize

import (
	"sort"

	"github.com/WindowsSov8forUs/sonolus-core-go/core/resource"
	"github.com/WindowsSov8forUs/sonolus-go/internal/compiler/ir"
)

type CoalesceSmallConditionalBlocks struct{}

func (CoalesceSmallConditionalBlocks) Name() string { return "CoalesceSmallConditionalBlocks" }
func (CoalesceSmallConditionalBlocks) Run(c Context, f *ir.Function) error {
	for _, block := range f.Blocks {
		if len(block.Phis) != 0 {
			return nil
		}
		seen := map[int]bool{block.ID: true}
		for {
			jump, ok := block.Terminator.(ir.Jump)
			if !ok || seen[jump.Target] {
				break
			}
			target := f.Blocks[jump.Target]
			if len(target.Phis) != 0 || len(target.Instructions) > 1 {
				break
			}
			seen[target.ID] = true
			for _, instruction := range target.Instructions {
				block.Instructions = append(block.Instructions, cloneInstruction(instruction))
			}
			block.Terminator = cloneTerminator(target.Terminator)
		}
	}
	if err := (FoldConstantControl{}).Run(c, f); err != nil {
		return err
	}
	return normalizeReachable(f)
}

type NormalizeBlocks struct{}

func (NormalizeBlocks) Name() string                        { return "NormalizeBlocks" }
func (NormalizeBlocks) Run(_ Context, f *ir.Function) error { return normalizeReachable(f) }

type FlattenAssociativeOps struct{}

func (FlattenAssociativeOps) Name() string { return "FlattenAssociativeOps" }
func (FlattenAssociativeOps) Run(_ Context, f *ir.Function) error {
	rewriteFunctionExpressions(f, flattenExpr)
	return nil
}
func flattenExpr(e ir.Expr) ir.Expr {
	call, ok := e.(ir.RuntimeCall)
	if !ok || !flattenable(call.Function) {
		return e
	}
	if len(call.Args) == 0 {
		return call
	}
	if child, ok := call.Args[0].(ir.RuntimeCall); ok && child.Function == call.Function && child.Pure {
		call.Args = append(append([]ir.Expr(nil), child.Args...), call.Args[1:]...)
	}
	return call
}
func flattenable(op resource.RuntimeFunction) bool {
	switch op {
	case resource.RuntimeFunctionAdd, resource.RuntimeFunctionMultiply, resource.RuntimeFunctionMod, resource.RuntimeFunctionRem:
		return true
	}
	return false
}

type UnflattenAssociativeOps struct{}

func (UnflattenAssociativeOps) Name() string { return "UnflattenAssociativeOps" }
func (UnflattenAssociativeOps) Run(_ Context, f *ir.Function) error {
	rewriteFunctionExpressions(f, func(e ir.Expr) ir.Expr {
		call, ok := e.(ir.RuntimeCall)
		if !ok || !flattenable(call.Function) || len(call.Args) <= 2 {
			return e
		}
		result := ir.Expr(ir.RuntimeCall{Function: call.Function, Args: []ir.Expr{call.Args[0], call.Args[1]}, Result: call.Result, Pure: call.Pure, Pos: call.Pos})
		for _, a := range call.Args[2:] {
			result = ir.RuntimeCall{Function: call.Function, Args: []ir.Expr{result, a}, Result: call.Result, Pure: call.Pure, Pos: call.Pos}
		}
		return result
	})
	return nil
}

type RemoveRedundantArguments struct{}

func (RemoveRedundantArguments) Name() string { return "RemoveRedundantArguments" }
func (RemoveRedundantArguments) Run(_ Context, f *ir.Function) error {
	rewriteFunctionExpressions(f, simplifyAlgebra)
	return nil
}
func simplifyAlgebra(e ir.Expr) ir.Expr {
	call, ok := e.(ir.RuntimeCall)
	if !ok || !call.Pure {
		return e
	}
	is := func(e ir.Expr, v float64) bool { c, ok := e.(ir.Const); return ok && c.Value == v }
	filterTail := func(identity float64) []ir.Expr {
		if len(call.Args) == 0 {
			return nil
		}
		args := []ir.Expr{call.Args[0]}
		for _, arg := range call.Args[1:] {
			if !is(arg, identity) {
				args = append(args, arg)
			}
		}
		return args
	}
	switch call.Function {
	case resource.RuntimeFunctionAdd, resource.RuntimeFunctionMultiply:
		identity := float64(0)
		if call.Function == resource.RuntimeFunctionMultiply {
			identity = 1
		}
		args := call.Args[:0]
		for _, arg := range call.Args {
			if !is(arg, identity) {
				args = append(args, arg)
			}
		}
		call.Args = args
	case resource.RuntimeFunctionSubtract, resource.RuntimeFunctionDivide:
		identity := float64(0)
		if call.Function == resource.RuntimeFunctionDivide {
			identity = 1
		}
		call.Args = filterTail(identity)
	}
	if len(call.Args) == 1 {
		return call.Args[0]
	}
	if call.Function == resource.RuntimeFunctionSubtract && len(call.Args) == 2 && is(call.Args[0], 0) {
		return ir.RuntimeCall{Function: resource.RuntimeFunctionNegate, Args: []ir.Expr{call.Args[1]}, Result: call.Result, Pure: true, Pos: call.Pos}
	}
	return call
}

type RewriteToSwitch struct{}

func (RewriteToSwitch) Name() string { return "RewriteToSwitch" }
func (RewriteToSwitch) Run(_ Context, f *ir.Function) error {
	preds := predecessors(f)
	for _, block := range f.Blocks {
		branch, ok := block.Terminator.(ir.Branch)
		if !ok {
			continue
		}
		discriminant, constant, ok := equalityCase(branch.Condition)
		if !ok {
			continue
		}
		cases := []ir.SwitchCase{{Value: constant, Target: branch.True}}
		fallback := branch.False
		for fallback != block.ID && len(preds[fallback]) == 1 && len(f.Blocks[fallback].Phis) == 0 && len(f.Blocks[fallback].Instructions) == 0 {
			next, ok := f.Blocks[fallback].Terminator.(ir.Branch)
			if !ok {
				break
			}
			nextDiscriminant, nextConstant, ok := equalityCase(next.Condition)
			if !ok || exprKey(nextDiscriminant) != exprKey(discriminant) {
				break
			}
			duplicate := false
			for _, item := range cases {
				duplicate = duplicate || item.Value == nextConstant
			}
			if !duplicate {
				cases = append(cases, ir.SwitchCase{Value: nextConstant, Target: next.True})
			}
			fallback = next.False
		}
		if len(cases) > 1 {
			block.Terminator = ir.Switch{Value: discriminant, Cases: cases, Default: fallback}
		}
	}
	return normalizeReachable(f)
}

func equalityCase(expr ir.Expr) (ir.Expr, float64, bool) {
	call, ok := expr.(ir.RuntimeCall)
	if !ok || !call.Pure || call.Function != resource.RuntimeFunctionEqual || len(call.Args) != 2 {
		return nil, 0, false
	}
	if constant, ok := call.Args[0].(ir.Const); ok {
		return call.Args[1], constant.Value, true
	}
	if constant, ok := call.Args[1].(ir.Const); ok {
		return call.Args[0], constant.Value, true
	}
	return nil, 0, false
}

type NormalizeSwitch struct{}

func (NormalizeSwitch) Name() string { return "NormalizeSwitch" }
func (NormalizeSwitch) Run(_ Context, f *ir.Function) error {
	for _, b := range f.Blocks {
		if s, ok := b.Terminator.(ir.Switch); ok {
			seen := map[float64]bool{}
			cases := s.Cases[:0]
			for _, c := range s.Cases {
				if !seen[c.Value] {
					seen[c.Value] = true
					cases = append(cases, c)
				}
			}
			s.Cases = cases
			if len(cases) >= 2 {
				sorted := append([]ir.SwitchCase(nil), cases...)
				sort.Slice(sorted, func(i, j int) bool { return sorted[i].Value < sorted[j].Value })
				offset, stride := sorted[0].Value, sorted[1].Value-sorted[0].Value
				valid := offset == float64(int64(offset)) && stride != 0 && stride == float64(int64(stride))
				for i, item := range sorted {
					valid = valid && item.Value == offset+float64(i)*stride
				}
				if valid && (offset != 0 || stride != 1) {
					number := ir.Type{Name: "number", Slots: 1}
					value := s.Value
					if offset != 0 {
						value = ir.RuntimeCall{Function: resource.RuntimeFunctionSubtract, Args: []ir.Expr{value, ir.Const{Value: offset}}, Result: number, Pure: true}
					}
					if stride != 1 {
						value = ir.RuntimeCall{Function: resource.RuntimeFunctionDivide, Args: []ir.Expr{value, ir.Const{Value: stride}}, Result: number, Pure: true}
					}
					s.Value = value
					for i := range sorted {
						sorted[i].Value = float64(i)
					}
					s.Cases = sorted
				}
			}
			b.Terminator = s
		}
	}
	return nil
}

type CombineExitBlocks struct{}

func (CombineExitBlocks) Name() string { return "CombineExitBlocks" }
func (CombineExitBlocks) Run(c Context, f *ir.Function) error {
	first := -1
	for _, block := range f.Blocks {
		ret, ok := block.Terminator.(ir.Return)
		if !ok || len(block.Phis) != 0 || len(block.Instructions) != 0 || len(ret.Value.Slots) != 0 {
			continue
		}
		if first < 0 {
			first = block.ID
		} else {
			replaceTarget(f, block.ID, first)
		}
	}
	if err := normalizeReachable(f); err != nil {
		return err
	}
	return (CoalesceFlow{}).Run(c, f)
}

type CopyCoalesce struct{}

func (CopyCoalesce) Name() string { return "CopyCoalesce" }
func (CopyCoalesce) Run(_ Context, f *ir.Function) error {
	interference := localInterference(f)
	parent := make([]int, len(f.Locals))
	for i := range parent {
		parent[i] = i
	}
	var find func(int) int
	find = func(id int) int {
		if parent[id] != id {
			parent[id] = find(parent[id])
		}
		return parent[id]
	}
	for _, block := range f.Blocks {
		for _, instruction := range block.Instructions {
			store, ok := instruction.(ir.Store)
			target, targetOK := store.Place.(ir.LocalPlace)
			load, loadOK := store.Value.(ir.Load)
			source, sourceOK := load.Place.(ir.LocalPlace)
			if !ok || !targetOK || !loadOK || !sourceOK || target.Offset != 0 || source.Offset != 0 || f.Locals[target.ID].Slots != 1 || f.Locals[source.ID].Slots != 1 {
				continue
			}
			a, b := find(target.ID), find(source.ID)
			if a != b && !interference[a][b] {
				if b < a {
					a, b = b, a
				}
				parent[b] = a
				for other := range interference[b] {
					if other != a {
						interference[a][other] = true
						interference[other][a] = true
					}
				}
			}
		}
	}
	representatives := map[int]int{}
	locals := []ir.Type{}
	for id := range parent {
		rep := find(id)
		if _, ok := representatives[rep]; !ok {
			representatives[rep] = len(locals)
			locals = append(locals, f.Locals[rep])
		}
	}
	remap := func(place ir.Place) ir.Place {
		switch value := place.(type) {
		case ir.LocalPlace:
			value.ID = representatives[find(value.ID)]
			return value
		case ir.IndexedLocalPlace:
			value.ID = representatives[find(value.ID)]
			return value
		default:
			return place
		}
	}
	for _, block := range f.Blocks {
		for i, instruction := range block.Instructions {
			if store, ok := instruction.(ir.Store); ok {
				store.Place = remap(store.Place)
				block.Instructions[i] = store
			}
		}
	}
	rewriteFunctionExpressions(f, func(expr ir.Expr) ir.Expr {
		if load, ok := expr.(ir.Load); ok {
			load.Place = remap(load.Place)
			return load
		}
		return expr
	})
	f.Locals = locals
	return (RemoveNoOps{}).Run(Context{}, f)
}

type LoopInvariantCodeMotion struct{}

func (LoopInvariantCodeMotion) Name() string          { return "LoopInvariantCodeMotion" }
func (LoopInvariantCodeMotion) Requires() []Analysis  { return []Analysis{AnalysisDominance} }
func (LoopInvariantCodeMotion) Preserves() []Analysis { return []Analysis{AnalysisDominance} }
func (LoopInvariantCodeMotion) Destroys() []Analysis  { return []Analysis{AnalysisLiveness} }
func (LoopInvariantCodeMotion) Run(context Context, f *ir.Function) error {
	dom := dominanceFor(context, f)
	preds := predecessors(f)
	for latch, block := range f.Blocks {
		for _, header := range terminatorTargets(block.Terminator) {
			if !dominates(dom, header, latch) {
				continue
			}
			loop := naturalLoop(preds, header, latch)
			outside := make([]int, 0, len(preds[header]))
			for _, predecessor := range preds[header] {
				if !loop[predecessor] {
					outside = append(outside, predecessor)
				}
			}
			if len(outside) != 1 {
				continue
			}
			preheader := f.Blocks[outside[0]]
			jump, ok := preheader.Terminator.(ir.Jump)
			if !ok || jump.Target != header {
				continue
			}
			defs := map[int]bool{}
			loopIDs := make([]int, 0, len(loop))
			for id := range loop {
				loopIDs = append(loopIDs, id)
			}
			sort.Ints(loopIDs)
			for _, id := range loopIDs {
				for _, instruction := range f.Blocks[id].Instructions {
					if store, ok := instruction.(ir.Store); ok {
						if place, ok := store.Place.(ir.SSAPlace); ok {
							defs[place.ID] = true
						}
					}
				}
			}
			invariant := map[int]bool{}
			for changed := true; changed; {
				changed = false
				for _, id := range loopIDs {
					instructions := f.Blocks[id].Instructions
					kept := instructions[:0]
					for _, instruction := range instructions {
						store, ok := instruction.(ir.Store)
						place, isSSA := store.Place.(ir.SSAPlace)
						if !ok || !isSSA || invariant[place.ID] || !loopInvariantExpr(store.Value, defs, invariant) {
							kept = append(kept, instruction)
							continue
						}
						preheader.Instructions = append(preheader.Instructions, store)
						invariant[place.ID] = true
						changed = true
					}
					f.Blocks[id].Instructions = kept
				}
			}
		}
	}
	return nil
}

func dominates(dom *Dominance, ancestor, block int) bool {
	for block >= 0 {
		if block == ancestor {
			return true
		}
		if dom.IDom[block] == block {
			break
		}
		block = dom.IDom[block]
	}
	return false
}

func naturalLoop(preds [][]int, header, latch int) map[int]bool {
	result := map[int]bool{header: true, latch: true}
	stack := []int{latch}
	for len(stack) > 0 {
		id := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		for _, predecessor := range preds[id] {
			if !result[predecessor] {
				result[predecessor] = true
				stack = append(stack, predecessor)
			}
		}
	}
	return result
}

func loopInvariantExpr(expr ir.Expr, defs, invariant map[int]bool) bool {
	switch value := expr.(type) {
	case ir.Const:
		return true
	case ir.Load:
		place, ok := value.Place.(ir.SSAPlace)
		return ok && (!defs[place.ID] || invariant[place.ID])
	case ir.RuntimeCall:
		if !value.Pure {
			return false
		}
		for _, arg := range value.Args {
			if !loopInvariantExpr(arg, defs, invariant) {
				return false
			}
		}
		return true
	default:
		return false
	}
}
