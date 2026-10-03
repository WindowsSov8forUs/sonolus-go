package optimize

import (
	"math"

	"github.com/WindowsSov8forUs/sonolus-core-go/core/resource"
	"github.com/WindowsSov8forUs/sonolus-go/v2/internal/compiler/catalog"
	"github.com/WindowsSov8forUs/sonolus-go/v2/internal/compiler/ir"
)

type DeadCodeElimination struct{}

func (DeadCodeElimination) Name() string { return "DeadCodeElimination" }
func (DeadCodeElimination) Run(_ Context, function *ir.Function) error {
	indexed := indexedLocalIDs(function)
	dependencies := map[temporaryPlaceKey][]temporaryPlaceKey{}
	live := map[temporaryPlaceKey]bool{}
	var queue []temporaryPlaceKey
	mark := func(key temporaryPlaceKey) {
		if !live[key] {
			live[key] = true
			queue = append(queue, key)
		}
	}
	collect := func(expr ir.Expr, visit func(temporaryPlaceKey)) {
		walkExpr(expr, func(value ir.Expr) {
			if load, ok := value.(ir.Load); ok {
				if key, valid := placeKey(load.Place); valid {
					visit(key)
				}
			}
		})
	}
	root := func(expr ir.Expr) { collect(expr, mark) }
	for _, block := range function.Blocks {
		for _, phi := range block.Phis {
			key, _ := placeKey(phi.Target)
			for _, arg := range phi.Args {
				input, _ := placeKey(arg.Value)
				dependencies[key] = append(dependencies[key], input)
			}
		}
		for _, instruction := range block.Instructions {
			switch value := instruction.(type) {
			case ir.Store:
				key, temporary := placeKey(value.Place)
				if temporary {
					collect(value.Value, func(input temporaryPlaceKey) { dependencies[key] = append(dependencies[key], input) })
					if expressionHasEffects(value.Value) {
						mark(key)
					}
					if place, local := value.Place.(ir.LocalPlace); local && indexed[place.ID] {
						mark(key)
					}
				} else {
					root(value.Value)
				}
				addPlaceExpressions(value.Place, root)
			case ir.Eval:
				root(value.Value)
			}
		}
		visitTerminator(block.Terminator, root)
	}
	// Trace definitions from observable roots instead of repeatedly counting
	// every syntactic use. Unobserved Phi cycles must not keep themselves alive.
	for len(queue) != 0 {
		key := queue[len(queue)-1]
		queue = queue[:len(queue)-1]
		for _, input := range dependencies[key] {
			mark(input)
		}
	}
	for _, block := range function.Blocks {
		phis := block.Phis[:0]
		for _, phi := range block.Phis {
			key, _ := placeKey(phi.Target)
			if live[key] {
				phis = append(phis, phi)
			}
		}
		block.Phis = phis
		instructions := block.Instructions[:0]
		for _, instruction := range block.Instructions {
			if store, ok := instruction.(ir.Store); ok {
				if key, temporary := placeKey(store.Place); temporary && !live[key] {
					continue
				}
			}
			instructions = append(instructions, instruction)
		}
		block.Instructions = instructions
	}
	return nil
}

type AdvancedDeadCodeElimination struct{}

func (AdvancedDeadCodeElimination) Requires() []Analysis  { return nil }
func (AdvancedDeadCodeElimination) Preserves() []Analysis { return nil }
func (AdvancedDeadCodeElimination) Destroys() []Analysis  { return []Analysis{AnalysisLiveness} }

