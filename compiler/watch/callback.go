// Package watch compiles and assembles Watch-mode engine data.
package watch

import (
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

// watchSetters maps each Watch callback name to its archetype field setter.
var watchSetters = map[string]func(*resource.EngineWatchDataArchetype, int, int){
	"preprocess":       func(a *resource.EngineWatchDataArchetype, i, o int) { a.Preprocess = &resource.EngineWatchDataArchetypeCallback{Index: i, Order: o} },
	"spawnTime":        func(a *resource.EngineWatchDataArchetype, i, o int) { a.SpawnTime = &resource.EngineWatchDataArchetypeCallback{Index: i, Order: o} },
	"despawnTime":      func(a *resource.EngineWatchDataArchetype, i, o int) { a.DespawnTime = &resource.EngineWatchDataArchetypeCallback{Index: i, Order: o} },
	"initialize":       func(a *resource.EngineWatchDataArchetype, i, o int) { a.Initialize = &resource.EngineWatchDataArchetypeCallback{Index: i, Order: o} },
	"updateSequential": func(a *resource.EngineWatchDataArchetype, i, o int) { a.UpdateSequential = &resource.EngineWatchDataArchetypeCallback{Index: i, Order: o} },
	"updateParallel":   func(a *resource.EngineWatchDataArchetype, i, o int) { a.UpdateParallel = &resource.EngineWatchDataArchetypeCallback{Index: i, Order: o} },
	"terminate":        func(a *resource.EngineWatchDataArchetype, i, o int) { a.Terminate = &resource.EngineWatchDataArchetypeCallback{Index: i, Order: o} },
}

// SetWatchCallback is the modecompile.SetCallback for Watch mode, created from the
// watchSetters dispatch table.
var SetWatchCallback = modecompile.NewCallbackSetter(watchSetters)
