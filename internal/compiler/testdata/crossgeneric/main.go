package main

import (
	"github.com/WindowsSov8forUs/sonolus-go/v2/internal/compiler/testdata/crossgeneric/helper"
	"github.com/WindowsSov8forUs/sonolus-go/v2/sonolus/play"
)

type Note struct {
	play.Archetype `archetype:"name=Note"`
}

func (*Note) Preprocess() {
	left, right := helper.Pair[float64](1)
	play.Debug.Log(left + right)
}

func main() {}