func (AdvancedDeadCodeElimination) Name() string { return "AdvancedDeadCodeElimination" }
func (AdvancedDeadCodeElimination) Run(_ Context, f *ir.Function) error {
	indexed := indexedLocalIDs(f)
	ids := map[temporaryPlaceKey]int{}
	intern := func(key temporaryPlaceKey) {
		if _, exists := ids[key]; !exists {
			ids[key] = len(ids)
		}
	}
	for i := range f.Blocks {
		add := func(expr ir.Expr) {
			walkExpr(expr, func(value ir.Expr) {
				if load, ok := value.(ir.Load); ok {
					key, valid := placeKey(load.Place)
					if valid {
						intern(key)
					}
				}
			})
		}
		for _, instruction := range f.Blocks[i].Instructions {
			switch value := instruction.(type) {
			case ir.Store:
				add(value.Value)
				addPlaceExpressions(value.Place, add)
				if key, valid := placeKey(value.Place); valid {
					intern(key)
				}
			case ir.Eval:
				add(value.Value)
			}
		}
		visitTerminator(f.Blocks[i].Terminator, add)
	}
	// Propagate demand through retained instructions only. Ordinary liveness
	// counts reads in dead stores, keeping entire dead chains (and cycles)
	// alive across block boundaries even after their final consumer disappears.
	type transfer struct {
		target    int
		removable bool
		uses      bitSet
	}
	transfers := make([][]transfer, len(f.Blocks))
	terminalUses := make([]bitSet, len(f.Blocks))
	collect := func(expr ir.Expr, set bitSet) {
		walkExpr(expr, func(value ir.Expr) {
			if load, ok := value.(ir.Load); ok {
				if key, valid := placeKey(load.Place); valid {
					set.set(ids[key])
				}
			}
		})
	}
	for i, block := range f.Blocks {
		terminalUses[i] = newBitSet(len(ids))
		visitTerminator(block.Terminator, func(expr ir.Expr) { collect(expr, terminalUses[i]) })
		for _, instruction := range block.Instructions {
			item := transfer{target: -1, uses: newBitSet(len(ids))}
			switch value := instruction.(type) {
			case ir.Store:
				if key, valid := placeKey(value.Place); valid {
					item.target = ids[key]
					item.removable = !expressionHasEffects(value.Value)
					if local, ok := value.Place.(ir.LocalPlace); ok && indexed[local.ID] {
						item.removable = false
					}
				}
				collect(value.Value, item.uses)
				addPlaceExpressions(value.Place, func(expr ir.Expr) { collect(expr, item.uses) })
			case ir.Eval:
				collect(value.Value, item.uses)
			}
			transfers[i] = append(transfers[i], item)
		}
	}
	liveIn, liveOut := make([]bitSet, len(f.Blocks)), make([]bitSet, len(f.Blocks))
	queued := make([]bool, len(f.Blocks))
	queue := make([]int, 0, len(f.Blocks))
	for i := range f.Blocks {
		liveIn[i], liveOut[i] = newBitSet(len(ids)), newBitSet(len(ids))
		queue = append(queue, i)
		queued[i] = true
	}
	preds := predecessors(f)
	working := newBitSet(len(ids))
	for len(queue) != 0 {
		id := queue[len(queue)-1]
		queue = queue[:len(queue)-1]
		queued[id] = false
		forEachTerminatorTarget(f.Blocks[id].Terminator, func(next int) { liveOut[id].union(liveIn[next]) })
		copy(working, liveOut[id])
		working.union(terminalUses[id])
		for i := len(transfers[id]) - 1; i >= 0; i-- {
			item := transfers[id][i]
			if item.removable && !working.has(item.target) {
				continue
			}
			if item.target >= 0 {
				working.clear(item.target)
			}
			working.union(item.uses)
		}
		if !working.equal(liveIn[id]) {
			copy(liveIn[id], working)
			for _, previous := range preds[id] {
				if !queued[previous] {
					queued[previous] = true
					queue = append(queue, previous)
				}
			}
		}
	}
	live := newBitSet(len(ids))
	addLive := func(expr ir.Expr) {
		walkExpr(expr, func(value ir.Expr) {
			if load, ok := value.(ir.Load); ok {
				if key, valid := placeKey(load.Place); valid {
					live.set(ids[key])
				}
			}
		})
	}
	for _, block := range f.Blocks {
		copy(live, liveOut[block.ID])
		visitTerminator(block.Terminator, addLive)
		kept := make([]ir.Instruction, 0, len(block.Instructions))
		for i := len(block.Instructions) - 1; i >= 0; i-- {
			instruction := block.Instructions[i]
			if store, ok := instruction.(ir.Store); ok {
				key, temporary := placeKey(store.Place)
				addressTaken := false
				if place, local := store.Place.(ir.LocalPlace); local {
					addressTaken = indexed[place.ID]
				}
				if temporary && !addressTaken && !live.has(ids[key]) && !expressionHasEffects(store.Value) {
					continue
				}
				if temporary {
					live.clear(ids[key])
				}
				addLive(store.Value)
				addPlaceExpressions(store.Place, addLive)
			} else if eval, ok := instruction.(ir.Eval); ok {
				addLive(eval.Value)
			}
			kept = append(kept, instruction)
		}
		for left, right := 0, len(kept)-1; left < right; left, right = left+1, right-1 {
			kept[left], kept[right] = kept[right], kept[left]
		}
		block.Instructions = kept
	}
	return nil
}

func indexedLocalIDs(function *ir.Function) map[int]bool {
	result := map[int]bool{}
	visit := func(expr ir.Expr) {
		walkExpr(expr, func(value ir.Expr) {
			if load, ok := value.(ir.Load); ok {
				if place, indexed := load.Place.(ir.IndexedLocalPlace); indexed {
					result[place.ID] = true
				}
			}
		})
	}
	for _, block := range function.Blocks {
		for _, instruction := range block.Instructions {
			switch value := instruction.(type) {
			case ir.Store:
				if place, indexed := value.Place.(ir.IndexedLocalPlace); indexed {
					result[place.ID] = true
				}
				addPlaceExpressions(value.Place, visit)
				visit(value.Value)
			case ir.Eval:
				visit(value.Value)
			}
		}
		visitTerminator(block.Terminator, visit)
	}
	return result
}

func addPlaceExpressions(place ir.Place, fn func(ir.Expr)) {
	switch value := place.(type) {
	case ir.IndexedLocalPlace:
		fn(value.Index)
	case ir.MemoryPlace:
		fn(value.Index)
	}
}

func isTemporaryPlace(p ir.Place) bool {
	switch p.(type) {
	case ir.LocalPlace, ir.SSAPlace:
		return true
	}
	return false
}

type temporaryPlaceKey uint64

