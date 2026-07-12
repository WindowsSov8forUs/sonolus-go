package main

import (
	"github.com/WindowsSov8forUs/sonolus-go/v2/sonolus"
	"github.com/WindowsSov8forUs/sonolus-go/v2/sonolus/play"
)

type Note struct {
	play.Archetype `archetype:"name=Note"`
	Data           sonolus.VarArray[float64] `archetype:"data,cap=32"`
	Memory         sonolus.VarArray[float64] `archetype:"memory,cap=64"`
}

func main() {}
