//go:build play

package main

import (
	"github.com/WindowsSov8forUs/sonolus-go/sonolus"
	"github.com/WindowsSov8forUs/sonolus-go/sonolus/play"
)

//sonolus:resource skin standard
type SkinData struct {
	Notes [2]sonolus.Sprite
}

//sonolus:resource skin standard
var Skin = &SkinData{
	Notes: [2]sonolus.Sprite{
		sonolus.SkinSprite("#NOTE_HEAD_CYAN"),
		sonolus.SkinSprite("conformance.note"),
	},
}

//sonolus:resource effect
type EffectData struct {
	Hit sonolus.Clip
}

//sonolus:resource effect
var Effects = &EffectData{
	Hit: sonolus.EffectClip("#PERFECT"),
}

//sonolus:resource particle
type ParticleData struct {
	Hit sonolus.Effect
}

//sonolus:resource particle
var Particles = &ParticleData{
	Hit: sonolus.ParticleEffect("#NOTE_CIRCULAR_TAP_CYAN"),
}

//sonolus:resource buckets
type BucketData struct {
	Tap sonolus.Bucket
}

//sonolus:resource buckets
var Buckets = &BucketData{
	Tap: sonolus.JudgmentBucket("#MILLISECONDS", sonolus.JudgmentBucketSprite(Skin.Notes[0], 0, 0, 1, 1, 0)),
}

type Note struct {
	play.Archetype      `sonolus:"name=ConformanceNote,hasInput=true"`
	play.CallbackOrders `sonolus:"preprocess=-10"`

	Beat   float64 `sonolus:"imported,name=#BEAT,default=0"`
	Value  float64 `sonolus:"memory"`
	Result float64 `sonolus:"exported,name=result"`
}

func (n *Note) Preprocess() {
	result := sum(1.0, 2.0, 3.0)
	offset := 1.0
	transform := func(value float64) float64 {
		return value + offset
	}
	offset = 2
	result = transform(result)

	values := [3]float64{n.Beat, 2, 4}
	for _, value := range values {
		result += value
	}
	for i := range 3 {
		result += float64(i)
	}
	switch int(result) % 2 {
	case 0:
		result += 1
	default:
		result -= 1
	}
	n.Value = result
	n.Result = result
	play.Debug.Log(result)
}

func (*Note) ShouldSpawn() bool {
	return true
}
