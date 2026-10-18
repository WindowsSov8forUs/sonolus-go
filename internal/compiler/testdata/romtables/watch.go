//go:build watch

package main

import "github.com/WindowsSov8forUs/sonolus-go/v2/sonolus/watch"

type Table struct {
	watch.Archetype `archetype:"name=Table"`
	Index           float64 `archetype:"memory"`
	Exact           float64 `archetype:"memory"`
	PairX           float64 `archetype:"memory"`
	PairY           float64 `archetype:"memory"`
	Inexact         float64 `archetype:"memory"`
}

func (t *Table) Preprocess() {
	i := int(t.Index)
	t.Exact = exact[i] + watchOnly[i]
	t.PairX = pairs[i].X
}

func (*Table) SpawnTime() float64   { return 0 }
func (*Table) DespawnTime() float64 { return 1 }
