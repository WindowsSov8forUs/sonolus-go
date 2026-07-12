package main

import (
	"github.com/WindowsSov8forUs/sonolus-go/internal/newcompiler/testdata/crossgeneric/helper"
	"github.com/WindowsSov8forUs/sonolus-go/sonolus/play"
)

type Note struct {
	play.Archetype `sonolus:"name=Note"`
}

func (*Note) Preprocess() {
	left, right := helper.Pair[float64](1)
	play.Debug.Log(left + right)
}

func main() {}
