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
	for changed := true; changed; {
		changed = false
		uses := map[temporaryPlaceKey]int{}
		visit := func(expr ir.Expr) {
			walkExpr(expr, func(e ir.Expr) {
				if l, ok := e.(ir.Load); ok {
					if key, valid := placeKey(l.Place); valid {
						uses[key]++
					}
				}
			})
		}
		for _, b := range function.Blocks {
			for _, p := range b.Phis {
				for _, a := range p.Args {
					key, _ := placeKey(a.Value)
					uses[key]++
				}
			}
			for _, in := range b.Instructions {
				switch v := in.(type) {
				case ir.Store:
					visit(v.Value)
					addPlaceExpressions(v.Place, visit)
				case ir.Eval:
					visit(v.Value)
				}
			}
			visitTerminator(b.Terminator, visit)
		}
		for _, b := range function.Blocks {
			out := b.Instructions[:0]
			for _, in := range b.Instructions {
				store, ok := in.(ir.Store)
				addressTaken := false
				if place, local := store.Place.(ir.LocalPlace); local {
					addressTaken = indexed[place.ID]
				}
				key, temporary := placeKey(store.Place)
				if ok && temporary && !addressTaken && uses[key] == 0 && !expressionHasEffects(store.Value) {
					changed = true
					continue
				}
				out = append(out, in)
			}
			b.Instructions = out
		}
	}
	return nil
}

type AdvancedDeadCodeElimination struct{}

func (AdvancedDeadCodeElimination) Requires() []Analysis  { return nil }
func (AdvancedDeadCodeElimination) Preserves() []Analysis { return []Analysis{AnalysisLiveness} }
func (AdvancedDeadCodeElimination) Destroys() []Analysis  { return nil }

func (AdvancedDeadCodeElimination) Name() string { return "AdvancedDeadCodeElimination" }
func (AdvancedDeadCodeElimination) Run(_ Context, f *ir.Function) error {
	indexed := indexedLocalIDs(f)
	use := make([]map[temporaryPlaceKey]bool, len(f.Blocks))
	def := make([]map[temporaryPlaceKey]bool, len(f.Blocks))
	for i := range f.Blocks {
		use[i], def[i] = map[temporaryPlaceKey]bool{}, map[temporaryPlaceKey]bool{}
		add := func(expr ir.Expr) {
			walkExpr(expr, func(value ir.Expr) {
				if load, ok := value.(ir.Load); ok {
					key, valid := placeKey(load.Place)
					if valid && !def[i][key] {
						use[i][key] = true
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
					def[i][key] = true
				}
			case ir.Eval:
				add(value.Value)
			}
		}
		visitTerminator(f.Blocks[i].Terminator, add)
	}
	liveIn := make([]map[temporaryPlaceKey]bool, len(f.Blocks))
	liveOut := make([]map[temporaryPlaceKey]bool, len(f.Blocks))
	for i := range f.Blocks {
		liveIn[i], liveOut[i] = map[temporaryPlaceKey]bool{}, map[temporaryPlaceKey]bool{}
	}
	for changed := true; changed; {
		changed = false
		for i := len(f.Blocks) - 1; i >= 0; i-- {
			out := map[temporaryPlaceKey]bool{}
			forEachTerminatorTarget(f.Blocks[i].Terminator, func(successor int) {
				for key := range liveIn[successor] {
					out[key] = true
				}
			})
			in := cloneStringSet(use[i])
			for key := range out {
				if !def[i][key] {
					in[key] = true
				}
			}
			if !sameStringSet(out, liveOut[i]) || !sameStringSet(in, liveIn[i]) {
				liveOut[i], liveIn[i], changed = out, in, true
			}
		}
	}
	for _, block := range f.Blocks {
		live := cloneStringSet(liveOut[block.ID])
		visitTerminator(block.Terminator, func(expr ir.Expr) { addExpressionKeys(expr, live) })
		kept := make([]ir.Instruction, 0, len(block.Instructions))
		for i := len(block.Instructions) - 1; i >= 0; i-- {
			instruction := block.Instructions[i]
			if store, ok := instruction.(ir.Store); ok {
				key, temporary := placeKey(store.Place)
				addressTaken := false
				if place, local := store.Place.(ir.LocalPlace); local {
					addressTaken = indexed[place.ID]
				}
				if temporary && !addressTaken && !live[key] && !expressionHasEffects(store.Value) {
					continue
				}
				if temporary {
					delete(live, key)
				}
				addExpressionKeys(store.Value, live)
				addPlaceExpressions(store.Place, func(expr ir.Expr) { addExpressionKeys(expr, live) })
			} else if eval, ok := instruction.(ir.Eval); ok {
				addExpressionKeys(eval.Value, live)
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

func addExpressionKeys(expr ir.Expr, values map[temporaryPlaceKey]bool) {
	walkExpr(expr, func(value ir.Expr) {
		if load, ok := value.(ir.Load); ok {
			if key, valid := placeKey(load.Place); valid {
				values[key] = true
			}
		}
	})
}

func cloneStringSet(input map[temporaryPlaceKey]bool) map[temporaryPlaceKey]bool {
	result := make(map[temporaryPlaceKey]bool, len(input))
	for key := range input {
		result[key] = true
	}
	return result
}

func sameStringSet(a, b map[temporaryPlaceKey]bool) bool {
	if len(a) != len(b) {
		return false
	}
	for key := range a {
		if !b[key] {
			return false
		}
	}
	return true
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

func (p InlineVars) Name() string {
	if p.Aggressive {
		return "InlineVarsAggressive"
	}
	return "InlineVars"
}
func (p InlineVars) Run(context Context, f *ir.Function) error {
	defs := map[temporaryPlaceKey]ir.Expr{}
	defCounts := map[temporaryPlaceKey]int{}
	uses := map[temporaryPlaceKey]int{}
	for _, b := range f.Blocks {
		for _, in := range b.Instructions {
			if s, ok := in.(ir.Store); ok && isTemporaryPlace(s.Place) {
				k, _ := placeKey(s.Place)
				defs[k] = s.Value
				defCounts[k]++
			}
		}
	}
	for _, block := range f.Blocks {
		for _, instruction := range block.Instructions {
			switch value := instruction.(type) {
			case ir.Store:
				countLoads(value.Value, uses)
			case ir.Eval:
				countLoads(value.Value, uses)
			}
		}
		visitTerminator(block.Terminator, func(expr ir.Expr) { countLoads(expr, uses) })
	}
	rewriteFunctionExpressionsChanged(f, func(e ir.Expr) (ir.Expr, bool) {
		l, ok := e.(ir.Load)
		if !ok {
			return e, false
		}
		k, valid := placeKey(l.Place)
		if !valid {
			return e, false
		}
		v, ok := defs[k]
		if !ok || defCounts[k] != 1 || !movableExpression(context, v) || (!p.Aggressive && uses[k] != 1) {
			return e, false
		}
		return cloneExpr(v), true
	})
	return nil
}

func countLoads(expr ir.Expr, counts map[temporaryPlaceKey]int) {
	walkExpr(expr, func(value ir.Expr) {
		if load, ok := value.(ir.Load); ok {
			if key, valid := placeKey(load.Place); valid {
				counts[key]++
			}
		}
	})
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
