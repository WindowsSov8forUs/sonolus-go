package main

import (
	"github.com/WindowsSov8forUs/sonolus-go/sonolus"
	"github.com/WindowsSov8forUs/sonolus-go/sonolus/play"
)

type Note struct {
	play.Archetype `archetype:"name=Note"`
}

func (*Note) Preprocess() {
	first := sonolus.NewVarArray[float64](2)
	second := sonolus.NewVarArray[float64](2)
	values := first
	if play.Environment.Debug() {
		values = second
	}
	values.Append(1)
}

func main() {}
