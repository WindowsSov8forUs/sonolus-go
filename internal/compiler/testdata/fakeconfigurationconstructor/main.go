package main

import "github.com/WindowsSov8forUs/sonolus-go/sonolus"

type ConfigData struct {
	sonolus.Configuration
	Speed float64
}

func SliderOption(sonolus.SliderOptionConfig) float64 { return 1 }

var Config = ConfigData{Speed: SliderOption(sonolus.SliderOptionConfig{Default: 1, Min: 0, Max: 2, Step: 1})}

func main() {}
