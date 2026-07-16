package main

import "github.com/WindowsSov8forUs/sonolus-go/sonolus"

//sonolus:resource skin lightweight
type SkinData struct {
	Anything sonolus.Sprite
	Other    sonolus.Sprite
	Group    [2]sonolus.Sprite
}

//sonolus:resource skin lightweight
var Skin = &SkinData{
	Anything: sonolus.SkinSprite("#NOTE_HEAD"),
	Other:    sonolus.SkinSprite("custom.sprite"),
	Group: [2]sonolus.Sprite{
		sonolus.SkinSprite("group.0"),
		sonolus.SkinSprite("group.1"),
	},
}

func main() {}
