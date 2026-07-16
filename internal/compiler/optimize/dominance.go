package optimize

import "github.com/WindowsSov8forUs/sonolus-go/v2/internal/compiler/ir"

type Dominance struct {
	Order    []int
	IDom     []int
	Children [][]int
	Frontier []map[int]bool
}

func ComputeDominance(function *ir.Function) *Dominance {
	order := reversePostorder(function)
	n := len(function.Blocks)
	number := make([]int, n)
	for i := range number {
		number[i] = -1
	}
	for i, id := range order {
		number[id] = i
	}
	preds := predecessors(function)
	idom := make([]int, n)
	for i := range idom {
		idom[i] = -1
	}
	idom[function.Entry] = function.Entry
	intersect := func(a, b int) int {
		for a != b {
			for number[a] > number[b] {
				a = idom[a]
			}
			for number[b] > number[a] {
				b = idom[b]
			}
		}
		return a
	}
	for changed := true; changed; {
		changed = false
		for _, b := range order[1:] {
			next := -1
			for _, p := range preds[b] {
				if idom[p] < 0 {
					continue
				}
				if next < 0 {
					next = p
				} else {
					next = intersect(p, next)
				}
			}
			if idom[b] != next {
				idom[b] = next
				changed = true
			}
		}
	}
	children := make([][]int, n)
	for _, b := range order {
		if idom[b] >= 0 && idom[b] != b {
			children[idom[b]] = append(children[idom[b]], b)
		}
	}
	frontier := make([]map[int]bool, n)
	for i := range frontier {
		frontier[i] = map[int]bool{}
	}
	for _, b := range order {
		if len(preds[b]) < 2 {
			continue
		}
		for _, p := range preds[b] {
			for runner := p; runner != idom[b] && runner >= 0; runner = idom[runner] {
				frontier[runner][b] = true
			}
		}
	}
	return &Dominance{Order: order, IDom: idom, Children: children, Frontier: frontier}
}

func reversePostorder(function *ir.Function) []int {
	seen := map[int]bool{}
	var post []int
	var visit func(int)
	visit = func(id int) {
		if seen[id] {
			return
		}
		seen[id] = true
		for _, t := range terminatorTargets(function.Blocks[id].Terminator) {
			visit(t)
		}
		post = append(post, id)
	}
	visit(function.Entry)
	for i, j := 0, len(post)-1; i < j; i, j = i+1, j-1 {
		post[i], post[j] = post[j], post[i]
	}
	return post
}
