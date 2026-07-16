//go:build preview

package main

import (
	"github.com/WindowsSov8forUs/sonolus-go/sonolus"
	"github.com/WindowsSov8forUs/sonolus-go/sonolus/preview"
)

//sonolus:resource skin lightweight
type SkinData struct {
	Note sonolus.Sprite
}

//sonolus:resource skin lightweight
var Skin = &SkinData{Note: sonolus.SkinSprite("#NOTE_HEAD_CYAN")}

type Note struct {
	preview.Archetype `sonolus:"name=ConformanceNote"`
	Beat              float64 `sonolus:"imported,name=#BEAT,default=0"`
}

func (n *Note) Render() {
	preview.Debug.Log(n.Beat)
}
