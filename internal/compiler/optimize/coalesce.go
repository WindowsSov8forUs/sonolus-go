package optimize

import "github.com/WindowsSov8forUs/sonolus-go/v2/internal/compiler/ir"

type CoalesceFlow struct{}

func (CoalesceFlow) Requires() []Analysis  { return nil }
func (CoalesceFlow) Preserves() []Analysis { return []Analysis{AnalysisLiveness} }
func (CoalesceFlow) Destroys() []Analysis  { return nil }

func (CoalesceFlow) Name() string { return "CoalesceFlow" }

func (CoalesceFlow) Run(_ Context, function *ir.Function) error {
	for {
		cycles := cyclicBlocks(function)
		changed := false
		forward := make([]int, len(function.Blocks))
		for _, block := range function.Blocks {
			forward[block.ID] = block.ID
			if block.ID == function.Entry || len(block.Phis) != 0 || len(block.Instructions) != 0 {
				continue
			}
			jump, ok := block.Terminator.(ir.Jump)
			if !ok || jump.Target == block.ID || len(function.Blocks[jump.Target].Phis) != 0 || cycles[block.ID] {
				continue
			}
			forward[block.ID] = jump.Target
			changed = true
		}
		if changed {
			// Eligible blocks are outside cycles. Compress all forwarding chains
			// before rewriting edges, then normalize the graph once for the batch.
			for id := range forward {
				target := id
				for forward[target] != target {
					target = forward[target]
				}
				for current := id; forward[current] != target; {
					next := forward[current]
					forward[current] = target
					current = next
				}
			}
			for _, block := range function.Blocks {
				terminator, err := remapTerminator(block.Terminator, forward)
				if err != nil {
					return err
				}
				block.Terminator = terminator
			}
			if err := normalizeReachable(function); err != nil {
				return err
			}
			continue
		}
		predecessors := predecessorCounts(function)
		removed := make([]bool, len(function.Blocks))
		for _, block := range function.Blocks {
			if removed[block.ID] {
				continue
			}
			for {
				jump, ok := block.Terminator.(ir.Jump)
				if !ok || jump.Target == block.ID || jump.Target == function.Entry || predecessors[jump.Target] != 1 || cycles[jump.Target] {
					break
				}
				target := function.Blocks[jump.Target]
				for _, phi := range target.Phis {
					for _, arg := range phi.Args {
						if arg.Predecessor == block.ID {
							block.Instructions = append(block.Instructions, ir.Store{Place: phi.Target, Value: ir.Load{Place: arg.Value}})
							break
						}
					}
				}
				block.Instructions = append(block.Instructions, target.Instructions...)
				block.Terminator = target.Terminator
				forEachTerminatorTarget(block.Terminator, func(successor int) {
					for i := range function.Blocks[successor].Phis {
						for j := range function.Blocks[successor].Phis[i].Args {
							if function.Blocks[successor].Phis[i].Args[j].Predecessor == target.ID {
								function.Blocks[successor].Phis[i].Args[j].Predecessor = block.ID
							}
						}
					}
				})
				removed[target.ID] = true
				changed = true
			}
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
		forEachTerminatorTarget(block.Terminator, func(target int) {
			if target >= 0 && target < len(result) {
				result[target]++
			}
		})
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

func cyclicBlocks(function *ir.Function) []bool {
	count := len(function.Blocks)
	indices := make([]int, count)
	lowLink := make([]int, count)
	onStack := make([]bool, count)
	for index := range indices {
		indices[index] = -1
	}
	stack := make([]int, 0, count)
	cycles := make([]bool, count)
	nextIndex := 0
	var visit func(int)
	visit = func(block int) {
		indices[block], lowLink[block] = nextIndex, nextIndex
		nextIndex++
		stack = append(stack, block)
		onStack[block] = true
		forEachTerminatorTarget(function.Blocks[block].Terminator, func(target int) {
			if target < 0 || target >= count {
				return
			}
			if indices[target] < 0 {
				visit(target)
				lowLink[block] = min(lowLink[block], lowLink[target])
			} else if onStack[target] {
				lowLink[block] = min(lowLink[block], indices[target])
			}
		})
		if lowLink[block] != indices[block] {
			return
		}
		start := len(stack) - 1
		for start > 0 && stack[start] != block {
			start--
		}
		component := stack[start:]
		stack = stack[:start]
		for _, member := range component {
			onStack[member] = false
		}
		if len(component) > 1 {
			for _, member := range component {
				cycles[member] = true
			}
			return
		}
		forEachTerminatorTarget(function.Blocks[block].Terminator, func(target int) {
			if target == block {
				cycles[block] = true
			}
		})
	}
	for block := range function.Blocks {
		if indices[block] < 0 {
			visit(block)
		}
	}
	return cycles
}
