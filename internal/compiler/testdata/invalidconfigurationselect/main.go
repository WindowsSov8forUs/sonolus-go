package main

import "github.com/WindowsSov8forUs/sonolus-go/sonolus"

type ConfigData struct {
	sonolus.Configuration
	Lane int
}

var Config = ConfigData{Lane: sonolus.SelectOption(sonolus.SelectOptionConfig{Default: 2, Values: []string{"4", "6"}})}

func main() {}
