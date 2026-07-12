package main

import "github.com/WindowsSov8forUs/sonolus-go/sonolus"

type ConfigData struct {
	sonolus.Configuration
	Speed float64
}

var Config = ConfigData{}

func main() {}
