package main

import "github.com/WindowsSov8forUs/sonolus-go/sonolus"

type ConfigData struct {
	sonolus.Configuration
	Speed float64 `sonolus:"slider,def=1,min=0,max=2,step=0.1"`
}

var Config ConfigData

func main() {}
