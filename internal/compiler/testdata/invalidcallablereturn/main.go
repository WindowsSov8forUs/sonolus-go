package main

import "github.com/WindowsSov8forUs/sonolus-go/sonolus/play"

type Note struct {
	play.Archetype `archetype:"name=Note"`
}

func helper()        {}
func choose() func() { return helper }

func (*Note) Preprocess() { choose()() }

func main() {}
