//go:build watch

package main

import "github.com/WindowsSov8forUs/sonolus-go/sonolus"

//sonolus:resource skin
type SkinData struct{ Item sonolus.Sprite }

//sonolus:resource skin
var Skin = &SkinData{Item: sonolus.SkinSprite("watch.sprite")}
