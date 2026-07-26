package main

import (
	"github.com/WindowsSov8forUs/sonolus-go/v2/sonolus/play"
)

type nilOnlyProvider interface {
	Value() float64
}

type Note struct {
	play.Archetype `archetype:"name=Note"`
}

func alwaysNilProvider() nilOnlyProvider {
	return nil
}

func (*Note) Preprocess() {
	_ = alwaysNilProvider().Value()
}
