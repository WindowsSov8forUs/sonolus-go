package main

import "github.com/WindowsSov8forUs/sonolus-go/v2/sonolus"

type ConfigData struct {
	sonolus.Configuration
	Categories []sonolus.OptionCategory
	Speed      float64
	Mirror     bool
	UI         sonolus.UIConfig
}

var Config = ConfigData{
	Categories: []sonolus.OptionCategory{{Name: "gameplay", Title: "#GAMEPLAY"}, {Name: "visual", Title: "Visual"}},
	Speed: sonolus.SliderOption(sonolus.SliderOptionConfig{
		Name: "#SPEED", Category: "gameplay", Description: "#SPEED", Standard: true, Default: 1, Min: 0.5, Max: 2, Step: 0.05,
	}),
	Mirror: sonolus.ToggleOption(sonolus.ToggleOptionConfig{Name: "#MIRROR", Category: "visual"}),
}

func main() {}