func placeKey(p ir.Place) (temporaryPlaceKey, bool) {
	switch v := p.(type) {
	case ir.LocalPlace:
		return temporaryPlaceKey(uint64(uint32(v.ID))<<32 | uint64(uint32(v.Offset))), true
	case ir.SSAPlace:
		return temporaryPlaceKey(uint64(1)<<63 | uint64(uint32(v.ID))), true
	}
	return 0, false
}
func itoa(v int) string {
	if v == 0 {
		return "0"
	}
	neg := v < 0
	if neg {
		v = -v
	}
	var b [24]byte
	i := len(b)
	for v > 0 {
		i--
		b[i] = byte('0' + v%10)
		v /= 10
	}
	if neg {
		i--
		b[i] = '-'
	}
	return string(b[i:])
}
func walkExpr(expr ir.Expr, fn func(ir.Expr)) {
	fn(expr)
	switch v := expr.(type) {
	case ir.Load:
		switch p := v.Place.(type) {
		case ir.IndexedLocalPlace:
			walkExpr(p.Index, fn)
		case ir.MemoryPlace:
			walkExpr(p.Index, fn)
		}
	case ir.RuntimeCall:
		for _, a := range v.Args {
			walkExpr(a, fn)
		}
	}
}
func visitTerminator(t ir.Terminator, fn func(ir.Expr)) {
	switch v := t.(type) {
	case ir.Branch:
		fn(v.Condition)
	case ir.Switch:
		fn(v.Value)
	case ir.Return:
		for _, e := range v.Value.Slots {
			fn(e)
		}
	}
}

type SparseConditionalConstantPropagation struct{}

func (SparseConditionalConstantPropagation) Name() string {
	return "SparseConditionalConstantPropagation"
}
func (SparseConditionalConstantPropagation) Run(_ Context, function *ir.Function) error {
	states := map[int]constantState{}
	reachable := map[int]bool{function.Entry: true}
	for {
		changed := false
		for _, block := range function.Blocks {
			if !reachable[block.ID] {
				continue
			}
			for _, phi := range block.Phis {
				state := constantState{}
				for _, arg := range phi.Args {
					if edgeExecutable(function.Blocks[arg.Predecessor], block.ID, states, reachable) {
						state = joinConstantStates(state, states[arg.Value.ID])
					}
				}
				if updateConstantState(states, phi.Target.ID, state) {
					changed = true
				}
			}
			for _, instruction := range block.Instructions {
				if store, ok := instruction.(ir.Store); ok {
					if place, ok := store.Place.(ir.SSAPlace); ok && updateConstantState(states, place.ID, evaluateConstantState(store.Value, states)) {
						changed = true
					}
				}
			}
			forEachExecutableTarget(block.Terminator, states, func(successor int) {
				if !reachable[successor] {
					reachable[successor] = true
					changed = true
				}
			})
		}
		if changed {
			continue
		}
		promoted := false
		for id := range referencedSSA(function) {
			if states[id].kind == constantUnknown {
				states[id] = constantState{kind: constantOverdefined}
				promoted = true
			}
		}
		if !promoted {
			break
		}
	}
	constants := map[int]ir.Const{}
	for id, state := range states {
		if state.kind == constantValue {
			constants[id] = ir.Const{Value: state.value}
		}
	}
	rewriteFunctionExpressionsChanged(function, func(expr ir.Expr) (ir.Expr, bool) {
		folded := foldExpr(expr, constants)
		switch value := expr.(type) {
		case ir.Load:
			if place, ok := value.Place.(ir.SSAPlace); ok {
				_, changed := constants[place.ID]
				return folded, changed
			}
		case ir.RuntimeCall:
			_, changed := folded.(ir.Const)
			return folded, changed
		}
		return folded, false
	})
	return (FoldConstantControl{}).Run(Context{}, function)
}

type constantKind uint8

const (
	constantUnknown constantKind = iota
	constantValue
	constantOverdefined
)

type constantState struct {
	kind  constantKind
	value float64
}

func evaluateConstantState(expr ir.Expr, states map[int]constantState) constantState {
	switch value := expr.(type) {
	case ir.Const:
		return constantState{kind: constantValue, value: value.Value}
	case ir.Load:
		if place, ok := value.Place.(ir.SSAPlace); ok {
			return states[place.ID]
		}
		return constantState{kind: constantOverdefined}
	case ir.RuntimeCall:
		if !value.Pure {
			return constantState{kind: constantOverdefined}
		}
		args := make([]float64, len(value.Args))
		for i, arg := range value.Args {
			state := evaluateConstantState(arg, states)
			if state.kind == constantOverdefined {
				return state
			}
			if state.kind == constantUnknown {
				return state
			}
			args[i] = state.value
		}
		if result, ok := evaluateRuntime(value.Function, args); ok {
			return constantState{kind: constantValue, value: result}
		}
		return constantState{kind: constantOverdefined}
	default:
		return constantState{kind: constantOverdefined}
	}
}

func joinConstantStates(a, b constantState) constantState {
	if a.kind == constantUnknown {
		return b
	}
	if b.kind == constantUnknown {
		return a
	}
	if a.kind == constantOverdefined || b.kind == constantOverdefined || math.Float64bits(a.value) != math.Float64bits(b.value) {
		return constantState{kind: constantOverdefined}
	}
	return a
}

