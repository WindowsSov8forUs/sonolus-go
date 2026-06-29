package ir

// Walk traverses the IR tree depth-first, calling visit for each node. Nodes
// are visited bottom-up: children first, then the node itself. Unlike Map,
// Walk does not reconstruct nodes — it is intended for read-only analysis
// (use counting, liveness collection, etc.).
func Walk(n Node, visit func(Node)) {
	if n == nil {
		visit(nil)
		return
	}
	switch t := n.(type) {
	case Instr:
		for _, a := range t.Args {
			Walk(a, visit)
		}
	case Get:
		Walk(t.Place, visit)
	case Set:
		Walk(t.Place, visit)
		Walk(t.Value, visit)
	case BlockPlace:
		Walk(t.Block, visit)
		Walk(t.Index, visit)
	}
	visit(n)
}

// Map transforms the IR tree bottom-up. For each node, children are mapped
// first (with recursive calls to Map), then a structurally-equivalent node is
// reconstructed with the new children (preserving Instr.ID, Set.ID, and
// Instr.Pure), and finally fn is called on that result. fn may return a
// different node to replace the original.
func Map(n Node, fn func(Node) Node) Node {
	if n == nil {
		return fn(nil)
	}
	n = mapChildren(n, fn)
	return fn(n)
}

// mapChildren recursively maps the children of n, returning a structurally-
// equivalent node. It does NOT call fn on n itself — that happens in Map.
func mapChildren(n Node, fn func(Node) Node) Node {
	switch t := n.(type) {
	case Instr:
		args := make([]Node, len(t.Args))
		for i, a := range t.Args {
			args[i] = Map(a, fn)
		}
		return Instr{ID: t.ID, Op: t.Op, Args: args, Pure: t.Pure}
	case Get:
		return Get{Place: mapPlaceChildren(t.Place, fn)}
	case Set:
		return Set{ID: t.ID, Place: mapPlaceChildren(t.Place, fn), Value: Map(t.Value, fn)}
	case BlockPlace:
		idx := t.Index
		if idx != nil {
			idx = Map(idx, fn)
		}
		return BlockPlace{Block: Map(t.Block, fn), Index: idx, Offset: t.Offset}
	default:
		// Const, SSAPlace, *TempBlock — leaves.
		return n
	}
}

// mapPlaceChildren recursively maps the children of a Place. Like mapChildren
// but returns Place specifically. SSAPlace is returned as-is (it has no
// children). The caller is responsible for calling fn on the result if needed.
func mapPlaceChildren(p Place, fn func(Node) Node) Place {
	if bp, ok := p.(BlockPlace); ok {
		idx := bp.Index
		if idx != nil {
			idx = Map(idx, fn)
		}
		return BlockPlace{Block: Map(bp.Block, fn), Index: idx, Offset: bp.Offset}
	}
	// SSAPlace — leaf.
	return p
}
