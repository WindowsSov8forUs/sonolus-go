//go:build play

package main

import (
	"github.com/WindowsSov8forUs/sonolus-go/sonolus"
	"github.com/WindowsSov8forUs/sonolus-go/sonolus/play"
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
	play.Archetype      `sonolus:"name=TapNote,hasInput=true"`
	play.CallbackOrders `sonolus:"preprocess=-10"`

	Beat float64 `sonolus:"imported,name=#BEAT,default=0"`
	X    float64 `sonolus:"memory"`
	Hit  float64 `sonolus:"exported,name=hitTime"`
}

func (n *TapNote) Preprocess() {
	n.X = n.Beat
}

func (*TapNote) ShouldSpawn() bool {
	return true
}
