//go:build watch

package main

import (
	"github.com/WindowsSov8forUs/sonolus-go/sonolus"
	"github.com/WindowsSov8forUs/sonolus-go/sonolus/watch"
)

type SkinData struct {
	sonolus.SkinResource

	Note sonolus.Sprite
}

var Skin = &SkinData{
	Note: sonolus.SkinSprite("#NOTE_HEAD_CYAN"),
}

type Globals struct {
	watch.GlobalCallbacks
}

var Global Globals

func UpdateSpawn() float64 {
	return 0
}

type TapNote struct {
	watch.Archetype `sonolus:"name=TapNote"`
	Beat            float64 `sonolus:"imported,name=#BEAT,default=0"`
}

func (n *TapNote) SpawnTime() float64 {
	return n.Beat
}
