//go:build play

package main

import (
	"github.com/WindowsSov8forUs/sonolus-go/v2/sonolus"
	"github.com/WindowsSov8forUs/sonolus-go/v2/sonolus/play"
)

type DataState struct {
	sonolus.LevelDataResource
	Value float64
}

var Data = DataState{}

type Note struct {
	play.Archetype `archetype:"name=Note"`
}

func (*Note) UpdateParallel() { Data.Value = 1 }