func updateConstantState(states map[int]constantState, id int, next constantState) bool {
	current := states[id]
	joined := joinConstantStates(current, next)
	if current.kind == joined.kind && (current.kind != constantValue || math.Float64bits(current.value) == math.Float64bits(joined.value)) {
		return false
	}
	states[id] = joined
	return true
}

func forEachExecutableTarget(terminator ir.Terminator, states map[int]constantState, visit func(int)) {
	switch value := terminator.(type) {
	case ir.Jump:
		visit(value.Target)
	case ir.Branch:
		state := evaluateConstantState(value.Condition, states)
		if state.kind == constantUnknown {
			return
		}
		if state.kind == constantValue {
			if state.value != 0 {
				visit(value.True)
				return
			}
			visit(value.False)
			return
		}
		visit(value.True)
		if value.False != value.True {
			visit(value.False)
		}
	case ir.Switch:
		state := evaluateConstantState(value.Value, states)
		if state.kind == constantUnknown {
			return
		}
		if state.kind == constantValue {
			for _, item := range value.Cases {
				if item.Value == state.value {
					visit(item.Target)
					return
				}
			}
			visit(value.Default)
			return
		}
		forEachTerminatorTarget(terminator, visit)
	}
}

func edgeExecutable(predecessor *ir.Block, successor int, states map[int]constantState, reachable map[int]bool) bool {
	if predecessor == nil || !reachable[predecessor.ID] {
		return false
	}
	found := false
	forEachExecutableTarget(predecessor.Terminator, states, func(target int) {
		if target == successor {
			found = true
		}
	})
	return found
}

func referencedSSA(function *ir.Function) map[int]bool {
	result := map[int]bool{}
	for _, block := range function.Blocks {
		for _, phi := range block.Phis {
			result[phi.Target.ID] = true
			for _, arg := range phi.Args {
				result[arg.Value.ID] = true
			}
		}
		for _, instruction := range block.Instructions {
			switch value := instruction.(type) {
			case ir.Store:
				if place, ok := value.Place.(ir.SSAPlace); ok {
					result[place.ID] = true
				}
				collectSSA(value.Value, result)
			case ir.Eval:
				collectSSA(value.Value, result)
			}
		}
		visitTerminator(block.Terminator, func(expr ir.Expr) { collectSSA(expr, result) })
	}
	return result
}

func collectSSA(expr ir.Expr, result map[int]bool) {
	walkExpr(expr, func(value ir.Expr) {
		if load, ok := value.(ir.Load); ok {
			if place, ok := load.Place.(ir.SSAPlace); ok {
				result[place.ID] = true
			}
		}
	})
}

func foldExpr(expr ir.Expr, constants map[int]ir.Const) ir.Expr {
	if load, ok := expr.(ir.Load); ok {
		if p, ok := load.Place.(ir.SSAPlace); ok {
			if c, ok := constants[p.ID]; ok {
				return c
			}
		}
		return load
	}
	call, ok := expr.(ir.RuntimeCall)
	if !ok || !call.Pure {
		return expr
	}
	values := make([]float64, len(call.Args))
	for i, a := range call.Args {
		c, ok := a.(ir.Const)
		if !ok {
			return expr
		}
		values[i] = c.Value
	}
	if result, ok := evaluateRuntime(call.Function, values); ok {
		return ir.Const{Value: result}
	}
	return expr
}

