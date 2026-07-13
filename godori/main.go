package main

import (
	_ "embed"

	"github.com/WindowsSov8forUs/sonolus-go/v2/sonolus"
)

type GodoriConfiguration struct {
	sonolus.Configuration

	NoteSpeed float64
	NoteSize  float64
	LaneWidth float64
	Effects   bool
	UI        sonolus.UIConfig
}

var Config = GodoriConfiguration{
	NoteSpeed: sonolus.SliderOption(sonolus.SliderOptionConfig{
		Name: "Note Speed", Default: 10, Min: 5, Max: 20, Step: 0.5, Scope: "godori",
	}),
	NoteSize: sonolus.SliderOption(sonolus.SliderOptionConfig{
		Name: "Note Size", Default: 1, Min: 0.5, Max: 1.5, Step: 0.05, Scope: "godori",
	}),
	LaneWidth: sonolus.SliderOption(sonolus.SliderOptionConfig{
		Name: "Lane Width", Default: 1, Min: 0.75, Max: 1.25, Step: 0.05, Scope: "godori",
	}),
	Effects: sonolus.ToggleOption(sonolus.ToggleOptionConfig{
		Name: "Effects", Default: true, Scope: "godori",
	}),
	UI: sonolus.UIConfig{
		Scope:           "godori",
		PrimaryMetric:   sonolus.UIMetricArcade,
		SecondaryMetric: sonolus.UIMetricLife,
	},
}

var ROM = sonolus.ROMValues{}

//sonolus:level
//go:embed dev-level.json
var DevelopmentLevel sonolus.LevelFile

func main() {}
