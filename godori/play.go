//go:build play

package main

import (
	"github.com/WindowsSov8forUs/sonolus-go/v2/sonolus"
	"github.com/WindowsSov8forUs/sonolus-go/v2/sonolus/play"
)

type PlaySkin struct {
	sonolus.SkinResource
	Lane, JudgmentLine, Note sonolus.Sprite
}

var Skin = &PlaySkin{
	SkinResource: sonolus.SkinResource{RenderMode: sonolus.RenderModeLightweight},
	Lane:         sonolus.SkinSprite(sonolus.StandardSpriteLane), JudgmentLine: sonolus.SkinSprite(sonolus.StandardSpriteJudgmentLine),
	Note: sonolus.SkinSprite(sonolus.StandardSpriteNoteHeadCyan),
}

type PlayEffects struct {
	sonolus.EffectResource
	Perfect, Great, Good sonolus.Clip
}

var Effects = &PlayEffects{
	Perfect: sonolus.EffectClip(sonolus.StandardClipPerfect), Great: sonolus.EffectClip(sonolus.StandardClipGreat),
	Good: sonolus.EffectClip(sonolus.StandardClipGood),
}

type PlayParticles struct {
	sonolus.ParticleResource
	Tap sonolus.Effect
}

var Particles = &PlayParticles{Tap: sonolus.ParticleEffect(sonolus.StandardEffectNoteCircularTapCyan)}

type PlayBuckets struct {
	sonolus.BucketsResource
	Tap sonolus.Bucket
}

var Buckets = &PlayBuckets{Tap: sonolus.JudgmentBucket(
	"#MILLISECONDS",
	sonolus.JudgmentBucketSprite(Skin.Note, 0, 0, 2, 2, 0),
)}

type PlayBPMChange struct {
	play.Archetype `archetype:"name=#BPM_CHANGE"`
	Beat           float64 `archetype:"imported,name=#BEAT"`
	BPM            float64 `archetype:"imported,name=#BPM"`
}

type PlayStage struct {
	play.Archetype `archetype:"name=Stage"`
}

func (*PlayStage) SpawnOrder() float64 { return -1e8 }
func (*PlayStage) ShouldSpawn() bool   { return true }
func (*PlayStage) Preprocess() {
	play.UI.SetPrimaryMetricValue(sonolus.RuntimeUILayout{
		Anchor: sonolus.NewVec2(-0.95, 0.9), Pivot: sonolus.NewVec2(0, 0.5), Size: sonolus.NewVec2(0.8, 0.12), Alpha: 1,
	})
	play.UI.SetSecondaryMetricBar(sonolus.RuntimeUILayout{
		Anchor: sonolus.NewVec2(0.95, 0.9), Pivot: sonolus.NewVec2(1, 0.5), Size: sonolus.NewVec2(0.8, 0.08), Alpha: 1,
	})
	play.UI.SetSecondaryMetricValue(sonolus.RuntimeUILayout{
		Anchor: sonolus.NewVec2(0.95, 0.82), Pivot: sonolus.NewVec2(1, 0.5), Size: sonolus.NewVec2(0.8, 0.1), Alpha: 1,
	})
}
func (*PlayStage) UpdateParallel() {
	Skin.Lane.Draw(stageRect().ToQuad(), 10, 0.8)
	Skin.JudgmentLine.Draw(judgmentRect().ToQuad(), 20, 1)
}

type PlayTapNote struct {
	play.Archetype      `archetype:"name=TapNote,hasInput=true"`
	play.CallbackOrders `archetype:"preprocess=-10"`
	Beat                float64 `archetype:"imported,name=#BEAT"`
	Lane                float64 `archetype:"imported,name=lane"`
	TargetTime          float64 `archetype:"data"`
}

func (n *PlayTapNote) Preprocess() {
	n.TargetTime = play.Time.BeatToTime(n.Beat)
	result := play.Entity.Result()
	result.Bucket = Buckets.Tap
	play.Entity.SetResult(result)
}

func (n *PlayTapNote) SpawnOrder() float64 { return n.TargetTime - noteTravelTime }
func (n *PlayTapNote) ShouldSpawn() bool   { return play.Time.Now() >= n.TargetTime-noteTravelTime }

func (n *PlayTapNote) UpdateSequential() {
	if play.Time.Now() > n.TargetTime+0.15 {
		result := play.Entity.Result()
		result.Judgment = sonolus.JudgmentMiss
		result.Accuracy = 0.15
		result.Bucket = Buckets.Tap
		result.BucketValue = 150
		play.Entity.SetResult(result)
		play.Entity.SetDespawn(true)
	}
}

func (n *PlayTapNote) Touch() {
	for i := 0; i < play.Touches.Count(); i++ {
		touch := play.Touches.Get(i)
		if touch.Started && hitbox(n.Lane).Contains(touch.Position) {
			accuracy := touch.StartTime - n.TargetTime
			windows := sonolus.JudgmentWindows{
				Perfect: sonolus.Range{Min: -0.05, Max: 0.05},
				Great:   sonolus.Range{Min: -0.1, Max: 0.1},
				Good:    sonolus.Range{Min: -0.15, Max: 0.15},
			}
			judgment := play.Input.Judge(touch.StartTime, n.TargetTime, windows)
			result := play.Entity.Result()
			result.Judgment = judgment
			result.Accuracy = accuracy
			result.Bucket = Buckets.Tap
			result.BucketValue = accuracy * 1000
			play.Entity.SetResult(result)
			if Config.Effects {
				if judgment == sonolus.JudgmentPerfect {
					play.Audio.Play(Effects.Perfect, 0.02)
				} else if judgment == sonolus.JudgmentGreat {
					play.Audio.Play(Effects.Great, 0.02)
				} else if judgment == sonolus.JudgmentGood {
					play.Audio.Play(Effects.Good, 0.02)
				}
				Particles.Tap.Spawn(noteRect(n.Lane, judgmentLineY).Scale(1.5).ToQuad(), 0.3, false)
			}
			play.Entity.SetDespawn(true)
			return
		}
	}
}

func (n *PlayTapNote) UpdateParallel() {
	Skin.Note.Draw(noteRect(n.Lane, noteY(n.TargetTime, play.Time.Now())).ToQuad(), 30, 1)
}
