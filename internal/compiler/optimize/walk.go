package optimize

import "github.com/WindowsSov8forUs/sonolus-go/v2/internal/compiler/ir"

func rewriteExpr(expr ir.Expr, fn func(ir.Expr) ir.Expr) ir.Expr {
	switch value := expr.(type) {
	case ir.Load:
		value.Place = rewritePlace(value.Place, fn)
		expr = value
	case ir.RuntimeCall:
		args := make([]ir.Expr, len(value.Args))
		for i, arg := range value.Args {
			args[i] = rewriteExpr(arg, fn)
		}
		value.Args = args
		expr = value
	}
	return fn(expr)
}

func rewritePlace(place ir.Place, fn func(ir.Expr) ir.Expr) ir.Place {
	switch value := place.(type) {
	case ir.IndexedLocalPlace:
		value.Index = rewriteExpr(value.Index, fn)
		return value
	case ir.MemoryPlace:
		value.Index = rewriteExpr(value.Index, fn)
		return value
	default:
		return place
	}
}

func rewriteFunctionExpressions(function *ir.Function, fn func(ir.Expr) ir.Expr) {
	for _, block := range function.Blocks {
		for i, instruction := range block.Instructions {
			switch value := instruction.(type) {
			case ir.Store:
				value.Place = rewritePlace(value.Place, fn)
				value.Value = rewriteExpr(value.Value, fn)
				block.Instructions[i] = value
			case ir.Eval:
				value.Value = rewriteExpr(value.Value, fn)
				block.Instructions[i] = value
			}
		}
		switch value := block.Terminator.(type) {
		case ir.Branch:
			value.Condition = rewriteExpr(value.Condition, fn)
			block.Terminator = value
		case ir.Switch:
			value.Value = rewriteExpr(value.Value, fn)
			block.Terminator = value
		case ir.Return:
			for i, slot := range value.Value.Slots {
				value.Value.Slots[i] = rewriteExpr(slot, fn)
			}
			block.Terminator = value
		}
	}
}

func predecessors(function *ir.Function) [][]int {
	result := make([][]int, len(function.Blocks))
	for _, block := range function.Blocks {
		for _, target := range terminatorTargets(block.Terminator) {
			result[target] = append(result[target], block.ID)
		}
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
