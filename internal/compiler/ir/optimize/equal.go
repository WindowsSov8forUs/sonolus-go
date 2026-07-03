package optimize

import "github.com/WindowsSov8forUs/sonolus-go/internal/compiler/ir"

// placeEqual reports whether two Places are structurally equal.
func placeEqual(a, b ir.Place) bool {
	pa, oka := a.(ir.BlockPlace)
	pb, okb := b.(ir.BlockPlace)
	if oka && okb {
		return nodeEqual(pa.Block, pb.Block) && nodeEqual(pa.Index, pb.Index) && pa.Offset == pb.Offset
	}
	// SSAPlace, *TempBlock, and nil are compared by identity (canonical by construction).
	return a == b
}

// nodeEqual reports whether two IR nodes are structurally equal.
func nodeEqual(a, b ir.Node) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	switch ta := a.(type) {
	case ir.Const:
		tb, ok := b.(ir.Const)
		return ok && float64(ta) == float64(tb)
	case *ir.TempBlock:
		tb, ok := b.(*ir.TempBlock)
		return ok && ta == tb
	case ir.Get:
		tb, ok := b.(ir.Get)
		return ok && placeEqual(ta.Place, tb.Place)
	case ir.Set:
		tb, ok := b.(ir.Set)
		return ok && placeEqual(ta.Place, tb.Place) && nodeEqual(ta.Value, tb.Value)
	case ir.Instr:
		tb, ok := b.(ir.Instr)
		return ok && ta.Op == tb.Op && nodeSliceEqual(ta.Args, tb.Args)
	case ir.BlockPlace:
		tb, ok := b.(ir.BlockPlace)
		return ok && placeEqual(ta, tb)
	case ir.SSAPlace:
		_, ok := b.(ir.SSAPlace)
		return ok && ta == b
	default:
		return false
	}
}

// nodeSliceEqual reports whether two slices of IR nodes are structurally equal.
func nodeSliceEqual(a, b []ir.Node) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if !nodeEqual(a[i], b[i]) {
			return false
		}
	}
	return true
}
