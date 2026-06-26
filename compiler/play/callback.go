package play

import (
	"github.com/WindowsSov8forUs/sonolus-core-go/core/resource"

	"github.com/WindowsSov8forUs/sonolus-go/compiler/snode"
)

// Callback is a play-mode archetype callback name. The values match the JSON
// field names on EnginePlayDataArchetype.
type Callback string

const (
	CallbackPreprocess       Callback = "preprocess"
	CallbackSpawnOrder       Callback = "spawnOrder"
	CallbackShouldSpawn      Callback = "shouldSpawn"
	CallbackInitialize       Callback = "initialize"
	CallbackUpdateSequential Callback = "updateSequential"
	CallbackTouch            Callback = "touch"
	CallbackUpdateParallel   Callback = "updateParallel"
	CallbackTerminate        Callback = "terminate"
)

// PlayCallbacks lists the play-mode callbacks in their canonical order.
var PlayCallbacks = []Callback{
	CallbackPreprocess,
	CallbackSpawnOrder,
	CallbackShouldSpawn,
	CallbackInitialize,
	CallbackUpdateSequential,
	CallbackTouch,
	CallbackUpdateParallel,
	CallbackTerminate,
}

// CompileResult is one compiled callback ready to be folded into EnginePlayData.
type CompileResult struct {
	ArchetypeIndex int
	Callback       Callback
	Order          int
	Node           snode.SNode
}

// CompileCallback optimizes one archetype callback's SNode tree and applies the
// reference compiler's no-op / undefined rules (sonolus.js-compiler
// build/play/tasks/compile/callback.ts). archetypeIndex identifies the owning
// archetype for later assembly. It returns nil when the callback should be
// omitted from the archetype.
func CompileCallback(archetypeIndex int, cb Callback, node snode.SNode, order int) *CompileResult {
	s := snode.Optimize(node)

	switch cb {
	case CallbackSpawnOrder:
		// A constant-zero spawn order is the default and is omitted.
		if isValueZero(s, true) {
			return nil
		}
	case CallbackShouldSpawn:
		// An always-true (constant non-zero) shouldSpawn is omitted.
		if isValueZero(s, false) {
			return nil
		}
	default:
		// A pure constant body does nothing observable.
		if _, ok := s.(snode.Value); ok {
			return nil
		}
		// Execute(..., 0) discards its trailing return value.
		if f, ok := s.(snode.Func); ok &&
			f.Func == resource.RuntimeFunctionExecute &&
			len(f.Args) > 0 {
			if last, ok := f.Args[len(f.Args)-1].(snode.Value); ok && float64(last) == 0 {
				s = ignoreReturn(f)
			}
		}
	}

	return &CompileResult{ArchetypeIndex: archetypeIndex, Callback: cb, Order: order, Node: s}
}

// isValueZero reports whether s is a constant. When wantZero is true it matches
// the value 0; when false it matches any non-zero value.
func isValueZero(s snode.SNode, wantZero bool) bool {
	v, ok := s.(snode.Value)
	if !ok {
		return false
	}
	if wantZero {
		return float64(v) == 0
	}
	return float64(v) != 0
}

// ignoreReturn drops the trailing constant return value of an Execute node,
// mirroring build/shared/utils/compile.ts.
func ignoreReturn(f snode.Func) snode.SNode {
	if f.Func != resource.RuntimeFunctionExecute {
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
		Func: resource.RuntimeFunctionExecute,
		Args: append([]snode.SNode{}, f.Args[:len(f.Args)-1]...),
	}
}
