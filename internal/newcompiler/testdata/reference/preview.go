//go:build preview

package main

import (
	"github.com/WindowsSov8forUs/sonolus-go/sonolus"
	"github.com/WindowsSov8forUs/sonolus-go/sonolus/preview"
)

//sonolus:resource skin lightweight
type SkinData struct{ Note sonolus.Sprite }

//sonolus:resource skin lightweight
var Skin = &SkinData{Note: sonolus.SkinSprite("note")}

type Note struct {
	preview.Archetype `sonolus:"name=Note"`
	Beat              float64 `sonolus:"imported,name=#BEAT"`
}

func (*Note) Render() {}
