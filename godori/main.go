package main

import (
	_ "embed"

	"github.com/WindowsSov8forUs/sonolus-go/v2/sonolus"
)

type GodoriConfiguration struct {
	sonolus.Configuration

	Speed          float64
	NoteSpeed      float64
	NoteSize       float64
	LaneWidth      float64
	LaneLength     float64
	ConnectorAlpha float64
	NoteEffects    bool
	LaneEffects    bool
	SimLines       bool
	SimLineAlpha   float64
	SFX            bool
	AutoSFX        bool
	Mirror         bool
	UI             sonolus.UIConfig
}

var Config = GodoriConfiguration{
	Speed: sonolus.SliderOption(sonolus.SliderOptionConfig{
		Name: "#SPEED", Standard: true, Default: 1, Min: 0.5, Max: 2, Step: 0.05, Unit: "#PERCENTAGE",
	}),
	NoteSpeed: sonolus.SliderOption(sonolus.SliderOptionConfig{
		Name: "#NOTE_SPEED", Default: 10, Min: 1, Max: 20, Step: 0.05, Scope: "godori",
	}),
	NoteSize: sonolus.SliderOption(sonolus.SliderOptionConfig{
		Name: "#NOTE_SIZE", Default: 1, Min: 0.1, Max: 2, Step: 0.05, Unit: "#PERCENTAGE", Scope: "godori",
	}),
	LaneWidth: sonolus.SliderOption(sonolus.SliderOptionConfig{
		Name: "#LANE_SIZE", Default: 1, Min: 0.1, Max: 1.5, Step: 0.05, Unit: "#PERCENTAGE", Scope: "godori",
	}),
	LaneLength: sonolus.SliderOption(sonolus.SliderOptionConfig{
		Name: "Lane Length", Default: 0.8, Min: 0.1, Max: 1, Step: 0.05, Unit: "#PERCENTAGE", Scope: "godori",
	}),
	ConnectorAlpha: sonolus.SliderOption(sonolus.SliderOptionConfig{
		Name: "#CONNECTOR_ALPHA", Default: 0.8, Min: 0.1, Max: 1, Step: 0.05, Unit: "#PERCENTAGE", Scope: "godori",
	}),
	NoteEffects: sonolus.ToggleOption(sonolus.ToggleOptionConfig{
		Name: "#NOTE_EFFECT", Default: true, Scope: "godori",
	}),
	LaneEffects: sonolus.ToggleOption(sonolus.ToggleOptionConfig{
		Name: "#LANE_EFFECT", Default: true, Scope: "godori",
	}),
	SimLines: sonolus.ToggleOption(sonolus.ToggleOptionConfig{
		Name: "#SIMLINE", Default: true, Scope: "godori",
	}),
	SimLineAlpha: sonolus.SliderOption(sonolus.SliderOptionConfig{
		Name: "#SIMLINE_ALPHA", Default: 0.5, Min: 0.1, Max: 1, Step: 0.05, Unit: "#PERCENTAGE", Scope: "godori",
	}),
	SFX: sonolus.ToggleOption(sonolus.ToggleOptionConfig{
		Name: "#EFFECT", Default: true, Scope: "godori",
	}),
	AutoSFX: sonolus.ToggleOption(sonolus.ToggleOptionConfig{
		Name: "#EFFECT_AUTO", Default: false, Scope: "godori",
	}),
	Mirror: sonolus.ToggleOption(sonolus.ToggleOptionConfig{
		Name: "#MIRROR", Default: false, Scope: "godori",
	}),
	UI: sonolus.UIConfig{
		Scope:                         "godori",
		PrimaryMetric:                 sonolus.UIMetricArcade,
		SecondaryMetric:               sonolus.UIMetricLife,
		MenuVisibility:                sonolus.UIVisibility{Scale: 1, Alpha: 1},
		JudgmentVisibility:            sonolus.UIVisibility{Scale: 1, Alpha: 1},
		ComboVisibility:               sonolus.UIVisibility{Scale: 1, Alpha: 1},
		PrimaryMetricVisibility:       sonolus.UIVisibility{Scale: 1, Alpha: 1},
		SecondaryMetricVisibility:     sonolus.UIVisibility{Scale: 1, Alpha: 1},
		ProgressVisibility:            sonolus.UIVisibility{Scale: 1, Alpha: 1},
		TutorialNavigationVisibility:  sonolus.UIVisibility{Scale: 1, Alpha: 1},
		TutorialInstructionVisibility: sonolus.UIVisibility{Scale: 1, Alpha: 1},
		JudgmentAnimation: sonolus.UIAnimation{
			Scale: sonolus.UITween{From: 0, To: 1, Duration: 0.1, Ease: sonolus.UIEaseOutCubic},
			Alpha: sonolus.UITween{From: 1, To: 0, Duration: 0.3, Ease: sonolus.UIEaseNone},
		},
		ComboAnimation: sonolus.UIAnimation{
			Scale: sonolus.UITween{From: 1.2, To: 1, Duration: 0.2, Ease: sonolus.UIEaseInCubic},
			Alpha: sonolus.UITween{From: 1, To: 1, Ease: sonolus.UIEaseNone},
		},
		JudgmentErrorStyle:     sonolus.UIJudgmentErrorLate,
		JudgmentErrorPlacement: sonolus.UIJudgmentErrorTop,
		JudgmentErrorMin:       20,
	},
}

var ROM = sonolus.ROMValues{}

//go:generate go run ./levelgen -o dev-level.json

//sonolus:level
//go:embed dev-level.json
var DevelopmentLevel sonolus.LevelFile

func main() {}
