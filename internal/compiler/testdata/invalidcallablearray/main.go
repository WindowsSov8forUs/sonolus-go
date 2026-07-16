package main

import "github.com/WindowsSov8forUs/sonolus-go/v2/sonolus/play"

func first()  {}
func second() {}

var operations = [2]func(){first, second}

type Note struct {
	play.Archetype `archetype:"name=Note"`
}

func (*Note) Preprocess() {
	operations[0] = second
}

func main() {}
