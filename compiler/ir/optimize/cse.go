package optimize

import (
	"strconv"

	"github.com/WindowsSov8forUs/sonolus-go/compiler/ir"
)

// CSE is global common-subexpression elimination using the dominator tree.
// Port of sonolus.py cse.CommonSubexpressionElimination.
type CSE struct{}

func (CSE) Name() string { return "CSE" }

func (CSE) Run(entry *ir.BasicBlock) *ir.BasicBlock {
	dom := ComputeDominance(entry)

	// Phase 1: canonicalize commutative ops.
	for _, b := range ir.Preorder(entry) {
		for i, s := range b.Statements {
			b.Statements[i] = cseCanonicalize(s)
		}
		b.Test = cseCanonicalize(b.Test)
	}

	// Phase 2: dominator-tree extraction.
	cs := cseState{table: map[string]ir.SSAPlace{}, nextID: 0}
	cs.process(entry, dom)
	return entry
}

var cseCommutative = map[ir.Op]bool{
	"Equal": true, "NotEqual": true, "Max": true, "Min": true,
}

type cseState struct {
	table  map[string]ir.SSAPlace
	nextID int
}

func (c *cseState) newSSA() ir.SSAPlace {
	p := ir.SSAPlace{Name: "cse", Num: c.nextID}
	c.nextID++
	return p
}

func (c *cseState) process(block *ir.BasicBlock, dom *Dominance) {
	var added []string
	var stmts []ir.Node

	for _, s := range block.Statements {
		var pre []ir.Node
		s2 := c.rewrite(s, &pre, &added)
		stmts = append(stmts, pre...)
		stmts = append(stmts, s2)
	}
	var pre []ir.Node
	block.Test = c.rewrite(block.Test, &pre, &added)
	stmts = append(stmts, pre...)
	block.Statements = stmts

	for _, child := range dom.DomChildren[block] {
		c.process(child, dom)
	}
	for _, k := range added {
		delete(c.table, k)
	}
}

func (c *cseState) rewrite(n ir.Node, pre *[]ir.Node, added *[]string) ir.Node {
	switch t := n.(type) {
	case ir.Instr:
		k := cseKey(t)
		if p, ok := c.table[k]; ok {
			return ir.Get{Place: p}
		}
		args := make([]ir.Node, len(t.Args))
		for i, a := range t.Args {
			args[i] = c.rewrite(a, pre, added)
		}
		n2 := ir.Instr{Op: t.Op, Args: args, Pure: t.Pure || ir.Pure(t.Op)}
		if ir.Pure(t.Op) && !ir.SideEffects(t.Op) && cseCost(n2) >= 4 {
			p := c.newSSA()
			*pre = append(*pre, ir.Set{Place: p, Value: n2})
			c.table[k] = p
			*added = append(*added, k)
			return ir.Get{Place: p}
		}
		return n2

	case ir.Get:
		bp, ok := t.Place.(ir.BlockPlace)
		if !ok {
			return t
		}
		nb := c.rewrite(bp.Block, pre, added)
		ni := c.rewrite(bp.Index, pre, added)
		if nb != bp.Block || ni != bp.Index {
			return ir.Get{Place: ir.BlockPlace{Block: nb, Index: ni, Offset: bp.Offset}}
		}
		return t

	case ir.Set:
		if bp, ok := t.Place.(ir.BlockPlace); ok {
			p2 := ir.BlockPlace{Block: c.rewrite(bp.Block, pre, added), Index: c.rewrite(bp.Index, pre, added), Offset: bp.Offset}
			v2 := c.rewrite(t.Value, pre, added)
			return ir.Set{Place: p2, Value: v2}
		}
		return t

	default:
		return n
	}
}

func cseCost(n ir.Node) int {
	switch t := n.(type) {
	case ir.Instr:
		s := 1
		for _, a := range t.Args {
			s += cseCost(a)
		}
		return s
	default:
		return 1
	}
}

func cseCanonicalize(n ir.Node) ir.Node {
	t, ok := n.(ir.Instr)
	if !ok || !ir.Pure(t.Op) || ir.SideEffects(t.Op) || !cseCommutative[t.Op] {
		return n
	}
	args := make([]ir.Node, len(t.Args))
	for i, a := range t.Args {
		args[i] = cseCanonicalize(a)
	}
	cseSortArgs(args)
	return ir.Instr{Op: t.Op, Args: args, Pure: true}
}

func cseSortArgs(args []ir.Node) {
	for i := 1; i < len(args); i++ {
		for j := i; j > 0 && cseKey(args[j]) < cseKey(args[j-1]); j-- {
			args[j], args[j-1] = args[j-1], args[j]
		}
	}
}

func cseKey(n ir.Node) string {
	switch t := n.(type) {
	case ir.Const:
		return fmtFloat(float64(t))
	case ir.SSAPlace:
		return t.Name + "." + itoa(t.Num)
	case ir.Instr:
		s := string(t.Op) + "("
		for i, a := range t.Args {
			if i > 0 {
				s += ","
			}
			s += cseKey(a)
		}
		return s + ")"
	case ir.Get:
		return "G(" + cseKey(t.Place) + ")"
	case ir.BlockPlace:
		return "B(" + cseKey(t.Block) + "," + cseKey(t.Index) + "," + itoa(t.Offset) + ")"
	default:
		return "?"
	}
}

func fmtFloat(f float64) string {
	if f == float64(int64(f)) && f != 0 {
		return itoa(int(f))
	}
	return strconv.FormatFloat(f, 'g', -1, 64)
}
