//go:build watch

package main

import (
	"github.com/WindowsSov8forUs/sonolus-go/v2/sonolus"
	"github.com/WindowsSov8forUs/sonolus-go/v2/sonolus/watch"
)

var lifecycleRoot = &packageState{Value: 7}
var updateOnlyRoot = &packageState{Value: 13}

type ProbeMemory struct {
	sonolus.LevelMemoryResource
	Observed float64
}

var Probe = ProbeMemory{}

type Writer struct {
	watch.Archetype `archetype:"name=Writer"`
}

func (*Writer) Preprocess() {
	lifecycleRoot.Value = 24
}

type Reader struct {
	watch.Archetype `archetype:"name=Reader"`
}

func (*Reader) Preprocess() {
	Probe.Observed = lifecycleRoot.Value
}

type UpdateOnly struct {
	watch.Archetype `archetype:"name=UpdateOnly"`
}

func (*UpdateOnly) UpdateSequential() {
	Probe.Observed = updateOnlyRoot.Value
}
