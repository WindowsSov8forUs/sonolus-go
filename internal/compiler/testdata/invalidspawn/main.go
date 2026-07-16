package main

import "github.com/WindowsSov8forUs/sonolus-go/v2/sonolus/play"

type Note struct {
	play.Archetype `archetype:"name=Note"`
}

type NotArchetype struct{ Value float64 }

func (*Note) Preprocess() {
	play.Spawn(NotArchetype{Value: 1})
}

func main() {}