func evaluateRuntime(op resource.RuntimeFunction, a []float64) (float64, bool) {
	for _, value := range a {
		if math.IsNaN(value) || math.IsInf(value, 0) {
			return 0, false
		}
	}
	var result float64
	var ok bool
	switch op {
	case resource.RuntimeFunctionAnd:
		for _, value := range a {
			if value == 0 {
				return 0, true
			}
		}
		return 1, true
	case resource.RuntimeFunctionOr:
		for _, value := range a {
			if value != 0 {
				return 1, true
			}
		}
		return 0, true
	case resource.RuntimeFunctionAdd:
		for _, value := range a {
			result += value
		}
		return finiteConstant(result)
	case resource.RuntimeFunctionSubtract:
		if len(a) == 0 {
			return 0, true
		}
		result = a[0]
		for _, value := range a[1:] {
			result -= value
		}
		return finiteConstant(result)
	case resource.RuntimeFunctionMultiply:
		result = 1
		for _, value := range a {
			result *= value
		}
		return finiteConstant(result)
	case resource.RuntimeFunctionDivide:
		if len(a) == 0 {
			return 1, true
		}
		result = a[0]
		for _, value := range a[1:] {
			if value == 0 {
				return 0, false
			}
			result /= value
		}
		return finiteConstant(result)
	case resource.RuntimeFunctionPower:
		if len(a) == 0 {
			return 1, true
		}
		result = a[0]
		for _, value := range a[1:] {
			result = math.Pow(result, value)
		}
		return finiteConstant(result)
	}
	if len(a) == 1 {
		switch op {
		case resource.RuntimeFunctionAbs:
			result, ok = math.Abs(a[0]), true
		case resource.RuntimeFunctionFloor:
			result, ok = math.Floor(a[0]), true
		case resource.RuntimeFunctionCeil:
			result, ok = math.Ceil(a[0]), true
		case resource.RuntimeFunctionRound:
			result, ok = sonolusRound(a[0]), true
		case resource.RuntimeFunctionTrunc:
			result, ok = math.Trunc(a[0]), true
		case resource.RuntimeFunctionLog:
			result, ok = math.Log(a[0]), true
		case resource.RuntimeFunctionFrac:
			result, ok = a[0]-math.Floor(a[0]), true
		case resource.RuntimeFunctionSin:
			result, ok = math.Sin(a[0]), true
		case resource.RuntimeFunctionCos:
			result, ok = math.Cos(a[0]), true
		case resource.RuntimeFunctionTan:
			result, ok = math.Tan(a[0]), true
		case resource.RuntimeFunctionSinh:
			result, ok = math.Sinh(a[0]), true
		case resource.RuntimeFunctionCosh:
			result, ok = math.Cosh(a[0]), true
		case resource.RuntimeFunctionTanh:
			result, ok = math.Tanh(a[0]), true
		case resource.RuntimeFunctionArcsin:
			result, ok = math.Asin(a[0]), true
		case resource.RuntimeFunctionArccos:
			result, ok = math.Acos(a[0]), true
		case resource.RuntimeFunctionArctan:
			result, ok = math.Atan(a[0]), true
		case resource.RuntimeFunctionNegate:
			result, ok = -a[0], true
		case resource.RuntimeFunctionDegree:
			result, ok = a[0]*180/math.Pi, true
		case resource.RuntimeFunctionRadian:
			result, ok = a[0]*math.Pi/180, true
		case resource.RuntimeFunctionNot:
			if a[0] == 0 {
				result, ok = 1, true
				break
			}
			result, ok = 0, true
		}
	}
	if len(a) == 2 {
		switch op {
		case resource.RuntimeFunctionMod:
			if a[1] != 0 {
				result, ok = a[0]-math.Floor(a[0]/a[1])*a[1], true
			}
		case resource.RuntimeFunctionRem:
			if a[1] != 0 {
				result, ok = math.Mod(a[0], a[1]), true
			}
		case resource.RuntimeFunctionMin:
			result, ok = math.Min(a[0], a[1]), true
		case resource.RuntimeFunctionMax:
			result, ok = math.Max(a[0], a[1]), true
		case resource.RuntimeFunctionEqual:
			if a[0] == a[1] {
				result = 1
			}
			ok = true
		case resource.RuntimeFunctionNotEqual:
			if a[0] != a[1] {
				result = 1
			}
			ok = true
		case resource.RuntimeFunctionLess:
			if a[0] < a[1] {
				result = 1
			}
			ok = true
		case resource.RuntimeFunctionLessOr:
			if a[0] <= a[1] {
				result = 1
			}
			ok = true
		case resource.RuntimeFunctionGreater:
			if a[0] > a[1] {
				result = 1
			}
			ok = true
		case resource.RuntimeFunctionGreaterOr:
			if a[0] >= a[1] {
				result = 1
			}
			ok = true
		case resource.RuntimeFunctionArctan2:
			result, ok = math.Atan2(a[0], a[1]), true
		}
	}
	if len(a) == 3 {
		switch op {
		case resource.RuntimeFunctionClamp:
			result, ok = math.Min(math.Max(a[0], a[1]), a[2]), true
		case resource.RuntimeFunctionLerp:
			result, ok = a[0]+(a[1]-a[0])*a[2], true
		case resource.RuntimeFunctionLerpClamped:
			t := math.Max(0, math.Min(1, a[2]))
			result, ok = a[0]+(a[1]-a[0])*t, true
		}
	}
	if len(a) == 5 && a[1] != a[0] {
		switch op {
		case resource.RuntimeFunctionRemap:
			result, ok = a[2]+(a[3]-a[2])*(a[4]-a[0])/(a[1]-a[0]), true
		case resource.RuntimeFunctionRemapClamped:
			t := math.Max(0, math.Min(1, (a[4]-a[0])/(a[1]-a[0])))
			result, ok = a[2]+(a[3]-a[2])*t, true
		}
	}
	if !ok || math.IsNaN(result) || math.IsInf(result, 0) {
		return 0, false
	}
	return result, true
}

func finiteConstant(value float64) (float64, bool) {
	if math.IsNaN(value) || math.IsInf(value, 0) {
		return 0, false
	}
	return value, true
}

func sonolusRound(value float64) float64 {
	if math.IsNaN(value) || math.IsInf(value, 0) || value == 0 {
		return value
	}
	result := math.Floor(value + 0.5)
	if result == 0 && value < 0 {
		return math.Copysign(0, -1)
	}
	return result
}

type InlineVars struct{ Aggressive bool }

func (InlineVars) Requires() []Analysis  { return []Analysis{AnalysisDominance} }
func (InlineVars) Preserves() []Analysis { return []Analysis{AnalysisDominance, AnalysisSSA} }
func (InlineVars) Destroys() []Analysis  { return []Analysis{AnalysisLiveness} }

