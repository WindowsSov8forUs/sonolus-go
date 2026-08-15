//go:build tutorial

package main

import (
	"github.com/WindowsSov8forUs/sonolus-go/v2/sonolus"
	"github.com/WindowsSov8forUs/sonolus-go/v2/sonolus/tutorial"
)

var tutorialRoot = &packageState{Value: 7}

type ProbeMemory struct {
	sonolus.LevelMemoryResource
	Observed float64
}

var Probe = ProbeMemory{}

type Globals struct{ tutorial.GlobalCallbacks }

var Global = Globals{}

func Navigate() {
	Probe.Observed = tutorialRoot.Value
}
