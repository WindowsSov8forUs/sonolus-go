//go:build watch

package main

import (
	"github.com/WindowsSov8forUs/sonolus-go/sonolus"
	"github.com/WindowsSov8forUs/sonolus-go/sonolus/watch"
)

type SkinData struct {
	sonolus.SkinResource
	Note sonolus.Sprite
}

var Skin = &SkinData{Note: sonolus.SkinSprite("note")}

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

var Buckets = &BucketData{Tap: sonolus.JudgmentBucket("#MILLISECONDS")}

type Globals struct{ watch.GlobalCallbacks }

var Global Globals

func UpdateSpawn() float64 { return 3 }

type Note struct {
	watch.Archetype `sonolus:"name=Note"`
	Beat            float64 `sonolus:"imported,name=#BEAT"`
}

func (*Note) SpawnTime() float64 { return 1 }
