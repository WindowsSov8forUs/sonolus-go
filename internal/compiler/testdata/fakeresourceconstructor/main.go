package main

import "github.com/WindowsSov8forUs/sonolus-go/sonolus"

func SkinSprite(string) sonolus.Sprite { return sonolus.Sprite{} }

//sonolus:resource skin
type SkinData struct{ Note sonolus.Sprite }

//sonolus:resource skin
var Skin = &SkinData{Note: SkinSprite("fake")}

func main() {}
