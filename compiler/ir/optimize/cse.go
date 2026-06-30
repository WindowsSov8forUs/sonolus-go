package optimize

import (
	"bytes"
	"hash"
	"hash/fnv"
	"math"
	"sort"
	"strconv"

	"github.com/WindowsSov8forUs/sonolus-go/compiler/ir"
)

// CSE is global common-subexpression elimination using the dominator tree.
// Port of sonolus.py cse.CommonSubexpressionElimination.
type CSE struct{}

func (CSE) Name() string { return "CSE" }

func (CSE) Run(gen *ir.IDGen, entry *ir.BasicBlock) *ir.BasicBlock {
	return (CSE{}).RunWithDom(gen, entry, &DominanceCache{})
}

func (CSE) RunWithDom(gen *ir.IDGen, entry *ir.BasicBlock, dc *DominanceCache) *ir.BasicBlock {
	dom := dc.Get(entry)

	for _, b := range ir.Preorder(entry) {
		for i, s := range b.Statements {
			b.Statements[i] = cseCanonicalize(s)
		}
		b.Test = cseCanonicalize(b.Test)
	}

	cs := cseState{table: map[cseKeyType]ir.SSAPlace{}, nextID: 0, gen: gen}
	cs.process(entry, dom)
	return entry
}

var cseCommutative = map[ir.Op]bool{
	opEqual: true, opNotEqual: true, opMax: true, opMin: true,
}

// cseKeyType is the map key type for the CSE expression table.
type cseKeyType = [16]byte // fnv.New128a produces 16 bytes

type cseState struct {
	table  map[cseKeyType]ir.SSAPlace
	nextID int
	gen    *ir.IDGen
}

func (c *cseState) newSSA() ir.SSAPlace {
	p := ir.SSAPlace{Name: "cse", Num: c.nextID}
	c.nextID++
	return p
}

func (c *cseState) process(block *ir.BasicBlock, dom *Dominance) {
	var added []cseKeyType
	var stmts []ir.Node
	h := fnv.New128a()

	for _, s := range block.Statements {
		var pre []ir.Node
		s2 := c.rewrite(s, &pre, &added, h)
		stmts = append(stmts, pre...)
		stmts = append(stmts, s2)
	}
	var pre []ir.Node
	block.Test = c.rewrite(block.Test, &pre, &added, h)
	stmts = append(stmts, pre...)
	block.Statements = stmts

	for _, child := range dom.DomChildren[block] {
		c.process(child, dom)
	}
	for _, k := range added {
		delete(c.table, k)
	}
}

func (c *cseState) rewrite(n ir.Node, pre *[]ir.Node, added *[]cseKeyType, h hash.Hash) ir.Node {
	switch t := n.(type) {
	case ir.Instr:
		k := cseKey(t, h)
		if p, ok := c.table[k]; ok {
			return ir.Get{Place: p}
		}
		args := make([]ir.Node, len(t.Args))
		for i, a := range t.Args {
			args[i] = c.rewrite(a, pre, added, h)
		}
		n2 := ir.Instr{ID: t.ID, Op: t.Op, Args: args, Pure: t.Pure || ir.Pure(t.Op)}
		if ir.Pure(t.Op) && !ir.SideEffects(t.Op) && exprCost(n2) >= inlineCostThreshold {
			p := c.newSSA()
			*pre = append(*pre, ir.Set{ID: c.gen.Next(), Place: p, Value: n2})
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
		nb := c.rewrite(bp.Block, pre, added, h)
		ni := c.rewrite(bp.Index, pre, added, h)
		if nb != bp.Block || ni != bp.Index {
			return ir.Get{Place: ir.BlockPlace{Block: nb, Index: ni, Offset: bp.Offset}}
		}
		return t

	case ir.Set:
		if bp, ok := t.Place.(ir.BlockPlace); ok {
			p2 := ir.BlockPlace{Block: c.rewrite(bp.Block, pre, added, h), Index: c.rewrite(bp.Index, pre, added, h), Offset: bp.Offset}
			v2 := c.rewrite(t.Value, pre, added, h)
			return ir.Set{ID: t.ID, Place: p2, Value: v2}
		}
		return t

	default:
		return n
	}
}

func cseCanonicalize(n ir.Node) ir.Node {
	t, ok := n.(ir.Instr)
	if !ok || !ir.Pure(t.Op) || ir.SideEffects(t.Op) || !cseCommutative[t.Op] {
		return n
	}
	type argKey struct {
		arg ir.Node
		key cseKeyType
	}
	pairs := make([]argKey, len(t.Args))
	h := fnv.New128a()
	for i, a := range t.Args {
		pairs[i].arg = cseCanonicalize(a)
		pairs[i].key = cseKey(pairs[i].arg, h)
	}
	sort.Slice(pairs, func(i, j int) bool {
		return bytes.Compare(pairs[i].key[:], pairs[j].key[:]) < 0
	})
	args := make([]ir.Node, len(pairs))
	for i, p := range pairs {
		args[i] = p.arg
	}
	return ir.Instr{ID: t.ID, Op: t.Op, Args: args, Pure: true}
}

// cseKey returns a hash-based key for the given IR node. It uses incremental
// FNV-128 hashing to avoid the allocation overhead of recursive string
// concatenation. The caller provides a pre-allocated hasher; it is reset
// before use so a single hasher can be reused across the hot loop.
func cseKey(n ir.Node, h hash.Hash) cseKeyType {
	h.Reset()
	cseHash(n, h)
	var out cseKeyType
	h.Sum(out[:0])
	return out
}

func cseHash(n ir.Node, h hash.Hash) {
	switch t := n.(type) {
	case ir.Const:
		h.Write([]byte("c"))
		writeFloat64Hex(h, float64(t))
	case ir.SSAPlace:
		h.Write([]byte("v"))
		h.Write([]byte(t.Name))
		writeIntComma(h, t.Num)
	case ir.Instr:
		h.Write([]byte("i"))
		h.Write([]byte(string(t.Op)))
		writeIntComma(h, len(t.Args))
		for _, a := range t.Args {
			cseHash(a, h)
		}
	case ir.Get:
		h.Write([]byte("g"))
		cseHash(t.Place, h)
	case ir.BlockPlace:
		h.Write([]byte("b"))
		cseHash(t.Block, h)
		if t.Index != nil {
			cseHash(t.Index, h)
		} else {
			h.Write([]byte{0})
		}
		writeIntComma(h, t.Offset)
	default:
		h.Write([]byte("?"))
	}
}

// --- zero-allocation hash-writing helpers (from flow.go) ---

func writeFloat64Hex(h hash.Hash, v float64) {
	var buf [20]byte
	b := strconv.AppendUint(buf[:0], math.Float64bits(v), 16)
	b = append(b, ',')
	h.Write(b)
}

func writeIntComma(h hash.Hash, v int) {
	var buf [20]byte
	b := strconv.AppendInt(buf[:0], int64(v), 10)
	b = append(b, ',')
	h.Write(b)
}

// Requires implements ManagedPass — CSE uses dominance via DominanceCache (RunWithDom).
func (CSE) Requires() []Analysis { return nil }

// Preserves implements ManagedPass.
func (CSE) Preserves() []Analysis { return nil }

// Destroys implements ManagedPass.
func (CSE) Destroys() []Analysis { return nil }
