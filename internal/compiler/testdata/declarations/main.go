package main

import (
	"math"
	"math/rand"

	"github.com/WindowsSov8forUs/sonolus-go/sonolus"
	"github.com/WindowsSov8forUs/sonolus-go/sonolus/play"
	"github.com/WindowsSov8forUs/sonolus-go/sonolus/preview"
	"github.com/WindowsSov8forUs/sonolus-go/sonolus/tutorial"
	"github.com/WindowsSov8forUs/sonolus-go/sonolus/watch"
)

type GameConfiguration struct {
	sonolus.Configuration
	Speed  float64 `configuration:"slider,name=speed,def=1,min=0.5,max=2,step=0.1"`
	Mirror bool    `configuration:"toggle,name=mirror,def=false"`
	Lane   int     `configuration:"select,name=lane,def=0,values=normal|wide"`
}

var Config GameConfiguration

//sonolus:resource skin standard
type PlaySkin struct {
	Note sonolus.Sprite
}

//sonolus:resource skin standard
var PlayAssets = &PlaySkin{Note: sonolus.SkinSprite("#NOTE_HEAD_CYAN")}

type TapNote struct {
	play.Archetype      `sonolus:"name=TapNote,hasInput=true"`
	play.CallbackOrders `sonolus:"preprocess=-10,updateSequential=5"`
	Beat                float64      `sonolus:"imported,name=#BEAT,default=1"`
	Position            sonolus.Vec2 `sonolus:"memory"`
	Shared              float64      `sonolus:"shared"`
	HitTime             float64      `sonolus:"exported,name=hitTime"`
}

func (*TapNote) Preprocess() {
	_ = math.Sin(math.Pi)
	_ = rand.Float64()
	_ = rand.Intn(2)
}
func (*TapNote) UpdateSequential() {}
func (*TapNote) ShouldSpawn() bool { return true }

type WatchCallbacks struct{ watch.GlobalCallbacks }

var WatchGlobals WatchCallbacks

func UpdateSpawn() float64 { return 0 }

type WatchNote struct {
	watch.Archetype `sonolus:"name=TapNote,hasInput=true"`
	Beat            float64 `sonolus:"imported,name=#BEAT"`
}

func (*WatchNote) SpawnTime() float64 { return 0 }

type PreviewNote struct {
	preview.Archetype `sonolus:"name=TapNote"`
	Beat              float64 `sonolus:"imported,name=#BEAT"`
}

func (*PreviewNote) Render() {}

type TutorialCallbacks struct{ tutorial.GlobalCallbacks }

var TutorialGlobals TutorialCallbacks

func Preprocess() {}
func Navigate()   {}
func Update()     {}

var ROM = sonolus.ROMValues{1, 2, 3}

func main() {}
