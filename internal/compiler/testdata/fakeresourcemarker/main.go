package main

import "github.com/WindowsSov8forUs/sonolus-go/v2/sonolus"

type SkinResource struct{}

type SkinData struct {
	SkinResource
	Note sonolus.Sprite
}

var Skin = SkinData{Note: sonolus.SkinSprite("note")}

func main() {}
