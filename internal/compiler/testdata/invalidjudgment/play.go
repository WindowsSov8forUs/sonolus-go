//go:build play

package main

import (
	"github.com/WindowsSov8forUs/sonolus-go/v2/sonolus"
	"github.com/WindowsSov8forUs/sonolus-go/v2/sonolus/play"
)

type Initialization struct {
	play.Archetype `archetype:"name=Initialization"`
}

func (*Initialization) Preprocess() {
	play.Score.SetConsecutive(sonolus.JudgmentMiss, play.ConsecutiveScore{Multiplier: 1, Step: 1, Cap: 1})
}
