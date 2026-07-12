//go:build watch

package main

import "github.com/WindowsSov8forUs/sonolus-go/v2/sonolus"

type SkinData struct {
	sonolus.SkinResource
	Item sonolus.Sprite
}

var Skin = &SkinData{Item: sonolus.SkinSprite("watch.sprite")}
