package main

import "github.com/WindowsSov8forUs/sonolus-go/sonolus"

type GameConfiguration struct {
	sonolus.Configuration
	Speed float64
	Auto  bool
}

var Configuration = GameConfiguration{
	Speed: sonolus.SliderOption(sonolus.SliderOptionConfig{Name: "Speed", Default: 1, Min: 0.5, Max: 2, Step: 0.1}),
	Auto:  sonolus.ToggleOption(sonolus.ToggleOptionConfig{Name: "Auto"}),
}
var ROM = sonolus.ROMValues{1, 2, 3}

func sum[T ~float64](values ...T) float64 {
	result := 0.0
	for _, value := range values {
		result += float64(value)
	}
	return result
}

func main() {}
