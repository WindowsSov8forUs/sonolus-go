//go:build preview

package main

import (
	"github.com/WindowsSov8forUs/sonolus-go/sonolus"
	"github.com/WindowsSov8forUs/sonolus-go/sonolus/preview"
)

type SkinData struct {
	sonolus.SkinResource

	Note sonolus.Sprite
}

var Skin = &SkinData{
	SkinResource: sonolus.SkinResource{RenderMode: sonolus.RenderModeLightweight}, Note: sonolus.SkinSprite("#NOTE_HEAD_CYAN")}

type Note struct {
	preview.Archetype `sonolus:"name=ConformanceNote"`
	Beat              float64 `sonolus:"imported,name=#BEAT,default=0"`
}

func (n *Note) Render() {
	preview.Debug.Log(n.Beat)
}
