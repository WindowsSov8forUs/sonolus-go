package main

import (
	"github.com/WindowsSov8forUs/sonolus-go/v2/sonolus"
	"github.com/WindowsSov8forUs/sonolus-go/v2/sonolus/play"
)

type Note struct {
	play.Archetype `archetype:"name=Note"`
}

func (*Note) Preprocess() { _ = complex(1.0, 2.0) }
func (*Note) Initialize() { sonolus.Unreachable("reachable path") }
func (*Note) UpdateSequential() {
	_ = sonolus.Zero[sonolus.Stream[int]]()
}
func (*Note) UpdateParallel() { _ = new(sonolus.Stream[int]) }

func main() {}
