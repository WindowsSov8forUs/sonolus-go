package main

import "github.com/WindowsSov8forUs/sonolus-go/v2/sonolus/play"

type Note struct {
	play.Archetype `archetype:"name=Note"`
}

func escape(values ...float64) []float64 { return values }

func (*Note) Preprocess() { _ = escape(1) }

func main() {}
