// Package modecompile contains shared compilation helpers used by the play,
// watch, and preview mode packages. It extracts the common Assemble loop,
// CompileCallback omission rules, and SNode utilities that were previously
// duplicated across the three mode packages.
package modecompile

import (
	"fmt"

	"github.com/WindowsSov8forUs/sonolus-core-go/core/resource"

	"github.com/WindowsSov8forUs/sonolus-go/compiler/snode"
)

// Result is one compiled callback ready to be folded into engine data.
type Result struct {
	ArchetypeIndex int
	Callback       string
	Node           snode.SNode
}

// OmitFunc decides whether a compiled callback should be omitted. It returns
// (omit, handled): if handled is true the general omission rules (pure-constant
// and trailing-zero stripping) are skipped — the caller has already made the
// decision. This is needed by spawnOrder/shouldSpawn which omit only specific
// constant values but keep other constants.
type OmitFunc func(node snode.SNode, callback string) (omit, handled bool)

// SetCallback stores a compiled callback index and declaration order on an
// archetype. The archetype type parameter A is the mode-specific archetype struct.
// order is the 0-based declaration index of this callback within the archetype,
// matching sonolus.js-compiler's {callback}Order convention.
type SetCallback[A any] func(arch *A, callback string, index int, order int) error

// CompileCallback optimizes one archetype callback's SNode tree and applies the
// caller-supplied omission rules. It returns nil when the callback should be
// omitted.
func CompileCallback(archetypeIndex int, callback string, node snode.SNode, omit OmitFunc) *Result {
	s := snode.Peephole(node)

	if omit != nil {
		shouldOmit, handled := omit(s, callback)
		if shouldOmit {
			return nil
		}
		if handled {
			return &Result{ArchetypeIndex: archetypeIndex, Callback: callback, Node: s}
		}
	}

	// A pure constant body does nothing observable.
	if _, ok := s.(snode.Value); ok {
		return nil
	}
	// Execute(..., 0) discards its trailing return value.
	if f, ok := s.(snode.Func); ok &&
		f.Op == resource.RuntimeFunctionExecute &&
		len(f.Args) > 0 {
		if last, ok := f.Args[len(f.Args)-1].(snode.Value); ok && float64(last) == 0 {
			s = ignoreReturn(f)
		}
	}

	return &Result{ArchetypeIndex: archetypeIndex, Callback: callback, Node: s}
}

// Assemble folds compiled callbacks into the engine data skeleton. Each callback's
// SNode tree is appended (with shared dedup) to data.Nodes. nil results are
// skipped. setCb assigns the resulting index to the owning archetype.
func Assemble[A any](
	nodes *[]resource.EngineDataNode,
	archetypes []A,
	results []*Result,
	setCb SetCallback[A],
) error {
	app := snode.NewAppender(nodes)

	// Track the next Order value per archetype. sonolus.js-compiler sets
	// {callback}Order from the archetype's {callback}Order property, which
	// is the declaration order of callbacks. We approximate this by
	// assigning Order in the order callbacks appear in results.
	orders := make(map[int]int)

	for _, c := range results {
		if c == nil {
			continue
		}
		if c.ArchetypeIndex < 0 || c.ArchetypeIndex >= len(archetypes) {
			return fmt.Errorf("assemble: archetype index %d out of range", c.ArchetypeIndex)
		}

		index, err := app.Append(c.Node)
		if err != nil {
			return fmt.Errorf("assemble: archetype %d callback %s: %w", c.ArchetypeIndex, c.Callback, err)
		}

		order := orders[c.ArchetypeIndex]
		orders[c.ArchetypeIndex] = order + 1

		if err := setCb(&archetypes[c.ArchetypeIndex], c.Callback, index, order); err != nil {
			return err
		}
	}

	return nil
}

// NewCallbackSetter creates a SetCallback from a map of callback names to field
// setters. Each setter stores a properly-typed callback value (with the given
// index and order) into the correct field on the archetype.
func NewCallbackSetter[T any](setters map[string]func(*T, int, int)) SetCallback[T] {
	return func(arch *T, cb string, index int, order int) error {
		set, ok := setters[cb]
		if !ok {
			return fmt.Errorf("assemble: unknown callback %q", cb)
		}
		set(arch, index, order)
		return nil
	}
}

// NormalizeSlice returns a non-nil slice so that JSON serialization produces []
// instead of null. It matches the reference toolchain's serialization contract:
// empty collections must serialize as empty arrays, never as null.
func NormalizeSlice[T any](s []T) []T {
	if s == nil {
		return []T{}
	}
	return s
}

// IsConstZero reports whether s is the constant value 0.
func IsConstZero(s snode.SNode) bool {
	v, ok := s.(snode.Value)
	return ok && float64(v) == 0
}

// IsConstNonZero reports whether s is a non-zero constant value.
func IsConstNonZero(s snode.SNode) bool {
	v, ok := s.(snode.Value)
	return ok && float64(v) != 0
}

// ignoreReturn drops the trailing constant return value of an Execute node,
// mirroring build/shared/utils/compile.ts in sonolus.js-compiler.
func ignoreReturn(f snode.Func) snode.SNode {
	if f.Op != resource.RuntimeFunctionExecute {
		return f
	}
	if len(f.Args) == 0 {
		return f
	}
	if _, ok := f.Args[len(f.Args)-1].(snode.Value); !ok {
		return f
	}
	if len(f.Args) == 2 {
		return f.Args[0]
	}
	return snode.Func{
		Op: resource.RuntimeFunctionExecute,
		Args: append([]snode.SNode{}, f.Args[:len(f.Args)-1]...),
	}
}
