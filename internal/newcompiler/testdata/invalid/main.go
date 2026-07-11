package main

import (
	"github.com/WindowsSov8forUs/sonolus-go/sonolus"
	"github.com/WindowsSov8forUs/sonolus-go/sonolus/play"
)

type Bad struct {
	play.Archetype `sonolus:"name=Bad,typo=true"`
	Value          float64 `sonolus:"memory,unknown=true"`
}

type Config struct {
	sonolus.Configuration
	Bad float64 `sonolus:"slider,def=0,min=0,max=1,step=1,mystery=true"`
}

var Configuration Config
