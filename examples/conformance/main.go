package main

import "github.com/WindowsSov8forUs/sonolus-go/sonolus"

type GameConfiguration struct {
	sonolus.Configuration
	Speed float64 `configuration:"slider,name=Speed,def=1,min=0.5,max=2,step=0.1"`
	Auto  bool    `configuration:"toggle,name=Auto,def=false"`
}

var Configuration GameConfiguration
var ROM = sonolus.ROMValues{1, 2, 3}

func sum[T ~float64](values ...T) float64 {
	result := 0.0
	for _, value := range values {
		result += float64(value)
	}
	return result
}

func main() {}
