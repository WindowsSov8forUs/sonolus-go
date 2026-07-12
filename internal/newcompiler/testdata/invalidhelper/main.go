package main

import (
	"github.com/WindowsSov8forUs/sonolus-go/sonolus"
	"github.com/WindowsSov8forUs/sonolus-go/sonolus/play"
)

type Note struct {
	play.Archetype `sonolus:"name=Note"`
}

func configure() { play.UI.SetMenu(sonolus.RuntimeUILayout{}) }

func (*Note) UpdateParallel() { configure() }

func main() {}
