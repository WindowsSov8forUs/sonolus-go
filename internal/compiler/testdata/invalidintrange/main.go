package main

import "github.com/WindowsSov8forUs/sonolus-go/sonolus/play"

type Note struct {
	play.Archetype `sonolus:"name=Note"`
}

func (*Note) Preprocess() {
	for range uint(3) {
	}
}

func main() {}
