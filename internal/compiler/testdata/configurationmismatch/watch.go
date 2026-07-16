//go:build watch

package main

import "github.com/WindowsSov8forUs/sonolus-go/sonolus"

type GameConfiguration struct {
	sonolus.Configuration
	Speed float64 `configuration:"slider,name=Speed,def=2,min=0,max=2,step=1"`
}

var Config GameConfiguration
