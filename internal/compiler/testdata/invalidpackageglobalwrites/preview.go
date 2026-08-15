//go:build preview

package main

import "github.com/WindowsSov8forUs/sonolus-go/v2/sonolus/preview"

type state struct{ Value float64 }

var root = &state{}

type Note struct {
	preview.Archetype `archetype:"name=Note"`
}

func (*Note) Render() {
	root.Value = 1
}
