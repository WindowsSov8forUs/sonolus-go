//go:build preview

package main

import (
	"github.com/WindowsSov8forUs/sonolus-go/v2/sonolus"
	"github.com/WindowsSov8forUs/sonolus-go/v2/sonolus/preview"
)

var lifecycleRoot = &packageState{Value: 7}

type ProbeData struct {
	sonolus.LevelDataResource
	Observed float64
}

var Probe = ProbeData{}

type Writer struct {
	preview.Archetype `archetype:"name=Writer"`
}

func (*Writer) Preprocess() {
	lifecycleRoot.Value = 24
}

type Reader struct {
	preview.Archetype `archetype:"name=Reader"`
}

func (*Reader) Preprocess() {
	Probe.Observed = lifecycleRoot.Value
}
