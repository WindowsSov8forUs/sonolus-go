//go:build play

package main

import (
	"github.com/WindowsSov8forUs/sonolus-go/v2/internal/compiler/testdata/invalidstructmixin/shared"
	"github.com/WindowsSov8forUs/sonolus-go/v2/sonolus/play"
)

type DuplicateMixin struct {
	play.Archetype `archetype:"name=DuplicateMixin"`
	shared.Left
	shared.Right
}

type BaseWithMixin struct {
	play.Archetype `archetype:"abstract"`
	shared.Leaf
}

type DuplicateThroughBase struct {
	BaseWithMixin  `archetype:"base"`
	play.Archetype `archetype:"name=DuplicateThroughBase"`
	shared.Leaf
}

type DuplicateExternal struct {
	play.Archetype `archetype:"name=DuplicateExternal"`
	shared.Imported
	Value float64 `archetype:"imported,name=duplicate"`
}

type OversizedMixin struct {
	play.Archetype `archetype:"name=OversizedMixin"`
	shared.Oversized
}

type InvalidCallbackMixin struct {
	play.Archetype `archetype:"name=InvalidCallbackMixin"`
	shared.BadCallback
}

func main() {}
