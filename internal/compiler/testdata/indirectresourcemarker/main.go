package main

import "github.com/WindowsSov8forUs/sonolus-go/sonolus"

type ResourceBase struct{ sonolus.SkinResource }

type SkinData struct {
	ResourceBase
	Note sonolus.Sprite
}

var Skin = SkinData{Note: sonolus.SkinSprite("note")}

func main() {}
