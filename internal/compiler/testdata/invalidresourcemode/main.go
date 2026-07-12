package main

import "github.com/WindowsSov8forUs/sonolus-go/v2/sonolus"

type SkinData struct {
	sonolus.SkinResource
	Note sonolus.Sprite
}

var Skin = SkinData{
	SkinResource: sonolus.SkinResource{RenderMode: sonolus.RenderMode("invalid")},
	Note:         sonolus.SkinSprite("note"),
}

func main() {}
