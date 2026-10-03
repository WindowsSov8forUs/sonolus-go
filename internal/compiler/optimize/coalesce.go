package optimize

import (
	"github.com/WindowsSov8forUs/sonolus-core-go/core/resource"
	"github.com/WindowsSov8forUs/sonolus-go/v2/internal/compiler/ir"
)

type CoalesceFlow struct{}

func (CoalesceFlow) Requires() []Analysis  { return nil }
func (CoalesceFlow) Preserves() []Analysis { return nil }
func (CoalesceFlow) Destroys() []Analysis  { return []Analysis{AnalysisLiveness} }

func (CoalesceFlow) Name() string { return "CoalesceFlow" }

func (CoalesceFlow) Run(_ Context, function *ir.Function) error {
	for {
		if threadRepeatedConditions(function) {
			if err := normalizeReachable(function); err != nil {
				return err
			}
			continue
		}
		changed := false
		forward := make([]int, len(function.Blocks))
		for _, block := range function.Blocks {
			forward[block.ID] = block.ID
			if block.ID == function.Entry || len(block.Phis) != 0 || len(block.Instructions) != 0 {
				continue
			}
			jump, ok := block.Terminator.(ir.Jump)
			if !ok || jump.Target == block.ID || len(function.Blocks[jump.Target].Phis) != 0 {
				continue
			}
			forward[block.ID] = jump.Target
			changed = true
		}
		if changed {
			// A loop backedge can contain an empty forwarding block without
			// forming a cycle in the forwarding graph itself. Only preserve
			// cycles consisting entirely of empty jumps.
			compressForwarding(forward)
			changed = false
			for id, target := range forward {
				changed = changed || id != target
			}
		}
		if changed {
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
				// A unique-predecessor successor can merge even inside a loop.
				// Entry and self edges remain boundaries; Phi predecessor IDs
				// are repaired below when the successor is absorbed.
				if !ok || jump.Target == block.ID || jump.Target == function.Entry || predecessors[jump.Target] != 1 {
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

// An empty successor cannot change a pure condition already evaluated on the
// incoming edge. Keep Phi boundaries intact rather than inventing edge values.
func threadRepeatedConditions(function *ir.Function) bool {
	changed := false
	preds := predecessorCounts(function)
	for _, block := range function.Blocks {
		branch, ok := block.Terminator.(ir.Branch)
		if !ok || expressionHasEffects(branch.Condition) {
			continue
		}
		key := exprKey(branch.Condition)
		thread := func(target int, taken bool) int {
			next := function.Blocks[target]
			// An edge determines logical negation even for non-normalized
			// numbers. Do not replace the number itself (including signed zero).
			// Only rewrite the first pure RHS, before any write can invalidate
			// the observation; shared successors and Phi edges carry no fact.
			if branch.True != branch.False && target != function.Entry && preds[target] == 1 && len(next.Phis) == 0 && len(next.Instructions) != 0 {
				if store, ok := next.Instructions[0].(ir.Store); ok && !expressionHasEffects(store.Value) {
					pureAddress := true
					addPlaceExpressions(store.Place, func(expr ir.Expr) { pureAddress = pureAddress && !expressionHasEffects(expr) })
					if _, constant := branch.Condition.(ir.Const); pureAddress && !constant {
						value, rewritten := rewriteExprChanged(store.Value, func(expr ir.Expr) (ir.Expr, bool) {
							if call, ok := expr.(ir.RuntimeCall); ok && call.Pure && call.Function == resource.RuntimeFunctionNot && len(call.Args) == 1 && exprKey(call.Args[0]) == key {
								value := 1.0
								if taken {
									value = 0
								}
								return ir.Const{Value: value}, true
							}
							return expr, false
						})
						if rewritten {
							store.Value = value
							next.Instructions[0] = store
							changed = true
						}
					}
				}
			}
			if len(next.Phis) != 0 || len(next.Instructions) != 0 {
				return target
			}
			nested, ok := next.Terminator.(ir.Branch)
			if !ok || expressionHasEffects(nested.Condition) || exprKey(nested.Condition) != key {
				return target
			}
			selected := nested.False
			if taken {
				selected = nested.True
			}
			if selected == target || len(function.Blocks[selected].Phis) != 0 {
				return target
			}
			changed = true
			return selected
		}
		branch.True = thread(branch.True, true)
		branch.False = thread(branch.False, false)
		block.Terminator = branch
	}
	return changed
}

func compressForwarding(forward []int) {
	state := make([]uint8, len(forward))
	path := make([]int, 0, len(forward))
	for start := range forward {
		if state[start] != 0 || forward[start] == start {
			continue
		}
		path = path[:0]
		target := start
		for state[target] == 0 && forward[target] != target {
			state[target] = 1
			path = append(path, target)
			target = forward[target]
		}
		if state[target] == 1 {
			for id := target; ; {
				next := forward[id]
				forward[id] = id
				if next == target {
					break
				}
				id = next
			}
		}
		for index := len(path) - 1; index >= 0; index-- {
			id := path[index]
			forward[id] = forward[forward[id]]
			state[id] = 2
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
