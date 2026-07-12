//go:build preview

package main

import (
	"github.com/WindowsSov8forUs/sonolus-go/v2/sonolus"
	"github.com/WindowsSov8forUs/sonolus-go/v2/sonolus/preview"
)

type SkinData struct {
	sonolus.SkinResource

	Note sonolus.Sprite
}

var Skin = &SkinData{
	SkinResource: sonolus.SkinResource{RenderMode: sonolus.RenderModeLightweight},
	Note:         sonolus.SkinSprite("#NOTE_HEAD_CYAN"),
}

type TapNote struct {
	preview.Archetype `archetype:"name=TapNote"`
	Beat              float64 `archetype:"imported,name=#BEAT,default=0"`
}

func (*TapNote) Render() {}
