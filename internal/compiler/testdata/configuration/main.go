package main

import "github.com/WindowsSov8forUs/sonolus-go/sonolus"

type ConfigData struct {
	sonolus.Configuration
	Speed    float64          `configuration:"slider,name=Speed,def=1,min=0,max=2,step=0.1"`
	Enabled  bool             `configuration:"toggle,name=Enabled,def=true"`
	Lane     int              `configuration:"select,name=Lane,def=1,values=4|6|8"`
	UI       sonolus.UIConfig `configuration:"ui"`
	Fallback []string         `configuration:"replayFallback"`
}

var Config = ConfigData{
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
