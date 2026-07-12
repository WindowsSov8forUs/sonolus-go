//go:build play

package main

import (
	"github.com/WindowsSov8forUs/sonolus-go/v2/sonolus"
	"github.com/WindowsSov8forUs/sonolus-go/v2/sonolus/play"
)

type SkinData struct {
	sonolus.SkinResource

	Note sonolus.Sprite
}

var Skin = &SkinData{
	SkinResource: sonolus.SkinResource{RenderMode: sonolus.RenderModeStandard},
	Note:         sonolus.SkinSprite("#NOTE_HEAD_CYAN"),
}

type TapNote struct {
	play.Archetype      `archetype:"name=TapNote,hasInput=true"`
	play.CallbackOrders `archetype:"preprocess=-10"`

	Beat float64 `archetype:"imported,name=#BEAT,default=0"`
	X    float64 `archetype:"memory"`
	Hit  float64 `archetype:"exported,name=hitTime"`
}

func (n *TapNote) Preprocess() {
	n.X = n.Beat
}

func (*TapNote) ShouldSpawn() bool {
	return true
}
