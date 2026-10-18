//go:build play

package main

import "github.com/WindowsSov8forUs/sonolus-go/v2/sonolus/play"

type Table struct {
	play.Archetype `archetype:"name=Table"`
	Index          float64 `archetype:"memory"`
	Exact          float64 `archetype:"memory"`
	PairX          float64 `archetype:"memory"`
	PairY          float64 `archetype:"memory"`
	Inexact        float64 `archetype:"memory"`
}

func (t *Table) Preprocess() {
	i := int(t.Index)
	t.Exact = exact[i]
	pair := pairs[i]
	t.PairX, t.PairY = pair.X, pair.Y
	t.Inexact = inexact[i]
}
