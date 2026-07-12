//go:build play

package main

import (
	"github.com/WindowsSov8forUs/sonolus-go/sonolus"
	"github.com/WindowsSov8forUs/sonolus-go/sonolus/play"
)

//sonolus:resource skin standard
type SkinData struct{ Note, Fallback sonolus.Sprite }

//sonolus:resource skin standard
var Skin = &SkinData{Note: sonolus.SkinSprite("note"), Fallback: sonolus.SkinSprite("fallback")}

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
