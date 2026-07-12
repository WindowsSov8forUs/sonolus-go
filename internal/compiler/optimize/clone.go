package optimize

import "github.com/WindowsSov8forUs/sonolus-go/v2/internal/compiler/ir"

func CloneFunction(function *ir.Function) *ir.Function {
	if function == nil {
		return nil
	}
	result := &ir.Function{
		Name: function.Name, Result: cloneType(function.Result), Entry: function.Entry,
		Locals: make([]ir.Type, len(function.Locals)), Blocks: make([]*ir.Block, len(function.Blocks)), Allocated: function.Allocated,
	}
	for i, local := range function.Locals {
		result.Locals[i] = cloneType(local)
	}
	for i, block := range function.Blocks {
		if block == nil {
			continue
		}
		cloned := &ir.Block{ID: block.ID, Phis: make([]ir.Phi, len(block.Phis)), Instructions: make([]ir.Instruction, len(block.Instructions))}
		for j, phi := range block.Phis {
			cloned.Phis[j] = ir.Phi{Target: phi.Target, Local: phi.Local, Args: append([]ir.PhiArg(nil), phi.Args...)}
		}
		for j, instruction := range block.Instructions {
			cloned.Instructions[j] = cloneInstruction(instruction)
		}
		cloned.Terminator = cloneTerminator(block.Terminator)
		result.Blocks[i] = cloned
	}
	return result
}

func cloneType(value ir.Type) ir.Type {
	result := ir.Type{Name: value.Name, Slots: value.Slots, Fields: make([]ir.Field, len(value.Fields))}
	for i, field := range value.Fields {
		result.Fields[i] = ir.Field{Name: field.Name, Offset: field.Offset, Type: cloneType(field.Type)}
	}
	return result
}

func cloneValue(value ir.Value) ir.Value {
	result := ir.Value{Type: cloneType(value.Type), Slots: make([]ir.Expr, len(value.Slots))}
	for i, slot := range value.Slots {
		result.Slots[i] = cloneExpr(slot)
	}
	return result
}

func cloneExpr(expression ir.Expr) ir.Expr {
	switch expression := expression.(type) {
	case ir.Const:
		return expression
	case ir.Load:
		return ir.Load{Place: clonePlace(expression.Place)}
	case ir.RuntimeCall:
		args := make([]ir.Expr, len(expression.Args))
		for i, argument := range expression.Args {
			args[i] = cloneExpr(argument)
		}
		return ir.RuntimeCall{Function: expression.Function, Args: args, Result: cloneType(expression.Result), Pure: expression.Pure, Pos: expression.Pos}
	default:
		return expression
	}
}

func clonePlace(place ir.Place) ir.Place {
	switch place := place.(type) {
	case ir.SSAPlace:
		return place
	case ir.LocalPlace:
		return place
	case ir.IndexedLocalPlace:
		place.Index = cloneExpr(place.Index)
		return place
	case ir.MemoryPlace:
		place.Index = cloneExpr(place.Index)
		return place
	default:
		return place
	}
}

func cloneInstruction(instruction ir.Instruction) ir.Instruction {
	switch instruction := instruction.(type) {
	case ir.Store:
		return ir.Store{Place: clonePlace(instruction.Place), Value: cloneExpr(instruction.Value), Pos: instruction.Pos}
	case ir.Eval:
		return ir.Eval{Value: cloneExpr(instruction.Value)}
	default:
		return instruction
	}
}

func cloneTerminator(terminator ir.Terminator) ir.Terminator {
	switch terminator := terminator.(type) {
	case ir.Jump:
		return terminator
	case ir.Branch:
		return ir.Branch{Condition: cloneExpr(terminator.Condition), True: terminator.True, False: terminator.False}
	case ir.Switch:
		return ir.Switch{Value: cloneExpr(terminator.Value), Cases: append([]ir.SwitchCase(nil), terminator.Cases...), Default: terminator.Default}
	case ir.Return:
		return ir.Return{Value: cloneValue(terminator.Value)}
	case ir.Unreachable:
		return terminator
	default:
		return terminator
	}
}
