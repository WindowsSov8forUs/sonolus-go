package main

import (
	"github.com/WindowsSov8forUs/sonolus-go/v2/sonolus/play"
	"github.com/WindowsSov8forUs/sonolus-go/v2/sonolus/watch"
)

type Note struct {
	play.Archetype `archetype:"name=Note"`
}

func (*Note) Preprocess() {
	_ = watch.Replay.IsReplay()
}

func main() {}
