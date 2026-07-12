//go:build play

package main

import (
	"github.com/WindowsSov8forUs/sonolus-go/sonolus"
	"github.com/WindowsSov8forUs/sonolus-go/sonolus/play"
)

type SkinData struct {
	sonolus.SkinResource
	Note, Fallback sonolus.Sprite
}

var Skin = &SkinData{
	SkinResource: sonolus.SkinResource{RenderMode: sonolus.RenderModeStandard}, Note: sonolus.SkinSprite("note"), Fallback: sonolus.SkinSprite("fallback")}

type EffectData struct {
	sonolus.EffectResource
	Hit sonolus.Clip
}

var Effects = &EffectData{Hit: sonolus.EffectClip("hit")}

type ParticleData struct {
	sonolus.ParticleResource
	Hit sonolus.Effect
}

var Particles = &ParticleData{Hit: sonolus.ParticleEffect("hit")}

type BucketData struct {
	sonolus.BucketsResource
	Tap sonolus.Bucket
}

var Buckets = &BucketData{Tap: sonolus.JudgmentBucket("#MILLISECONDS", sonolus.JudgmentBucketSprite(Skin.Note, 0, 0, 1, 1, 0))}

type Note struct {
	play.Archetype      `sonolus:"name=Note,hasInput=true"`
	play.CallbackOrders `sonolus:"preprocess=-2"`
	Beat                float64 `sonolus:"imported,name=#BEAT,default=1"`
	Value               float64 `sonolus:"memory"`
	Hit                 float64 `sonolus:"exported,name=hit"`
}

func (n *Note) Preprocess() {
	if n.Beat > 0 {
		n.Value = n.Beat + 1
	} else {
		n.Value = 0
	}
}

func (*Note) ShouldSpawn() bool { return true }
