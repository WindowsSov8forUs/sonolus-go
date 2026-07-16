package optimize

import "github.com/WindowsSov8forUs/sonolus-go/v2/internal/compiler/ir"

func rewriteExpr(expr ir.Expr, fn func(ir.Expr) ir.Expr) ir.Expr {
	result, _ := rewriteExprChanged(expr, func(value ir.Expr) (ir.Expr, bool) {
		return fn(value), true
	})
	return result
}

func rewriteExprChanged(expr ir.Expr, fn func(ir.Expr) (ir.Expr, bool)) (ir.Expr, bool) {
	childrenChanged := false
	switch value := expr.(type) {
	case ir.Load:
		place, changed := rewritePlaceChanged(value.Place, fn)
		if changed {
			value.Place = place
			expr = value
			childrenChanged = true
		}
	case ir.RuntimeCall:
		var args []ir.Expr
		for i, arg := range value.Args {
			rewritten, changed := rewriteExprChanged(arg, fn)
			if changed && args == nil {
				args = make([]ir.Expr, len(value.Args))
				copy(args, value.Args[:i])
			}
			if args != nil {
				args[i] = rewritten
			}
		}
		if args != nil {
			value.Args = args
			expr = value
			childrenChanged = true
		}
	}
	result, changed := fn(expr)
	return result, childrenChanged || changed
}

func rewritePlace(place ir.Place, fn func(ir.Expr) ir.Expr) ir.Place {
	result, _ := rewritePlaceChanged(place, func(value ir.Expr) (ir.Expr, bool) {
		return fn(value), true
	})
	return result
}

func rewritePlaceChanged(place ir.Place, fn func(ir.Expr) (ir.Expr, bool)) (ir.Place, bool) {
	switch value := place.(type) {
	case ir.IndexedLocalPlace:
		index, changed := rewriteExprChanged(value.Index, fn)
		if changed {
			value.Index = index
		}
		return value, changed
	case ir.MemoryPlace:
		index, changed := rewriteExprChanged(value.Index, fn)
		if changed {
			value.Index = index
		}
		return value, changed
	default:
		return place, false
	}
}

func rewriteFunctionExpressions(function *ir.Function, fn func(ir.Expr) ir.Expr) {
	rewriteFunctionExpressionsChanged(function, func(value ir.Expr) (ir.Expr, bool) {
		return fn(value), true
	})
}

func rewriteFunctionExpressionsChanged(function *ir.Function, fn func(ir.Expr) (ir.Expr, bool)) {
	for _, block := range function.Blocks {
		for i, instruction := range block.Instructions {
			switch value := instruction.(type) {
			case ir.Store:
				value.Place, _ = rewritePlaceChanged(value.Place, fn)
				value.Value, _ = rewriteExprChanged(value.Value, fn)
				block.Instructions[i] = value
			case ir.Eval:
				value.Value, _ = rewriteExprChanged(value.Value, fn)
				block.Instructions[i] = value
			}
		}
		switch value := block.Terminator.(type) {
		case ir.Branch:
			value.Condition, _ = rewriteExprChanged(value.Condition, fn)
			block.Terminator = value
		case ir.Switch:
			value.Value, _ = rewriteExprChanged(value.Value, fn)
			block.Terminator = value
		case ir.Return:
			for i, slot := range value.Value.Slots {
				value.Value.Slots[i], _ = rewriteExprChanged(slot, fn)
			}
			block.Terminator = value
		}
	}
}

func predecessors(function *ir.Function) [][]int {
	result := make([][]int, len(function.Blocks))
	for _, block := range function.Blocks {
		forEachTerminatorTarget(block.Terminator, func(target int) {
			result[target] = append(result[target], block.ID)
		})
	}
	return result
}

func localUsesExpr(expr ir.Expr, uses map[int]bool) {
	switch value := expr.(type) {
	case ir.Load:
		localUsesPlace(value.Place, uses)
	case ir.RuntimeCall:
		for _, arg := range value.Args {
			localUsesExpr(arg, uses)
		}
	}
}

func localUsesPlace(place ir.Place, uses map[int]bool) {
	switch value := place.(type) {
	case ir.LocalPlace:
		uses[value.ID] = true
	case ir.IndexedLocalPlace:
		uses[value.ID] = true
		localUsesExpr(value.Index, uses)
	case ir.MemoryPlace:
		localUsesExpr(value.Index, uses)
	}
}
