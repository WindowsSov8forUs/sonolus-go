package main

import (
	"github.com/WindowsSov8forUs/sonolus-go/sonolus"
	"github.com/WindowsSov8forUs/sonolus-go/sonolus/play"
)

type Note struct {
	play.Archetype `sonolus:"name=Note"`
	Data           sonolus.VarArray[float64] `sonolus:"data,cap=32"`
	Memory         sonolus.VarArray[float64] `sonolus:"memory,cap=64"`
}

func main() {}
