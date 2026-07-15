//go:build play

package main

import (
	"github.com/WindowsSov8forUs/sonolus-go/v2/sonolus"
	"github.com/WindowsSov8forUs/sonolus-go/v2/sonolus/play"
)

type ReplayStreams struct {
	sonolus.StreamResource
	Notes   [2]sonolus.Stream[StreamState]
	Summary sonolus.StreamData[sonolus.Vec2]
}

var Replay = ReplayStreams{}

type Note struct {
	play.Archetype `archetype:"name=Note"`
}

func (note *Note) Preprocess() {
	Replay.Notes[0].Set(1, StreamState{Value: 2, Point: sonolus.NewVec2(3, 4)})
	Replay.Summary.Set(sonolus.NewVec2(5, 6))
}
