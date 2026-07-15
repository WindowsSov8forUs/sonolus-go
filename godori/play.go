//go:build play

package main

import (
	"math"

	"github.com/WindowsSov8forUs/sonolus-go/v2/sonolus"
	"github.com/WindowsSov8forUs/sonolus-go/v2/sonolus/play"
)

type PlayReplayStreams struct {
	sonolus.StreamResource
	Reserved   [100]sonolus.Stream[float64]
	EmptyLanes [7]sonolus.Stream[float64]
}

var Replay = PlayReplayStreams{}

type PlaySkin struct {
	sonolus.SkinResource
	Lane, JudgmentLine, Note, Flick, FlickArrow, RightFlick, RightFlickArrow, LeftFlick, LeftFlickArrow sonolus.Sprite
	HoldHead, HoldTail, HoldConnector, SimLine, HoldTick                                                sonolus.Sprite
	StageMiddle, LeftBorder, RightBorder, Slot, Cover                                                   sonolus.Sprite
}

var Skin = &PlaySkin{
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
	StageMiddle:   sonolus.SkinSprite(sonolus.StandardSpriteStageMiddle), LeftBorder: sonolus.SkinSprite(sonolus.StandardSpriteStageLeftBorder),
	RightBorder: sonolus.SkinSprite(sonolus.StandardSpriteStageRightBorder), Slot: sonolus.SkinSprite(sonolus.StandardSpriteNoteSlot),
	Cover: sonolus.SkinSprite(sonolus.StandardSpriteStageCover),
}

type PlayEffects struct {
	sonolus.EffectResource
	Stage, Perfect, Great, Good                           sonolus.Clip
	PerfectAlternative, GreatAlternative, GoodAlternative sonolus.Clip
	Hold                                                  sonolus.Clip
}

var Effects = &PlayEffects{
	Stage: sonolus.EffectClip(sonolus.StandardClipStage), Perfect: sonolus.EffectClip(sonolus.StandardClipPerfect),
	Great: sonolus.EffectClip(sonolus.StandardClipGreat), Good: sonolus.EffectClip(sonolus.StandardClipGood),
	PerfectAlternative: sonolus.EffectClip(sonolus.StandardClipPerfectAlternative),
	GreatAlternative:   sonolus.EffectClip(sonolus.StandardClipGreatAlternative),
	GoodAlternative:    sonolus.EffectClip(sonolus.StandardClipGoodAlternative),
	Hold:               sonolus.EffectClip(sonolus.StandardClipHold),
}

type PlayParticles struct {
	sonolus.ParticleResource
	Tap, Flick, RightFlick, LeftFlick, Hold, Lane                         sonolus.Effect
	TapLinear, FlickLinear, RightFlickLinear, LeftFlickLinear, HoldLinear sonolus.Effect
	HoldActive                                                            sonolus.Effect
}

var Particles = &PlayParticles{
	Tap: sonolus.ParticleEffect(sonolus.StandardEffectNoteCircularTapCyan), Flick: sonolus.ParticleEffect(sonolus.StandardEffectNoteCircularAlternativeRed),
	RightFlick: sonolus.ParticleEffect(sonolus.StandardEffectNoteCircularAlternativeYellow), LeftFlick: sonolus.ParticleEffect(sonolus.StandardEffectNoteCircularAlternativePurple),
	Hold: sonolus.ParticleEffect(sonolus.StandardEffectNoteCircularTapGreen), Lane: sonolus.ParticleEffect(sonolus.StandardEffectLaneLinear),
	TapLinear: sonolus.ParticleEffect(sonolus.StandardEffectNoteLinearTapCyan), FlickLinear: sonolus.ParticleEffect(sonolus.StandardEffectNoteLinearAlternativeRed),
	RightFlickLinear: sonolus.ParticleEffect(sonolus.StandardEffectNoteLinearAlternativeYellow), LeftFlickLinear: sonolus.ParticleEffect(sonolus.StandardEffectNoteLinearAlternativePurple),
	HoldLinear: sonolus.ParticleEffect(sonolus.StandardEffectNoteLinearTapGreen), HoldActive: sonolus.ParticleEffect(sonolus.StandardEffectNoteCircularHoldGreen),
}

type PlayBuckets struct {
	sonolus.BucketsResource
	Tap, HoldHead, HoldEnd, HoldTick, Flick, DirectionalFlick sonolus.Bucket
}

