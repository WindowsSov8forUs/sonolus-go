package main

import "github.com/WindowsSov8forUs/sonolus-go/v2/sonolus"

type ConfigData struct {
	sonolus.Configuration
	Speed float64
}

var speed = sonolus.SliderOption(sonolus.SliderOptionConfig{Default: 1, Min: 0.5, Max: 2, Step: 0.1})
var Config = ConfigData{Speed: speed}

func main() {}
