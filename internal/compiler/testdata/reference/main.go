package main

import "github.com/WindowsSov8forUs/sonolus-go/sonolus"

type Configuration struct {
	sonolus.Configuration
	Speed float64
}

var Config = Configuration{Speed: sonolus.SliderOption(sonolus.SliderOptionConfig{Name: "Speed", Default: 1, Min: 0.5, Max: 2, Step: 0.1})}
var ROM = sonolus.ROMValues{1, 2}

func main() {}
