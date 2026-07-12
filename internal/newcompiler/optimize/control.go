package optimize

import (
	"fmt"

	"github.com/WindowsSov8forUs/sonolus-go/internal/newcompiler/ir"
)

type FoldConstantControl struct{}

func (FoldConstantControl) Name() string { return "FoldConstantControl" }

func (FoldConstantControl) Run(_ Context, function *ir.Function) error {
	for _, block := range function.Blocks {
		switch terminator := block.Terminator.(type) {
		case ir.Branch:
			if terminator.True == terminator.False {
				block.Terminator = ir.Jump{Target: terminator.True}
				continue
			}
			if condition, ok := terminator.Condition.(ir.Const); ok {
				target := terminator.False
				if condition.Value != 0 {
					target = terminator.True
				}
				block.Terminator = ir.Jump{Target: target}
			}
		case ir.Switch:
			if len(terminator.Cases) == 0 {
				block.Terminator = ir.Jump{Target: terminator.Default}
				continue
			}
			if value, ok := terminator.Value.(ir.Const); ok {
				target := terminator.Default
				for _, item := range terminator.Cases {
					if item.Value == value.Value {
						target = item.Target
						break
					}
				}
				block.Terminator = ir.Jump{Target: target}
			}
		}
	}
	return nil
}

type RemoveUnreachable struct{}

func (RemoveUnreachable) Name() string { return "RemoveUnreachable" }

func (RemoveUnreachable) Run(_ Context, function *ir.Function) error {
	return normalizeReachable(function)
}

type RenumberBlocks struct{}

func (RenumberBlocks) Name() string { return "RenumberBlocks" }

func (RenumberBlocks) Run(_ Context, function *ir.Function) error {
	return normalizeReachable(function)
}

func normalizeReachable(function *ir.Function) error {
	seen := map[int]bool{}
	var postorder []*ir.Block
	var visit func(int) error
	visit = func(id int) error {
		if seen[id] {
			return nil
		}
		if id < 0 || id >= len(function.Blocks) || function.Blocks[id] == nil {
			return fmt.Errorf("block target %d does not exist", id)
		}
		seen[id] = true
		block := function.Blocks[id]
		for _, target := range terminatorTargets(block.Terminator) {
			if err := visit(target); err != nil {
				return err
			}
		}
		postorder = append(postorder, block)
		return nil
	}
	if err := visit(function.Entry); err != nil {
		return err
	}
	for left, right := 0, len(postorder)-1; left < right; left, right = left+1, right-1 {
		postorder[left], postorder[right] = postorder[right], postorder[left]
	}
	ids := make(map[int]int, len(postorder))
	for id, block := range postorder {
		ids[block.ID] = id
	}
	entry, ok := ids[function.Entry]
	if !ok {
		return fmt.Errorf("entry block %d is unreachable", function.Entry)
	}
	for id, block := range postorder {
		terminator, err := remapTerminator(block.Terminator, ids)
		if err != nil {
			return err
		}
		block.ID = id
		block.Terminator = terminator
	}
	function.Entry = entry
	function.Blocks = postorder
	return nil
}

func terminatorTargets(terminator ir.Terminator) []int {
	switch terminator := terminator.(type) {
	case ir.Jump:
		return []int{terminator.Target}
	case ir.Branch:
		return []int{terminator.True, terminator.False}
	case ir.Switch:
		result := []int{terminator.Default}
		for _, item := range terminator.Cases {
			result = append(result, item.Target)
		}
		return result
	default:
		return nil
	}
}

func remapTerminator(terminator ir.Terminator, ids map[int]int) (ir.Terminator, error) {
	target := func(old int) (int, error) {
		id, ok := ids[old]
		if !ok {
			return 0, fmt.Errorf("reachable terminator targets removed block %d", old)
		}
		return id, nil
	}
	switch terminator := terminator.(type) {
	case ir.Jump:
		id, err := target(terminator.Target)
		return ir.Jump{Target: id}, err
	case ir.Branch:
		whenTrue, err := target(terminator.True)
		if err != nil {
			return nil, err
		}
		whenFalse, err := target(terminator.False)
		return ir.Branch{Condition: terminator.Condition, True: whenTrue, False: whenFalse}, err
	case ir.Switch:
		defaultTarget, err := target(terminator.Default)
		if err != nil {
			return nil, err
		}
		cases := make([]ir.SwitchCase, len(terminator.Cases))
		for i, item := range terminator.Cases {
			caseTarget, err := target(item.Target)
			if err != nil {
				return nil, err
			}
			cases[i] = ir.SwitchCase{Value: item.Value, Target: caseTarget}
		}
		return ir.Switch{Value: terminator.Value, Cases: cases, Default: defaultTarget}, nil
	default:
		return terminator, nil
	}
}