func (p InlineVars) Name() string {
	if p.Aggressive {
		return "InlineVarsAggressive"
	}
	return "InlineVars"
}
func (p InlineVars) Run(context Context, f *ir.Function) error {
	inlineSSAAliases(f)
	type definition struct {
		value              ir.Expr
		block, instruction int
	}
	indexed := indexedLocalIDs(f)
	dom := dominanceFor(context, f)
	defs := map[temporaryPlaceKey]definition{}
	defCounts := map[temporaryPlaceKey]int{}
	uses := map[temporaryPlaceKey]int{}
	unsafeUses := map[temporaryPlaceKey]bool{}
	crossBlock := map[temporaryPlaceKey]bool{}
	crossLoop := map[temporaryPlaceKey]bool{}
	loops := map[int]map[int]bool{}
	if !p.Aggressive {
		preds := predecessors(f)
		for latch, block := range f.Blocks {
			forEachTerminatorTarget(block.Terminator, func(header int) {
				if !dominates(dom, header, latch) {
					return
				}
				if loops[header] == nil {
					loops[header] = map[int]bool{}
				}
				for id := range naturalLoop(preds, header, latch) {
					loops[header][id] = true
				}
			})
		}
	}
	for _, b := range f.Blocks {
		for index, in := range b.Instructions {
			if s, ok := in.(ir.Store); ok && isTemporaryPlace(s.Place) {
				k, _ := placeKey(s.Place)
				defs[k] = definition{s.Value, b.ID, index}
				defCounts[k]++
			}
		}
	}
	for _, block := range f.Blocks {
		instructionIndex := 0
		visit := func(expr ir.Expr) {
			walkExpr(expr, func(value ir.Expr) {
				load, ok := value.(ir.Load)
				if !ok {
					return
				}
				key, valid := placeKey(load.Place)
				if !valid {
					return
				}
				uses[key]++
				if def, exists := defs[key]; exists && def.block != block.ID {
					crossBlock[key] = true
					for _, body := range loops {
						if body[block.ID] && !body[def.block] {
							crossLoop[key] = true
						}
					}
				}
				if place, local := load.Place.(ir.LocalPlace); local {
					def, exists := defs[key]
					// Indexed writes can redefine every slot of this local.
					// A unique fixed-slot definition is not sufficient.
					if indexed[place.ID] || !exists || !dominates(dom, def.block, block.ID) ||
						def.block == block.ID && def.instruction >= instructionIndex {
						unsafeUses[key] = true
					}
				}
			})
		}
		for index, instruction := range block.Instructions {
			instructionIndex = index
			switch value := instruction.(type) {
			case ir.Store:
				visit(value.Value)
				addPlaceExpressions(value.Place, visit)
			case ir.Eval:
				visit(value.Value)
			}
		}
		instructionIndex = len(block.Instructions)
		visitTerminator(block.Terminator, visit)
		for _, phi := range block.Phis {
			for _, arg := range phi.Args {
				key, _ := placeKey(arg.Value)
				uses[key]++
				// Phi inputs must stay in SSA places. Their definitions cannot
				// be eliminated by expression substitution alone.
				crossBlock[key] = true
			}
		}
	}
	// Budget the fully expanded expression as well as the original one:
	// substituting a cheap wrapper must not duplicate an expensive dependency.
	affordable := func(k temporaryPlaceKey, value ir.Expr) bool {
		if p.Aggressive {
			return true
		}
		cost := expressionCost(value)
		// Keep LICM's work outside loops unless substitution costs no more
		// than a temporary Get. Repeated same-block uses can inline when
		// they cost at most one Set plus the original Gets.
		return !(crossLoop[k] && cost > 3 || uses[k] != 1 && cost > 3 && (crossBlock[k] || uses[k] < 2 || cost*(uses[k]-1) > 3*(uses[k]+1)))
	}
	expanding := map[temporaryPlaceKey]bool{}
	expanded := map[temporaryPlaceKey]ir.Expr{}
	var inline func(ir.Expr) (ir.Expr, bool)
	inline = func(e ir.Expr) (ir.Expr, bool) {
		l, ok := e.(ir.Load)
		if !ok {
			return e, false
		}
		k, valid := placeKey(l.Place)
		if !valid {
			return e, false
		}
		v, ok := defs[k]
		if !ok || defCounts[k] != 1 || unsafeUses[k] || expanding[k] {
			return e, false
		}
		movable := movableExpression(context, v.value)
		if !movable && uses[k] == 1 && !crossBlock[k] && v.instruction == len(f.Blocks[v.block].Instructions)-1 && !expressionHasEffects(v.value) {
			// A single use in the immediately following pure terminator can
			// read a mutable value directly: no intervening store or effect
			// can change it. Never move this read across an effectful operand.
			movable = true
			visitTerminator(f.Blocks[v.block].Terminator, func(expr ir.Expr) {
				if expressionHasEffects(expr) {
					movable = false
				}
			})
		}
		if !movable {
			return e, false
		}
		if !affordable(k, v.value) {
			return e, false
		}
		replacement, cached := expanded[k]
		if !cached {
			expanding[k] = true
			replacement, _ = rewriteExprChanged(v.value, inline)
			delete(expanding, k)
			expanded[k] = replacement
		}
		if !affordable(k, replacement) {
			return e, false
		}
		// Later passes may mutate expression argument slices. Each use must
		// own its tree even though expansion is cached within this callback.
		return cloneExpr(replacement), true
	}
	rewriteFunctionExpressionsChanged(f, inline)
	return nil
}

