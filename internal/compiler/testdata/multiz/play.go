//go:build play

package main

import (
	"github.com/WindowsSov8forUs/sonolus-go/v2/sonolus"
	"github.com/WindowsSov8forUs/sonolus-go/v2/sonolus/play"
)

type LayeredNote struct {
	play.Archetype `archetype:"name=LayeredNote"`
	Lane           float64 `archetype:"imported,name=lane"`
}

func (n *LayeredNote) UpdateParallel() {
	q := sonolus.Rect{T: 1, R: 1}.ToQuad()
	c := sonolus.Vec2{X: 0.5, Y: 2}
	Skin.Note.Draw(q, 5, 1)
	Skin.Note.DrawZ(q, sonolus.ZIndex{Z1: 1, Z2: n.Lane, Z3: 3, Z4: 4}, 0.5)
	Skin.Note.DrawCurvedTZ(q, c, 4, sonolus.ZIndex{Z1: 7}, 1)
	Skin.Note.DrawCurvedLRZ(q, c, c, 4, sonolus.ZIndex{Z1: 1, Z2: 2, Z3: 3, Z4: n.Lane}, 1)
}

// IndexedNote selects resources and constants by a runtime index.
type IndexedNote struct {
	play.Archetype `archetype:"name=IndexedNote"`
	Lane           float64 `archetype:"imported,name=lane"`
	Value          float64 `archetype:"memory"`
}

func (n *IndexedNote) UpdateParallel() {
	q := sonolus.Rect{T: 1, R: 1}.ToQuad()
	for i := 0; i < 2; i++ {
		Skin.Lanes[int(n.Lane)+i].Draw(q, 1, 1)
	}
}

func (n *IndexedNote) Preprocess() {
	k := int(n.Lane)
	n.Value = [3]float64{10, 20, 30}[k]*100 + [3]float64{1, 4, 2}[k]
}
