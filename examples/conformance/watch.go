//go:build watch

package main

import (
	"github.com/WindowsSov8forUs/sonolus-go/sonolus"
	"github.com/WindowsSov8forUs/sonolus-go/sonolus/watch"
)

//sonolus:resource skin
type SkinData struct {
	Note sonolus.Sprite
}

//sonolus:resource skin
var Skin = &SkinData{Note: sonolus.SkinSprite("#NOTE_HEAD_CYAN")}

type Globals struct{ watch.GlobalCallbacks }

var Global Globals

func UpdateSpawn() float64 { return 0 }

type Note struct {
	watch.Archetype `sonolus:"name=ConformanceNote"`
	Beat            float64 `sonolus:"imported,name=#BEAT,default=0"`
}

func (n *Note) SpawnTime() float64 {
	return sum(n.Beat, 1)
}