// SSA aliases are immutable snapshots. Resolve them before counting uses so
// alias chains and Phi edges do not hide the actual number of consumers.
// Local and memory loads deliberately remain outside this substitution.
func inlineSSAAliases(f *ir.Function) {
	aliases := map[ir.SSAPlace]ir.SSAPlace{}
	for _, block := range f.Blocks {
		for _, instruction := range block.Instructions {
			store, ok := instruction.(ir.Store)
			if !ok {
				continue
			}
			target, ok := store.Place.(ir.SSAPlace)
			if !ok {
				continue
			}
			load, ok := store.Value.(ir.Load)
			if !ok {
				continue
			}
			if source, ok := load.Place.(ir.SSAPlace); ok {
				aliases[target] = source
			}
		}
	}
	if len(aliases) == 0 {
		return
	}
	resolve := func(place ir.SSAPlace) ir.SSAPlace {
		root := place
		for steps := 0; steps <= len(aliases); steps++ {
			next, ok := aliases[root]
			if !ok {
				for place != root {
					next := aliases[place]
					aliases[place] = root
					place = next
				}
				return root
			}
			root = next
		}
		// Valid SSA cannot contain a cycle of ordinary copy definitions.
		// Leave malformed input unchanged for the validator to diagnose.
		return place
	}
	rewriteFunctionExpressionsChanged(f, func(expr ir.Expr) (ir.Expr, bool) {
		load, ok := expr.(ir.Load)
		if !ok {
			return expr, false
		}
		place, ok := load.Place.(ir.SSAPlace)
		if !ok {
			return expr, false
		}
		root := resolve(place)
		return ir.Load{Place: root}, root != place
	})
	for _, block := range f.Blocks {
		for i := range block.Phis {
			for j := range block.Phis[i].Args {
				arg := &block.Phis[i].Args[j]
				arg.Value = resolve(arg.Value)
			}
		}
	}
}

func movableExpression(context Context, expr ir.Expr) bool {
	switch value := expr.(type) {
	case ir.Const:
		return true
	case ir.Load:
		switch place := value.Place.(type) {
		case ir.SSAPlace:
			return true
		case ir.MemoryPlace:
			return place.Read && !place.Write && catalog.MemoryReadonly(context.Mode, context.Callback, place.Storage) && movableExpression(context, place.Index)
		default:
			return false
		}
	case ir.RuntimeCall:
		if !value.Pure {
			return false
		}
		for _, arg := range value.Args {
			if !movableExpression(context, arg) {
				return false
			}
		}
		return true
	default:
		return false
	}
}

type CommonSubexpressionElimination struct{}

func (CommonSubexpressionElimination) Name() string          { return "CommonSubexpressionElimination" }
func (CommonSubexpressionElimination) Requires() []Analysis  { return []Analysis{AnalysisDominance} }
func (CommonSubexpressionElimination) Preserves() []Analysis { return []Analysis{AnalysisDominance} }
func (CommonSubexpressionElimination) Destroys() []Analysis  { return nil }
func (CommonSubexpressionElimination) Run(context Context, f *ir.Function) error {
	dom := dominanceFor(context, f)
	nextSSA := maxSSAID(f) + 1
	var visit func(int, map[string]ir.SSAPlace)
	visit = func(id int, inherited map[string]ir.SSAPlace) {
		available := make(map[string]ir.SSAPlace, len(inherited))
		for key, place := range inherited {
			available[key] = place
		}
		block := f.Blocks[id]
		original := block.Instructions
		block.Instructions = nil
		for _, instruction := range original {
			switch value := instruction.(type) {
			case ir.Store:
				value.Place = rewritePlace(value.Place, func(expr ir.Expr) ir.Expr {
					return cseExpression(context, expr, block, available, &nextSSA, true)
				})
				value.Value = cseExpression(context, value.Value, block, available, &nextSSA, true)
				_, constant := value.Value.(ir.Const)
				if target, ok := value.Place.(ir.SSAPlace); ok && !constant && movableExpression(context, value.Value) {
					key := exprKey(value.Value)
					if previous, exists := available[key]; exists {
						value.Value = ir.Load{Place: previous}
					} else {
						available[key] = target
					}
				}
				block.Instructions = append(block.Instructions, value)
			case ir.Eval:
				value.Value = cseExpression(context, value.Value, block, available, &nextSSA, true)
				block.Instructions = append(block.Instructions, value)
			default:
				block.Instructions = append(block.Instructions, instruction)
			}
		}
		switch value := block.Terminator.(type) {
		case ir.Branch:
			value.Condition = cseExpression(context, value.Condition, block, available, &nextSSA, true)
			block.Terminator = value
		case ir.Switch:
			value.Value = cseExpression(context, value.Value, block, available, &nextSSA, true)
			block.Terminator = value
		case ir.Return:
			for index, slot := range value.Value.Slots {
				value.Value.Slots[index] = cseExpression(context, slot, block, available, &nextSSA, true)
			}
			block.Terminator = value
		}
		for _, child := range dom.Children[id] {
			visit(child, available)
		}
	}
	visit(f.Entry, nil)
	return nil
}

