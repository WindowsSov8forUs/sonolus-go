package optimize

import (
	"fmt"
	"sort"

	"github.com/WindowsSov8forUs/sonolus-go/internal/compiler/ir"
)

const TemporaryMemorySlots = 4096

type AllocateBasic struct{}

func (AllocateBasic) Name() string { return "AllocateBasic" }
func (AllocateBasic) Run(_ Context, function *ir.Function) error {
	return allocateLocals(function, false, false)
}

type TryAllocateBasic struct{}

func (TryAllocateBasic) Name() string { return "TryAllocateBasic" }
func (TryAllocateBasic) Run(_ Context, function *ir.Function) error {
	if sequentialSize(function) <= TemporaryMemorySlots {
		return allocateLocals(function, false, false)
	}
	return allocateLocals(function, true, true)
}

type Allocate struct{}

func (Allocate) Name() string          { return "Allocate" }
func (Allocate) Requires() []Analysis  { return []Analysis{AnalysisLiveness} }
func (Allocate) Preserves() []Analysis { return nil }
func (Allocate) Destroys() []Analysis  { return []Analysis{AnalysisLiveness} }
func (Allocate) Run(context Context, function *ir.Function) error {
	return allocateLocalsWithInterference(function, interferenceFor(context, function), false)
}

func sequentialSize(function *ir.Function) int {
	total := 0
	for _, local := range function.Locals {
		total += local.Slots
	}
	return total
}

func allocateLocals(function *ir.Function, reuse, fast bool) error {
	if reuse {
		return allocateLocalsWithInterference(function, localInterference(function), fast)
	}
	return allocateLocalsWithInterference(function, nil, fast)
}

func allocateLocalsWithInterference(function *ir.Function, interference []map[int]bool, fast bool) error {
	if function.Allocated {
		return nil
	}
	offsets := make([]int, len(function.Locals))
	size := 0
	if interference == nil {
		for id, local := range function.Locals {
			offsets[id] = size
			size += local.Slots
		}
	} else {
		order := make([]int, len(function.Locals))
		for i := range order {
			order[i] = i
		}
		sort.SliceStable(order, func(i, j int) bool {
			a, b := function.Locals[order[i]].Slots, function.Locals[order[j]].Slots
			if a != b {
				return a > b
			}
			return order[i] < order[j]
		})
		placed := map[int]bool{}
		for _, id := range order {
			width, offset := function.Locals[id].Slots, 0
			others := make([]int, 0, len(interference[id]))
			for other := range interference[id] {
				if placed[other] {
					others = append(others, other)
				}
			}
			sort.Slice(others, func(i, j int) bool { return offsets[others[i]] < offsets[others[j]] })
			if fast {
				for _, other := range others {
					if end := offsets[other] + function.Locals[other].Slots; end > offset {
						offset = end
					}
				}
			} else {
				for {
					moved := false
					for _, other := range others {
						start, end := offsets[other], offsets[other]+function.Locals[other].Slots
						if offset < end && offset+width > start {
							offset = end
							moved = true
							break
						}
					}
					if !moved {
						break
					}
				}
			}
			offsets[id], placed[id] = offset, true
			if offset+width > size {
				size = offset + width
			}
		}
	}
	if size > TemporaryMemorySlots {
		return fmt.Errorf("Temporary Memory requires %d slots; limit is %d", size, TemporaryMemorySlots)
	}
	remap := func(place ir.Place) ir.Place {
		switch value := place.(type) {
		case ir.LocalPlace:
			if value.ID < 0 || value.ID >= len(offsets) {
				return place
			}
			return ir.LocalPlace{ID: 0, Name: value.Name, Offset: offsets[value.ID] + value.Offset}
		case ir.IndexedLocalPlace:
			if value.ID < 0 || value.ID >= len(offsets) {
				return place
			}
			oldID := value.ID
			value.ID = 0
			value.Base += offsets[oldID]
			return value
		default:
			return place
		}
	}
	for _, block := range function.Blocks {
		for i, instruction := range block.Instructions {
			switch value := instruction.(type) {
			case ir.Store:
				value.Place = remap(rewritePlace(value.Place, func(e ir.Expr) ir.Expr { return e }))
				block.Instructions[i] = value
			}
		}
	}
	rewriteFunctionExpressions(function, func(expr ir.Expr) ir.Expr {
		if load, ok := expr.(ir.Load); ok {
			load.Place = remap(load.Place)
			return load
		}
		return expr
	})
	if size == 0 {
		function.Locals = nil
	} else {
		function.Locals = []ir.Type{{Name: "TemporaryMemory", Slots: size}}
	}
	function.Allocated = true
	return nil
}

