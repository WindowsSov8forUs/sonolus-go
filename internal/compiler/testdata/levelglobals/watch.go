//go:build watch

package main

import (
	"github.com/WindowsSov8forUs/sonolus-go/v2/sonolus"
	"github.com/WindowsSov8forUs/sonolus-go/v2/sonolus/watch"
)

type WatchMemoryState struct {
	sonolus.LevelMemoryResource
	Count int
}

var WatchMemory = WatchMemoryState{}

type WatchDataState struct {
	sonolus.LevelDataResource
	Value float64
}

var WatchData = WatchDataState{}

type WatchGlobalNote struct {
	watch.Archetype `archetype:"name=GlobalNote"`
}

func (*WatchGlobalNote) Preprocess() {
	WatchMemory.Count = 1
	WatchData.Value = 2
}

func (*WatchGlobalNote) UpdateSequential() { WatchMemory.Count++ }
func (*WatchGlobalNote) UpdateParallel()   { _ = WatchData.Value + float64(WatchMemory.Count) }
