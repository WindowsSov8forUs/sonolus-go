package main

import "github.com/WindowsSov8forUs/sonolus-go/v2/sonolus/play"

type Note struct {
	play.Archetype `archetype:"name=Note"`
}

func helper() {}

func (*Note) Preprocess() {
	for range 2 {
		defer helper()
	}
}

func main() {}
