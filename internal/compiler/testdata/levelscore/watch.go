//go:build watch

package main

import (
	"github.com/WindowsSov8forUs/sonolus-go/v2/sonolus"
	"github.com/WindowsSov8forUs/sonolus-go/v2/sonolus/watch"
)

type Initialization struct {
	watch.Archetype `archetype:"name=Initialization"`
	Judgment        float64 `archetype:"memory"`
	Value           float64 `archetype:"memory"`
}

func (i *Initialization) Preprocess() {
	watch.Score.SetConsecutive(sonolus.JudgmentPerfect, watch.ConsecutiveScore{Multiplier: 1, Step: 2, Cap: 3})
	watch.Score.SetConsecutive(sonolus.JudgmentGreat, watch.ConsecutiveScore{Multiplier: 4, Step: 5, Cap: 6})
	watch.Score.SetConsecutive(sonolus.JudgmentGood, watch.ConsecutiveScore{Multiplier: 7, Step: 8, Cap: 9})
	watch.Life.SetConsecutive(sonolus.JudgmentPerfect, watch.ConsecutiveLife{Increment: 10, Step: 11})
	watch.Life.SetConsecutive(sonolus.JudgmentGreat, watch.ConsecutiveLife{Increment: 12, Step: 13})
	watch.Life.SetConsecutive(sonolus.JudgmentGood, watch.ConsecutiveLife{Increment: 14, Step: 15})
	j := sonolus.Judgment(i.Judgment)
	i.Value = watch.Score.Consecutive(j).Cap + watch.Life.Consecutive(j).Step
}

func (*Initialization) SpawnTime() float64   { return 0 }
func (*Initialization) DespawnTime() float64 { return 1 }
