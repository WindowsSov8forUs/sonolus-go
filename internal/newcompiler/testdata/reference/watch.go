//go:build watch

package main

import (
	"github.com/WindowsSov8forUs/sonolus-go/sonolus"
	"github.com/WindowsSov8forUs/sonolus-go/sonolus/watch"
)

//sonolus:resource skin
type SkinData struct{ Note sonolus.Sprite }

//sonolus:resource skin
var Skin = &SkinData{Note: sonolus.SkinSprite("note")}

//sonolus:resource effect
type EffectData struct{ Hit sonolus.Clip }

//sonolus:resource effect
var Effects = &EffectData{Hit: sonolus.EffectClip("hit")}

//sonolus:resource particle
type ParticleData struct{ Hit sonolus.Effect }

//sonolus:resource particle
var Particles = &ParticleData{Hit: sonolus.ParticleEffect("hit")}

//sonolus:resource buckets
type BucketData struct{ Tap sonolus.Bucket }

//sonolus:resource buckets
var Buckets = &BucketData{Tap: sonolus.JudgmentBucket("#MILLISECONDS")}

type Globals struct{ watch.GlobalCallbacks }

var Global Globals

func UpdateSpawn() float64 { return 3 }

type Note struct {
	watch.Archetype `sonolus:"name=Note"`
	Beat            float64 `sonolus:"imported,name=#BEAT"`
}

func (*Note) SpawnTime() float64 { return 1 }
