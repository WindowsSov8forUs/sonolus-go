package play

import (
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

// playSetters maps each Play callback name to its archetype field setter.
var playSetters = map[string]func(*resource.EnginePlayDataArchetype, int, int){
	"preprocess":       func(a *resource.EnginePlayDataArchetype, i, o int) { a.Preprocess = &resource.EnginePlayDataArchetypeCallback{Index: i, Order: o} },
	"spawnOrder":       func(a *resource.EnginePlayDataArchetype, i, o int) { a.SpawnOrder = &resource.EnginePlayDataArchetypeCallback{Index: i, Order: o} },
	"shouldSpawn":      func(a *resource.EnginePlayDataArchetype, i, o int) { a.ShouldSpawn = &resource.EnginePlayDataArchetypeCallback{Index: i, Order: o} },
	"initialize":       func(a *resource.EnginePlayDataArchetype, i, o int) { a.Initialize = &resource.EnginePlayDataArchetypeCallback{Index: i, Order: o} },
	"updateSequential": func(a *resource.EnginePlayDataArchetype, i, o int) { a.UpdateSequential = &resource.EnginePlayDataArchetypeCallback{Index: i, Order: o} },
	"touch":            func(a *resource.EnginePlayDataArchetype, i, o int) { a.Touch = &resource.EnginePlayDataArchetypeCallback{Index: i, Order: o} },
	"updateParallel":   func(a *resource.EnginePlayDataArchetype, i, o int) { a.UpdateParallel = &resource.EnginePlayDataArchetypeCallback{Index: i, Order: o} },
	"terminate":        func(a *resource.EnginePlayDataArchetype, i, o int) { a.Terminate = &resource.EnginePlayDataArchetypeCallback{Index: i, Order: o} },
}

// setPlayCallback is the modecompile.SetCallback for Play mode, created from the
// playSetters dispatch table.
var setPlayCallback = modecompile.NewCallbackSetter(playSetters)
