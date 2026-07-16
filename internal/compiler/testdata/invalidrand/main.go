package main

import (
	"math/rand"

	"github.com/WindowsSov8forUs/sonolus-go/sonolus/play"
)

type Note struct {
	play.Archetype `sonolus:"name=Note"`
}

func (*Note) Preprocess() {
	_ = rand.Intn(0)
}

func main() {}
