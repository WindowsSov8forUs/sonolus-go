//go:build play

package main

import (
	"github.com/WindowsSov8forUs/sonolus-go/v2/internal/compiler/testdata/structmixin/shared"
	"github.com/WindowsSov8forUs/sonolus-go/v2/sonolus/play"
)

type ModeBase struct {
	play.Archetype `archetype:"abstract"`
	BaseValue      float64 `archetype:"memory"`
}

type TapNote struct {
	ModeBase       `archetype:"base"`
	play.Archetype `archetype:"name=TapNote,hasInput=true"`
	shared.BasicNote
	Local float64 `archetype:"data"`
}

func main() {}
