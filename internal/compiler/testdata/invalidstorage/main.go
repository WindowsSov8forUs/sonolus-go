package main

import "github.com/WindowsSov8forUs/sonolus-go/sonolus/play"

type Note struct {
	play.Archetype `archetype:"name=Note"`
	Data           float64 `archetype:"data"`
	Shared         float64 `archetype:"shared"`
}

func (n *Note) UpdateParallel() {
	n.Data = 1
	n.Shared = 2
}

func main() {}
