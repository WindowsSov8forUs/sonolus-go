//go:build play

package main

import (
	"github.com/WindowsSov8forUs/sonolus-go/v2/sonolus"
	"github.com/WindowsSov8forUs/sonolus-go/v2/sonolus/play"
)

type Base struct {
	play.Archetype `archetype:"abstract"`
	Value          float64 `archetype:"shared"`
	Scratch        float64 `archetype:"memory"`
}

func byValue(b Base) float64     { return b.Value }
func scratch(b *Base, v float64) { b.Scratch = v }

type Copy struct {
	play.Archetype `archetype:"name=Copy"`
	Other          sonolus.EntityRef[Head] `archetype:"imported,name=other"`
	Out            float64                 `archetype:"memory"`
}

func (n *Copy) Preprocess() {
	b := n.Other.Get().Base
	n.Out = b.Value
}

type Arg struct {
	play.Archetype `archetype:"name=Arg"`
	Other          sonolus.EntityRef[Head] `archetype:"imported,name=other"`
	Out            float64                 `archetype:"memory"`
}

func (n *Arg) Preprocess() { n.Out = byValue(n.Other.Get().Base) }

type Head struct {
	Base           `archetype:"base"`
	play.Archetype `archetype:"name=Head"`
	Other          sonolus.EntityRef[Head] `archetype:"imported,name=other"`
}

func (n *Head) Preprocess() { scratch(&n.Other.Get().Base, 1) }
