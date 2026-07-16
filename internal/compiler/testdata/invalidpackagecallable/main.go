package main

import "github.com/WindowsSov8forUs/sonolus-go/sonolus/play"

type Note struct {
	play.Archetype `archetype:"name=Note"`
}

func helper() {}

var fn = helper

func (*Note) Preprocess() { fn() }

func main() {}
