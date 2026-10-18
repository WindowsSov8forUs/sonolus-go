//go:build play

package main

import (
	"github.com/WindowsSov8forUs/sonolus-go/v2/sonolus"
	"github.com/WindowsSov8forUs/sonolus-go/v2/sonolus/play"
)

type Initialization struct {
	play.Archetype `archetype:"name=Initialization"`
	Judgment       float64 `archetype:"memory"`
	Value          float64 `archetype:"memory"`
}

func (i *Initialization) Preprocess() {
	play.Score.SetConsecutive(sonolus.JudgmentPerfect, play.ConsecutiveScore{Multiplier: 1, Step: 2, Cap: 3})
	play.Score.SetConsecutive(sonolus.JudgmentGreat, play.ConsecutiveScore{Multiplier: 4, Step: 5, Cap: 6})
	play.Score.SetConsecutive(sonolus.JudgmentGood, play.ConsecutiveScore{Multiplier: 7, Step: 8, Cap: 9})
	play.Life.SetConsecutive(sonolus.JudgmentPerfect, play.ConsecutiveLife{Increment: 10, Step: 11})
	play.Life.SetConsecutive(sonolus.JudgmentGreat, play.ConsecutiveLife{Increment: 12, Step: 13})
	play.Life.SetConsecutive(sonolus.JudgmentGood, play.ConsecutiveLife{Increment: 14, Step: 15})
	j := sonolus.Judgment(i.Judgment)
	i.Value = play.Score.Consecutive(j).Cap + play.Life.Consecutive(j).Step
}
