package main

import "github.com/WindowsSov8forUs/sonolus-go/sonolus/play"

type Note struct {
	play.Archetype `sonolus:"name=Note"`
}

func escape(values ...float64) []float64 { return values }

func (*Note) Preprocess() { _ = escape(1) }

func main() {}
