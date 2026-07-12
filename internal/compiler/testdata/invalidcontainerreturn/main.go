package main

import (
	"github.com/WindowsSov8forUs/sonolus-go/sonolus"
	"github.com/WindowsSov8forUs/sonolus-go/sonolus/play"
)

type Note struct {
	play.Archetype `sonolus:"name=Note"`
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