func maxSSAID(function *ir.Function) int {
	maximum := 0
	walk := func(expression ir.Expr) {
		walkExpr(expression, func(value ir.Expr) {
			if load, ok := value.(ir.Load); ok {
				if place, ok := load.Place.(ir.SSAPlace); ok && place.ID > maximum {
					maximum = place.ID
				}
			}
		})
	}
	for _, block := range function.Blocks {
		for _, phi := range block.Phis {
			if phi.Target.ID > maximum {
				maximum = phi.Target.ID
			}
			for _, argument := range phi.Args {
				if argument.Value.ID > maximum {
					maximum = argument.Value.ID
				}
			}
		}
		for _, instruction := range block.Instructions {
			switch value := instruction.(type) {
			case ir.Store:
				if place, ok := value.Place.(ir.SSAPlace); ok && place.ID > maximum {
					maximum = place.ID
				}
				walk(value.Value)
			case ir.Eval:
				walk(value.Value)
			}
		}
		visitTerminator(block.Terminator, walk)
	}
	return maximum
}

func cseExpression(context Context, expression ir.Expr, block *ir.Block, available map[string]ir.SSAPlace, next *int, extract bool) ir.Expr {
	if _, ok := expression.(ir.Const); ok {
		return expression
	}
	switch value := expression.(type) {
	case ir.Load:
		value.Place = rewritePlace(value.Place, func(index ir.Expr) ir.Expr {
			return cseExpression(context, index, block, available, next, true)
		})
		expression = value
	case ir.RuntimeCall:
		for index, argument := range value.Args {
			value.Args[index] = cseExpression(context, argument, block, available, next, true)
		}
		if isCSECommutative(value.Function) && len(value.Args) == 2 && exprKey(value.Args[1]) < exprKey(value.Args[0]) {
			value.Args[0], value.Args[1] = value.Args[1], value.Args[0]
		}
		expression = value
	}
	if !movableExpression(context, expression) {
		return expression
	}
	key := exprKey(expression)
	if previous, exists := available[key]; exists {
		return ir.Load{Place: previous}
	}
	if !extract || expressionCost(expression) < 4 {
		return expression
	}
	place := ir.SSAPlace{ID: *next, Name: "cse"}
	*next++
	block.Instructions = append(block.Instructions, ir.Store{Place: place, Value: expression})
	available[key] = place
	return ir.Load{Place: place}
}

func expressionCost(expression ir.Expr) int {
	switch value := expression.(type) {
	case ir.Const:
		return 1
	case ir.Load:
		switch place := value.Place.(type) {
		case ir.SSAPlace:
			return 3
		case ir.MemoryPlace:
			return 2 + expressionCost(place.Index)
		default:
			return 1
		}
	case ir.RuntimeCall:
		cost := 1
		for _, argument := range value.Args {
			cost += expressionCost(argument)
		}
		return cost
	default:
		return 1
	}
}

func isCSECommutative(function resource.RuntimeFunction) bool {
	switch function {
	case resource.RuntimeFunctionEqual, resource.RuntimeFunctionNotEqual, resource.RuntimeFunctionMin, resource.RuntimeFunctionMax:
		return true
	default:
		return false
	}
}
func exprKey(e ir.Expr) string {
	switch v := e.(type) {
	case ir.Const:
		return "c:" + fmtFloat(v.Value)
	case ir.Load:
		return "g:" + csePlaceKey(v.Place)
	case ir.RuntimeCall:
		s := "f:" + string(v.Function)
		arguments := v.Args
		if isCSECommutative(v.Function) && len(arguments) == 2 && exprKey(arguments[1]) < exprKey(arguments[0]) {
			arguments = []ir.Expr{arguments[1], arguments[0]}
		}
		for _, a := range arguments {
			s += "|" + exprKey(a)
		}
		return s
	}
	return "?"
}

func csePlaceKey(place ir.Place) string {
	switch value := place.(type) {
	case ir.LocalPlace:
		return "local:" + itoa(value.ID) + ":" + itoa(value.Offset)
	case ir.SSAPlace:
		return "ssa:" + itoa(value.ID)
	case ir.IndexedLocalPlace:
		return "indexed:" + itoa(value.ID) + ":" + exprKey(value.Index) + ":" + itoa(value.Base) + ":" + itoa(value.Length) + ":" + itoa(value.Stride) + ":" + itoa(value.Offset)
	case ir.MemoryPlace:
		return "memory:" + value.Storage + ":" + exprKey(value.Index) + ":" + itoa(value.Stride) + ":" + itoa(value.Offset)
	default:
		return "unknown"
	}
}
func fmtFloat(v float64) string {
	return itoa(int(math.Float64bits(v)>>32)) + ":" + itoa(int(uint32(math.Float64bits(v))))
}
