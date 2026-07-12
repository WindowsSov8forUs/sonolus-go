package main

import "github.com/WindowsSov8forUs/sonolus-go/v2/sonolus/play"

type Note struct {
	play.Archetype `archetype:"name=Note"`
}

func first()  {}
func second() {}

func (*Note) Preprocess() {
	fn := first
	fn = second
	fn()
}

func main() {}
