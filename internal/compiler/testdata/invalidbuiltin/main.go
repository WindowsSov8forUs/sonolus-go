package main

import "github.com/WindowsSov8forUs/sonolus-go/sonolus/play"

type Note struct {
	play.Archetype `sonolus:"name=Note"`
}

func (*Note) Preprocess() { _ = max(1.0, 2.0) }

func main() {}
