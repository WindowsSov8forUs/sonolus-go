package main

import (
	"github.com/WindowsSov8forUs/sonolus-go/sonolus"
	"github.com/WindowsSov8forUs/sonolus-go/sonolus/play"
)

type Note struct {
	play.Archetype `archetype:"name=Note"`
}

func (*Note) UpdateParallel() {
	play.UI.SetMenu(sonolus.RuntimeUILayout{})
}

func main() {}
