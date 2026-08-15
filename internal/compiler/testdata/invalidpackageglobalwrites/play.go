//go:build play

package main

import "github.com/WindowsSov8forUs/sonolus-go/v2/sonolus/play"

type state struct{ Value float64 }

var root = &state{}

func (value *state) Set(next float64) {
	value.Value = next
}

type Note struct {
	play.Archetype `archetype:"name=Note"`
}

func (*Note) UpdateParallel() {
	root.Set(1)
}
