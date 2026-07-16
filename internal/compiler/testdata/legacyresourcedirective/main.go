package main

import "github.com/WindowsSov8forUs/sonolus-go/sonolus"

//sonolus:resource skin
type SkinData struct{ Note sonolus.Sprite }

//sonolus:resource skin
var Skin = SkinData{Note: sonolus.SkinSprite("note")}

func main() {}
