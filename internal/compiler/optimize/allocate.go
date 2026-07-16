package optimize

import (
	"fmt"
	"math/bits"
	"sort"

	"github.com/WindowsSov8forUs/sonolus-go/v2/internal/compiler/ir"
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

func allocateLocalsWithInterference(function *ir.Function, interference interferenceGraph, fast bool) error {
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
		placed := newBitSet(len(function.Locals))
		occupied := newBitSet(TemporaryMemorySlots)
		for _, id := range order {
			width, offset := function.Locals[id].Slots, 0
			if fast {
				interference[id].each(func(other int) {
					if placed.has(other) {
						if end := offsets[other] + function.Locals[other].Slots; end > offset {
							offset = end
						}
					}
				})
			} else {
				for index := range occupied {
					occupied[index] = 0
				}
				interference[id].each(func(other int) {
					if !placed.has(other) {
						return
					}
					start := offsets[other]
					for slot, end := start, min(TemporaryMemorySlots, start+function.Locals[other].Slots); slot < end; slot++ {
						occupied.set(slot)
					}
				})
				limit := TemporaryMemorySlots - width
				for offset <= limit {
					available := true
					for slot := offset; slot < offset+width; slot++ {
						if occupied.has(slot) {
							offset = slot + 1
							available = false
							break
						}
					}
					if available {
						break
					}
				}
				if offset > limit {
					offset = TemporaryMemorySlots
				}
			}
			offsets[id] = offset
			placed.set(id)
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
				value.Place = remap(value.Place)
				block.Instructions[i] = value
			}
		}
	}
	rewriteFunctionExpressionsChanged(function, func(expr ir.Expr) (ir.Expr, bool) {
		if load, ok := expr.(ir.Load); ok {
			load.Place = remap(load.Place)
			return load, true
		}
		return expr, false
	})
	if size == 0 {
		function.Locals = nil
	} else {
		function.Locals = []ir.Type{{Name: "TemporaryMemory", Slots: size}}
	}
	function.Allocated = true
	return nil
}

type bitSet []uint64

func newBitSet(size int) bitSet { return make(bitSet, (size+63)/64) }

func (set bitSet) clone() bitSet { return append(bitSet(nil), set...) }

func (set bitSet) set(id int) {
	if id >= 0 && id/64 < len(set) {
		set[id/64] |= uint64(1) << (id % 64)
	}
}

func (set bitSet) clear(id int) {
	if id >= 0 && id/64 < len(set) {
		set[id/64] &^= uint64(1) << (id % 64)
	}
}

func (set bitSet) has(id int) bool {
	return id >= 0 && id/64 < len(set) && set[id/64]&(uint64(1)<<(id%64)) != 0
}

func (set bitSet) union(other bitSet) {
	for index := range set {
		set[index] |= other[index]
	}
}

func (set bitSet) andNot(other bitSet) {
	for index := range set {
		set[index] &^= other[index]
	}
}

func (set bitSet) equal(other bitSet) bool {
	if len(set) != len(other) {
		return false
	}
	for index := range set {
		if set[index] != other[index] {
			return false
		}
	}
	return true
}

func (set bitSet) each(visit func(int)) {
	for wordIndex, word := range set {
		for word != 0 {
			bit := bits.TrailingZeros64(word)
			visit(wordIndex*64 + bit)
			word &^= uint64(1) << bit
		}
	}
}

func (set bitSet) count() int {
	total := 0
	for _, word := range set {
		total += bits.OnesCount64(word)
	}
	return total
}

type interferenceGraph []bitSet

func newInterferenceGraph(size int) interferenceGraph {
	graph := make(interferenceGraph, size)
	words := (size + 63) / 64
	storage := make(bitSet, size*words)
	for index := range graph {
		graph[index] = storage[index*words : (index+1)*words]
	}
	return graph
}

func (graph interferenceGraph) clone() interferenceGraph {
	result := newInterferenceGraph(len(graph))
	for index := range graph {
		copy(result[index], graph[index])
	}
	return result
}

