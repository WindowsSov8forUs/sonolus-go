package main

import "github.com/WindowsSov8forUs/sonolus-go/sonolus/play"

type Note struct {
	play.Archetype `sonolus:"name=Note"`
	Data           float64 `sonolus:"data"`
	Shared         float64 `sonolus:"shared"`
}

func (n *Note) UpdateParallel() {
	n.Data = 1
	n.Shared = 2
}

func main() {}
