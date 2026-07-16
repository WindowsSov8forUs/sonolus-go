package optimize

import (
	"fmt"
	"sort"

	"github.com/WindowsSov8forUs/sonolus-go/v2/internal/compiler/ir"
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
	prunePhiArguments(function)
	return nil
}

func prunePhiArguments(function *ir.Function) {
	for _, block := range function.Blocks {
		for i := range block.Phis {
			args := block.Phis[i].Args[:0]
			for _, arg := range append([]ir.PhiArg(nil), block.Phis[i].Args...) {
				if arg.Predecessor >= 0 && arg.Predecessor < len(function.Blocks) && terminatorHasTarget(function.Blocks[arg.Predecessor].Terminator, block.ID) {
					args = append(args, arg)
				}
			}
			block.Phis[i].Args = args
		}
	}
}

func terminatorHasTarget(terminator ir.Terminator, target int) bool {
	found := false
	forEachTerminatorTarget(terminator, func(candidate int) {
		if candidate == target {
			found = true
		}
	})
	return found
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
	seen := make([]bool, len(function.Blocks))
	postorder := make([]*ir.Block, 0, len(function.Blocks))
	var visit func(int) error
	visit = func(id int) error {
		if id < 0 || id >= len(function.Blocks) || function.Blocks[id] == nil {
			return fmt.Errorf("block target %d does not exist", id)
		}
		if seen[id] {
			return nil
		}
		seen[id] = true
		block := function.Blocks[id]
		var targetErr error
		forEachTerminatorTarget(block.Terminator, func(target int) {
			if targetErr == nil {
				targetErr = visit(target)
			}
		})
		if targetErr != nil {
			return targetErr
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
	ids := make([]int, len(function.Blocks))
	for index := range ids {
		ids[index] = -1
	}
	for id, block := range postorder {
		ids[block.ID] = id
	}
	entry := ids[function.Entry]
	if entry < 0 {
		return fmt.Errorf("entry block %d is unreachable", function.Entry)
	}
	for id, block := range postorder {
		terminator, err := remapTerminator(block.Terminator, ids)
		if err != nil {
			return err
		}
		block.ID = id
		block.Terminator = terminator
		for phiIndex := range block.Phis {
			args := block.Phis[phiIndex].Args[:0]
			for _, arg := range block.Phis[phiIndex].Args {
				predecessor := ids[arg.Predecessor]
				if predecessor < 0 {
					continue
				}
				arg.Predecessor = predecessor
				args = append(args, arg)
			}
			sort.Slice(args, func(i, j int) bool {
				return args[i].Predecessor < args[j].Predecessor
			})
			block.Phis[phiIndex].Args = args
		}
	}
	function.Entry = entry
	function.Blocks = postorder
	return nil
}

func forEachTerminatorTarget(terminator ir.Terminator, visit func(int)) {
	switch terminator := terminator.(type) {
	case ir.Jump:
		visit(terminator.Target)
	case ir.Branch:
		visit(terminator.True)
		if terminator.False != terminator.True {
			visit(terminator.False)
		}
	case ir.Switch:
		visit(terminator.Default)
		for index, item := range terminator.Cases {
			if item.Target == terminator.Default {
				continue
			}
			duplicate := false
			for previous := 0; previous < index; previous++ {
				duplicate = duplicate || terminator.Cases[previous].Target == item.Target
			}
			if !duplicate {
				visit(item.Target)
			}
		}
	}
}

func terminatorTargetCount(terminator ir.Terminator) int {
	count := 0
	forEachTerminatorTarget(terminator, func(int) { count++ })
	return count
}

func remapTerminator(terminator ir.Terminator, ids []int) (ir.Terminator, error) {
	target := func(old int) (int, error) {
		if old < 0 || old >= len(ids) || ids[old] < 0 {
			return 0, fmt.Errorf("reachable terminator targets removed block %d", old)
		}
		return ids[old], nil
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
		for i, item := range terminator.Cases {
			caseTarget, err := target(item.Target)
			if err != nil {
				return nil, err
			}
			terminator.Cases[i].Target = caseTarget
		}
		terminator.Default = defaultTarget
		return terminator, nil
	default:
		return terminator, nil
	}
}
