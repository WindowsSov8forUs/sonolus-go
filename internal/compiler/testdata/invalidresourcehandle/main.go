package main

import (
	"github.com/WindowsSov8forUs/sonolus-go/sonolus"
	"github.com/WindowsSov8forUs/sonolus-go/sonolus/play"
)

type Note struct {
	play.Archetype `archetype:"name=Note"`
}

func (*Note) Preprocess() {
	var first sonolus.Sprite
	first.Draw(sonolus.Quad{}, 0, 1)
	second := sonolus.Sprite{}
	_ = second.Exists()
	third := [2]sonolus.Sprite{}
	_ = third[0].Exists()
}

func main() {}
