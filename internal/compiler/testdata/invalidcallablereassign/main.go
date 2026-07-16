package main

import "github.com/WindowsSov8forUs/sonolus-go/sonolus/play"

type Note struct {
	play.Archetype `sonolus:"name=Note"`
}

func first()  {}
func second() {}

func (*Note) Preprocess() {
	fn := first
	fn = second
	fn()
}

func main() {}
