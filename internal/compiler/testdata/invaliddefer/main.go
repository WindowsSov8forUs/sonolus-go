package main

import "github.com/WindowsSov8forUs/sonolus-go/v2/sonolus/play"

type Note struct {
	play.Archetype `archetype:"name=Note"`
}

func helper() {}

func (*Note) Preprocess() { defer helper() }

func main() {}
