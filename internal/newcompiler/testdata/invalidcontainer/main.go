package main

import (
	"github.com/WindowsSov8forUs/sonolus-go/sonolus"
	"github.com/WindowsSov8forUs/sonolus-go/sonolus/play"
)

type Note struct {
	play.Archetype `sonolus:"name=Note"`
	Values         sonolus.VarArray[float64] `sonolus:"memory"`
}

func (*Note) Preprocess() {}
func main()               {}