func localInterference(function *ir.Function) interferenceGraph {
	n := len(function.Locals)
	use, def := make([]bitSet, len(function.Blocks)), make([]bitSet, len(function.Blocks))
	for i := range use {
		use[i], def[i] = newBitSet(n), newBitSet(n)
	}
	for _, block := range function.Blocks {
		for _, instruction := range block.Instructions {
			switch value := instruction.(type) {
			case ir.Store:
				addUsesBeforeDefsExpr(value.Value, use[block.ID], def[block.ID])
				if p, ok := value.Place.(ir.LocalPlace); ok && function.Locals[p.ID].Slots == 1 {
					def[block.ID].set(p.ID)
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
	liveIn, liveOut := make([]bitSet, len(function.Blocks)), make([]bitSet, len(function.Blocks))
	for i := range liveIn {
		liveIn[i], liveOut[i] = newBitSet(n), newBitSet(n)
	}
	for changed := true; changed; {
		changed = false
		for i := len(function.Blocks) - 1; i >= 0; i-- {
			out := newBitSet(n)
			forEachTerminatorTarget(function.Blocks[i].Terminator, func(target int) {
				out.union(liveIn[target])
			})
			in := out.clone()
			in.andNot(def[i])
			in.union(use[i])
			if !out.equal(liveOut[i]) || !in.equal(liveIn[i]) {
				liveOut[i], liveIn[i], changed = out, in, true
			}
		}
	}
	graph := newInterferenceGraph(n)
	for _, block := range function.Blocks {
		live := liveOut[block.ID].clone()
		addTerminatorUses(block.Terminator, func(expr ir.Expr) { localUsesExprBitSet(expr, live) })
		addClique(graph, live)
		for i := len(block.Instructions) - 1; i >= 0; i-- {
			uses := newBitSet(n)
			switch value := block.Instructions[i].(type) {
			case ir.Store:
				if place, ok := value.Place.(ir.LocalPlace); ok && function.Locals[place.ID].Slots == 1 {
					addInterferenceSet(graph, place.ID, live)
					live.clear(place.ID)
				} else {
					localUsesPlaceBitSet(value.Place, uses)
				}
				localUsesExprBitSet(value.Value, uses)
			case ir.Eval:
				localUsesExprBitSet(value.Value, uses)
			}
			addLiveUses(graph, live, uses)
		}
	}
	return graph
}

func addLiveUses(graph interferenceGraph, live, uses bitSet) {
	uses.each(func(id int) {
		if live.has(id) {
			return
		}
		addInterferenceSet(graph, id, live)
		live.set(id)
	})
}

func addUsesBeforeDefsExpr(expr ir.Expr, use, def bitSet) {
	locals := newBitSet(len(use) * 64)
	localUsesExprBitSet(expr, locals)
	locals.each(func(id int) {
		if !def.has(id) {
			use.set(id)
		}
	})
}

func addUsesBeforeDefsPlace(place ir.Place, use, def bitSet) {
	locals := newBitSet(len(use) * 64)
	localUsesPlaceBitSet(place, locals)
	locals.each(func(id int) {
		if !def.has(id) {
			use.set(id)
		}
	})
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

func localUsesExprBitSet(expr ir.Expr, uses bitSet) {
	switch value := expr.(type) {
	case ir.Load:
		localUsesPlaceBitSet(value.Place, uses)
	case ir.RuntimeCall:
		for _, argument := range value.Args {
			localUsesExprBitSet(argument, uses)
		}
	}
}

func localUsesPlaceBitSet(place ir.Place, uses bitSet) {
	switch value := place.(type) {
	case ir.LocalPlace:
		uses.set(value.ID)
	case ir.IndexedLocalPlace:
		uses.set(value.ID)
		localUsesExprBitSet(value.Index, uses)
	case ir.MemoryPlace:
		localUsesExprBitSet(value.Index, uses)
	}
}

func addInterference(graph interferenceGraph, a, b int) {
	if a == b || a < 0 || b < 0 || a >= len(graph) || b >= len(graph) {
		return
	}
	graph[a].set(b)
	graph[b].set(a)
}

func addInterferenceSet(graph interferenceGraph, id int, others bitSet) {
	if id < 0 || id >= len(graph) {
		return
	}
	graph[id].union(others)
	graph[id].clear(id)
	others.each(func(other int) {
		if other >= 0 && other < len(graph) && other != id {
			graph[other].set(id)
		}
	})
}

func addClique(graph interferenceGraph, values bitSet) {
	values.each(func(a int) {
		if a >= 0 && a < len(graph) {
			graph[a].union(values)
			graph[a].clear(a)
		}
	})
}
