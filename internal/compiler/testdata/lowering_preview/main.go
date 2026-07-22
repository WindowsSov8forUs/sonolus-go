package main

import (
	"github.com/WindowsSov8forUs/sonolus-go/v2/sonolus"
	"github.com/WindowsSov8forUs/sonolus-go/v2/sonolus/preview"
)

type Note struct {
	preview.Archetype `archetype:"name=Note"`
}

type SkinData struct {
	sonolus.SkinResource
	Note sonolus.Sprite
}

var Skin = &SkinData{Note: sonolus.SkinSprite("note")}

type EffectsData struct {
	sonolus.EffectResource
	Hit sonolus.Clip
}

var Effects = &EffectsData{Hit: sonolus.EffectClip("hit")}

type ParticlesData struct {
	sonolus.ParticleResource
	Hit sonolus.Effect
}

var Particles = &ParticlesData{Hit: sonolus.ParticleEffect("hit")}

type BucketsData struct {
	sonolus.BucketsResource
	Tap sonolus.Bucket
}

var Buckets = &BucketsData{Tap: sonolus.JudgmentBucket("#MILLISECONDS", sonolus.JudgmentBucketSprite(Skin.Note, 0, 0, 1, 1, 0))}

type InstructionsData struct {
	sonolus.InstructionResource
	Hit sonolus.Text
}

var Instructions = &InstructionsData{Hit: sonolus.InstructionText("hit")}

type InstructionIconsData struct {
	sonolus.InstructionIconResource
	Hit sonolus.Icon
}

var InstructionIcons = &InstructionIconsData{Hit: sonolus.InstructionIcon("hit")}

type ConfigurationData struct {
	sonolus.Configuration
	Speed float64
}

var Configuration = ConfigurationData{Speed: sonolus.SliderOption(sonolus.SliderOptionConfig{Name: "Speed", Default: 1, Min: 0.5, Max: 2, Step: 0.1})}

func (*Note) Preprocess() {
	transform := preview.SkinTransform.Get()
	preview.SkinTransform.Set(transform)
	preview.Canvas.Set(preview.CanvasOptions{Scroll: preview.ScrollTopToBottom, Size: 10})
	preview.UI.SetMenu(preview.UI.Menu())
	preview.UI.SetProgress(preview.UI.Progress())
	value := preview.Canvas.Size() + preview.Screen.Rect().Width() + preview.SafeArea.Rect().Width()
	value += preview.Time.BeatToBPM(1) + preview.Time.BeatToTime(1)
	value += preview.Time.BeatToStartingBeat(1) + preview.Time.BeatToStartingTime(1)
	value += preview.Time.TimeToScaledTime(1) + preview.Time.TimeToStartingScaledTime(1)
	value += preview.Time.TimeToStartingTime(1) + preview.Time.TimeToTimeScale(1)
	value += preview.Entity.Key() + preview.CurrentEntityRef[Note]().Index + float64(preview.ArchetypeID[Note]()) + preview.ArchetypeKey[Note]()
	value += preview.LevelData.Get(0)
	preview.LevelData.Set(1, value)
	value += preview.UI.MenuConfiguration().Scale + preview.UI.ProgressConfiguration().Scale
	_, _ = preview.Entity.Info(), preview.Entity.InfoAt(0)
	_, _, _ = preview.Canvas.Scroll(), preview.Environment.Debug(), preview.Environment.AspectRatio()
	preview.Debug.Log(value)
	preview.Debug.Pause()
}

func (*Note) Render() {
	preview.Canvas.Print(preview.PrintOptions{
		Value: 1, Format: sonolus.PrintFormatNumber, DecimalPlaces: 2,
		Anchor: sonolus.NewVec2(0, 0), Pivot: sonolus.NewVec2(0.5, 0.5), Size: sonolus.NewVec2(1, 1),
		Alpha: 1,
	})
	Skin.Note.Draw(sonolus.Rect{T: 1, R: 1, B: -1, L: -1}.ToQuad(), 0, 1)
}

func main() {}
