package ir

import (
	"crypto/sha256"
	"encoding/hex"
	"hash"
	"math"
	"sort"
	"strconv"
)

// FlowEdge is a directed CFG edge with a branch condition. Cond semantics
// (matching sonolus.py): nil = unconditional / default / true branch; 0 = false
// branch; other numbers = switch cases.
type FlowEdge struct {
	Src  *BasicBlock
	Dst  *BasicBlock
	Cond *float64
}

// Phi is an SSA phi node placed at a control-flow merge. Var is the original
// variable (the temp block) it was created for; Target is the SSA value it
// defines (assigned during SSA renaming); Args maps each predecessor block to
// the SSA value flowing in along that edge.
type Phi struct {
	Var    *TempBlock
	Target Place
	Args   map[*BasicBlock]Place
}

// BasicBlock is a CFG node: phi nodes, a list of statements, a branch test
// expression, and incoming/outgoing edges. Port of sonolus.py BasicBlock.
type BasicBlock struct {
	Phis       []*Phi
	Statements []Node
	Test       Node // defaults to Const(0)
	Incoming   []*FlowEdge
	Outgoing   []*FlowEdge
}

// NewBlock creates an empty block with a default test of Const(0).
func NewBlock() *BasicBlock {
	return &BasicBlock{Test: Const(0)}
}

// ConnectTo adds an edge from b to other with the given condition (nil = default).
func (b *BasicBlock) ConnectTo(other *BasicBlock, cond *float64) {
	edge := &FlowEdge{Src: b, Dst: other, Cond: cond}
	b.Outgoing = append(b.Outgoing, edge)
	other.Incoming = append(other.Incoming, edge)
}

// Cond returns a pointer to a branch condition value, for use with ConnectTo.
func Cond(v float64) *float64 { return &v }

// sortedOutgoing returns b's outgoing edges ordered by (cond is nil, cond):
// concrete conditions ascending first, then the default (nil) edge last. This
// ordering is load-bearing for deterministic block numbering and switch lowering.
func sortedOutgoing(b *BasicBlock) []*FlowEdge {
	edges := append([]*FlowEdge(nil), b.Outgoing...)
	sort.SliceStable(edges, func(i, j int) bool {
		ei, ej := edges[i], edges[j]
		if (ei.Cond == nil) != (ej.Cond == nil) {
			return ei.Cond != nil // non-nil (false) sorts before nil (true)
		}
		if ei.Cond == nil {
			return false
		}
		return *ei.Cond < *ej.Cond
	})
	return edges
}

// ReversePostorder returns reachable blocks in reverse-postorder (the order used
// for block numbering in finalization and dominance).
func ReversePostorder(entry *BasicBlock) []*BasicBlock {
	return traverseReversePostorder(entry)
}

// Preorder returns reachable blocks in a BFS preorder that visits outgoing edges
// in sortedOutgoing order. Mirrors sonolus.py traverse_cfg_preorder.
func Preorder(entry *BasicBlock) []*BasicBlock {
	visited := map[*BasicBlock]bool{}
	var out []*BasicBlock
	queue := []*BasicBlock{entry}
	for len(queue) > 0 {
		b := queue[0]
		queue = queue[1:]
		if visited[b] {
			continue
		}
		visited[b] = true
		out = append(out, b)
		for _, e := range sortedOutgoing(b) {
			queue = append(queue, e.Dst)
		}
	}
	return out
}

// traverseReversePostorder numbers blocks for finalization. It is reversed
// postorder of a DFS that visits outgoing edges in sortedOutgoing order, with
// the entry pre-marked (so back edges to the entry are not retraversed).
func traverseReversePostorder(entry *BasicBlock) []*BasicBlock {
	visited := map[*BasicBlock]bool{entry: true}
	var post []*BasicBlock

	var dfs func(*BasicBlock)
	dfs = func(cur *BasicBlock) {
		for _, e := range sortedOutgoing(cur) {
			if visited[e.Dst] {
				continue
			}
			visited[e.Dst] = true
			dfs(e.Dst)
		}
		post = append(post, cur)
	}
	dfs(entry)

	// reverse
	for i, j := 0, len(post)-1; i < j; i, j = i+1, j-1 {
		post[i], post[j] = post[j], post[i]
	}
	return post
}

// HashCFG computes a deterministic hash of the CFG structure for caching.
// Port of sonolus.py hash_cfg.
func HashCFG(entry *BasicBlock) string {
	blocks := Preorder(entry)
	idx := map[*BasicBlock]int{}
	for i, b := range blocks {
		idx[b] = i
	}

	h := sha256.New()
	for _, b := range blocks {
		writeIntComma(h, idx[b])
		writeIntComma(h, len(b.Statements))
		writeIntComma(h, len(b.Outgoing))
		for _, s := range b.Statements {
			hashNode(h, s)
		}
		if b.Test != nil {
			hashNode(h, b.Test)
		} else {
			h.Write([]byte{0})
		}
	}

	return hex.EncodeToString(h.Sum(nil))
}

func hashNode(h hash.Hash, n Node) {
	switch t := n.(type) {
	case Const:
		h.Write([]byte("c"))
		writeUintHexComma(h, math.Float64bits(float64(t)))
	case Instr:
		h.Write([]byte("i"))
		h.Write([]byte(string(t.Op)))
		writeIntRaw(h, len(t.Args))
		writeBoolRaw(h, t.Pure)
		h.Write([]byte{','})
		for _, a := range t.Args {
			hashNode(h, a)
		}
	case Get:
		h.Write([]byte("g"))
		hashPlace(h, t.Place)
	case Set:
		h.Write([]byte("s"))
		hashPlace(h, t.Place)
		hashNode(h, t.Value)
	case BlockPlace:
		h.Write([]byte("b"))
		hashNode(h, t.Block)
		if t.Index != nil {
			hashNode(h, t.Index)
		} else {
			h.Write([]byte{0})
		}
		writeIntComma(h, t.Offset)
	case SSAPlace:
		h.Write([]byte("v"))
		h.Write([]byte(t.Name))
		writeIntComma(h, t.Num)
	case *TempBlock:
		h.Write([]byte("t"))
		h.Write([]byte(t.Name))
		writeIntComma(h, t.Size)
	}
}

func hashPlace(h hash.Hash, p Place) {
	switch p := p.(type) {
	case BlockPlace:
		h.Write([]byte("b"))
		hashNode(h, p.Block)
		if p.Index != nil {
			hashNode(h, p.Index)
		} else {
			h.Write([]byte{0})
		}
		writeIntComma(h, p.Offset)
	case SSAPlace:
		h.Write([]byte("v"))
		h.Write([]byte(p.Name))
		writeIntComma(h, p.Num)
	}
}

// --- low-allocation hash-writing helpers ---
// Each uses a small stack-allocated buffer and strconv to avoid the
// reflection and format-parsing overhead of fmt.Fprintf.

func writeIntComma(h hash.Hash, v int) {
	var buf [20]byte
	b := strconv.AppendInt(buf[:0], int64(v), 10)
	b = append(b, ',')
	h.Write(b)
}

func writeIntRaw(h hash.Hash, v int) {
	var buf [20]byte
	b := strconv.AppendInt(buf[:0], int64(v), 10)
	h.Write(b)
}

func writeUintHexComma(h hash.Hash, v uint64) {
	var buf [20]byte
	b := strconv.AppendUint(buf[:0], v, 16)
	b = append(b, ',')
	h.Write(b)
}

func writeBoolRaw(h hash.Hash, v bool) {
	if v {
		h.Write([]byte("true"))
	} else {
		h.Write([]byte("false"))
	}
}
