package play

import (
	"fmt"

	"github.com/WindowsSov8forUs/sonolus-core-go/core/resource"

	"github.com/WindowsSov8forUs/sonolus-go/compiler/modecompile"
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

// playOmit returns true when play-mode omission rules say the callback should be
// skipped: constant-zero spawnOrder, always-true shouldSpawn.
func playOmit(s snode.SNode, cb string) (omit, handled bool) {
	switch Callback(cb) {
	case CallbackSpawnOrder:
		return modecompile.IsConstZero(s), true
	case CallbackShouldSpawn:
		return modecompile.IsConstNonZero(s), true
	}
	return false, false
}

// CompileCallback optimizes one archetype callback's SNode tree and applies
// play-specific omission rules.
func CompileCallback(archetypeIndex int, cb Callback, node snode.SNode) *modecompile.Result {
	return modecompile.CompileCallback(archetypeIndex, string(cb), node, playOmit)
}

// setPlayCallback assigns a compiled callback to the matching typed field.
func setPlayCallback(arch *resource.EnginePlayDataArchetype, cb string, index int) error {
	value := resource.EnginePlayDataArchetypeCallback{Index: index}
	switch Callback(cb) {
	case CallbackPreprocess:
		arch.Preprocess = &value
	case CallbackSpawnOrder:
		arch.SpawnOrder = &value
	case CallbackShouldSpawn:
		arch.ShouldSpawn = &value
	case CallbackInitialize:
		arch.Initialize = &value
	case CallbackUpdateSequential:
		arch.UpdateSequential = &value
	case CallbackTouch:
		arch.Touch = &value
	case CallbackUpdateParallel:
		arch.UpdateParallel = &value
	case CallbackTerminate:
		arch.Terminate = &value
	default:
		return fmt.Errorf("assemble: unknown play callback %q", cb)
	}
	return nil
}
