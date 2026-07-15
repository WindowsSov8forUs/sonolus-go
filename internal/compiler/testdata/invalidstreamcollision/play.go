//go:build play

package main

import (
	"github.com/WindowsSov8forUs/sonolus-go/v2/sonolus"
	"github.com/WindowsSov8forUs/sonolus-go/v2/sonolus/play"
)

type ReplayStreams struct {
	sonolus.StreamResource
	Value sonolus.Stream[float64]
}

var Replay = ReplayStreams{}

type Note struct {
	play.Archetype `archetype:"name=Note"`
}

func (*Note) Preprocess() {
	play.Streams.Set(1, 0, 1)
}
