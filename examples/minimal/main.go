package main

import "github.com/WindowsSov8forUs/sonolus-go/v2/sonolus"

type GameConfiguration struct {
	sonolus.Configuration
	Speed float64
}

var Configuration = GameConfiguration{Speed: sonolus.SliderOption(sonolus.SliderOptionConfig{
	Name: "Speed", Default: 1, Min: 0.5, Max: 2, Step: 0.1,
})}
var ROM = sonolus.ROMValues{}

func main() {}
