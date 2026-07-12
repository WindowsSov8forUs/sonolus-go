package optimize

import "github.com/WindowsSov8forUs/sonolus-go/internal/newcompiler/ir"

type RemoveNoOps struct{}

func (RemoveNoOps) Name() string { return "RemoveNoOps" }

func (RemoveNoOps) Run(_ Context, function *ir.Function) error {
	for _, block := range function.Blocks {
		instructions := make([]ir.Instruction, 0, len(block.Instructions))
		for _, instruction := range block.Instructions {
			if removableInstruction(instruction) {
				continue
			}
			instructions = append(instructions, instruction)
		}
		block.Instructions = instructions
	}
	return nil
}

func removableInstruction(instruction ir.Instruction) bool {
	switch instruction := instruction.(type) {
	case ir.Eval:
		call, ok := instruction.Value.(ir.RuntimeCall)
		return ok && call.Pure && call.Result.Slots == 0 && !argumentsHaveEffects(call.Args)
	case ir.Store:
		target, targetOK := instruction.Place.(ir.LocalPlace)
		load, loadOK := instruction.Value.(ir.Load)
		source, sourceOK := load.Place.(ir.LocalPlace)
		return targetOK && loadOK && sourceOK && target.ID == source.ID && target.Offset == source.Offset
	default:
		return false
	}
}

func argumentsHaveEffects(arguments []ir.Expr) bool {
	for _, argument := range arguments {
		if expressionHasEffects(argument) {
			return true
		}
	}
	return false
}

func expressionHasEffects(expression ir.Expr) bool {
	switch expression := expression.(type) {
	case ir.Const:
		return false
	case ir.Load:
		switch place := expression.Place.(type) {
		case ir.IndexedLocalPlace:
			return expressionHasEffects(place.Index)
		case ir.MemoryPlace:
			return expressionHasEffects(place.Index)
		default:
			return false
		}
	case ir.RuntimeCall:
		return !expression.Pure || argumentsHaveEffects(expression.Args)
	default:
		return true
	}
}
