package main

import "github.com/WindowsSov8forUs/sonolus-go/v2/sonolus/play"

type Note struct {
	play.Archetype `archetype:"name=Note"`
}

func sum(values ...float64) float64 { return 0 }

func (*Note) Preprocess() { _ = sum(1, 2) }

func main() {}
