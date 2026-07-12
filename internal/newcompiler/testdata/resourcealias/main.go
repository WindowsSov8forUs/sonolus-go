package main

import "github.com/WindowsSov8forUs/sonolus-go/sonolus"

const spriteName = "custom.alias"

//sonolus:resource skin
type SkinData struct{ Note sonolus.Sprite }

var skinValue = &SkinData{Note: sonolus.SkinSprite(spriteName)}

//sonolus:resource skin
var Skin = skinValue

func main() {}
