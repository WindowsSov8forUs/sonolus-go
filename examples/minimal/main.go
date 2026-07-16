package main

import "github.com/WindowsSov8forUs/sonolus-go/sonolus"

type GameConfiguration struct {
	sonolus.Configuration
	Speed float64 `configuration:"slider,name=Speed,def=1,min=0.5,max=2,step=0.1"`
}

var Configuration GameConfiguration
var ROM = sonolus.ROMValues{}

func main() {}
