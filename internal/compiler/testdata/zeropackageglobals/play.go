//go:build play

package main

import "github.com/WindowsSov8forUs/sonolus-go/v2/sonolus/play"

type zeroState struct{ Value float64 }

var zeroRoot = &zeroState{}

type ZeroNote struct {
	play.Archetype `archetype:"name=ZeroNote"`
}

func (*ZeroNote) UpdateSequential() {
	zeroRoot.Value++
}
