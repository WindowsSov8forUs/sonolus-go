package main

import "github.com/WindowsSov8forUs/sonolus-go/v2/sonolus"

type ConfigData struct {
	sonolus.Configuration
	Categories []sonolus.OptionCategory
	Speed      float64
	Mirror     bool
	Lane       int
	UI         sonolus.UIConfig
}

// Speed has no category, Mirror names an undeclared one, and the category
// list repeats "gameplay".
var Config = ConfigData{
	Categories: []sonolus.OptionCategory{{Name: "gameplay", Title: "Gameplay"}, {Name: "gameplay", Title: "Again"}},
	Speed:      sonolus.SliderOption(sonolus.SliderOptionConfig{Name: "#SPEED", Default: 1, Min: 0.5, Max: 2, Step: 0.05}),
	Mirror:     sonolus.ToggleOption(sonolus.ToggleOptionConfig{Name: "#MIRROR", Category: "visual"}),
	Lane:       sonolus.SelectOption(sonolus.SelectOptionConfig{Name: "Lane", Category: "gameplay", Values: []string{"4", "7"}}),
}

func main() {}
