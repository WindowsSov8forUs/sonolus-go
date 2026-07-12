package optimize

import "github.com/WindowsSov8forUs/sonolus-go/internal/newcompiler/ir"

type CoalesceFlow struct{}

func (CoalesceFlow) Name() string { return "CoalesceFlow" }

func (CoalesceFlow) Run(_ Context, function *ir.Function) error {
	for {
		predecessors := predecessorCounts(function)
		changed := false

		for _, block := range function.Blocks {
			if block.ID == function.Entry || len(block.Instructions) != 0 {
				continue
			}
			jump, ok := block.Terminator.(ir.Jump)
			if !ok || jump.Target == block.ID || inCycle(function, block.ID) {
				continue
			}
			replaceTarget(function, block.ID, jump.Target)
			changed = true
			break
		}
		if changed {
			if err := normalizeReachable(function); err != nil {
				return err
			}
			continue
		}

		for _, block := range function.Blocks {
			jump, ok := block.Terminator.(ir.Jump)
			if !ok || jump.Target == block.ID || jump.Target == function.Entry || predecessors[jump.Target] != 1 || inCycle(function, jump.Target) {
				continue
			}
			target := function.Blocks[jump.Target]
			block.Instructions = append(block.Instructions, target.Instructions...)
			block.Terminator = target.Terminator
			changed = true
			break
		}
		if !changed {
			return nil
		}
		if err := normalizeReachable(function); err != nil {
			return err
		}
	}
}

func predecessorCounts(function *ir.Function) []int {
	result := make([]int, len(function.Blocks))
	for _, block := range function.Blocks {
		for _, target := range terminatorTargets(block.Terminator) {
			if target >= 0 && target < len(result) {
				result[target]++
			}
		}
	}
	return result
}

func replaceTarget(function *ir.Function, from, to int) {
	for _, block := range function.Blocks {
		switch terminator := block.Terminator.(type) {
		case ir.Jump:
			if terminator.Target == from {
				terminator.Target = to
				block.Terminator = terminator
			}
		case ir.Branch:
			if terminator.True == from {
				terminator.True = to
			}
			if terminator.False == from {
				terminator.False = to
			}
			block.Terminator = terminator
		case ir.Switch:
			if terminator.Default == from {
				terminator.Default = to
			}
			for i := range terminator.Cases {
				if terminator.Cases[i].Target == from {
					terminator.Cases[i].Target = to
				}
			}
			block.Terminator = terminator
		}
	}
}

func inCycle(function *ir.Function, id int) bool {
	for _, target := range terminatorTargets(function.Blocks[id].Terminator) {
		if reaches(function, target, id, map[int]bool{}) {
			return true
		}
	}
	return false
}

func reaches(function *ir.Function, current, target int, seen map[int]bool) bool {
	if current == target {
		return true
	}
	if current < 0 || current >= len(function.Blocks) || seen[current] {
		return false
	}
	seen[current] = true
	for _, next := range terminatorTargets(function.Blocks[current].Terminator) {
		if reaches(function, next, target, seen) {
			return true
		}
	}
	return false
}
