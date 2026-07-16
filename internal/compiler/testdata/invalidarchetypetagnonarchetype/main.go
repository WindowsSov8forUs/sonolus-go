package main

import "github.com/WindowsSov8forUs/sonolus-go/sonolus"

type SkinData struct {
	sonolus.SkinResource `archetype:"name=Skin"`
}

var Skin = SkinData{}

func main() {}
