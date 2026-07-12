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
	Speed  float64
	Mirror bool
	Lane   int
}

var Config = GameConfiguration{
	Speed:  sonolus.SliderOption(sonolus.SliderOptionConfig{Name: "speed", Default: 1, Min: 0.5, Max: 2, Step: 0.1}),
	Mirror: sonolus.ToggleOption(sonolus.ToggleOptionConfig{Name: "mirror"}),
	Lane:   sonolus.SelectOption(sonolus.SelectOptionConfig{Name: "lane", Values: []string{"normal", "wide"}}),
}

type PlaySkin struct {
	sonolus.SkinResource

	Note sonolus.Sprite
}

var PlayAssets = &PlaySkin{
	SkinResource: sonolus.SkinResource{RenderMode: sonolus.RenderModeStandard}, Note: sonolus.SkinSprite("#NOTE_HEAD_CYAN")}

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
