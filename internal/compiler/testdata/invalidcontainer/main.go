package main

import (
	"github.com/WindowsSov8forUs/sonolus-go/v2/sonolus"
	"github.com/WindowsSov8forUs/sonolus-go/v2/sonolus/play"
)

type Note struct {
	play.Archetype `archetype:"name=Note"`
	Values         sonolus.VarArray[float64] `archetype:"memory"`
}

func (*Note) Preprocess() {}
func main()               {}
