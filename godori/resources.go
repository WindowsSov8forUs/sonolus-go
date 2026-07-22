package main

import "github.com/WindowsSov8forUs/sonolus-go/v2/sonolus"

type SkinData struct {
	sonolus.SkinResource
	Lane, JudgmentLine, Note, Flick, FlickArrow, RightFlick, RightFlickArrow, LeftFlick, LeftFlickArrow sonolus.Sprite
	HoldHead, HoldTail, HoldConnector, SimLine, HoldTick                                                sonolus.Sprite
	BPM, Timescale, Measure, Time                                                                       sonolus.Sprite
	StageMiddle, LeftBorder, RightBorder, Slot, Cover                                                   sonolus.Sprite
}

var Skin = &SkinData{
	SkinResource: sonolus.SkinResource{RenderMode: sonolus.RenderModeLightweight},
	Lane:         sonolus.SkinSprite(sonolus.StandardSpriteLane), JudgmentLine: sonolus.SkinSprite(sonolus.StandardSpriteJudgmentLine),
	Note: sonolus.SkinSprite(sonolus.StandardSpriteNoteHeadCyan), Flick: sonolus.SkinSprite(sonolus.StandardSpriteNoteHeadRed),
	FlickArrow: sonolus.SkinSprite(sonolus.StandardSpriteDirectionalMarkerRed),
	RightFlick: sonolus.SkinSprite(sonolus.StandardSpriteNoteHeadYellow), RightFlickArrow: sonolus.SkinSprite(sonolus.StandardSpriteDirectionalMarkerYellow),
	LeftFlick: sonolus.SkinSprite(sonolus.StandardSpriteNoteHeadPurple), LeftFlickArrow: sonolus.SkinSprite(sonolus.StandardSpriteDirectionalMarkerPurple),
	HoldHead: sonolus.SkinSprite(sonolus.StandardSpriteNoteHeadGreen), HoldTail: sonolus.SkinSprite(sonolus.StandardSpriteNoteTailGreen),
	HoldConnector: sonolus.SkinSprite(sonolus.StandardSpriteNoteConnectionGreenSeamless),
	SimLine:       sonolus.SkinSprite(sonolus.StandardSpriteSimultaneousConnectionNeutralSeamless),
	HoldTick:      sonolus.SkinSprite(sonolus.StandardSpriteNoteTickGreen),
	BPM:           sonolus.SkinSprite(sonolus.StandardSpriteGridPurple),
	Timescale:     sonolus.SkinSprite(sonolus.StandardSpriteGridYellow),
	Measure:       sonolus.SkinSprite(sonolus.StandardSpriteGridNeutral),
	Time:          sonolus.SkinSprite(sonolus.StandardSpriteGridCyan),
	StageMiddle:   sonolus.SkinSprite(sonolus.StandardSpriteStageMiddle), LeftBorder: sonolus.SkinSprite(sonolus.StandardSpriteStageLeftBorder),
	RightBorder: sonolus.SkinSprite(sonolus.StandardSpriteStageRightBorder), Slot: sonolus.SkinSprite(sonolus.StandardSpriteNoteSlot),
	Cover: sonolus.SkinSprite(sonolus.StandardSpriteStageCover),
}

type EffectsData struct {
	sonolus.EffectResource
	Stage, Perfect, Great, Good                           sonolus.Clip
	PerfectAlternative, GreatAlternative, GoodAlternative sonolus.Clip
	Hold                                                  sonolus.Clip
}

var Effects = &EffectsData{
	Stage: sonolus.EffectClip(sonolus.StandardClipStage), Perfect: sonolus.EffectClip(sonolus.StandardClipPerfect),
	Great: sonolus.EffectClip(sonolus.StandardClipGreat), Good: sonolus.EffectClip(sonolus.StandardClipGood),
	PerfectAlternative: sonolus.EffectClip(sonolus.StandardClipPerfectAlternative),
	GreatAlternative:   sonolus.EffectClip(sonolus.StandardClipGreatAlternative),
	GoodAlternative:    sonolus.EffectClip(sonolus.StandardClipGoodAlternative),
	Hold:               sonolus.EffectClip(sonolus.StandardClipHold),
}

type ParticlesData struct {
	sonolus.ParticleResource
	Tap, Flick, RightFlick, LeftFlick, Hold, Lane                         sonolus.Effect
	TapLinear, FlickLinear, RightFlickLinear, LeftFlickLinear, HoldLinear sonolus.Effect
	HoldActive                                                            sonolus.Effect
}

