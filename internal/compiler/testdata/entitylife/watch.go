//go:build watch

package main

import "github.com/WindowsSov8forUs/sonolus-go/v2/sonolus/watch"

type LifeNote struct {
	watch.Archetype `archetype:"name=LifeNote,hasInput=true"`
	Value           float64 `archetype:"memory"`
}

func (n *LifeNote) Preprocess() {
	watch.Entity.SetLife(watch.LifeValues{Perfect: 1, Great: 2, Good: 3, Miss: -20})
	watch.Entity.SetScoreMultiplier(1.5)
	watch.Entity.SetLifeAt(2, watch.LifeValues{Miss: -7})
	watch.Entity.SetScoreMultiplierAt(1, 0.5)
	n.Value = watch.Entity.Life().Miss + watch.Entity.ScoreMultiplier() + watch.Entity.LifeAt(2).Miss + watch.Entity.ScoreMultiplierAt(1)
}

func (*LifeNote) SpawnTime() float64   { return 0 }
func (*LifeNote) DespawnTime() float64 { return 1 }
