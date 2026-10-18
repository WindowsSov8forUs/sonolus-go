//go:build play

package main

import (
	"github.com/WindowsSov8forUs/sonolus-go/v2/sonolus"
	"github.com/WindowsSov8forUs/sonolus-go/v2/sonolus/play"
)

// Base has a memory field, so a view of another entity's Base cannot be a
// plain value copy.
type Base struct {
	play.Archetype `archetype:"abstract"`
	Value          float64 `archetype:"shared"`
	Scratch        float64 `archetype:"memory"`
}

func mark(b *Base, v float64) { b.Value = v }

func read(b *Base) float64 { return b.Value }

// Head shares Base with the entity it references.
type Head struct {
	Base           `archetype:"base"`
	play.Archetype `archetype:"name=Head"`
	Other          sonolus.EntityRef[Head] `archetype:"imported,name=other"`
	Out            float64                 `archetype:"memory"`
}

func (n *Head) Preprocess() {
	mark(&n.Other.Get().Base, 7)
	mark(&n.Base, 3)
	n.Out = n.Other.Get().Base.Value*10 + read(&n.Other.Get().Base)
}

// Probe does not embed Base.
type Probe struct {
	play.Archetype `archetype:"name=Probe"`
	Target         sonolus.EntityRef[Head] `archetype:"imported,name=target"`
}

func (n *Probe) Preprocess() {
	mark(&n.Target.Get().Base, 5)
}

// Simple has only a shared field: before upcasting, &view.Simple of an
// archetype that also embeds Simple compiled to writes into a temporary.
type Simple struct {
	play.Archetype `archetype:"abstract"`
	Value          float64 `archetype:"shared"`
}

func markSimple(b *Simple, v float64) { b.Value = v }

type Leaf struct {
	Simple         `archetype:"base"`
	play.Archetype `archetype:"name=Leaf"`
	Other          sonolus.EntityRef[Leaf] `archetype:"imported,name=other"`
}

func (n *Leaf) Preprocess() { markSimple(&n.Other.Get().Simple, 9) }
