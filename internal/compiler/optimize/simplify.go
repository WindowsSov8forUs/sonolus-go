package optimize

import (
	"sort"

	"github.com/WindowsSov8forUs/sonolus-core-go/core/resource"
	"github.com/WindowsSov8forUs/sonolus-go/v2/internal/compiler/catalog"
	"github.com/WindowsSov8forUs/sonolus-go/v2/internal/compiler/ir"
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
	rewriteFunctionExpressionsChanged(f, flattenExprChanged)
	return nil
}
func flattenExprChanged(e ir.Expr) (ir.Expr, bool) {
	call, ok := e.(ir.RuntimeCall)
	if !ok || !flattenable(call.Function) {
		return e, false
	}
	if len(call.Args) == 0 {
		return call, false
	}
	if child, ok := call.Args[0].(ir.RuntimeCall); ok && child.Function == call.Function && child.Pure {
		call.Args = append(append([]ir.Expr(nil), child.Args...), call.Args[1:]...)
		return call, true
	}
	return call, false
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
	rewriteFunctionExpressionsChanged(f, func(e ir.Expr) (ir.Expr, bool) {
		call, ok := e.(ir.RuntimeCall)
		if !ok || !flattenable(call.Function) || len(call.Args) <= 2 {
			return e, false
		}
		result := ir.Expr(ir.RuntimeCall{Function: call.Function, Args: []ir.Expr{call.Args[0], call.Args[1]}, Result: call.Result, Pure: call.Pure, Pos: call.Pos})
		for _, a := range call.Args[2:] {
			result = ir.RuntimeCall{Function: call.Function, Args: []ir.Expr{result, a}, Result: call.Result, Pure: call.Pure, Pos: call.Pos}
		}
		return result, true
	})
	return nil
}

type RemoveRedundantArguments struct{}

