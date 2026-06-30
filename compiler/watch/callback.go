// Package watch compiles and assembles Watch-mode engine data.
package watch

import (
	"fmt"

	"github.com/WindowsSov8forUs/sonolus-core-go/core/resource"

	"github.com/WindowsSov8forUs/sonolus-go/compiler/modecompile"
	"github.com/WindowsSov8forUs/sonolus-go/compiler/snode"
)

// Callback is a watch-mode archetype callback name.
type Callback string

const (
	CallbackPreprocess       Callback = "preprocess"
	CallbackSpawnTime        Callback = "spawnTime"
	CallbackDespawnTime      Callback = "despawnTime"
	CallbackInitialize       Callback = "initialize"
	CallbackUpdateSequential Callback = "updateSequential"
	CallbackUpdateParallel   Callback = "updateParallel"
	CallbackTerminate        Callback = "terminate"
)

func watchOmit(s snode.SNode, cb string) (omit, handled bool) {
	switch Callback(cb) {
	case CallbackSpawnTime, CallbackDespawnTime:
		return modecompile.IsConstZero(s), true
	}
	return false, false
}

// CompileCallback optimizes one archetype callback's SNode tree and applies
// watch-specific omission rules.
func CompileCallback(archetypeIndex int, cb Callback, node snode.SNode) *modecompile.Result {
	return modecompile.CompileCallback(archetypeIndex, string(cb), node, watchOmit)
}

// SetWatchCallback assigns a compiled callback index to the matching typed field
// on a Watch-mode archetype. It satisfies modecompile.SetCallback[*EngineWatchDataArchetype].
func SetWatchCallback(arch *resource.EngineWatchDataArchetype, cb string, index int) error {
	value := resource.EngineWatchDataArchetypeCallback{Index: index}
	switch Callback(cb) {
	case CallbackPreprocess:
		arch.Preprocess = &value
	case CallbackSpawnTime:
		arch.SpawnTime = &value
	case CallbackDespawnTime:
		arch.DespawnTime = &value
	case CallbackInitialize:
		arch.Initialize = &value
	case CallbackUpdateSequential:
		arch.UpdateSequential = &value
	case CallbackUpdateParallel:
		arch.UpdateParallel = &value
	case CallbackTerminate:
		arch.Terminate = &value
	default:
		return fmt.Errorf("assemble: unknown watch callback %q", cb)
	}
	return nil
}
