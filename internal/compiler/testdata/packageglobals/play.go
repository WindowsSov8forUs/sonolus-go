//go:build play

package main

import (
	"github.com/WindowsSov8forUs/sonolus-go/v2/sonolus"
	"github.com/WindowsSov8forUs/sonolus-go/v2/sonolus/play"
)

var lifecycleRoot = &packageState{Value: 7}
var updateOnlyRoot = &packageState{Value: 13}

type ProbeMemory struct {
	sonolus.LevelMemoryResource
	Observed float64
}

var Probe = ProbeMemory{}

type Writer struct {
	play.Archetype `archetype:"name=Writer"`
}

func (*Writer) Preprocess() {
	lifecycleRoot.Value = 24
}

type Reader struct {
	play.Archetype `archetype:"name=Reader"`
}

func (*Reader) Preprocess() {
	Probe.Observed = lifecycleRoot.Value
}

type UpdateOnly struct {
	play.Archetype `archetype:"name=UpdateOnly"`
}

func (*UpdateOnly) UpdateSequential() {
	Probe.Observed = updateOnlyRoot.Value
}
