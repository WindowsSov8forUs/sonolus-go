package main

import "github.com/WindowsSov8forUs/sonolus-go/sonolus"

type SkinData struct {
	sonolus.SkinResource
	Note sonolus.Sprite
}

var SkinA = SkinData{Note: sonolus.SkinSprite("a")}
var SkinB = SkinData{Note: sonolus.SkinSprite("b")}

func main() {}
