package ir

import (
	"sort"
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
