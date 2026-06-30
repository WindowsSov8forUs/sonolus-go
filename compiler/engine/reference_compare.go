package engine

import (
	"fmt"
	"math"

	"github.com/WindowsSov8forUs/sonolus-core-go/core/resource"
)

// compareSNodeTrees compares two EngineDataNode slices for structural
// equivalence. Returns human-readable diffs, or nil if equivalent.
//
// Comparison tolerances:
//   - Floating-point values use epsilon 1e-9.
//   - Node count must match exactly.
func compareSNodeTrees(a, b []resource.EngineDataNode) []string {
	if len(a) != len(b) {
		return []string{fmt.Sprintf("node count: go=%d js=%d", len(a), len(b))}
	}

	var diffs []string
	for i := range a {
		diff := compareNode(a[i], b[i], i)
		diffs = append(diffs, diff...)
	}
	return diffs
}

// compareNode compares two individual EngineDataNode values.
func compareNode(a, b resource.EngineDataNode, index int) []string {
	prefix := fmt.Sprintf("node[%d]", index)

	switch av := a.(type) {
	case resource.EngineDataValueNode:
		bv, ok := b.(resource.EngineDataValueNode)
		if !ok {
			return []string{fmt.Sprintf("%s: go=ValueNode js=%T", prefix, b)}
		}
		if math.Abs(av.Value-bv.Value) > 1e-9 {
			return []string{fmt.Sprintf("%s.value: go=%.10g js=%.10g", prefix, av.Value, bv.Value)}
		}
		return nil

	case resource.EngineDataFunctionNode:
		bv, ok := b.(resource.EngineDataFunctionNode)
		if !ok {
			return []string{fmt.Sprintf("%s: go=FunctionNode js=%T", prefix, b)}
		}
		var diffs []string
		if av.Func != bv.Func {
			diffs = append(diffs, fmt.Sprintf("%s.func: go=%s js=%s", prefix, av.Func, bv.Func))
		}
		diffs = append(diffs, compareIntSlices(av.Args, bv.Args, prefix+".args")...)
		return diffs

	default:
		return []string{fmt.Sprintf("%s: unknown node type go=%T", prefix, a)}
	}
}

// compareIntSlices compares two int slices element by element.
func compareIntSlices(a, b []int, prefix string) []string {
	if len(a) != len(b) {
		return []string{fmt.Sprintf("%s: len go=%d js=%d", prefix, len(a), len(b))}
	}
	var diffs []string
	for i := range a {
		if a[i] != b[i] {
			diffs = append(diffs, fmt.Sprintf("%s[%d]: go=%d js=%d", prefix, i, a[i], b[i]))
		}
	}
	return diffs
}