var Buckets = &PlayBuckets{
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

type PlayBPMChange struct {
	play.Archetype `archetype:"name=#BPM_CHANGE"`
	Beat           float64 `archetype:"imported,name=#BEAT"`
	BPM            float64 `archetype:"imported,name=#BPM"`
}

type PlayTimescaleChange struct {
	play.Archetype `archetype:"name=#TIMESCALE_CHANGE"`
	Beat           float64 `archetype:"imported,name=#BEAT"`
	Timescale      float64 `archetype:"imported,name=#TIMESCALE"`
}

type PlayStage struct {
	play.Archetype      `archetype:"name=Stage"`
	play.CallbackOrders `archetype:"updateSequential=-20,touch=20"`
}

func (*PlayStage) SpawnOrder() float64 { return -1e8 }
func (*PlayStage) ShouldSpawn() bool   { return true }
func (*PlayStage) Initialize() {
	sonolus.Assert(Config.LaneWidth > 0, "lane width must be positive")
	play.Spawn(PlayInputManager{})
}
func (*PlayStage) Preprocess() {
	play.Score.SetBase(play.BaseScore{Perfect: 1, Great: 0.5, Good: 0.25})
	play.UI.SetMenu(menuLayout())
	play.UI.SetJudgment(judgmentLayout())
	play.UI.SetComboValue(comboValueLayout())
	play.UI.SetComboText(comboTextLayout())
	play.UI.SetPrimaryMetricBar(primaryMetricBarLayout())
	play.UI.SetPrimaryMetricValue(primaryMetricValueLayout())
	play.UI.SetSecondaryMetricBar(secondaryMetricBarLayout())
	play.UI.SetSecondaryMetricValue(secondaryMetricValueLayout())
}
func (*PlayStage) UpdateParallel() {
	drawPlayStage()
}

func (*PlayStage) UpdateSequential() {
	clearInputState()
}

func (*PlayStage) Touch() {
	for i := 0; i < play.Touches.Count(); i++ {
		touch := play.Touches.Get(i)
		if !touch.Started || isTouchClaimed(touch.ID) {
			continue
		}
		lanes := sonolus.NewVarArray[int](7)
		for lane := range laneSequence {
			lanes.Append(lane)
		}
		for lane := range lanes.Values() {
			if !laneRect(float64(lane)).Contains(touch.Position) {
				continue
			}
			claimTouch(touch.ID)
			if Config.SFX {
				play.Audio.Play(Effects.Stage, 0.02)
			}
			if Config.LaneEffects {
				Particles.Lane.Spawn(laneRect(float64(lane)).ToQuad(), 0.2, false)
			}
			Replay.EmptyLanes[lane+3].Set(play.Time.Now(), 1)
			break
		}
	}
}

type PlayInputManager struct {
	play.Archetype      `archetype:"name=InputManager"`
	play.CallbackOrders `archetype:"updateSequential=20"`
}

func (*PlayInputManager) UpdateSequential() {
	prepareActiveHitboxes()
}

func drawPlayStage() {
	Skin.StageMiddle.Draw(stageRect().ToQuad(), 5, 1)
	for lane := -3; lane <= 3; lane++ {
		Skin.Lane.Draw(laneRect(float64(lane)).ToQuad(), 10, 0.8)
		Skin.Slot.Draw(noteRect(float64(lane), judgmentLineY).Scale(0.65).ToQuad(), 15, 0.7)
	}
	Skin.LeftBorder.Draw(leftBorderRect().ToQuad(), 12, 1)
	Skin.RightBorder.Draw(rightBorderRect().ToQuad(), 12, 1)
	Skin.JudgmentLine.Draw(judgmentRect().ToQuad(), 20, 1)
	Skin.Cover.Draw(stageCoverRect().ToQuad(), 40, 1)
}

type PlayTapNote struct {
	play.Archetype      `archetype:"name=TapNote,hasInput=true"`
	play.CallbackOrders `archetype:"preprocess=-10"`
	Beat                float64 `archetype:"imported,name=#BEAT"`
	Lane                float64 `archetype:"imported,name=lane"`
	TargetTime          float64 `archetype:"data"`
	TargetScaledTime    float64 `archetype:"data"`
}

type PlayAccentTapNote struct {
	PlayTapNote    `archetype:"base"`
	play.Archetype `archetype:"name=AccentTapNote"`
}

func (n *PlayTapNote) Preprocess() {
	n.TargetTime = play.Time.BeatToTime(n.Beat)
	n.TargetScaledTime = play.Time.TimeToScaledTime(n.TargetTime)
	schedulePlaySFX(n.TargetTime, false)
	archetype := int(play.Entity.Info().Archetype)
	play.Score.SetArchetype(archetype, play.ArchetypeScore{Multiplier: 1})
	play.Life.SetArchetype(archetype, play.LifeValues{Perfect: 1, Miss: -100})
	result := play.Entity.Result()
	result.Bucket = Buckets.Tap
	play.Entity.SetResult(result)
}

func (n *PlayTapNote) SpawnOrder() float64 { return n.TargetScaledTime - noteTravelTime() }
func (n *PlayTapNote) ShouldSpawn() bool {
	return play.Time.Scaled() >= n.TargetScaledTime-noteTravelTime()
}

func (n *PlayTapNote) UpdateSequential() {
	if play.Time.Now() > n.TargetTime+0.15+play.Input.Offset() {
		result := play.Entity.Result()
		result.Judgment = sonolus.JudgmentMiss
		result.Accuracy = 0.15
		result.Bucket = Buckets.Tap
		result.BucketValue = 150
		play.Entity.SetResult(result)
		play.Entity.SetDespawn(true)
		return
	}
	registerActiveNote(n.TargetTime, n.Lane, 0, false)
}

func (n *PlayTapNote) Touch() {
	for i := 0; i < play.Touches.Count(); i++ {
		touch := play.Touches.Get(i)
		if !touch.Started || !simultaneousHitbox(n.TargetTime, n.Lane, 0).Contains(touch.Position) {
			continue
		}
		hitTime := touch.StartTime - play.Input.Offset()
		if !withinJudgmentWindow(hitTime, n.TargetTime) {
			continue
		}
		if isTouchClaimed(touch.ID) {
			continue
		}
		claimTouch(touch.ID)
		finishPlayNote(n.Lane, n.TargetTime, hitTime, Buckets.Tap, Particles.TapLinear, Particles.Tap, false)
		return
	}
}

func (n *PlayTapNote) UpdateParallel() {
	Skin.Note.Draw(noteRect(n.Lane, noteY(n.TargetScaledTime, play.Time.Scaled())).ToQuad(), 30, 1)
}

type PlayFlickNote struct {
	play.Archetype   `archetype:"name=FlickNote,hasInput=true"`
	Beat             float64 `archetype:"imported,name=#BEAT"`
	Lane             float64 `archetype:"imported,name=lane"`
	TargetTime       float64 `archetype:"data"`
	TargetScaledTime float64 `archetype:"data"`
	TouchID          float64 `archetype:"memory"`
	Captured         bool    `archetype:"memory"`
}

func (n *PlayFlickNote) Preprocess() {
	n.TargetTime = play.Time.BeatToTime(n.Beat)
	n.TargetScaledTime = play.Time.TimeToScaledTime(n.TargetTime)
	schedulePlaySFX(n.TargetTime, true)
	archetype := int(play.Entity.Info().Archetype)
	play.Score.SetArchetype(archetype, play.ArchetypeScore{Multiplier: 1})
	play.Life.SetArchetype(archetype, play.LifeValues{Perfect: 1, Miss: -100})
	result := play.Entity.Result()
	result.Bucket = Buckets.Flick
	play.Entity.SetResult(result)
}

func (n *PlayFlickNote) SpawnOrder() float64 { return n.TargetScaledTime - noteTravelTime() }
func (n *PlayFlickNote) ShouldSpawn() bool {
	return play.Time.Scaled() >= n.TargetScaledTime-noteTravelTime()
}

func (n *PlayFlickNote) UpdateSequential() {
	if play.Time.Now() > n.TargetTime+0.15+play.Input.Offset() {
		result := play.Entity.Result()
		result.Judgment = sonolus.JudgmentMiss
		result.Accuracy = 0.15
		result.Bucket = Buckets.Flick
		result.BucketValue = 150
		play.Entity.SetResult(result)
		play.Entity.SetDespawn(true)
		return
	}
	registerActiveNote(n.TargetTime, n.Lane, 0, n.Captured)
}

func (n *PlayFlickNote) Touch() {
	if !n.Captured {
		for i := 0; i < play.Touches.Count(); i++ {
			touch := play.Touches.Get(i)
			hitTime := touch.StartTime - play.Input.Offset()
			if touch.Started && !isTouchClaimed(touch.ID) && simultaneousHitbox(n.TargetTime, n.Lane, 0).Contains(touch.Position) && withinJudgmentWindow(hitTime, n.TargetTime) {
				claimTouch(touch.ID)
				n.TouchID = touch.ID
				n.Captured = true
				break
			}
		}
	}
	if !n.Captured {
		return
	}
	for i := 0; i < play.Touches.Count(); i++ {
		touch := play.Touches.Get(i)
		if touch.ID != n.TouchID {
			continue
		}
		if touch.Ended {
			n.Captured = false
			return
		}
		hitTime := play.Time.Now() - play.Input.Offset()
		if withinJudgmentWindow(hitTime, n.TargetTime) && touch.Speed >= flickSpeedThreshold*Config.LaneWidth {
			finishPlayNote(n.Lane, n.TargetTime, hitTime, Buckets.Flick, Particles.FlickLinear, Particles.Flick, true)
		}
		return
	}
}

func (n *PlayFlickNote) UpdateParallel() {
	y := noteY(n.TargetScaledTime, play.Time.Scaled())
	Skin.Flick.Draw(noteRect(n.Lane, y).ToQuad(), 30, 1)
	Skin.FlickArrow.Draw(noteRect(n.Lane, y).Scale(0.7).ToQuad(), 31, 1)
}

type PlayDirectionalFlickNote struct {
	play.Archetype   `archetype:"name=DirectionalFlickNote,hasInput=true"`
	Beat             float64 `archetype:"imported,name=#BEAT"`
	Lane             float64 `archetype:"imported,name=lane"`
	Direction        float64 `archetype:"imported,name=direction"`
	TargetTime       float64 `archetype:"data"`
	TargetScaledTime float64 `archetype:"data"`
	TouchID          float64 `archetype:"memory"`
	Captured         bool    `archetype:"memory"`
}

func (n *PlayDirectionalFlickNote) Preprocess() {
	n.TargetTime = play.Time.BeatToTime(n.Beat)
	n.TargetScaledTime = play.Time.TimeToScaledTime(n.TargetTime)
	schedulePlaySFX(n.TargetTime, true)
	archetype := int(play.Entity.Info().Archetype)
	play.Score.SetArchetype(archetype, play.ArchetypeScore{Multiplier: 1})
	play.Life.SetArchetype(archetype, play.LifeValues{Perfect: 1, Miss: -100})
	result := play.Entity.Result()
	result.Bucket = Buckets.DirectionalFlick
	play.Entity.SetResult(result)
}

func (n *PlayDirectionalFlickNote) SpawnOrder() float64 { return n.TargetScaledTime - noteTravelTime() }
func (n *PlayDirectionalFlickNote) ShouldSpawn() bool {
	return play.Time.Scaled() >= n.TargetScaledTime-noteTravelTime()
}

func (n *PlayDirectionalFlickNote) UpdateSequential() {
	if play.Time.Now() > n.TargetTime+0.15+play.Input.Offset() {
		result := play.Entity.Result()
		result.Judgment = sonolus.JudgmentMiss
		result.Accuracy = 0.15
		result.Bucket = Buckets.DirectionalFlick
		result.BucketValue = 150
		play.Entity.SetResult(result)
		play.Entity.SetDespawn(true)
		return
	}
	registerActiveNote(n.TargetTime, n.Lane, displayDirection(n.Direction), n.Captured)
}

func (n *PlayDirectionalFlickNote) Touch() {
	if !n.Captured {
		for i := 0; i < play.Touches.Count(); i++ {
			touch := play.Touches.Get(i)
			hitTime := touch.StartTime - play.Input.Offset()
			if touch.Started && !isTouchClaimed(touch.ID) && simultaneousHitbox(n.TargetTime, n.Lane, displayDirection(n.Direction)).Contains(touch.Position) && withinJudgmentWindow(hitTime, n.TargetTime) {
				claimTouch(touch.ID)
				n.TouchID = touch.ID
				n.Captured = true
				break
			}
		}
	}
	if !n.Captured {
		return
	}
	for i := 0; i < play.Touches.Count(); i++ {
		touch := play.Touches.Get(i)
		if touch.ID != n.TouchID {
			continue
		}
		if touch.Ended {
			n.Captured = false
			return
		}
		hitTime := play.Time.Now() - play.Input.Offset()
		direction := displayDirection(n.Direction)
		if withinJudgmentWindow(hitTime, n.TargetTime) &&
			touch.Speed >= directionalFlickSpeedThreshold(n.Direction) && touch.Velocity.X*direction > 0 {
			if direction > 0 {
				finishPlayNote(n.Lane, n.TargetTime, hitTime, Buckets.DirectionalFlick, Particles.RightFlickLinear, Particles.RightFlick, true)
			} else {
				finishPlayNote(n.Lane, n.TargetTime, hitTime, Buckets.DirectionalFlick, Particles.LeftFlickLinear, Particles.LeftFlick, true)
			}
		}
		return
	}
}

func (n *PlayDirectionalFlickNote) UpdateParallel() {
	y := noteY(n.TargetScaledTime, play.Time.Scaled())
	if displayDirection(n.Direction) > 0 {
		drawDirectionalFlickNote(Skin.RightFlick, Skin.RightFlickArrow, n.Lane, y, n.Direction)
	} else {
		drawDirectionalFlickNote(Skin.LeftFlick, Skin.LeftFlickArrow, n.Lane, y, n.Direction)
	}
}

type PlayHoldHeadNote struct {
	play.Archetype   `archetype:"name=HoldHeadNote,hasInput=true"`
	Beat             float64                               `archetype:"imported,name=#BEAT"`
	Lane             float64                               `archetype:"imported,name=lane"`
	Anchor           sonolus.EntityRef[PlayHoldAnchorNote] `archetype:"imported,name=anchor"`
	End              sonolus.EntityRef[PlayHoldEndNote]    `archetype:"imported,name=end"`
	FlickEnd         sonolus.EntityRef[PlayHoldFlickNote]  `archetype:"imported,name=flickEnd"`
	TargetTime       float64                               `archetype:"data"`
	AnchorTime       float64                               `archetype:"data"`
	EndTime          float64                               `archetype:"data"`
	TargetScaledTime float64                               `archetype:"data"`
	AnchorScaledTime float64                               `archetype:"data"`
	EndScaledTime    float64                               `archetype:"data"`
	TouchID          float64                               `archetype:"shared"`
	Active           bool                                  `archetype:"shared"`
	Judged           bool                                  `archetype:"shared"`
}

func (n *PlayHoldHeadNote) Preprocess() {
	n.TargetTime = play.Time.BeatToTime(n.Beat)
	n.AnchorTime = play.Time.BeatToTime(n.Anchor.Get().Beat)
	n.EndTime = play.Time.BeatToTime(n.endBeat())
	n.TargetScaledTime = play.Time.TimeToScaledTime(n.TargetTime)
	n.AnchorScaledTime = play.Time.TimeToScaledTime(n.AnchorTime)
	n.EndScaledTime = play.Time.TimeToScaledTime(n.EndTime)
	schedulePlaySFX(n.TargetTime, false)
	archetype := int(play.Entity.Info().Archetype)
	play.Score.SetArchetype(archetype, play.ArchetypeScore{Multiplier: 1})
	play.Life.SetArchetype(archetype, play.LifeValues{Perfect: 1, Miss: -100})
	result := play.Entity.Result()
	result.Bucket = Buckets.HoldHead
	play.Entity.SetResult(result)
}

func (n *PlayHoldHeadNote) SpawnOrder() float64 { return n.TargetScaledTime - noteTravelTime() }
func (n *PlayHoldHeadNote) ShouldSpawn() bool {
	return play.Time.Scaled() >= n.TargetScaledTime-noteTravelTime()
}

func (*PlayHoldHeadNote) Initialize() {
	play.Spawn(PlayHoldManager{Head: sonolus.EntityRef[PlayHoldHeadNote]{Index: play.Entity.Info().Index}})
}

func (n *PlayHoldHeadNote) UpdateSequential() {
	now := play.Time.Now() - play.Input.Offset()
	if !n.Judged && now > n.TargetTime+0.15 {
		n.Judged = true
		failPlayHold(Buckets.HoldHead)
		return
	}
	if !n.Judged {
		registerActiveNote(n.TargetTime, n.Lane, 0, n.Active)
	}
}

func (n *PlayHoldHeadNote) Touch() {
	if n.Judged {
		return
	}
	for i := 0; i < play.Touches.Count(); i++ {
		touch := play.Touches.Get(i)
		hitTime := touch.StartTime - play.Input.Offset()
		if !touch.Started || isTouchClaimed(touch.ID) || !simultaneousHitbox(n.TargetTime, n.Lane, 0).Contains(touch.Position) || !withinJudgmentWindow(hitTime, n.TargetTime) {
			continue
		}
		claimTouch(touch.ID)
		n.TouchID = touch.ID
		n.Active = true
		n.Judged = true
		Replay.Reserved[int(play.Entity.Info().Index)].Set(play.Time.Now(), 1)
		finishPlayNote(n.Lane, n.TargetTime, hitTime, Buckets.HoldHead, Particles.HoldLinear, Particles.Hold, false)
		return
	}
}

func (n *PlayHoldHeadNote) UpdateParallel() {
	now := play.Time.Scaled()
	startLane := n.Lane
	startY := noteY(n.TargetScaledTime, now)
	if startY < judgmentLineY {
		startY = judgmentLineY
	}
	if n.Active && now >= n.TargetScaledTime {
		startLane = n.currentScaledLane(now)
	}
	Skin.HoldHead.Draw(noteRect(startLane, startY).ToQuad(), 30, 1)
}

func (n *PlayHoldHeadNote) currentLane(now float64) float64 {
	return holdChainLane(now, n.TargetTime, n.AnchorTime, n.EndTime, n.Lane, n.Anchor.Get().Lane, n.endLane())
}

func (n *PlayHoldHeadNote) currentScaledLane(now float64) float64 {
	return holdChainLane(now, n.TargetScaledTime, n.AnchorScaledTime, n.EndScaledTime, n.Lane, n.Anchor.Get().Lane, n.endLane())
}

func (n *PlayHoldHeadNote) endBeat() float64 {
	if n.FlickEnd.Index > 0 {
		return n.FlickEnd.Get().Beat
	}
	return n.End.Get().Beat
}

func (n *PlayHoldHeadNote) endLane() float64 {
	if n.FlickEnd.Index > 0 {
		return n.FlickEnd.Get().Lane
	}
	return n.End.Get().Lane
}

type PlayHoldManager struct {
	play.Archetype      `archetype:"name=HoldManager,hasInput=true"`
	play.CallbackOrders `archetype:"touch=1"`
	Head                sonolus.EntityRef[PlayHoldHeadNote] `archetype:"memory"`
	EffectsActive       bool                                `archetype:"memory"`
	LoopID              float64                             `archetype:"memory"`
	ParticleID          float64                             `archetype:"memory"`
}

func (n *PlayHoldManager) UpdateParallel() {
	head := n.Head.Get()
	if play.Time.Scaled() >= head.EndScaledTime {
		n.stopEffects()
		play.Entity.SetDespawn(true)
		return
	}
	if !head.Active {
		n.stopEffects()
		return
	}
	lane := holdChainLane(
		play.Time.Scaled(),
		head.TargetScaledTime,
		head.AnchorScaledTime,
		head.EndScaledTime,
		head.Lane,
		head.Anchor.Get().Lane,
		head.endLane(),
	)
	Skin.HoldHead.Draw(noteRect(lane, judgmentLineY).ToQuad(), 30, 1)
	n.updateEffects(lane)
}

func (n *PlayHoldManager) Terminate() { n.stopEffects() }

func (n *PlayHoldManager) Touch() {
	head := n.Head.Get()
	if head.Active {
		for i := 0; i < play.Touches.Count(); i++ {
			touch := play.Touches.Get(i)
			if touch.ID != head.TouchID {
				continue
			}
			if touch.Ended {
				head.Active = false
				Replay.Reserved[int(n.Head.Index)].Set(play.Time.Now(), 0)
			}
			return
		}
		head.Active = false
		Replay.Reserved[int(n.Head.Index)].Set(play.Time.Now(), 0)
	}
	if !head.Judged || play.Time.Now()-play.Input.Offset() < head.TargetTime {
		return
	}
	lane := holdChainLane(
		play.Time.Now()-play.Input.Offset(),
		head.TargetTime,
		head.AnchorTime,
		head.EndTime,
		head.Lane,
		head.Anchor.Get().Lane,
		head.endLane(),
	)
	for i := 0; i < play.Touches.Count(); i++ {
		touch := play.Touches.Get(i)
		if touch.Ended || isTouchClaimed(touch.ID) || !hitbox(lane).Contains(touch.Position) {
			continue
		}
		claimTouch(touch.ID)
		head.TouchID = touch.ID
		head.Active = true
		Replay.Reserved[int(n.Head.Index)].Set(play.Time.Now(), 1)
		return
	}
}

func (n *PlayHoldManager) updateEffects(lane float64) {
	quad := noteRect(lane, judgmentLineY).Scale(1.5).ToQuad()
	if !n.EffectsActive {
		n.EffectsActive = true
		if Config.SFX {
			n.LoopID = Effects.Hold.PlayLooped().ID
		}
		if Config.NoteEffects {
			n.ParticleID = Particles.HoldActive.Spawn(quad, 0.3, true).ID
		}
	} else if Config.NoteEffects {
		sonolus.ParticleHandle{ID: n.ParticleID}.Move(quad)
	}
}

func (n *PlayHoldManager) stopEffects() {
	if !n.EffectsActive {
		return
	}
	if Config.NoteEffects {
		sonolus.ParticleHandle{ID: n.ParticleID}.Destroy()
	}
	if Config.SFX {
		sonolus.LoopedEffectHandle{ID: n.LoopID}.Stop()
	}
	n.EffectsActive = false
}

type PlayHoldAnchorNote struct {
	play.Archetype `archetype:"name=HoldAnchorNote"`
	Beat           float64 `archetype:"imported,name=#BEAT"`
	Lane           float64 `archetype:"imported,name=lane"`
}

type PlayHoldEndNote struct {
	play.Archetype   `archetype:"name=HoldEndNote,hasInput=true"`
	Head             sonolus.EntityRef[PlayHoldHeadNote] `archetype:"imported,name=head"`
	Beat             float64                             `archetype:"imported,name=#BEAT"`
	Lane             float64                             `archetype:"imported,name=lane"`
	TargetTime       float64                             `archetype:"data"`
	TargetScaledTime float64                             `archetype:"data"`
	Resolved         bool                                `archetype:"memory"`
}

func (n *PlayHoldEndNote) Preprocess() {
	n.TargetTime = play.Time.BeatToTime(n.Beat)
	n.TargetScaledTime = play.Time.TimeToScaledTime(n.TargetTime)
	schedulePlaySFX(n.TargetTime, false)
	archetype := int(play.Entity.Info().Archetype)
	play.Score.SetArchetype(archetype, play.ArchetypeScore{Multiplier: 1})
	play.Life.SetArchetype(archetype, play.LifeValues{Perfect: 1, Miss: -100})
	result := play.Entity.Result()
	result.Bucket = Buckets.HoldEnd
	play.Entity.SetResult(result)
}
func (n *PlayHoldEndNote) SpawnOrder() float64 { return n.TargetScaledTime - noteTravelTime() }
func (n *PlayHoldEndNote) ShouldSpawn() bool {
	return play.Time.Scaled() >= n.TargetScaledTime-noteTravelTime()
}
func (n *PlayHoldEndNote) UpdateSequential() {
	if n.Resolved {
		return
	}
	head := n.Head.Get()
	if play.Time.Now()-play.Input.Offset() > n.TargetTime+0.15 {
		n.Resolved = true
		head.Active = false
		Replay.Reserved[int(n.Head.Index)].Set(play.Time.Now(), 0)
		failPlayHold(Buckets.HoldEnd)
		return
	}
	registerActiveNote(n.TargetTime, n.Lane, 0, head.Active)
}
func (n *PlayHoldEndNote) Touch() {
	if n.Resolved {
		return
	}
	head := n.Head.Get()
	if !head.Active {
		for i := 0; i < play.Touches.Count(); i++ {
			touch := play.Touches.Get(i)
			if touch.Ended || isTouchClaimed(touch.ID) || !simultaneousHitbox(n.TargetTime, n.Lane, 0).Contains(touch.Position) {
				continue
			}
			claimTouch(touch.ID)
			head.TouchID = touch.ID
			head.Active = true
			Replay.Reserved[int(n.Head.Index)].Set(play.Time.Now(), 1)
			return
		}
	}
	for i := 0; i < play.Touches.Count(); i++ {
		touch := play.Touches.Get(i)
		if touch.ID != head.TouchID || !touch.Ended {
			continue
		}
		n.Resolved = true
		head.Active = false
		Replay.Reserved[int(n.Head.Index)].Set(play.Time.Now(), 0)
		hitTime := play.Time.Now() - play.Input.Offset()
		if simultaneousHitbox(n.TargetTime, n.Lane, 0).Contains(touch.Position) && withinJudgmentWindow(hitTime, n.TargetTime) {
			finishPlayNote(n.Lane, n.TargetTime, hitTime, Buckets.HoldEnd, Particles.HoldLinear, Particles.Hold, false)
		} else {
			failPlayHold(Buckets.HoldEnd)
		}
		return
	}
}
func (n *PlayHoldEndNote) UpdateParallel() {
	Skin.HoldTail.Draw(noteRect(n.Lane, noteY(n.TargetScaledTime, play.Time.Scaled())).ToQuad(), 30, 1)
}

type PlayHoldFlickNote struct {
	play.Archetype   `archetype:"name=HoldFlickNote,hasInput=true"`
	Head             sonolus.EntityRef[PlayHoldHeadNote] `archetype:"imported,name=head"`
	Beat             float64                             `archetype:"imported,name=#BEAT"`
	Lane             float64                             `archetype:"imported,name=lane"`
	TargetTime       float64                             `archetype:"data"`
	TargetScaledTime float64                             `archetype:"data"`
	BestJudgmentTime float64                             `archetype:"memory"`
	Resolved         bool                                `archetype:"memory"`
}

func (n *PlayHoldFlickNote) Preprocess() {
	n.TargetTime = play.Time.BeatToTime(n.Beat)
	n.TargetScaledTime = play.Time.TimeToScaledTime(n.TargetTime)
	n.BestJudgmentTime = unsetJudgmentTime
	schedulePlaySFX(n.TargetTime, true)
	archetype := int(play.Entity.Info().Archetype)
	play.Score.SetArchetype(archetype, play.ArchetypeScore{Multiplier: 1})
	play.Life.SetArchetype(archetype, play.LifeValues{Perfect: 1, Miss: -100})
	result := play.Entity.Result()
	result.Bucket = Buckets.Flick
	play.Entity.SetResult(result)
}

func (n *PlayHoldFlickNote) SpawnOrder() float64 { return n.TargetScaledTime - noteTravelTime() }
func (n *PlayHoldFlickNote) ShouldSpawn() bool {
	return play.Time.Scaled() >= n.TargetScaledTime-noteTravelTime()
}

func (n *PlayHoldFlickNote) UpdateSequential() {
	if n.Resolved {
		return
	}
	hitTime, ready := holdTickResolution(n.BestJudgmentTime, n.TargetTime, play.Time.Now()-play.Input.Offset())
	if ready {
		n.resolve(hitTime)
		return
	}
	registerActiveNote(n.TargetTime, n.Lane, 0, n.Head.Get().Active)
}

func (n *PlayHoldFlickNote) Touch() {
	if n.Resolved {
		return
	}
	hitTime := play.Time.Now() - play.Input.Offset()
	if !withinJudgmentWindow(hitTime, n.TargetTime) {
		return
	}
	head := n.Head.Get()
	if !head.Active {
		return
	}
	for i := 0; i < play.Touches.Count(); i++ {
		touch := play.Touches.Get(i)
		if touch.ID != head.TouchID || touch.Ended || touch.Speed < flickSpeedThreshold*Config.LaneWidth || !simultaneousHitbox(n.TargetTime, n.Lane, 0).Contains(touch.Position) {
			continue
		}
		if n.BestJudgmentTime <= unsetJudgmentTime || math.Abs(hitTime-n.TargetTime) < math.Abs(n.BestJudgmentTime-n.TargetTime) {
			n.BestJudgmentTime = hitTime
		}
		if n.BestJudgmentTime >= n.TargetTime {
			n.resolve(n.BestJudgmentTime)
		}
		return
	}
}

func (n *PlayHoldFlickNote) resolve(hitTime float64) {
	n.Resolved = true
	head := n.Head.Get()
	head.Active = false
	Replay.Reserved[int(n.Head.Index)].Set(play.Time.Now(), 0)
	if hitTime > unsetJudgmentTime {
		finishPlayNote(n.Lane, n.TargetTime, hitTime, Buckets.Flick, Particles.FlickLinear, Particles.Flick, true)
	} else {
		failPlayHold(Buckets.Flick)
	}
}

func (n *PlayHoldFlickNote) UpdateParallel() {
	y := noteY(n.TargetScaledTime, play.Time.Scaled())
	Skin.Flick.Draw(noteRect(n.Lane, y).ToQuad(), 30, 1)
	Skin.FlickArrow.Draw(noteRect(n.Lane, y).Scale(0.7).ToQuad(), 31, 1)
}

type PlayHoldConnector struct {
	play.Archetype   `archetype:"name=HoldConnector"`
	Head             sonolus.EntityRef[PlayHoldHeadNote]   `archetype:"imported,name=head"`
	Anchor           sonolus.EntityRef[PlayHoldAnchorNote] `archetype:"imported,name=anchor"`
	End              sonolus.EntityRef[PlayHoldEndNote]    `archetype:"imported,name=end"`
	FlickEnd         sonolus.EntityRef[PlayHoldFlickNote]  `archetype:"imported,name=flickEnd"`
	Segment          float64                               `archetype:"imported,name=segment"`
	TargetScaledTime float64                               `archetype:"data"`
	EndScaledTime    float64                               `archetype:"data"`
	FirstLane        float64                               `archetype:"data"`
	SecondLane       float64                               `archetype:"data"`
}

func (n *PlayHoldConnector) Preprocess() {
	targetBeat := n.Head.Get().Beat
	endBeat := n.Anchor.Get().Beat
	n.FirstLane = n.Head.Get().Lane
	n.SecondLane = n.Anchor.Get().Lane
	if n.Segment != 0 {
		targetBeat = n.Anchor.Get().Beat
		endBeat = n.Head.Get().endBeat()
		n.FirstLane = n.Anchor.Get().Lane
		n.SecondLane = n.Head.Get().endLane()
	}
	targetTime := play.Time.BeatToTime(targetBeat)
	endTime := play.Time.BeatToTime(endBeat)
	n.TargetScaledTime = play.Time.TimeToScaledTime(targetTime)
	n.EndScaledTime = play.Time.TimeToScaledTime(endTime)
}
func (n *PlayHoldConnector) SpawnOrder() float64 { return n.TargetScaledTime - noteTravelTime() }
func (n *PlayHoldConnector) ShouldSpawn() bool {
	return play.Time.Scaled() >= n.TargetScaledTime-noteTravelTime()
}
func (n *PlayHoldConnector) UpdateParallel() {
	if play.Time.Scaled() >= n.EndScaledTime {
		play.Entity.SetDespawn(true)
		return
	}
	now := play.Time.Scaled()
	startLane := n.FirstLane
	startY := noteY(n.TargetScaledTime, now)
	if startY < judgmentLineY {
		startY = judgmentLineY
	}
	if n.Head.Get().Active && now >= n.TargetScaledTime {
		startLane = holdLane(now, n.TargetScaledTime, n.EndScaledTime, n.FirstLane, n.SecondLane)
	}
	endY := noteY(n.EndScaledTime, now)
	Skin.HoldConnector.Draw(holdConnectorQuad(startLane, n.SecondLane, startY, endY), 25, Config.ConnectorAlpha)
}

type PlaySimLine struct {
	play.Archetype   `archetype:"name=SimLine"`
	First            sonolus.EntityRef[PlayTapNote] `archetype:"imported,name=first"`
	Second           sonolus.EntityRef[PlayTapNote] `archetype:"imported,name=second"`
	TargetTime       float64                        `archetype:"data"`
	TargetScaledTime float64                        `archetype:"data"`
}

func (n *PlaySimLine) Preprocess() {
	n.TargetTime = play.Time.BeatToTime(n.First.Get().Beat)
	n.TargetScaledTime = play.Time.TimeToScaledTime(n.TargetTime)
}
func (n *PlaySimLine) SpawnOrder() float64 {
	return n.TargetScaledTime - noteTravelTime()
}
func (n *PlaySimLine) ShouldSpawn() bool {
	return play.Time.Scaled() >= n.TargetScaledTime-noteTravelTime()
}
func (n *PlaySimLine) UpdateParallel() {
	if !Config.SimLines {
		return
	}
	first := play.Entity.InfoAt(int(n.First.Index))
	second := play.Entity.InfoAt(int(n.Second.Index))
	if first.State == sonolus.EntityStateDespawned || second.State == sonolus.EntityStateDespawned {
		play.Entity.SetDespawn(true)
		return
	}
	y := noteY(n.TargetScaledTime, play.Time.Scaled())
	Skin.SimLine.Draw(simLineQuad(n.First.Get().Lane, n.Second.Get().Lane, y), 24, Config.ConnectorAlpha)
}

type PlayHoldTickNote struct {
	play.Archetype   `archetype:"name=HoldTickNote,hasInput=true"`
	Head             sonolus.EntityRef[PlayHoldHeadNote] `archetype:"imported,name=head"`
	Beat             float64                             `archetype:"imported,name=#BEAT"`
	Lane             float64                             `archetype:"data"`
	TargetTime       float64                             `archetype:"data"`
	TargetScaledTime float64                             `archetype:"data"`
	BestJudgmentTime float64                             `archetype:"memory"`
	Resolved         bool                                `archetype:"memory"`
}

func (n *PlayHoldTickNote) Preprocess() {
	n.Lane = holdChainLane(
		n.Beat,
		n.Head.Get().Beat,
		n.Head.Get().Anchor.Get().Beat,
		n.Head.Get().endBeat(),
		n.Head.Get().Lane,
		n.Head.Get().Anchor.Get().Lane,
		n.Head.Get().endLane(),
	)
	n.TargetTime = play.Time.BeatToTime(n.Beat)
	n.TargetScaledTime = play.Time.TimeToScaledTime(n.TargetTime)
	n.BestJudgmentTime = unsetJudgmentTime
	schedulePlaySFX(n.TargetTime, false)
	archetype := int(play.Entity.Info().Archetype)
	play.Score.SetArchetype(archetype, play.ArchetypeScore{Multiplier: 1})
	play.Life.SetArchetype(archetype, play.LifeValues{Perfect: 1, Miss: -20})
	result := play.Entity.Result()
	result.Bucket = Buckets.HoldTick
	play.Entity.SetResult(result)
}

func (n *PlayHoldTickNote) SpawnOrder() float64 { return n.TargetScaledTime - noteTravelTime() }
func (n *PlayHoldTickNote) ShouldSpawn() bool {
	return play.Time.Scaled() >= n.TargetScaledTime-noteTravelTime()
}
func (n *PlayHoldTickNote) UpdateSequential() {
	if n.Resolved {
		return
	}
	offsetAdjustedTime := play.Time.Now() - play.Input.Offset()
	hitTime, ready := holdTickResolution(n.BestJudgmentTime, n.TargetTime, offsetAdjustedTime)
	if ready {
		n.resolve(hitTime)
		return
	}
	registerActiveNote(n.TargetTime, n.Lane, 0, n.Head.Get().Active)
}

func (n *PlayHoldTickNote) Touch() {
	if n.Resolved {
		return
	}
	offsetAdjustedTime := play.Time.Now() - play.Input.Offset()
	if !withinJudgmentWindow(offsetAdjustedTime, n.TargetTime) {
		return
	}
	head := n.Head.Get()
	if !head.Active {
		return
	}
	for i := 0; i < play.Touches.Count(); i++ {
		touch := play.Touches.Get(i)
		if touch.ID != head.TouchID || touch.Ended || !simultaneousHitbox(n.TargetTime, n.Lane, 0).Contains(touch.Position) {
			continue
		}
		if n.BestJudgmentTime <= unsetJudgmentTime || math.Abs(offsetAdjustedTime-n.TargetTime) < math.Abs(n.BestJudgmentTime-n.TargetTime) {
			n.BestJudgmentTime = offsetAdjustedTime
		}
		if n.BestJudgmentTime >= n.TargetTime {
			n.resolve(n.BestJudgmentTime)
		}
		return
	}
}

func (n *PlayHoldTickNote) resolve(hitTime float64) {
	n.Resolved = true
	result := play.Entity.Result()
	result.Bucket = Buckets.HoldTick
	if hitTime > unsetJudgmentTime {
		judgment := play.Input.Judge(hitTime, n.TargetTime, judgmentWindows())
		result.Judgment = judgment
		result.Accuracy = hitTime - n.TargetTime
		result.BucketValue = result.Accuracy * 1000
		if Config.SFX && !Config.AutoSFX {
			playJudgmentSFX(judgment, false)
		}
		if Config.NoteEffects && judgment != sonolus.JudgmentMiss {
			spawnPlayNoteParticles(n.Lane, 1.2, 0.2, Particles.HoldLinear, Particles.Hold)
		}
		if Config.LaneEffects && judgment != sonolus.JudgmentMiss {
			Particles.Lane.Spawn(laneRect(n.Lane).ToQuad(), 0.2, false)
		}
	} else {
		result.Judgment = sonolus.JudgmentMiss
		result.Accuracy = 0.15
		result.BucketValue = 150
	}
	play.Entity.SetResult(result)
	play.Entity.SetDespawn(true)
}
func (n *PlayHoldTickNote) UpdateParallel() {
	Skin.HoldTick.Draw(noteRect(n.Lane, noteY(n.TargetScaledTime, play.Time.Scaled())).Scale(0.7).ToQuad(), 29, 1)
}

func failPlayHold(bucket sonolus.Bucket) {
	result := play.Entity.Result()
	result.Judgment = sonolus.JudgmentMiss
	result.Accuracy = 0.15
	result.Bucket = bucket
	result.BucketValue = 150
	play.Entity.SetResult(result)
	play.Entity.SetDespawn(true)
}

func finishPlayNote(lane, targetTime, hitTime float64, bucket sonolus.Bucket, linear, circular sonolus.Effect, alternative bool) {
	accuracy := hitTime - targetTime
	judgment := play.Input.Judge(hitTime, targetTime, judgmentWindows())
	result := play.Entity.Result()
	result.Judgment = judgment
	result.Accuracy = accuracy
	result.Bucket = bucket
	result.BucketValue = accuracy * 1000
	play.Entity.SetResult(result)
	if judgment != sonolus.JudgmentMiss {
		if Config.SFX && !Config.AutoSFX {
			playJudgmentSFX(judgment, alternative)
		}
		if Config.NoteEffects {
			spawnPlayNoteParticles(lane, 1.5, 0.3, linear, circular)
		}
	}
	if Config.LaneEffects && judgment != sonolus.JudgmentMiss {
		Particles.Lane.Spawn(laneRect(lane).ToQuad(), 0.2, false)
	}
	play.Entity.SetDespawn(true)
}

func spawnPlayNoteParticles(lane, scale, duration float64, linear, circular sonolus.Effect) {
	particleRect := noteRect(lane, judgmentLineY).Scale(scale)
	width := particleRect.R - particleRect.L
	linear.Spawn(sonolus.Rect{T: judgmentLineY + width, R: particleRect.R, B: judgmentLineY, L: particleRect.L}.ToQuad(), duration, false)
	circular.Spawn(particleRect.ToQuad(), duration, false)
}

func playJudgmentSFX(judgment sonolus.Judgment, alternative bool) {
	if judgment == sonolus.JudgmentPerfect {
		if alternative {
			play.Audio.Play(Effects.PerfectAlternative, 0.02)
		} else {
			play.Audio.Play(Effects.Perfect, 0.02)
		}
	} else if judgment == sonolus.JudgmentGreat {
		if alternative {
			play.Audio.Play(Effects.GreatAlternative, 0.02)
		} else {
			play.Audio.Play(Effects.Great, 0.02)
		}
	} else if judgment == sonolus.JudgmentGood {
		if alternative {
			play.Audio.Play(Effects.GoodAlternative, 0.02)
		} else {
			play.Audio.Play(Effects.Good, 0.02)
		}
	}
}

func schedulePlaySFX(targetTime float64, alternative bool) {
	if Config.SFX && Config.AutoSFX {
		if alternative {
			play.Audio.PlayScheduled(Effects.PerfectAlternative, targetTime, 0.02)
		} else {
			play.Audio.PlayScheduled(Effects.Perfect, targetTime, 0.02)
		}
	}
}

const (
	claimedTouchCountIndex = 0
	claimedTouchStartIndex = 1
	claimedTouchCapacity   = 16
	activeNoteCountIndex   = 20
	activeNoteStartIndex   = 21
	activeNoteCapacity     = 16
	activeNoteStride       = 6
)

func clearInputState() {
	play.LevelMemory.Set(claimedTouchCountIndex, 0)
	play.LevelMemory.Set(activeNoteCountIndex, 0)
}

func isTouchClaimed(id float64) bool {
	count := int(play.LevelMemory.Get(claimedTouchCountIndex))
	for i := 0; i < count; i++ {
		if play.LevelMemory.Get(claimedTouchStartIndex+i) == id {
			return true
		}
	}
	return false
}

func claimTouch(id float64) {
	if isTouchClaimed(id) {
		return
	}
	count := int(play.LevelMemory.Get(claimedTouchCountIndex))
	if count >= claimedTouchCapacity {
		return
	}
	play.LevelMemory.Set(claimedTouchStartIndex+count, id)
	play.LevelMemory.Set(claimedTouchCountIndex, float64(count+1))
}

func registerActiveNote(targetTime, lane, direction float64, hasActiveTouch bool) {
	if hasActiveTouch {
		return
	}
	now := play.Time.Now() - play.Input.Offset()
	if !withinJudgmentWindow(now, targetTime) {
		return
	}
	count := int(play.LevelMemory.Get(activeNoteCountIndex))
	if count >= activeNoteCapacity {
		return
	}
	base := activeNoteStartIndex + count*activeNoteStride
	play.LevelMemory.Set(base, play.Entity.Info().Index)
	play.LevelMemory.Set(base+1, targetTime)
	play.LevelMemory.Set(base+2, lane)
	play.LevelMemory.Set(base+3, direction)
	box := noteHitbox(lane, direction)
	play.LevelMemory.Set(base+4, box.L)
	play.LevelMemory.Set(base+5, box.R)
	play.LevelMemory.Set(activeNoteCountIndex, float64(count+1))
}

func simultaneousHitbox(targetTime, lane, direction float64) sonolus.Rect {
	entityIndex := play.Entity.Info().Index
	count := int(play.LevelMemory.Get(activeNoteCountIndex))
	for i := 0; i < count; i++ {
		base := activeNoteStartIndex + i*activeNoteStride
		if play.LevelMemory.Get(base) == entityIndex {
			return sonolus.Rect{
				T: judgmentLineY + 0.35,
				R: play.LevelMemory.Get(base + 5),
				B: judgmentLineY - 0.35,
				L: play.LevelMemory.Get(base + 4),
			}
		}
	}
	return noteHitbox(lane, direction)
}

func prepareActiveHitboxes() {
	count := int(play.LevelMemory.Get(activeNoteCountIndex))
	for i := 0; i < count; i++ {
		base := activeNoteStartIndex + i*activeNoteStride
		targetTime := play.LevelMemory.Get(base + 1)
		lane := play.LevelMemory.Get(base + 2)
		box := noteHitbox(lane, play.LevelMemory.Get(base+3))
		leftOverlap := 0.0
		rightOverlap := 0.0
		for j := 0; j < count; j++ {
			otherBase := activeNoteStartIndex + j*activeNoteStride
			if j == i || math.Abs(play.LevelMemory.Get(otherBase+1)-targetTime) > 0.005 {
				continue
			}
			otherLane := play.LevelMemory.Get(otherBase + 2)
			other := noteHitbox(otherLane, play.LevelMemory.Get(otherBase+3))
			left, right := hitboxOverlap(lane, otherLane, box, other)
			if left > leftOverlap {
				leftOverlap = left
			}
			if right > rightOverlap {
				rightOverlap = right
			}
		}
		play.LevelMemory.Set(base+4, box.L+leftOverlap/2)
		play.LevelMemory.Set(base+5, box.R-rightOverlap/2)
	}
}
