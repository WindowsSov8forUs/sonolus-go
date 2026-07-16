package main

import "github.com/WindowsSov8forUs/sonolus-go/sonolus/play"

type Note struct {
	play.Archetype `sonolus:"name=Note"`
}

func helper() {}

func (*Note) Preprocess() { defer helper() }

func main() {}
