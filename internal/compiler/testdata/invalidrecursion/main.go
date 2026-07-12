package main

import "github.com/WindowsSov8forUs/sonolus-go/v2/sonolus/play"

type Note struct {
	play.Archetype `archetype:"name=Note"`
}

func first(n *Note)  { second(n) }
func second(n *Note) { first(n) }

func (n *Note) Preprocess() { first(n) }

func main() {}
