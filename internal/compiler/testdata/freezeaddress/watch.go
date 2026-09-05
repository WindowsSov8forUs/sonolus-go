//go:build watch

package main

import (
	"github.com/WindowsSov8forUs/sonolus-go/v2/internal/compiler/testdata/freezeaddress/model"
	"github.com/WindowsSov8forUs/sonolus-go/v2/sonolus"
	"github.com/WindowsSov8forUs/sonolus-go/v2/sonolus/native"
	"github.com/WindowsSov8forUs/sonolus-go/v2/sonolus/watch"
)

type MemoryResource struct {
	sonolus.LevelMemoryResource
	Value model.State
	Pair  model.Pair
}

var memory = MemoryResource{}

type Memory struct{ watch.Archetype }

type Once struct{ watch.Archetype }

func (n *Once) Preprocess() {
	memory.Value.Init()
	memory.Value.Once()
	for _, value := range memory.Value.Values() {
		native.DebugLog(value)
	}
}

type Branch struct {
	watch.Archetype
	Index int `archetype:"imported,name=Index"`
}

func (n *Branch) Preprocess() {
	memory.Value.Init()
	memory.Value.Branch(n.Index != 0)
	for _, value := range memory.Value.Values() {
		native.DebugLog(value)
	}
}

type MemoryAlias struct {
	watch.Archetype
	Index int `archetype:"imported,name=Index"`
	Enter int `archetype:"imported,name=Enter"`
}

func (n *MemoryAlias) Preprocess() {
	memory.Pair.Init()
	for _, value := range memory.Pair.Alias(n.Index, n.Enter != 0) {
		native.DebugLog(value)
	}
}

type LocalAlias struct {
	watch.Archetype
	Index int `archetype:"imported,name=Index"`
	Enter int `archetype:"imported,name=Enter"`
}

func (n *LocalAlias) Preprocess() {
	var pair model.Pair
	pair.Init()
	for _, value := range pair.Alias(n.Index, n.Enter != 0) {
		native.DebugLog(value)
	}
}

type Conversion struct {
	watch.Archetype
	Value float64 `archetype:"imported,name=Value"`
}

func (n *Conversion) Preprocess() {
	for _, value := range model.Conversion(n.Value) {
		native.DebugLog(value)
	}
}

func (n *Memory) Preprocess() {
	memory.Value.Init()
	memory.Value.Exercise()
	for _, value := range memory.Value.Values() {
		native.DebugLog(value)
	}
}

type Local struct {
	watch.Archetype
	Index int `archetype:"imported,name=Index"`
}

func (n *Local) Preprocess() {
	var values [2]State
	values[n.Index].Init()
	values[n.Index].Exercise()
	for _, value := range values[n.Index].Values() {
		native.DebugLog(value)
	}
}

type Static struct{ watch.Archetype }

type ReadMemory struct{ watch.Archetype }

type ReadDirect struct{ watch.Archetype }

func (n *ReadDirect) Preprocess() {
	memory.Value.Init()
	for _, value := range memory.Value.ReadDirect() {
		native.DebugLog(value)
	}
}

func (n *ReadMemory) Preprocess() {
	memory.Value.Init()
	for _, value := range memory.Value.Observe() {
		native.DebugLog(value)
	}
}

func (n *Static) Preprocess() {
	var value State
	value.Init()
	value.Exercise()
	for _, item := range value.Values() {
		native.DebugLog(item)
	}
}
