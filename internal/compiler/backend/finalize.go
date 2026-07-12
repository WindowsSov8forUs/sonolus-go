package backend

import (
	"fmt"
	"math"
	"sort"

	"github.com/WindowsSov8forUs/sonolus-core-go/core/resource"
	"github.com/WindowsSov8forUs/sonolus-go/internal/compiler/ir"
	"github.com/WindowsSov8forUs/sonolus-go/internal/compiler/mode"
)

type finalizer struct {
	mode     mode.Mode
	function *ir.Function
}

func finalizeFunction(m mode.Mode, function *ir.Function) (snode, error) {
	if err := ir.ValidateFinal(function); err != nil {
		return nil, fmt.Errorf("backend: validate %s: %w", function.Name, err)
	}
	if len(function.Locals) > 1 {
		return nil, fmt.Errorf("backend: finalize %s: optimizer left %d physical locals", function.Name, len(function.Locals))
	}
	if len(function.Locals) == 1 && function.Locals[0].Slots > 4096 {
		return nil, fmt.Errorf("backend: finalize %s: temporary memory requires %d slots; limit is 4096", function.Name, function.Locals[0].Slots)
	}
	f := &finalizer{mode: m, function: function}
	order := function.Blocks
	for index, block := range order {
		if block.ID != index {
			return nil, fmt.Errorf("backend: finalize %s: block %d has non-normalized ID %d", function.Name, index, block.ID)
		}
	}
	if len(order) == 1 && len(order[0].Instructions) == 0 {
		if result, ok := order[0].Terminator.(ir.Return); ok {
			if len(result.Value.Slots) == 0 {
				return valueNode(0), nil
			}
			if len(result.Value.Slots) == 1 {
				return f.expression(result.Value.Slots[0])
			}
		}
	}
	indexes := make(map[int]int, len(order))
	for index, block := range order {
		indexes[block.ID] = index
	}
	exit := len(order)
	blocks := make([]snode, 0, len(order)+1)
	for _, block := range order {
		statements := make([]snode, 0, len(block.Instructions)+1)
		for _, instruction := range block.Instructions {
			lowered, err := f.instruction(instruction)
			if err != nil {
				return nil, err
			}
			statements = append(statements, lowered)
		}
		control, err := f.terminator(block.Terminator, indexes, exit)
		if err != nil {
			return nil, err
		}
		statements = append(statements, control)
		blocks = append(blocks, call(resource.RuntimeFunctionExecute, statements...))
	}
	blocks = append(blocks, valueNode(0))
	return call(resource.RuntimeFunctionBlock, call(resource.RuntimeFunctionJumpLoop, blocks...)), nil
}

func (f *finalizer) expression(expression ir.Expr) (snode, error) {
	switch expression := expression.(type) {
	case ir.Const:
		switch {
		case math.IsNaN(expression.Value):
			return call(resource.RuntimeFunctionGet, valueNode(engineROMBlock), valueNode(0)), nil
		case math.IsInf(expression.Value, 1):
			return call(resource.RuntimeFunctionGet, valueNode(engineROMBlock), valueNode(1)), nil
		case math.IsInf(expression.Value, -1):
			return call(resource.RuntimeFunctionGet, valueNode(engineROMBlock), valueNode(2)), nil
		default:
			return valueNode(expression.Value), nil
		}
	case ir.Load:
		return f.load(expression.Place)
	case ir.RuntimeCall:
		args := make([]snode, len(expression.Args))
		for i, argument := range expression.Args {
			lowered, err := f.expression(argument)
			if err != nil {
				return nil, err
			}
			args[i] = lowered
		}
		return call(expression.Function, args...), nil
	default:
		return nil, fmt.Errorf("backend: unsupported expression %T", expression)
	}
}

