package main

import (
	"math/rand"

	"github.com/WindowsSov8forUs/sonolus-go/v2/sonolus/play"
)

type Note struct {
	play.Archetype `archetype:"name=Note"`
}

func (*Note) Preprocess() {
	_ = rand.Intn(0)
}

func main() {}
