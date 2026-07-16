package main

import (
	"github.com/WindowsSov8forUs/sonolus-go/v2/sonolus"
	"github.com/WindowsSov8forUs/sonolus-go/v2/sonolus/play"
)

type Bad struct {
	play.Archetype `archetype:"name=Bad,typo=true"`
	Value          float64 `archetype:"memory,unknown=true"`
}

type Config struct {
	sonolus.Configuration
	Bad float64 `configuration:"slider,def=0,min=0,max=1,step=1,mystery=true"`
}

var Configuration Config
