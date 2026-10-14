//go:build play

package main

import "github.com/WindowsSov8forUs/sonolus-go/v2/sonolus/play"

type LifeNote struct {
	play.Archetype `archetype:"name=LifeNote,hasInput=true"`
	Value          float64 `archetype:"memory"`
}

func (n *LifeNote) Preprocess() {
	play.Entity.SetLife(play.LifeValues{Perfect: 1, Great: 2, Good: 3, Miss: -20})
	play.Entity.SetScoreMultiplier(1.5)
	play.Entity.SetLifeAt(2, play.LifeValues{Miss: -7})
	play.Entity.SetScoreMultiplierAt(1, 0.5)
	n.Value = play.Entity.Life().Miss + play.Entity.ScoreMultiplier() + play.Entity.LifeAt(2).Miss + play.Entity.ScoreMultiplierAt(1)
}
