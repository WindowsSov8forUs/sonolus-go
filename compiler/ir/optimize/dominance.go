package optimize

import (
	"fmt"
	"hash/fnv"

	"github.com/WindowsSov8forUs/sonolus-go/compiler/ir"
)

// DominanceCache caches the dominance tree across multiple passes within a
// single Optimize() invocation. It invalidates automatically when the CFG
// structure changes (detected via structural hash).
type DominanceCache struct {
	dom  *Dominance
	hash uint64
}

// Get returns the cached dominance tree, recomputing it if the CFG structure
// has changed since the last call.
func (c *DominanceCache) Get(entry *ir.BasicBlock) *Dominance {
	h := cfgStructuralHash(entry)
	if c.dom != nil && c.hash == h {
		return c.dom
	}
	c.dom = ComputeDominance(entry)
	c.hash = h
	return c.dom
}

// Invalidate forces the next Get to recompute the dominance tree regardless
// of CFG structure changes. Call after passes that modify the CFG structurally.
func (c *DominanceCache) Invalidate() {
	c.dom = nil
}

// cfgStructuralHash computes a fast hash of the CFG topology (blocks + edges).
// It uses block pointer identity and edge targets; instruction content is ignored
// since dominance depends only on graph shape.
func cfgStructuralHash(entry *ir.BasicBlock) uint64 {
	h := fnv.New64()
	for i, b := range ir.Preorder(entry) {
		// Mix block preorder index (monotonically assigned) and edge targets.
		fmt.Fprintf(h, "%d", i)
		for _, e := range b.Outgoing {
			fmt.Fprintf(h, "%d", preorderIndex(e.Dst, entry))
		}
	}
	return h.Sum64()
}

// preorderIndex returns the preorder index of b within the CFG rooted at entry.
func preorderIndex(b *ir.BasicBlock, entry *ir.BasicBlock) int {
	for i, blk := range ir.Preorder(entry) {
		if blk == b {
			return i
		}
	}
	return -1
}

// Dominance holds dominator information for a CFG: reverse-postorder, block
// numbering, immediate dominators, the dominator-tree children, and dominance
// frontiers. Port of sonolus.py dominance.DominanceFrontiers (returned as data
// rather than stored on blocks).
type Dominance struct {
	Order       []*ir.BasicBlock
	Num         map[*ir.BasicBlock]int
	IDom        map[*ir.BasicBlock]*ir.BasicBlock
	DomChildren map[*ir.BasicBlock][]*ir.BasicBlock
	DF          map[*ir.BasicBlock]map[*ir.BasicBlock]bool
}

// ComputeDominance computes dominators and dominance frontiers for the CFG
// rooted at entry using the Cooper-Harvey-Kennedy algorithm.
func ComputeDominance(entry *ir.BasicBlock) *Dominance {
	order := ir.ReversePostorder(entry)
	d := &Dominance{
		Order:       order,
		Num:         make(map[*ir.BasicBlock]int, len(order)),
		IDom:        make(map[*ir.BasicBlock]*ir.BasicBlock, len(order)),
		DomChildren: map[*ir.BasicBlock][]*ir.BasicBlock{},
		DF:          map[*ir.BasicBlock]map[*ir.BasicBlock]bool{},
	}
	for i, b := range order {
		d.Num[b] = i
		d.IDom[b] = nil
	}
	d.IDom[entry] = entry

	// Iteratively compute immediate dominators.
	for changed := true; changed; {
		changed = false
		for _, b := range order[1:] {
			var newIDom *ir.BasicBlock
			for _, e := range b.Incoming {
				p := e.Src
				if _, ok := d.Num[p]; !ok || d.IDom[p] == nil {
					continue
				}
				if newIDom == nil {
					newIDom = p
				} else {
					newIDom = d.intersect(p, newIDom)
				}
			}
			if d.IDom[b] != newIDom {
				d.IDom[b] = newIDom
				changed = true
			}
		}
	}

	// Dominator tree (children in reverse-postorder).
	for _, b := range order {
		if idom := d.IDom[b]; idom != nil && idom != b {
			d.DomChildren[idom] = append(d.DomChildren[idom], b)
		}
	}

	// Dominance frontiers.
	for _, b := range order {
		d.DF[b] = map[*ir.BasicBlock]bool{}
	}
	for _, b := range order {
		if reachablePredCount(d, b) < 2 {
			continue
		}
		for _, e := range b.Incoming {
			p := e.Src
			if _, ok := d.Num[p]; !ok {
				continue
			}
			for runner := p; runner != d.IDom[b]; runner = d.IDom[runner] {
				d.DF[runner][b] = true
			}
		}
	}
	return d
}

func (d *Dominance) intersect(b1, b2 *ir.BasicBlock) *ir.BasicBlock {
	for b1 != b2 {
		for d.Num[b1] > d.Num[b2] {
			b1 = d.IDom[b1]
		}
		for d.Num[b2] > d.Num[b1] {
			b2 = d.IDom[b2]
		}
	}
	return b1
}

func reachablePredCount(d *Dominance, b *ir.BasicBlock) int {
	n := 0
	for _, e := range b.Incoming {
		if _, ok := d.Num[e.Src]; ok {
			n++
		}
	}
	return n
}
