package main

import "github.com/WindowsSov8forUs/sonolus-go/v2/sonolus"

type ConfigData struct {
	sonolus.Configuration
	Speed    float64
	Enabled  bool
	Lane     int
	UI       sonolus.UIConfig
	Fallback []string
}

var Config = ConfigData{
	Speed: sonolus.SliderOption(sonolus.SliderOptionConfig{
		Name: "Speed", Title: "Speed Option", Description: "Scroll speed",
		Standard: true, Scope: "game", Default: 1, Min: 0, Max: 2, Step: 0.1, Unit: "#TIMES",
	}),
	Enabled: sonolus.ToggleOption(sonolus.ToggleOptionConfig{Name: "Enabled", Default: true}),
	Lane:    sonolus.SelectOption(sonolus.SelectOptionConfig{Name: "Lane", Default: 1, Values: []string{"4", "6", "8"}}),
	UI: sonolus.UIConfig{
		Scope:                  "game",
		PrimaryMetric:          sonolus.UIMetricArcade,
		SecondaryMetric:        sonolus.UIMetricLife,
		JudgmentErrorStyle:     sonolus.UIJudgmentErrorPlus,
		JudgmentErrorPlacement: sonolus.UIJudgmentErrorTop,
	},
	Fallback: []string{"Speed", "Lane"},
}

func main() {}
