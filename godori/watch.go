//go:build watch

package main

import (
	"github.com/WindowsSov8forUs/sonolus-go/v2/sonolus"
	"github.com/WindowsSov8forUs/sonolus-go/v2/sonolus/watch"
)

type WatchSkin struct {
	sonolus.SkinResource
	Lane, JudgmentLine, Note sonolus.Sprite
}

var Skin = &WatchSkin{
	SkinResource: sonolus.SkinResource{RenderMode: sonolus.RenderModeLightweight},
	Lane:         sonolus.SkinSprite(sonolus.StandardSpriteLane), JudgmentLine: sonolus.SkinSprite(sonolus.StandardSpriteJudgmentLine),
	Note: sonolus.SkinSprite(sonolus.StandardSpriteNoteHeadCyan),
}

type WatchEffects struct {
	sonolus.EffectResource
	Perfect, Great, Good sonolus.Clip
}

var Effects = &WatchEffects{
	Perfect: sonolus.EffectClip(sonolus.StandardClipPerfect), Great: sonolus.EffectClip(sonolus.StandardClipGreat),
	Good: sonolus.EffectClip(sonolus.StandardClipGood),
}

type WatchParticles struct {
	sonolus.ParticleResource
	Tap sonolus.Effect
}

var Particles = &WatchParticles{Tap: sonolus.ParticleEffect(sonolus.StandardEffectNoteCircularTapCyan)}

type WatchBuckets struct {
	sonolus.BucketsResource
	Tap sonolus.Bucket
}

var Buckets = &WatchBuckets{Tap: sonolus.JudgmentBucket("#MILLISECONDS", sonolus.JudgmentBucketSprite(Skin.Note, 0, 0, 2, 2, 0))}

type WatchGlobals struct{ watch.GlobalCallbacks }

var Global WatchGlobals

func UpdateSpawn() float64 { return watch.Time.Scaled() }

type WatchBPMChange struct {
	watch.Archetype `archetype:"name=#BPM_CHANGE"`
	Beat            float64 `archetype:"imported,name=#BEAT"`
	BPM             float64 `archetype:"imported,name=#BPM"`
}

type WatchStage struct {
	watch.Archetype `archetype:"name=Stage"`
}

func (*WatchStage) SpawnTime() float64   { return -1e8 }
func (*WatchStage) DespawnTime() float64 { return 1e8 }
func (*WatchStage) UpdateParallel() {
	Skin.Lane.Draw(stageRect().ToQuad(), 10, 0.8)
	Skin.JudgmentLine.Draw(judgmentRect().ToQuad(), 20, 1)
}

type WatchTapNote struct {
	watch.Archetype `archetype:"name=TapNote"`
	Beat            float64          `archetype:"imported,name=#BEAT"`
	Lane            float64          `archetype:"imported,name=lane"`
	Judgment        sonolus.Judgment `archetype:"imported,name=#JUDGMENT"`
	Accuracy        float64          `archetype:"imported,name=#ACCURACY"`
	TargetTime      float64          `archetype:"data"`
	Played          bool             `archetype:"memory"`
}

func (n *WatchTapNote) Preprocess() {
	n.TargetTime = watch.Time.BeatToTime(n.Beat)
	result := watch.Entity.Result()
	result.TargetTime = n.TargetTime
	result.Bucket = Buckets.Tap
	result.BucketValue = n.Accuracy * 1000
	watch.Entity.SetResult(result)
}

func (n *WatchTapNote) SpawnTime() float64   { return n.TargetTime - noteTravelTime }
func (n *WatchTapNote) DespawnTime() float64 { return n.TargetTime + 0.5 }
func (n *WatchTapNote) UpdateSequential() {
	if !n.Played && watch.Time.Now() >= n.TargetTime+n.Accuracy {
		n.Played = true
		if Config.Effects {
			if n.Judgment == sonolus.JudgmentPerfect {
				watch.Audio.Play(Effects.Perfect, 0.02)
			} else if n.Judgment == sonolus.JudgmentGreat {
				watch.Audio.Play(Effects.Great, 0.02)
			} else if n.Judgment == sonolus.JudgmentGood {
				watch.Audio.Play(Effects.Good, 0.02)
			}
			Particles.Tap.Spawn(noteRect(n.Lane, judgmentLineY).Scale(1.5).ToQuad(), 0.3, false)
		}
	}
}
func (n *WatchTapNote) UpdateParallel() {
	Skin.Note.Draw(noteRect(n.Lane, noteY(n.TargetTime, watch.Time.Now())).ToQuad(), 30, 1)
}
