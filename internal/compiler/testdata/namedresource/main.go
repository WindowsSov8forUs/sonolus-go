package main

import "github.com/WindowsSov8forUs/sonolus-go/sonolus"

type SkinData struct {
	sonolus.SkinResource

	Anything sonolus.Sprite
	Other    sonolus.Sprite
	Group    [2]sonolus.Sprite
}

var Skin = &SkinData{
	SkinResource: sonolus.SkinResource{RenderMode: sonolus.RenderModeLightweight},
	Anything:     sonolus.SkinSprite("#NOTE_HEAD"),
	Other:        sonolus.SkinSprite("custom.sprite"),
	Group: [2]sonolus.Sprite{
		sonolus.SkinSprite("group.0"),
		sonolus.SkinSprite("group.1"),
	},
}

func main() {}