func (RemoveRedundantArguments) Name() string { return "RemoveRedundantArguments" }
func (RemoveRedundantArguments) Run(_ Context, f *ir.Function) error {
	rewriteFunctionExpressionsChanged(f, simplifyAlgebraChanged)
	return nil
}
func simplifyAlgebraChanged(e ir.Expr) (ir.Expr, bool) {
	call, ok := e.(ir.RuntimeCall)
	if !ok || !call.Pure {
		return e, false
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
	changed := false
	identityOperation := false
	switch call.Function {
	case resource.RuntimeFunctionAdd, resource.RuntimeFunctionMultiply:
		identityOperation = true
		identity := float64(0)
		if call.Function == resource.RuntimeFunctionMultiply {
			identity = 1
		}
		args := make([]ir.Expr, 0, len(call.Args))
		for _, arg := range call.Args {
			if !is(arg, identity) {
				args = append(args, arg)
			} else {
				changed = true
			}
		}
		if changed {
			call.Args = args
		}
	case resource.RuntimeFunctionSubtract, resource.RuntimeFunctionDivide:
		identityOperation = true
		identity := float64(0)
		if call.Function == resource.RuntimeFunctionDivide {
			identity = 1
		}
		args := filterTail(identity)
		if len(args) != len(call.Args) {
			call.Args = args
			changed = true
		}
	}
	if identityOperation && len(call.Args) == 1 {
		return call.Args[0], true
	}
	if call.Function == resource.RuntimeFunctionSubtract && len(call.Args) == 2 && is(call.Args[0], 0) {
		return ir.RuntimeCall{Function: resource.RuntimeFunctionNegate, Args: []ir.Expr{call.Args[1]}, Result: call.Result, Pure: true, Pos: call.Pos}, true
	}
	return call, changed
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

func (NormalizeSwitch) Requires() []Analysis  { return nil }
func (NormalizeSwitch) Preserves() []Analysis { return []Analysis{AnalysisLiveness} }
func (NormalizeSwitch) Destroys() []Analysis  { return nil }

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

func (CombineExitBlocks) Requires() []Analysis  { return nil }
func (CombineExitBlocks) Preserves() []Analysis { return []Analysis{AnalysisLiveness} }
func (CombineExitBlocks) Destroys() []Analysis  { return nil }

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

func (CopyCoalesce) Name() string          { return "CopyCoalesce" }
func (CopyCoalesce) Requires() []Analysis  { return []Analysis{AnalysisLiveness} }
func (CopyCoalesce) Preserves() []Analysis { return []Analysis{AnalysisLiveness} }
func (CopyCoalesce) Destroys() []Analysis  { return nil }
func (CopyCoalesce) Run(context Context, f *ir.Function) error {
	interference := interferenceFor(context, f).clone()
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
			if a != b && !interference[a].has(b) {
				if b < a {
					a, b = b, a
				}
				parent[b] = a
				interference[b].each(func(other int) {
					if other != a {
						addInterference(interference, a, other)
					}
				})
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
	rewriteFunctionExpressionsChanged(f, func(expr ir.Expr) (ir.Expr, bool) {
		if load, ok := expr.(ir.Load); ok {
			load.Place = remap(load.Place)
			return load, true
		}
		return expr, false
	})
	f.Locals = locals
	coalesced := newInterferenceGraph(len(locals))
	for oldID := range interference {
		left := representatives[find(oldID)]
		interference[oldID].each(func(other int) {
			right := representatives[find(other)]
			if left != right {
				addInterference(coalesced, left, right)
			}
		})
	}
	if context.analyses != nil {
		context.analyses.values[AnalysisLiveness] = coalesced
	}
	return (RemoveNoOps{}).Run(Context{}, f)
}

type LoopInvariantCodeMotion struct{}

func (LoopInvariantCodeMotion) Name() string          { return "LoopInvariantCodeMotion" }
func (LoopInvariantCodeMotion) Requires() []Analysis  { return []Analysis{AnalysisDominance} }
func (LoopInvariantCodeMotion) Preserves() []Analysis { return nil }
func (LoopInvariantCodeMotion) Destroys() []Analysis {
	return []Analysis{AnalysisDominance, AnalysisLiveness}
}
func (LoopInvariantCodeMotion) Run(context Context, f *ir.Function) error {
	dom := dominanceFor(context, f)
	preds := predecessors(f)
	latchesByHeader := map[int][]int{}
	for latch, block := range f.Blocks {
		forEachTerminatorTarget(block.Terminator, func(header int) {
			if dominates(dom, header, latch) {
				latchesByHeader[header] = append(latchesByHeader[header], latch)
			}
		})
	}
	headers := make([]int, 0, len(latchesByHeader))
	for header := range latchesByHeader {
		headers = append(headers, header)
	}
	sort.Ints(headers)
	for _, header := range headers {
		latches := latchesByHeader[header]
		loop := map[int]bool{header: true}
		for _, latch := range latches {
			for id := range naturalLoop(preds, header, latch) {
				loop[id] = true
			}
		}
		outside := make([]int, 0, len(preds[header]))
		for _, predecessor := range preds[header] {
			if !loop[predecessor] {
				outside = append(outside, predecessor)
			}
		}
		preheader := licmPreheader(f, header, loop, outside)
		if preheader == nil {
			continue
		}
		defs := map[int]bool{}
		loopIDs := make([]int, 0, len(loop))
		for id := range loop {
			loopIDs = append(loopIDs, id)
		}
		sort.Ints(loopIDs)
		for _, id := range loopIDs {
			for _, phi := range f.Blocks[id].Phis {
				defs[phi.Target.ID] = true
			}
			for _, instruction := range f.Blocks[id].Instructions {
				if store, ok := instruction.(ir.Store); ok {
					if place, ok := store.Place.(ir.SSAPlace); ok {
						defs[place.ID] = true
					}
				}
			}
		}
		nextSSA := maxSSAID(f) + 1
		hoisted := map[string]ir.SSAPlace{}
		var rewrite func(ir.Expr) ir.Expr
		rewrite = func(expression ir.Expr) ir.Expr {
			if loopInvariantExpr(context, expression, defs, nil) && expressionCost(expression) >= 4 {
				key := exprKey(expression)
				place, exists := hoisted[key]
				if !exists {
					place = ir.SSAPlace{ID: nextSSA, Name: "licm"}
					nextSSA++
					hoisted[key] = place
					preheader.Instructions = append(preheader.Instructions, ir.Store{Place: place, Value: cloneExpr(expression)})
				}
				return ir.Load{Place: place}
			}
			switch value := expression.(type) {
			case ir.Load:
				value.Place = rewritePlace(value.Place, rewrite)
				return value
			case ir.RuntimeCall:
				value.Args = append([]ir.Expr(nil), value.Args...)
				for index := range value.Args {
					value.Args[index] = rewrite(value.Args[index])
				}
				return value
			default:
				return expression
			}
		}
		for _, id := range loopIDs {
			dominatesEveryLatch := true
			for _, latch := range latches {
				if !dominates(dom, id, latch) {
					dominatesEveryLatch = false
					break
				}
			}
			if !dominatesEveryLatch {
				continue
			}
			block := f.Blocks[id]
			for index, instruction := range block.Instructions {
				switch value := instruction.(type) {
				case ir.Store:
					value.Place = rewritePlace(value.Place, rewrite)
					value.Value = rewrite(value.Value)
					block.Instructions[index] = value
				case ir.Eval:
					value.Value = rewrite(value.Value)
					block.Instructions[index] = value
				}
			}
			switch value := block.Terminator.(type) {
			case ir.Branch:
				value.Condition = rewrite(value.Condition)
				block.Terminator = value
			case ir.Switch:
				value.Value = rewrite(value.Value)
				block.Terminator = value
			case ir.Return:
				for index := range value.Value.Slots {
					value.Value.Slots[index] = rewrite(value.Value.Slots[index])
				}
				block.Terminator = value
			}
		}
	}
	return nil
}

func licmPreheader(function *ir.Function, header int, loop map[int]bool, outside []int) *ir.Block {
	if len(outside) == 0 {
		return nil
	}
	sort.Ints(outside)
	if len(outside) == 1 {
		candidate := function.Blocks[outside[0]]
		jump, ok := candidate.Terminator.(ir.Jump)
		if ok && jump.Target == header && len(candidate.Phis) == 0 {
			return candidate
		}
	}

	id := len(function.Blocks)
	preheader := &ir.Block{ID: id, Terminator: ir.Jump{Target: header}}
	function.Blocks = append(function.Blocks, preheader)
	for _, predecessor := range outside {
		retargetEdge(function.Blocks[predecessor], header, id)
	}

	nextSSA := maxSSAID(function) + 1
	headerBlock := function.Blocks[header]
	for phiIndex := range headerBlock.Phis {
		phi := &headerBlock.Phis[phiIndex]
		outsideArgs := make([]ir.PhiArg, 0, len(outside))
		loopArgs := phi.Args[:0]
		for _, argument := range phi.Args {
			if loop[argument.Predecessor] {
				loopArgs = append(loopArgs, argument)
			} else {
				outsideArgs = append(outsideArgs, argument)
			}
		}
		if len(outsideArgs) == 0 {
			phi.Args = loopArgs
			continue
		}
		value := outsideArgs[0].Value
		if len(outsideArgs) > 1 {
			value = ir.SSAPlace{ID: nextSSA, Name: "licm.preheader"}
			nextSSA++
			preheader.Phis = append(preheader.Phis, ir.Phi{Target: value, Local: phi.Local, Args: outsideArgs})
		}
		phi.Args = append(loopArgs, ir.PhiArg{Predecessor: id, Value: value})
		sort.Slice(phi.Args, func(left, right int) bool {
			return phi.Args[left].Predecessor < phi.Args[right].Predecessor
		})
	}
	return preheader
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

func loopInvariantExpr(context Context, expr ir.Expr, defs, invariant map[int]bool) bool {
	switch value := expr.(type) {
	case ir.Const:
		return true
	case ir.Load:
		switch place := value.Place.(type) {
		case ir.SSAPlace:
			return !defs[place.ID] || invariant[place.ID]
		case ir.MemoryPlace:
			return place.Read && !place.Write && catalog.MemoryReadonly(context.Mode, context.Callback, place.Storage) && loopInvariantExpr(context, place.Index, defs, invariant)
		default:
			return false
		}
	case ir.RuntimeCall:
		if !value.Pure {
			return false
		}
		for _, arg := range value.Args {
			if !loopInvariantExpr(context, arg, defs, invariant) {
				return false
			}
		}
		return true
	default:
		return false
	}
}