func localInterference(function *ir.Function) []map[int]bool {
	n := len(function.Locals)
	use, def := make([]map[int]bool, len(function.Blocks)), make([]map[int]bool, len(function.Blocks))
	for i := range use {
		use[i], def[i] = map[int]bool{}, map[int]bool{}
	}
	for _, block := range function.Blocks {
		for _, instruction := range block.Instructions {
			switch value := instruction.(type) {
			case ir.Store:
				addUsesBeforeDefsExpr(value.Value, use[block.ID], def[block.ID])
				if p, ok := value.Place.(ir.LocalPlace); ok && function.Locals[p.ID].Slots == 1 {
					def[block.ID][p.ID] = true
				} else {
					addUsesBeforeDefsPlace(value.Place, use[block.ID], def[block.ID])
				}
			case ir.Eval:
				addUsesBeforeDefsExpr(value.Value, use[block.ID], def[block.ID])
			}
		}
		addTerminatorUses(block.Terminator, func(expr ir.Expr) {
			addUsesBeforeDefsExpr(expr, use[block.ID], def[block.ID])
		})
	}
	liveIn, liveOut := make([]map[int]bool, len(function.Blocks)), make([]map[int]bool, len(function.Blocks))
	for i := range liveIn {
		liveIn[i], liveOut[i] = map[int]bool{}, map[int]bool{}
	}
	for changed := true; changed; {
		changed = false
		for i := len(function.Blocks) - 1; i >= 0; i-- {
			out := map[int]bool{}
			for _, target := range terminatorTargets(function.Blocks[i].Terminator) {
				for v := range liveIn[target] {
					out[v] = true
				}
			}
			in := map[int]bool{}
			for v := range use[i] {
				in[v] = true
			}
			for v := range out {
				if !def[i][v] {
					in[v] = true
				}
			}
			if !sameSet(out, liveOut[i]) || !sameSet(in, liveIn[i]) {
				liveOut[i], liveIn[i], changed = out, in, true
			}
		}
	}
	graph := make([]map[int]bool, n)
	for i := range graph {
		graph[i] = map[int]bool{}
	}
	for _, block := range function.Blocks {
		live := cloneBoolSet(liveOut[block.ID])
		addTerminatorUses(block.Terminator, func(expr ir.Expr) { localUsesExpr(expr, live) })
		addClique(graph, live)
		for i := len(block.Instructions) - 1; i >= 0; i-- {
			switch value := block.Instructions[i].(type) {
			case ir.Store:
				if place, ok := value.Place.(ir.LocalPlace); ok && function.Locals[place.ID].Slots == 1 {
					for other := range live {
						addInterference(graph, place.ID, other)
					}
					delete(live, place.ID)
				} else {
					localUsesPlace(value.Place, live)
				}
				localUsesExpr(value.Value, live)
			case ir.Eval:
				localUsesExpr(value.Value, live)
			}
			addClique(graph, live)
		}
	}
	return graph
}

func addUsesBeforeDefsExpr(expr ir.Expr, use, def map[int]bool) {
	locals := map[int]bool{}
	localUsesExpr(expr, locals)
	for id := range locals {
		if !def[id] {
			use[id] = true
		}
	}
}

func addUsesBeforeDefsPlace(place ir.Place, use, def map[int]bool) {
	locals := map[int]bool{}
	localUsesPlace(place, locals)
	for id := range locals {
		if !def[id] {
			use[id] = true
		}
	}
}

func addTerminatorUses(terminator ir.Terminator, fn func(ir.Expr)) {
	switch value := terminator.(type) {
	case ir.Branch:
		fn(value.Condition)
	case ir.Switch:
		fn(value.Value)
	case ir.Return:
		for _, slot := range value.Value.Slots {
			fn(slot)
		}
	}
}

func cloneBoolSet(input map[int]bool) map[int]bool {
	result := make(map[int]bool, len(input))
	for id := range input {
		result[id] = true
	}
	return result
}

func addInterference(graph []map[int]bool, a, b int) {
	if a == b || a < 0 || b < 0 || a >= len(graph) || b >= len(graph) {
		return
	}
	graph[a][b] = true
	graph[b][a] = true
}

func addClique(graph []map[int]bool, values map[int]bool) {
	for a := range values {
		for b := range values {
			addInterference(graph, a, b)
		}
	}
}

func sameSet(a, b map[int]bool) bool {
	if len(a) != len(b) {
		return false
	}
	for k := range a {
		if !b[k] {
			return false
		}
	}
	return true
}
