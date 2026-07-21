//go:build watch

package main

import (
	"github.com/WindowsSov8forUs/sonolus-go/v2/internal/compiler/testdata/structmixin/shared"
	"github.com/WindowsSov8forUs/sonolus-go/v2/sonolus/watch"
)

type ModeBase struct {
	watch.Archetype `archetype:"abstract"`
	BaseValue       float64 `archetype:"memory"`
}

type TapNote struct {
	ModeBase        `archetype:"base"`
	watch.Archetype `archetype:"name=TapNote"`
	shared.BasicNote
	Local float64 `archetype:"data"`
}

func main() {}
