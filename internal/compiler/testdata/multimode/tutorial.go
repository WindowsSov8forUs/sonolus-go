//go:build tutorial

package main

import "github.com/WindowsSov8forUs/sonolus-go/sonolus"

type SkinData struct {
	sonolus.SkinResource
	Item sonolus.Sprite
}

var Skin = &SkinData{Item: sonolus.SkinSprite("tutorial.sprite")}
