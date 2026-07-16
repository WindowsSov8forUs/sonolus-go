package main

import "github.com/WindowsSov8forUs/sonolus-go/v2/sonolus"

type ConfigData struct {
	sonolus.Configuration
	UI sonolus.UIConfig
}

func makeUI() sonolus.UIConfig { return sonolus.UIConfig{} }

var Config = ConfigData{UI: makeUI()}

func main() {}