var Particles = &ParticlesData{
	Tap: sonolus.ParticleEffect(sonolus.StandardEffectNoteCircularTapCyan), Flick: sonolus.ParticleEffect(sonolus.StandardEffectNoteCircularAlternativeRed),
	RightFlick: sonolus.ParticleEffect(sonolus.StandardEffectNoteCircularAlternativeYellow), LeftFlick: sonolus.ParticleEffect(sonolus.StandardEffectNoteCircularAlternativePurple),
	Hold: sonolus.ParticleEffect(sonolus.StandardEffectNoteCircularTapGreen), Lane: sonolus.ParticleEffect(sonolus.StandardEffectLaneLinear),
	TapLinear: sonolus.ParticleEffect(sonolus.StandardEffectNoteLinearTapCyan), FlickLinear: sonolus.ParticleEffect(sonolus.StandardEffectNoteLinearAlternativeRed),
	RightFlickLinear: sonolus.ParticleEffect(sonolus.StandardEffectNoteLinearAlternativeYellow), LeftFlickLinear: sonolus.ParticleEffect(sonolus.StandardEffectNoteLinearAlternativePurple),
	HoldLinear: sonolus.ParticleEffect(sonolus.StandardEffectNoteLinearTapGreen), HoldActive: sonolus.ParticleEffect(sonolus.StandardEffectNoteCircularHoldGreen),
}

type BucketsData struct {
	sonolus.BucketsResource
	Tap, HoldHead, HoldEnd, HoldTick, Flick, DirectionalFlick sonolus.Bucket
}

var Buckets = &BucketsData{
	Tap: sonolus.JudgmentBucket("#MILLISECONDS",
		sonolus.JudgmentBucketSprite(Skin.Note, 0, 0, 2, 2, -90)),
	HoldHead: sonolus.JudgmentBucket("#MILLISECONDS",
		sonolus.JudgmentBucketSprite(Skin.HoldConnector, 0.5, 0, 2, 5, -90),
		sonolus.JudgmentBucketSprite(Skin.HoldHead, -2, 0, 2, 2, -90)),
	HoldEnd: sonolus.JudgmentBucket("#MILLISECONDS",
		sonolus.JudgmentBucketSprite(Skin.HoldConnector, -0.5, 0, 2, 5, -90),
		sonolus.JudgmentBucketSprite(Skin.HoldTail, 2, 0, 2, 2, -90)),
	HoldTick: sonolus.JudgmentBucket("#MILLISECONDS",
		sonolus.JudgmentBucketSprite(Skin.HoldConnector, 0, 0, 2, 5.5, -90),
		sonolus.JudgmentBucketSprite(Skin.HoldTick, 0, 0, 2, 2, -90)),
	Flick: sonolus.JudgmentBucket("#MILLISECONDS",
		sonolus.JudgmentBucketSprite(Skin.Flick, 0, 0, 2, 2, -90),
		sonolus.JudgmentBucketSprite(Skin.FlickArrow, 1, 0, 2, 2, -90)),
	DirectionalFlick: sonolus.JudgmentBucket("#MILLISECONDS",
		sonolus.JudgmentBucketSprite(Skin.RightFlick, 2, 0, 2, 2, -90),
		sonolus.JudgmentBucketSprite(Skin.LeftFlick, -2, 0, 2, 2, 90),
		sonolus.JudgmentBucketSprite(Skin.RightFlickArrow, 3, 0, 2, 2, -90),
		sonolus.JudgmentBucketSprite(Skin.LeftFlickArrow, -3, 0, 2, 2, 90)),
}

type InstructionsData struct {
	sonolus.InstructionResource
	Tap, TapFlick, TapHold, HoldFollow, Release, HoldFlick sonolus.Text
}

var Instructions = &InstructionsData{
	Tap:        sonolus.InstructionText("#TAP"),
	TapFlick:   sonolus.InstructionText("#TAP_FLICK"),
	TapHold:    sonolus.InstructionText("#TAP_HOLD"),
	HoldFollow: sonolus.InstructionText("#HOLD_FOLLOW"),
	Release:    sonolus.InstructionText("#RELEASE"),
	HoldFlick:  sonolus.InstructionText("#FLICK"),
}

type InstructionIconsData struct {
	sonolus.InstructionIconResource
	Hand sonolus.Icon
}

var InstructionIcons = &InstructionIconsData{Hand: sonolus.InstructionIcon("#HAND")}
