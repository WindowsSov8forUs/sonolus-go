//go:build play

package main

import "github.com/WindowsSov8forUs/sonolus-go/v2/sonolus/play"

type First struct {
	play.Archetype `archetype:"name=First"`
}

type Second struct {
	play.Archetype `archetype:"name=Second"`
}

func (*First) Preprocess() {
	_ = play.CurrentEntityRef[Second]()
}

func main() {}
