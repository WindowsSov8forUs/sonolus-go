package main

import (
	"github.com/WindowsSov8forUs/sonolus-go/v2/sonolus"
	"github.com/WindowsSov8forUs/sonolus-go/v2/sonolus/play"
)

type Note struct {
	play.Archetype `archetype:"name=Note"`
}

func chooseValues(condition bool) sonolus.VarArray[float64] {
	if condition {
		return sonolus.NewVarArray[float64](2)
	}
	return sonolus.NewVarArray[float64](2)
}

func (*Note) Preprocess() {
	_ = chooseValues(play.Environment.Debug())
}

func main() {}
