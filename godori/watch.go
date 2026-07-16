//go:build watch

package main

import (
	"github.com/WindowsSov8forUs/sonolus-go/v2/sonolus"
	"github.com/WindowsSov8forUs/sonolus-go/v2/sonolus/native"
	"github.com/WindowsSov8forUs/sonolus-go/v2/sonolus/watch"
)

type WatchReplayStreams struct {
	sonolus.StreamResource
	Reserved   [100]sonolus.Stream[float64]
	EmptyLanes [7]sonolus.Stream[float64]
}

var Replay = WatchReplayStreams{}

type WatchSkin struct {
	sonolus.SkinResource
	Lane, JudgmentLine, Note, Flick, FlickArrow, RightFlick, RightFlickArrow, LeftFlick, LeftFlickArrow sonolus.Sprite
	HoldHead, HoldTail, HoldConnector, SimLine, HoldTick                                                sonolus.Sprite
	StageMiddle, LeftBorder, RightBorder, Slot, Cover                                                   sonolus.Sprite
}

var Skin = &WatchSkin{
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

type WatchEffects struct {
	sonolus.EffectResource
	Stage, Perfect, Great, Good                           sonolus.Clip
	PerfectAlternative, GreatAlternative, GoodAlternative sonolus.Clip
	Hold                                                  sonolus.Clip
}

var Effects = &WatchEffects{
	Stage: sonolus.EffectClip(sonolus.StandardClipStage), Perfect: sonolus.EffectClip(sonolus.StandardClipPerfect),
	Great: sonolus.EffectClip(sonolus.StandardClipGreat), Good: sonolus.EffectClip(sonolus.StandardClipGood),
	PerfectAlternative: sonolus.EffectClip(sonolus.StandardClipPerfectAlternative),
	GreatAlternative:   sonolus.EffectClip(sonolus.StandardClipGreatAlternative),
	GoodAlternative:    sonolus.EffectClip(sonolus.StandardClipGoodAlternative),
	Hold:               sonolus.EffectClip(sonolus.StandardClipHold),
}

type WatchParticles struct {
	sonolus.ParticleResource
	Tap, Flick, RightFlick, LeftFlick, Hold, Lane                         sonolus.Effect
	TapLinear, FlickLinear, RightFlickLinear, LeftFlickLinear, HoldLinear sonolus.Effect
	HoldActive                                                            sonolus.Effect
}

var Particles = &WatchParticles{
	Tap: sonolus.ParticleEffect(sonolus.StandardEffectNoteCircularTapCyan), Flick: sonolus.ParticleEffect(sonolus.StandardEffectNoteCircularAlternativeRed),
	RightFlick: sonolus.ParticleEffect(sonolus.StandardEffectNoteCircularAlternativeYellow), LeftFlick: sonolus.ParticleEffect(sonolus.StandardEffectNoteCircularAlternativePurple),
	Hold: sonolus.ParticleEffect(sonolus.StandardEffectNoteCircularTapGreen), Lane: sonolus.ParticleEffect(sonolus.StandardEffectLaneLinear),
	TapLinear: sonolus.ParticleEffect(sonolus.StandardEffectNoteLinearTapCyan), FlickLinear: sonolus.ParticleEffect(sonolus.StandardEffectNoteLinearAlternativeRed),
	RightFlickLinear: sonolus.ParticleEffect(sonolus.StandardEffectNoteLinearAlternativeYellow), LeftFlickLinear: sonolus.ParticleEffect(sonolus.StandardEffectNoteLinearAlternativePurple),
	HoldLinear: sonolus.ParticleEffect(sonolus.StandardEffectNoteLinearTapGreen), HoldActive: sonolus.ParticleEffect(sonolus.StandardEffectNoteCircularHoldGreen),
}

type WatchBuckets struct {
	sonolus.BucketsResource
	Tap, HoldHead, HoldEnd, HoldTick, Flick, DirectionalFlick sonolus.Bucket
}

var Buckets = &WatchBuckets{
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

type WatchGlobals struct{ watch.GlobalCallbacks }

var Global WatchGlobals

func UpdateSpawn() float64 { return watch.Time.Scaled() }

type WatchBPMChange struct {
	watch.Archetype `archetype:"name=#BPM_CHANGE"`
	Beat            float64 `archetype:"imported,name=#BEAT"`
	BPM             float64 `archetype:"imported,name=#BPM"`
}

type WatchTimescaleChange struct {
	watch.Archetype `archetype:"name=#TIMESCALE_CHANGE"`
	Beat            float64 `archetype:"imported,name=#BEAT"`
	Timescale       float64 `archetype:"imported,name=#TIMESCALE"`
}

type WatchStage struct {
	watch.Archetype `archetype:"name=Stage"`
}

func (*WatchStage) SpawnTime() float64   { return -1e8 }
func (*WatchStage) DespawnTime() float64 { return 1e8 }
func (*WatchStage) Preprocess() {
	watch.Score.SetBase(watch.BaseScore{Perfect: 1, Great: 0.5, Good: 0.25})
	watch.UI.SetMenu(menuLayout())
	watch.UI.SetJudgment(judgmentLayout())
	watch.UI.SetComboValue(comboValueLayout())
	watch.UI.SetComboText(comboTextLayout())
	watch.UI.SetPrimaryMetricBar(primaryMetricBarLayout())
	watch.UI.SetPrimaryMetricValue(primaryMetricValueLayout())
	watch.UI.SetSecondaryMetricBar(secondaryMetricBarLayout())
	watch.UI.SetSecondaryMetricValue(secondaryMetricValueLayout())
	watch.UI.SetProgress(progressLayout())
}
func (*WatchStage) UpdateParallel() {
	drawWatchStage()
}

func (*WatchStage) UpdateSequential() {
	if !watch.Replay.IsReplay() || watch.Time.Skip() {
		return
	}
	now := watch.Time.Now()
	previous := watch.Time.Previous()
	for lane := -3; lane <= 3; lane++ {
		stream := Replay.EmptyLanes[lane+3]
		key := stream.PreviousKey(now)
		if !stream.Has(key) || key <= previous || key > now || stream.Get(key) == 0 {
			continue
		}
		watch.Spawn(WatchScheduledLaneEffect{Time: key, Lane: float64(lane)})
	}
}

type WatchScheduledLaneEffect struct {
	watch.Archetype `archetype:"name=ScheduledLaneEffect"`
	Time            float64 `archetype:"memory"`
	Lane            float64 `archetype:"memory"`
	Played          bool    `archetype:"memory"`
}

func (n *WatchScheduledLaneEffect) SpawnTime() float64 {
	return native.TimeToScaledTime(n.Time)
}

func (n *WatchScheduledLaneEffect) DespawnTime() float64 {
	return native.TimeToScaledTime(n.Time) + 1
}

func (n *WatchScheduledLaneEffect) UpdateParallel() {
	if n.Played || watch.Time.Now() < n.Time {
		return
	}
	n.Played = true
	if Config.SFX {
		watch.Audio.Play(Effects.Stage, 0.02)
	}
	spawnWatchLaneEffect(n.Lane)
}

func drawWatchStage() {
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

type WatchTapNote struct {
	watch.Archetype  `archetype:"name=TapNote"`
	Beat             float64          `archetype:"imported,name=#BEAT"`
	Lane             float64          `archetype:"imported,name=lane"`
	Judgment         sonolus.Judgment `archetype:"imported,name=#JUDGMENT"`
	Accuracy         float64          `archetype:"imported,name=#ACCURACY"`
	TargetTime       float64          `archetype:"data"`
	TargetScaledTime float64          `archetype:"data"`
	Played           bool             `archetype:"memory"`
}

type WatchAccentTapNote struct {
	WatchTapNote    `archetype:"base"`
	watch.Archetype `archetype:"name=AccentTapNote"`
}

func (n *WatchTapNote) Preprocess() {
	n.TargetTime = watch.Time.BeatToTime(n.Beat)
	n.TargetScaledTime = native.TimeToScaledTime(n.TargetTime)
	n.Judgment, n.Accuracy = normalizeWatchResult(n.Judgment, n.Accuracy)
	scheduleWatchSFX(n.Judgment, n.TargetTime+n.Accuracy, false)
	archetype := int(watch.Entity.Info().Archetype)
	watch.Score.SetArchetype(archetype, watch.ArchetypeScore{Multiplier: 1})
	watch.Life.SetArchetype(archetype, watch.LifeValues{Perfect: 1, Miss: -100})
	result := watch.Entity.Result()
	result.TargetTime = n.TargetTime
	result.Bucket = Buckets.Tap
	result.BucketValue = n.Accuracy * 1000
	watch.Entity.SetResult(result)
}

func (n *WatchTapNote) SpawnTime() float64   { return n.TargetScaledTime - noteTravelTime() }
func (n *WatchTapNote) DespawnTime() float64 { return n.TargetScaledTime + 0.5 }
func (n *WatchTapNote) UpdateSequential() {
	if watch.Time.Skip() {
		n.Played = true
		return
	}
	if !n.Played && watch.Time.Now() >= n.TargetTime+n.Accuracy {
		n.Played = true
		if Config.NoteEffects && n.Judgment != sonolus.JudgmentMiss {
			spawnWatchNoteParticles(n.Lane, 1.5, 0.3, Particles.TapLinear, Particles.Tap)
			spawnWatchLaneEffect(n.Lane)
		}
	}
}
func (n *WatchTapNote) UpdateParallel() {
	Skin.Note.Draw(noteRect(n.Lane, noteY(n.TargetScaledTime, watch.Time.Scaled())).ToQuad(), 30, 1)
}

type WatchFlickNote struct {
	watch.Archetype  `archetype:"name=FlickNote"`
	Beat             float64          `archetype:"imported,name=#BEAT"`
	Lane             float64          `archetype:"imported,name=lane"`
	Judgment         sonolus.Judgment `archetype:"imported,name=#JUDGMENT"`
	Accuracy         float64          `archetype:"imported,name=#ACCURACY"`
	TargetTime       float64          `archetype:"data"`
	TargetScaledTime float64          `archetype:"data"`
	Played           bool             `archetype:"memory"`
}

func (n *WatchFlickNote) Preprocess() {
	n.TargetTime = watch.Time.BeatToTime(n.Beat)
	n.TargetScaledTime = native.TimeToScaledTime(n.TargetTime)
	n.Judgment, n.Accuracy = normalizeWatchResult(n.Judgment, n.Accuracy)
	scheduleWatchSFX(n.Judgment, n.TargetTime+n.Accuracy, true)
	archetype := int(watch.Entity.Info().Archetype)
	watch.Score.SetArchetype(archetype, watch.ArchetypeScore{Multiplier: 1})
	watch.Life.SetArchetype(archetype, watch.LifeValues{Perfect: 1, Miss: -100})
	result := watch.Entity.Result()
	result.TargetTime = n.TargetTime
	result.Bucket = Buckets.Flick
	result.BucketValue = n.Accuracy * 1000
	watch.Entity.SetResult(result)
}

func (n *WatchFlickNote) SpawnTime() float64   { return n.TargetScaledTime - noteTravelTime() }
func (n *WatchFlickNote) DespawnTime() float64 { return n.TargetScaledTime + 0.5 }

func (n *WatchFlickNote) UpdateSequential() {
	if watch.Time.Skip() {
		n.Played = true
		return
	}
	if !n.Played && watch.Time.Now() >= n.TargetTime+n.Accuracy {
		n.Played = true
		if Config.NoteEffects && n.Judgment != sonolus.JudgmentMiss {
			spawnWatchNoteParticles(n.Lane, 1.5, 0.3, Particles.FlickLinear, Particles.Flick)
			spawnWatchLaneEffect(n.Lane)
		}
	}
}

func (n *WatchFlickNote) UpdateParallel() {
	y := noteY(n.TargetScaledTime, watch.Time.Scaled())
	Skin.Flick.Draw(noteRect(n.Lane, y).ToQuad(), 30, 1)
	Skin.FlickArrow.Draw(noteRect(n.Lane, y).Scale(0.7).ToQuad(), 31, 1)
}

type WatchDirectionalFlickNote struct {
	watch.Archetype  `archetype:"name=DirectionalFlickNote"`
	Beat             float64          `archetype:"imported,name=#BEAT"`
	Lane             float64          `archetype:"imported,name=lane"`
	Direction        float64          `archetype:"imported,name=direction"`
	Judgment         sonolus.Judgment `archetype:"imported,name=#JUDGMENT"`
	Accuracy         float64          `archetype:"imported,name=#ACCURACY"`
	TargetTime       float64          `archetype:"data"`
	TargetScaledTime float64          `archetype:"data"`
	Played           bool             `archetype:"memory"`
}

func (n *WatchDirectionalFlickNote) Preprocess() {
	n.TargetTime = watch.Time.BeatToTime(n.Beat)
	n.TargetScaledTime = native.TimeToScaledTime(n.TargetTime)
	n.Judgment, n.Accuracy = normalizeWatchResult(n.Judgment, n.Accuracy)
	scheduleWatchSFX(n.Judgment, n.TargetTime+n.Accuracy, true)
	archetype := int(watch.Entity.Info().Archetype)
	watch.Score.SetArchetype(archetype, watch.ArchetypeScore{Multiplier: 1})
	watch.Life.SetArchetype(archetype, watch.LifeValues{Perfect: 1, Miss: -100})
	result := watch.Entity.Result()
	result.TargetTime = n.TargetTime
	result.Bucket = Buckets.DirectionalFlick
	result.BucketValue = n.Accuracy * 1000
	watch.Entity.SetResult(result)
}

func (n *WatchDirectionalFlickNote) SpawnTime() float64 { return n.TargetScaledTime - noteTravelTime() }
func (n *WatchDirectionalFlickNote) DespawnTime() float64 {
	return n.TargetScaledTime + 0.5
}

func (n *WatchDirectionalFlickNote) UpdateSequential() {
	if watch.Time.Skip() {
		n.Played = true
		return
	}
	if !n.Played && watch.Time.Now() >= n.TargetTime+n.Accuracy {
		n.Played = true
		if Config.NoteEffects && n.Judgment != sonolus.JudgmentMiss {
			if displayDirection(n.Direction) > 0 {
				spawnWatchNoteParticles(n.Lane, 1.5, 0.3, Particles.RightFlickLinear, Particles.RightFlick)
			} else {
				spawnWatchNoteParticles(n.Lane, 1.5, 0.3, Particles.LeftFlickLinear, Particles.LeftFlick)
			}
			spawnWatchLaneEffect(n.Lane)
		}
	}
}

func (n *WatchDirectionalFlickNote) UpdateParallel() {
	y := noteY(n.TargetScaledTime, watch.Time.Scaled())
	if displayDirection(n.Direction) > 0 {
		drawDirectionalFlickNote(Skin.RightFlick, Skin.RightFlickArrow, n.Lane, y, n.Direction)
	} else {
		drawDirectionalFlickNote(Skin.LeftFlick, Skin.LeftFlickArrow, n.Lane, y, n.Direction)
	}
}

type WatchHoldHeadNote struct {
	watch.Archetype  `archetype:"name=HoldHeadNote"`
	Beat             float64                                `archetype:"imported,name=#BEAT"`
	Lane             float64                                `archetype:"imported,name=lane"`
	Anchor           sonolus.EntityRef[WatchHoldAnchorNote] `archetype:"imported,name=anchor"`
	End              sonolus.EntityRef[WatchHoldEndNote]    `archetype:"imported,name=end"`
	FlickEnd         sonolus.EntityRef[WatchHoldFlickNote]  `archetype:"imported,name=flickEnd"`
	Judgment         sonolus.Judgment                       `archetype:"imported,name=#JUDGMENT"`
	Accuracy         float64                                `archetype:"imported,name=#ACCURACY"`
	TargetTime       float64                                `archetype:"data"`
	AnchorTime       float64                                `archetype:"data"`
	EndTime          float64                                `archetype:"data"`
	TargetScaledTime float64                                `archetype:"data"`
	AnchorScaledTime float64                                `archetype:"data"`
	EndScaledTime    float64                                `archetype:"data"`
	Played           bool                                   `archetype:"memory"`
}

func (n *WatchHoldHeadNote) Preprocess() {
	n.TargetTime = watch.Time.BeatToTime(n.Beat)
	n.AnchorTime = watch.Time.BeatToTime(n.Anchor.Get().Beat)
	n.EndTime = watch.Time.BeatToTime(n.endBeat())
	n.TargetScaledTime = native.TimeToScaledTime(n.TargetTime)
	n.AnchorScaledTime = native.TimeToScaledTime(n.AnchorTime)
	n.EndScaledTime = native.TimeToScaledTime(n.EndTime)
	n.Judgment, n.Accuracy = normalizeWatchResult(n.Judgment, n.Accuracy)
	scheduleWatchSFX(n.Judgment, n.TargetTime+n.Accuracy, false)
	archetype := int(watch.Entity.Info().Archetype)
	watch.Score.SetArchetype(archetype, watch.ArchetypeScore{Multiplier: 1})
	watch.Life.SetArchetype(archetype, watch.LifeValues{Perfect: 1, Miss: -100})
	result := watch.Entity.Result()
	result.TargetTime = n.TargetTime
	result.Bucket = Buckets.HoldHead
	result.BucketValue = n.Accuracy * 1000
	watch.Entity.SetResult(result)
	scheduleWatchHoldSFX(watch.Entity.Info().Index, n.TargetTime, n.EndTime)
	watch.Spawn(WatchHoldManager{Head: sonolus.EntityRef[WatchHoldHeadNote]{Index: watch.Entity.Info().Index}})
}

func (n *WatchHoldHeadNote) SpawnTime() float64 { return n.TargetScaledTime - noteTravelTime() }
func (n *WatchHoldHeadNote) DespawnTime() float64 {
	return n.TargetScaledTime + 0.5
}

func (n *WatchHoldHeadNote) UpdateSequential() {
	if watch.Time.Skip() {
		n.Played = true
		return
	}
	if !n.Played && watch.Time.Now() >= n.TargetTime+n.Accuracy {
		n.Played = true
		if Config.NoteEffects && n.Judgment != sonolus.JudgmentMiss {
			spawnWatchNoteParticles(n.Lane, 1.5, 0.3, Particles.HoldLinear, Particles.Hold)
			spawnWatchLaneEffect(n.Lane)
		}
	}
}

func (n *WatchHoldHeadNote) UpdateParallel() {
	now := watch.Time.Scaled()
	startLane := n.Lane
	startY := noteY(n.TargetScaledTime, now)
	if startY < judgmentLineY {
		startY = judgmentLineY
	}
	active := !watch.Replay.IsReplay()
	streamID := watch.Entity.Info().Index
	stream := Replay.Reserved[int(streamID)]
	key := stream.PreviousKey(watch.Time.Now())
	if watch.Replay.IsReplay() {
		active = stream.Has(key) && stream.Get(key) != 0
	}
	if active && now >= n.TargetScaledTime {
		startLane = n.currentScaledLane(now)
	}
	Skin.HoldHead.Draw(noteRect(startLane, startY).ToQuad(), 30, 1)
}

func (n *WatchHoldHeadNote) currentScaledLane(now float64) float64 {
	return holdChainLane(now, n.TargetScaledTime, n.AnchorScaledTime, n.EndScaledTime, n.Lane, n.Anchor.Get().Lane, n.endLane())
}

func (n *WatchHoldHeadNote) endBeat() float64 {
	if n.FlickEnd.Index > 0 {
		return n.FlickEnd.Get().Beat
	}
	return n.End.Get().Beat
}

func (n *WatchHoldHeadNote) endLane() float64 {
	if n.FlickEnd.Index > 0 {
		return n.FlickEnd.Get().Lane
	}
	return n.End.Get().Lane
}

type WatchHoldManager struct {
	watch.Archetype `archetype:"name=HoldManager"`
	Head            sonolus.EntityRef[WatchHoldHeadNote] `archetype:"memory"`
	ParticleActive  bool                                 `archetype:"memory"`
	ParticleID      float64                              `archetype:"memory"`
}

func (n *WatchHoldManager) SpawnTime() float64 { return n.Head.Get().TargetScaledTime }
func (n *WatchHoldManager) DespawnTime() float64 {
	return n.Head.Get().EndScaledTime
}

func (n *WatchHoldManager) UpdateParallel() {
	if watch.Time.Skip() {
		n.stopEffects()
		return
	}
	head := n.Head.Get()
	active := !watch.Replay.IsReplay()
	stream := Replay.Reserved[int(n.Head.Index)]
	key := stream.PreviousKey(watch.Time.Now())
	if watch.Replay.IsReplay() {
		active = stream.Has(key) && stream.Get(key) != 0
	}
	if !active || watch.Time.Scaled() < head.TargetScaledTime {
		n.stopEffects()
		return
	}
	lane := holdChainLane(
		watch.Time.Scaled(),
		head.TargetScaledTime,
		head.AnchorScaledTime,
		head.EndScaledTime,
		head.Lane,
		head.Anchor.Get().Lane,
		head.endLane(),
	)
	n.updateEffects(lane)
}

func (n *WatchHoldManager) Terminate() { n.stopEffects() }

func (n *WatchHoldManager) updateEffects(lane float64) {
	if !Config.NoteEffects {
		return
	}
	quad := noteRect(lane, judgmentLineY).Scale(1.5).ToQuad()
	if !n.ParticleActive {
		n.ParticleActive = true
		n.ParticleID = Particles.HoldActive.Spawn(quad, 0.3, true).ID
	} else {
		sonolus.ParticleHandle{ID: n.ParticleID}.Move(quad)
	}
}

func (n *WatchHoldManager) stopEffects() {
	if !n.ParticleActive {
		return
	}
	sonolus.ParticleHandle{ID: n.ParticleID}.Destroy()
	n.ParticleActive = false
}

type WatchHoldAnchorNote struct {
	watch.Archetype `archetype:"name=HoldAnchorNote"`
	Beat            float64 `archetype:"imported,name=#BEAT"`
	Lane            float64 `archetype:"imported,name=lane"`
}

type WatchHoldEndNote struct {
	watch.Archetype  `archetype:"name=HoldEndNote"`
	Head             sonolus.EntityRef[WatchHoldHeadNote] `archetype:"imported,name=head"`
	Beat             float64                              `archetype:"imported,name=#BEAT"`
	Lane             float64                              `archetype:"imported,name=lane"`
	Judgment         sonolus.Judgment                     `archetype:"imported,name=#JUDGMENT"`
	Accuracy         float64                              `archetype:"imported,name=#ACCURACY"`
	TargetTime       float64                              `archetype:"data"`
	TargetScaledTime float64                              `archetype:"data"`
	Played           bool                                 `archetype:"memory"`
}

func (n *WatchHoldEndNote) Preprocess() {
	n.TargetTime = watch.Time.BeatToTime(n.Beat)
	n.TargetScaledTime = native.TimeToScaledTime(n.TargetTime)
	n.Judgment, n.Accuracy = normalizeWatchResult(n.Judgment, n.Accuracy)
	scheduleWatchSFX(n.Judgment, n.TargetTime+n.Accuracy, false)
	archetype := int(watch.Entity.Info().Archetype)
	watch.Score.SetArchetype(archetype, watch.ArchetypeScore{Multiplier: 1})
	watch.Life.SetArchetype(archetype, watch.LifeValues{Perfect: 1, Miss: -100})
	result := watch.Entity.Result()
	result.TargetTime = n.TargetTime
	result.Bucket = Buckets.HoldEnd
	result.BucketValue = n.Accuracy * 1000
	watch.Entity.SetResult(result)
}
func (n *WatchHoldEndNote) SpawnTime() float64   { return n.TargetScaledTime - noteTravelTime() }
func (n *WatchHoldEndNote) DespawnTime() float64 { return n.TargetScaledTime + 0.5 }
func (n *WatchHoldEndNote) UpdateSequential() {
	if watch.Time.Skip() {
		n.Played = true
		return
	}
	if !n.Played && watch.Time.Now() >= n.TargetTime+n.Accuracy {
		n.Played = true
		if Config.NoteEffects && n.Judgment != sonolus.JudgmentMiss {
			spawnWatchNoteParticles(n.Lane, 1.5, 0.3, Particles.HoldLinear, Particles.Hold)
			spawnWatchLaneEffect(n.Lane)
		}
	}
}
func (n *WatchHoldEndNote) UpdateParallel() {
	Skin.HoldTail.Draw(noteRect(n.Lane, noteY(n.TargetScaledTime, watch.Time.Scaled())).ToQuad(), 30, 1)
}

type WatchHoldFlickNote struct {
	watch.Archetype  `archetype:"name=HoldFlickNote"`
	Head             sonolus.EntityRef[WatchHoldHeadNote] `archetype:"imported,name=head"`
	Beat             float64                              `archetype:"imported,name=#BEAT"`
	Lane             float64                              `archetype:"imported,name=lane"`
	Judgment         sonolus.Judgment                     `archetype:"imported,name=#JUDGMENT"`
	Accuracy         float64                              `archetype:"imported,name=#ACCURACY"`
	TargetTime       float64                              `archetype:"data"`
	TargetScaledTime float64                              `archetype:"data"`
	Played           bool                                 `archetype:"memory"`
}

func (n *WatchHoldFlickNote) Preprocess() {
	n.TargetTime = watch.Time.BeatToTime(n.Beat)
	n.TargetScaledTime = native.TimeToScaledTime(n.TargetTime)
	n.Judgment, n.Accuracy = normalizeWatchResult(n.Judgment, n.Accuracy)
	scheduleWatchSFX(n.Judgment, n.TargetTime+n.Accuracy, true)
	archetype := int(watch.Entity.Info().Archetype)
	watch.Score.SetArchetype(archetype, watch.ArchetypeScore{Multiplier: 1})
	watch.Life.SetArchetype(archetype, watch.LifeValues{Perfect: 1, Miss: -100})
	result := watch.Entity.Result()
	result.TargetTime = n.TargetTime
	result.Bucket = Buckets.Flick
	result.BucketValue = n.Accuracy * 1000
	watch.Entity.SetResult(result)
}

func (n *WatchHoldFlickNote) SpawnTime() float64 { return n.TargetScaledTime - noteTravelTime() }
func (n *WatchHoldFlickNote) DespawnTime() float64 {
	return n.TargetScaledTime + 0.5
}
func (n *WatchHoldFlickNote) UpdateSequential() {
	if watch.Time.Skip() {
		n.Played = true
		return
	}
	if !n.Played && watch.Time.Now() >= n.TargetTime+n.Accuracy {
		n.Played = true
		if Config.NoteEffects && n.Judgment != sonolus.JudgmentMiss {
			spawnWatchNoteParticles(n.Lane, 1.5, 0.3, Particles.FlickLinear, Particles.Flick)
			spawnWatchLaneEffect(n.Lane)
		}
	}
}
func (n *WatchHoldFlickNote) UpdateParallel() {
	y := noteY(n.TargetScaledTime, watch.Time.Scaled())
	Skin.Flick.Draw(noteRect(n.Lane, y).ToQuad(), 30, 1)
	Skin.FlickArrow.Draw(noteRect(n.Lane, y).Scale(0.7).ToQuad(), 31, 1)
}

type WatchHoldConnector struct {
	watch.Archetype  `archetype:"name=HoldConnector"`
	Head             sonolus.EntityRef[WatchHoldHeadNote]   `archetype:"imported,name=head"`
	Anchor           sonolus.EntityRef[WatchHoldAnchorNote] `archetype:"imported,name=anchor"`
	End              sonolus.EntityRef[WatchHoldEndNote]    `archetype:"imported,name=end"`
	FlickEnd         sonolus.EntityRef[WatchHoldFlickNote]  `archetype:"imported,name=flickEnd"`
	Segment          float64                                `archetype:"imported,name=segment"`
	TargetTime       float64                                `archetype:"data"`
	EndTime          float64                                `archetype:"data"`
	TargetScaledTime float64                                `archetype:"data"`
	EndScaledTime    float64                                `archetype:"data"`
	FirstLane        float64                                `archetype:"data"`
	SecondLane       float64                                `archetype:"data"`
}

func (n *WatchHoldConnector) Preprocess() {
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
	n.TargetTime = watch.Time.BeatToTime(targetBeat)
	n.EndTime = watch.Time.BeatToTime(endBeat)
	n.TargetScaledTime = native.TimeToScaledTime(n.TargetTime)
	n.EndScaledTime = native.TimeToScaledTime(n.EndTime)
}
func (n *WatchHoldConnector) SpawnTime() float64   { return n.TargetScaledTime - noteTravelTime() }
func (n *WatchHoldConnector) DespawnTime() float64 { return n.EndScaledTime + 0.5 }
func (n *WatchHoldConnector) UpdateParallel() {
	now := watch.Time.Scaled()
	startLane := n.FirstLane
	startY := noteY(n.TargetScaledTime, now)
	if startY < judgmentLineY {
		startY = judgmentLineY
	}
	active := !watch.Replay.IsReplay()
	stream := Replay.Reserved[int(n.Head.Index)]
	key := stream.PreviousKey(watch.Time.Now())
	if watch.Replay.IsReplay() {
		active = stream.Has(key) && stream.Get(key) != 0
	}
	if active && now >= n.TargetScaledTime {
		startLane = holdLane(now, n.TargetScaledTime, n.EndScaledTime, n.FirstLane, n.SecondLane)
	}
	endY := noteY(n.EndScaledTime, now)
	Skin.HoldConnector.Draw(holdConnectorQuad(startLane, n.SecondLane, startY, endY), 25, Config.ConnectorAlpha)
}

type WatchSimLine struct {
	watch.Archetype  `archetype:"name=SimLine"`
	First            sonolus.EntityRef[WatchTapNote] `archetype:"imported,name=first"`
	Second           sonolus.EntityRef[WatchTapNote] `archetype:"imported,name=second"`
	TargetTime       float64                         `archetype:"data"`
	TargetScaledTime float64                         `archetype:"data"`
}

func (n *WatchSimLine) Preprocess() {
	n.TargetTime = watch.Time.BeatToTime(n.First.Get().Beat)
	n.TargetScaledTime = native.TimeToScaledTime(n.TargetTime)
}
func (n *WatchSimLine) SpawnTime() float64 {
	return n.TargetScaledTime - noteTravelTime()
}
func (n *WatchSimLine) DespawnTime() float64 { return n.TargetScaledTime + 0.5 }
func (n *WatchSimLine) UpdateParallel() {
	if !Config.SimLines {
		return
	}
	first := watch.Entity.InfoAt(int(n.First.Index))
	second := watch.Entity.InfoAt(int(n.Second.Index))
	if first.State == sonolus.EntityStateDespawned || second.State == sonolus.EntityStateDespawned {
		return
	}
	y := noteY(n.TargetScaledTime, watch.Time.Scaled())
	Skin.SimLine.Draw(simLineQuad(n.First.Get().Lane, n.Second.Get().Lane, y), 24, Config.ConnectorAlpha)
}

type WatchHoldTickNote struct {
	watch.Archetype  `archetype:"name=HoldTickNote"`
	Head             sonolus.EntityRef[WatchHoldHeadNote] `archetype:"imported,name=head"`
	Beat             float64                              `archetype:"imported,name=#BEAT"`
	Lane             float64                              `archetype:"data"`
	Judgment         sonolus.Judgment                     `archetype:"imported,name=#JUDGMENT"`
	Accuracy         float64                              `archetype:"imported,name=#ACCURACY"`
	TargetTime       float64                              `archetype:"data"`
	TargetScaledTime float64                              `archetype:"data"`
	Played           bool                                 `archetype:"memory"`
}

func (n *WatchHoldTickNote) Preprocess() {
	n.Lane = holdChainLane(
		n.Beat,
		n.Head.Get().Beat,
		n.Head.Get().Anchor.Get().Beat,
		n.Head.Get().endBeat(),
		n.Head.Get().Lane,
		n.Head.Get().Anchor.Get().Lane,
		n.Head.Get().endLane(),
	)
	n.TargetTime = watch.Time.BeatToTime(n.Beat)
	n.TargetScaledTime = native.TimeToScaledTime(n.TargetTime)
	n.Judgment, n.Accuracy = normalizeWatchResult(n.Judgment, n.Accuracy)
	scheduleWatchSFX(n.Judgment, n.TargetTime+n.Accuracy, false)
	archetype := int(watch.Entity.Info().Archetype)
	watch.Score.SetArchetype(archetype, watch.ArchetypeScore{Multiplier: 1})
	watch.Life.SetArchetype(archetype, watch.LifeValues{Perfect: 1, Miss: -20})
	result := watch.Entity.Result()
	result.TargetTime = n.TargetTime
	result.Bucket = Buckets.HoldTick
	result.BucketValue = n.Accuracy * 1000
	watch.Entity.SetResult(result)
}

func (n *WatchHoldTickNote) SpawnTime() float64 { return n.TargetScaledTime - noteTravelTime() }
func (n *WatchHoldTickNote) DespawnTime() float64 {
	return n.TargetScaledTime + 0.5
}
func (n *WatchHoldTickNote) UpdateSequential() {
	if watch.Time.Skip() {
		n.Played = true
		return
	}
	if !n.Played && watch.Time.Now() >= n.TargetTime+n.Accuracy {
		n.Played = true
		if Config.NoteEffects && n.Judgment != sonolus.JudgmentMiss {
			spawnWatchNoteParticles(n.Lane, 1.2, 0.2, Particles.HoldLinear, Particles.Hold)
			spawnWatchLaneEffect(n.Lane)
		}
	}
}
func (n *WatchHoldTickNote) UpdateParallel() {
	Skin.HoldTick.Draw(noteRect(n.Lane, noteY(n.TargetScaledTime, watch.Time.Scaled())).Scale(0.7).ToQuad(), 29, 1)
}

func spawnWatchLaneEffect(lane float64) {
	if Config.LaneEffects {
		Particles.Lane.Spawn(laneRect(lane).ToQuad(), 0.2, false)
	}
}

func spawnWatchNoteParticles(lane, scale, duration float64, linear, circular sonolus.Effect) {
	particleRect := noteRect(lane, judgmentLineY).Scale(scale)
	width := particleRect.R - particleRect.L
	linear.Spawn(sonolus.Rect{T: judgmentLineY + width, R: particleRect.R, B: judgmentLineY, L: particleRect.L}.ToQuad(), duration, false)
	circular.Spawn(particleRect.ToQuad(), duration, false)
}

func normalizeWatchResult(judgment sonolus.Judgment, accuracy float64) (sonolus.Judgment, float64) {
	if watch.Replay.IsReplay() {
		return judgment, accuracy
	}
	return sonolus.JudgmentPerfect, 0
}

func scheduleWatchSFX(judgment sonolus.Judgment, at float64, alternative bool) {
	if !Config.SFX || judgment == sonolus.JudgmentMiss {
		return
	}
	if judgment == sonolus.JudgmentPerfect {
		if alternative {
			watch.Audio.PlayScheduled(Effects.PerfectAlternative, at, 0.02)
		} else {
			watch.Audio.PlayScheduled(Effects.Perfect, at, 0.02)
		}
	} else if judgment == sonolus.JudgmentGreat {
		if alternative {
			watch.Audio.PlayScheduled(Effects.GreatAlternative, at, 0.02)
		} else {
			watch.Audio.PlayScheduled(Effects.Great, at, 0.02)
		}
	} else if judgment == sonolus.JudgmentGood {
		if alternative {
			watch.Audio.PlayScheduled(Effects.GoodAlternative, at, 0.02)
		} else {
			watch.Audio.PlayScheduled(Effects.Good, at, 0.02)
		}
	}
}

func scheduleWatchHoldSFX(streamID, startTime, endTime float64) {
	if !Config.SFX {
		return
	}
	if !watch.Replay.IsReplay() {
		Effects.Hold.PlayLoopedScheduled(startTime).Stop(endTime)
		return
	}
	stream := Replay.Reserved[int(streamID)]
	key := stream.NextKey(unsetJudgmentTime)
	active := false
	activeStart := startTime
	for stream.Has(key) {
		if key > endTime {
			break
		}
		nextActive := stream.Get(key) != 0
		if key < startTime {
			active = nextActive
			activeStart = startTime
		} else if nextActive && !active {
			active = true
			activeStart = key
		} else if !nextActive && active {
			Effects.Hold.PlayLoopedScheduled(activeStart).Stop(key)
			active = false
		}
		key = stream.NextKey(key)
	}
	if active {
		Effects.Hold.PlayLoopedScheduled(activeStart).Stop(endTime)
	}
}
