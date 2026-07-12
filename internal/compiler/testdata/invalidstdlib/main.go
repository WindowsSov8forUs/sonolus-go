package main

import (
	"math/rand"

	"github.com/WindowsSov8forUs/sonolus-go/sonolus/play"
)

type Note struct{ play.Archetype }

func (*Note) Preprocess() { rand.Seed(1) }

func main() {}
