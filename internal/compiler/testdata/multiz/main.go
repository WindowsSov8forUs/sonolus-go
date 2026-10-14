package main

import "github.com/WindowsSov8forUs/sonolus-go/v2/sonolus"

type SkinData struct {
	sonolus.SkinResource
	Note  sonolus.Sprite
	Lanes [4]sonolus.Sprite
}

var Skin = &SkinData{
	Note:  sonolus.SkinSprite("note"),
	Lanes: [4]sonolus.Sprite{sonolus.SkinSprite("lane0"), sonolus.SkinSprite("lane1"), sonolus.SkinSprite("lane2"), sonolus.SkinSprite("lane3")},
}

func main() {}
