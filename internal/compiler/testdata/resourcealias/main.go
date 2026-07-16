package main

import "github.com/WindowsSov8forUs/sonolus-go/sonolus"

const spriteName = "custom.alias"

type SkinData struct {
	sonolus.SkinResource
	Note sonolus.Sprite
}

var note = sonolus.SkinSprite(spriteName)
var Skin = &SkinData{Note: note}

func main() {}
