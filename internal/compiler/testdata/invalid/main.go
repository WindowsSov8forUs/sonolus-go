package main

import (
	"github.com/WindowsSov8forUs/sonolus-go/v2/sonolus"
	"github.com/WindowsSov8forUs/sonolus-go/v2/sonolus/play"
)

type Bad struct {
	play.Archetype `archetype:"name=Bad,typo=true"`
	Value          float64 `archetype:"memory,unknown=true"`
}

type MultiSlotDefault struct {
	play.Archetype `archetype:"name=MultiSlotDefault"`
	Position       sonolus.Vec2 `archetype:"imported,name=position,default=1"`
}

type FlattenedCollision struct {
	play.Archetype `archetype:"name=FlattenedCollision"`
	Position       sonolus.Vec2 `archetype:"imported,name=value"`
	X              float64      `archetype:"imported,name=value.x"`
}

type CollisionBase struct {
	play.Archetype `archetype:"abstract"`
	Position       sonolus.Vec2 `archetype:"imported,name=point"`
}

type InheritedFlattenedCollision struct {
	CollisionBase  `archetype:"base"`
	play.Archetype `archetype:"name=InheritedFlattenedCollision"`
	X              float64 `archetype:"imported,name=point.x"`
}

type Config struct {
	sonolus.Configuration
	Bad float64 `configuration:"slider,def=0,min=0,max=1,step=1,mystery=true"`
}

var Configuration Config