func (f *finalizer) address(place ir.Place) (block int, offset snode, index snode, stride int, shifted bool, err error) {
	switch place := place.(type) {
	case ir.LocalPlace:
		if place.ID != 0 {
			return 0, nil, nil, 0, false, fmt.Errorf("backend: physical local has ID %d; expected 0", place.ID)
		}
		return temporaryMemoryBlock, valueNode(place.Offset), nil, 0, false, nil
	case ir.IndexedLocalPlace:
		if place.ID != 0 {
			return 0, nil, nil, 0, false, fmt.Errorf("backend: physical indexed local has ID %d; expected 0", place.ID)
		}
		idx, e := f.expression(place.Index)
		return temporaryMemoryBlock, valueNode(place.Base + place.Offset), idx, place.Stride, true, e
	case ir.MemoryPlace:
		if place.Storage == "exported" {
			return 0, valueNode(place.Offset), nil, 0, false, nil
		}
		block, e := memoryBlock(f.mode, place.Storage)
		if e != nil {
			return 0, nil, nil, 0, false, e
		}
		idx, e := f.expression(place.Index)
		if e != nil {
			return 0, nil, nil, 0, false, e
		}
		if place.Stride > 0 {
			return block, valueNode(place.Offset), idx, place.Stride, true, nil
		}
		if constant, ok := idx.(valueNode); ok {
			return block, valueNode(float64(constant) + float64(place.Offset)), nil, 0, false, nil
		}
		if place.Offset != 0 {
			idx = call(resource.RuntimeFunctionAdd, idx, valueNode(place.Offset))
		}
		return block, idx, nil, 0, false, nil
	default:
		return 0, nil, nil, 0, false, fmt.Errorf("backend: unsupported place %T", place)
	}
}

func (f *finalizer) load(place ir.Place) (snode, error) {
	if memory, ok := place.(ir.MemoryPlace); ok && memory.Storage == "exported" {
		return nil, fmt.Errorf("backend: exported storage cannot be read")
	}
	block, offset, index, stride, shifted, err := f.address(place)
	if err != nil {
		return nil, err
	}
	if shifted {
		return call(resource.RuntimeFunctionGetShifted, valueNode(block), offset, index, valueNode(stride)), nil
	}
	return call(resource.RuntimeFunctionGet, valueNode(block), offset), nil
}

func (f *finalizer) instruction(instruction ir.Instruction) (snode, error) {
	switch instruction := instruction.(type) {
	case ir.Store:
		value, err := f.expression(instruction.Value)
		if err != nil {
			return nil, err
		}
		if memory, ok := instruction.Place.(ir.MemoryPlace); ok && memory.Storage == "exported" {
			return call(resource.RuntimeFunctionExportValue, valueNode(memory.Offset), value), nil
		}
		block, offset, index, stride, shifted, err := f.address(instruction.Place)
		if err != nil {
			return nil, err
		}
		if shifted {
			return call(resource.RuntimeFunctionSetShifted, valueNode(block), offset, index, valueNode(stride), value), nil
		}
		return call(resource.RuntimeFunctionSet, valueNode(block), offset, value), nil
	case ir.Eval:
		return f.expression(instruction.Value)
	default:
		return nil, fmt.Errorf("backend: unsupported instruction %T", instruction)
	}
}

func (f *finalizer) terminator(terminator ir.Terminator, indexes map[int]int, exit int) (snode, error) {
	switch terminator := terminator.(type) {
	case ir.Jump:
		return valueNode(indexes[terminator.Target]), nil
	case ir.Branch:
		condition, err := f.expression(terminator.Condition)
		if err != nil {
			return nil, err
		}
		return call(resource.RuntimeFunctionIf, condition, valueNode(indexes[terminator.True]), valueNode(indexes[terminator.False])), nil
	case ir.Switch:
		value, err := f.expression(terminator.Value)
		if err != nil {
			return nil, err
		}
		cases := append([]ir.SwitchCase(nil), terminator.Cases...)
		sort.SliceStable(cases, func(i, j int) bool { return cases[i].Value < cases[j].Value })
		dense := len(cases) != 0
		for i, item := range cases {
			dense = dense && item.Value == float64(i)
		}
		args := []snode{value}
		if dense {
			for _, item := range cases {
				args = append(args, valueNode(indexes[item.Target]))
			}
			args = append(args, valueNode(indexes[terminator.Default]))
			return call(resource.RuntimeFunctionSwitchIntegerWithDefault, args...), nil
		}
		for _, item := range cases {
			args = append(args, valueNode(item.Value), valueNode(indexes[item.Target]))
		}
		args = append(args, valueNode(indexes[terminator.Default]))
		return call(resource.RuntimeFunctionSwitchWithDefault, args...), nil
	case ir.Return:
		result := snode(valueNode(0))
		if len(terminator.Value.Slots) == 1 {
			var err error
			result, err = f.expression(terminator.Value.Slots[0])
			if err != nil {
				return nil, err
			}
		} else if len(terminator.Value.Slots) > 1 {
			return nil, fmt.Errorf("backend: callback return has %d slots", len(terminator.Value.Slots))
		}
		return call(resource.RuntimeFunctionBreak, valueNode(1), result), nil
	case ir.Unreachable:
		return call(resource.RuntimeFunctionBreak, valueNode(1), valueNode(0)), nil
	default:
		return valueNode(exit), fmt.Errorf("backend: unsupported terminator %T", terminator)
	}
}
