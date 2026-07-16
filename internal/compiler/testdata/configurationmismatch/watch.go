//go:build watch

package main

import "github.com/WindowsSov8forUs/sonolus-go/sonolus"

type GameConfiguration struct {
	sonolus.Configuration
	Speed float64
}

var Config = GameConfiguration{Speed: sonolus.SliderOption(sonolus.SliderOptionConfig{Name: "Speed", Default: 2, Min: 0, Max: 2, Step: 1})}
