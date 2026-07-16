package main

import "github.com/WindowsSov8forUs/sonolus-go/v2/sonolus"

type ConfigData struct {
	sonolus.Configuration
	Lane int
	UI   sonolus.UIConfig
}

var Config = ConfigData{
	Lane: sonolus.SelectOption(sonolus.SelectOptionConfig{Default: 2, Values: []string{"4", "6"}}),
	UI: sonolus.UIConfig{
		JudgmentErrorStyle: sonolus.UIJudgmentErrorStyle("arrow"),
	},
}

func main() {}
