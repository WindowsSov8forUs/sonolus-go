//go:build play

package main

import "github.com/WindowsSov8forUs/sonolus-go/v2/sonolus/play"

type AbstractNote struct {
	play.Archetype `archetype:"abstract"`
}

type ConcreteNote struct {
	AbstractNote   `archetype:"base"`
	play.Archetype `archetype:"name=ConcreteNote"`
}

func (*ConcreteNote) Preprocess() { play.Spawn(AbstractNote{}) }
