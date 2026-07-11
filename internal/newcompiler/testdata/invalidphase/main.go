package main

import (
	"github.com/WindowsSov8forUs/sonolus-go/sonolus"
	"github.com/WindowsSov8forUs/sonolus-go/sonolus/play"
)

type Note struct {
	play.Archetype `sonolus:"name=Note"`
}

func (*Note) UpdateParallel() {
	play.UI.Configure(sonolus.UIConfig{})
}

func main() {}
