package main

import "github.com/WindowsSov8forUs/sonolus-go/v2/sonolus"

func SkinSprite(string) sonolus.Sprite { return sonolus.Sprite{} }

type SkinData struct {
	sonolus.SkinResource
	Note sonolus.Sprite
}

var Skin = &SkinData{Note: SkinSprite("fake")}

func main() {}
