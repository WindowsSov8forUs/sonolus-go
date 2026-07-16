//go:build watch

package main

import (
	"math"

	"github.com/WindowsSov8forUs/sonolus-go/v2/sonolus"
	"github.com/WindowsSov8forUs/sonolus-go/v2/sonolus/watch"
)

type WatchReplayStreams struct {
	sonolus.StreamResource
	Reserved   [99999]sonolus.Stream[float64]
	EmptyLanes sonolus.Stream[[7]bool]
}

var Replay = WatchReplayStreams{}

type WatchLayoutData struct {
	sonolus.LevelDataResource
	Transform sonolus.Transform2D
}

var Layout = WatchLayoutData{}

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
	Layout.Transform = stageTransform()
	watch.SkinTransform.Set(Layout.Transform)
	watch.Score.SetBase(watch.BaseScore{Perfect: 1, Great: 0.5, Good: 0.25})
	window := noteBucketWindowMilliseconds()
	Buckets.Tap.SetWindow(window)
	Buckets.HoldHead.SetWindow(window)
	Buckets.HoldEnd.SetWindow(window)
	Buckets.HoldTick.SetWindow(window)
	Buckets.Flick.SetWindow(window)
	Buckets.DirectionalFlick.SetWindow(window)
	configureWatchNoteArchetype[WatchTapNote]()
	configureWatchNoteArchetype[WatchAccentTapNote]()
	configureWatchNoteArchetype[WatchFlickNote]()
	configureWatchNoteArchetype[WatchDirectionalFlickNote]()
	configureWatchNoteArchetype[WatchHoldHeadNote]()
	configureWatchNoteArchetype[WatchHoldAnchorNote]()
	configureWatchNoteArchetype[WatchHoldEndNote]()
	configureWatchNoteArchetype[WatchHoldFlickNote]()
	configureWatchNoteArchetype[WatchHoldTickNote]()
	screen := watch.Screen.Rect()
	menuConfig := watch.UI.MenuConfiguration()
	comboConfig := watch.UI.ComboConfiguration()
	primaryConfig := watch.UI.PrimaryMetricConfiguration()
	secondaryConfig := watch.UI.SecondaryMetricConfiguration()
	watch.UI.SetMenu(menuLayout(screen, menuConfig))
	watch.UI.SetJudgment(judgmentLayout(watch.UI.JudgmentConfiguration()))
	watch.UI.SetComboValue(comboValueLayout(screen, comboConfig))
	watch.UI.SetComboText(comboTextLayout(screen, comboConfig))
	watch.UI.SetPrimaryMetricBar(primaryMetricBarLayout(screen, primaryConfig))
	watch.UI.SetPrimaryMetricValue(primaryMetricValueLayout(screen, primaryConfig))
	watch.UI.SetSecondaryMetricBar(secondaryMetricBarLayout(screen, secondaryConfig, menuConfig))
	watch.UI.SetSecondaryMetricValue(secondaryMetricValueLayout(screen, secondaryConfig, menuConfig, primaryConfig))
	watch.UI.SetProgress(progressLayout(screen, watch.UI.ProgressConfiguration()))
	if watch.Replay.IsReplay() {
		for effectTime, lanes := range Replay.EmptyLanes.ItemsFrom(-10) {
			if Config.SFX {
				watch.Audio.PlayScheduled(Effects.Stage, effectTime, 0.02)
			}
			for lane, active := range lanes {
				if active {
					watch.Spawn(WatchScheduledLaneEffect{Time: effectTime, Lane: float64(lane - 3)})
				}
			}
		}
	}
}

func configureWatchNoteArchetype[T any]() {
	archetype := watch.ArchetypeID[T]()
	missLife := -100.0
	if watch.ArchetypeKey[T]() == 5 {
		missLife = -20
	}
	watch.Score.SetArchetype(archetype, watch.ArchetypeScore{Multiplier: 1})
	watch.Life.SetArchetype(archetype, watch.LifeValues{Perfect: 1, Miss: missLife})
}
func (*WatchStage) UpdateParallel() {
	drawWatchStage()
}

type WatchScheduledLaneEffect struct {
	watch.Archetype `archetype:"name=ScheduledLaneEffect"`
	Time            float64 `archetype:"memory"`
	Lane            float64 `archetype:"memory"`
}

func (n *WatchScheduledLaneEffect) SpawnTime() float64 {
	return watch.Time.TimeToScaledTime(n.Time)
}

func (n *WatchScheduledLaneEffect) DespawnTime() float64 {
	return watch.Time.TimeToScaledTime(n.Time) + 1
}

func (n *WatchScheduledLaneEffect) UpdateParallel() {
	if watch.Time.Previous() >= n.Time || n.Time > watch.Time.Now() {
		return
	}
	spawnWatchLaneEffect(n.Lane)
}

func drawWatchStage() {
	for lane := -3; lane <= 3; lane++ {
		Skin.Lane.Draw(laneRect(float64(lane)).ToQuad(), layerLane, 1)
	}
	Skin.LeftBorder.Draw(leftBorderRect().ToQuad(), layerLane, 1)
	Skin.RightBorder.Draw(rightBorderRect().ToQuad(), layerLane, 1)
	Skin.JudgmentLine.Draw(judgmentRect().ToQuad(), layerJudgmentLine, 1)
}

type WatchBasicNote struct {
	watch.Archetype  `archetype:"abstract"`
	Beat             float64                           `archetype:"imported,name=#BEAT"`
	Lane             float64                           `archetype:"imported,name=lane"`
	Direction        float64                           `archetype:"imported,name=direction"`
	Previous         sonolus.EntityRef[WatchBasicNote] `archetype:"imported,name=prev"`
	Next             sonolus.EntityRef[WatchBasicNote] `archetype:"imported,name=next"`
	Judgment         sonolus.Judgment                  `archetype:"imported,name=#JUDGMENT"`
	Accuracy         float64                           `archetype:"imported,name=#ACCURACY"`
	ReplayEndTime    float64                           `archetype:"imported,name=end_time"`
	TargetTime       float64                           `archetype:"data"`
	TargetScaledTime float64                           `archetype:"data"`
	EndTime          float64                           `archetype:"data"`
	EndScaledTime    float64                           `archetype:"data"`
	ReplayEndScaled  float64                           `archetype:"data"`
	HoldLane         float64                           `archetype:"shared"`
}

func (n *WatchBasicNote) SpawnTime() float64 { return n.TargetScaledTime - noteTravelTime() }
func (n *WatchBasicNote) DespawnTime() float64 {
	if watch.Replay.IsReplay() {
		return n.ReplayEndScaled
	}
	return n.TargetScaledTime
}

func (n *WatchBasicNote) holdHeadRef() sonolus.EntityRef[WatchHoldHeadNote] {
	return watchHoldHeadRef(watch.CurrentEntityRef[WatchBasicNote]())
}

func watchHoldHeadRef(ref sonolus.EntityRef[WatchBasicNote]) sonolus.EntityRef[WatchHoldHeadNote] {
	for previous := ref.Get().Previous; previous.Index > 0; previous = previous.Get().Previous {
		ref = previous
	}
	return sonolus.EntityRefAs[WatchHoldHeadNote](ref)
}

func (n *WatchBasicNote) Preprocess() {
	if Config.Mirror {
		n.Lane = -n.Lane
		n.Direction = -n.Direction
	}
	n.TargetTime = watch.Time.BeatToTime(n.Beat)
	n.TargetScaledTime = watch.Time.TimeToScaledTime(n.TargetTime)
	n.EndTime = watch.Time.BeatToTime(n.endBeat())
	n.EndScaledTime = watch.Time.TimeToScaledTime(n.EndTime)
	n.ReplayEndScaled = watch.Time.TimeToScaledTime(n.ReplayEndTime)
	key := int(watch.Entity.Key())
	if key == 6 {
		return
	}
	n.Judgment, n.Accuracy = normalizeWatchResult(n.Judgment, n.Accuracy)
	scheduleWatchSFX(n.Judgment, n.replayEventTime(), key == 2 || key == 3)
	result := watch.Entity.Result()
	result.TargetTime = n.TargetTime
	switch key {
	case 1:
		result.Bucket = Buckets.Tap
	case 2:
		result.Bucket = Buckets.Flick
	case 3:
		result.Bucket = Buckets.DirectionalFlick
	case 4:
		result.Bucket = Buckets.HoldHead
	case 5:
		result.Bucket = Buckets.HoldTick
	case 7:
		result.Bucket = Buckets.HoldEnd
	}
	result.BucketValue = n.Accuracy * 1000
	watch.Entity.SetResult(result)
	if key == 4 {
		scheduleWatchHoldSFX(watch.CurrentEntityRef[WatchBasicNote]().Index, n.TargetTime, n.EndTime)
		watch.Spawn(WatchHoldManager{Head: sonolus.EntityRefAs[WatchHoldHeadNote](watch.CurrentEntityRef[WatchBasicNote]())})
	}
}

func (n *WatchBasicNote) Terminate() {
	key := int(watch.Entity.Key())
	if key == 6 || watch.Time.Skip() {
		return
	}
	if !Config.NoteEffects || n.Judgment == sonolus.JudgmentMiss {
		return
	}
	switch key {
	case 1:
		spawnWatchNoteParticles(n.Lane, Particles.TapLinear, Particles.Tap)
	case 2:
		spawnWatchNoteParticles(n.Lane, Particles.FlickLinear, Particles.Flick)
	case 3:
		if displayDirection(n.Direction) > 0 {
			spawnWatchNoteParticles(n.Lane, Particles.RightFlickLinear, Particles.RightFlick)
		} else {
			spawnWatchNoteParticles(n.Lane, Particles.LeftFlickLinear, Particles.LeftFlick)
		}
	case 4:
		spawnWatchNoteParticles(n.Lane, Particles.HoldLinear, Particles.Hold)
	case 5:
		spawnWatchNoteParticles(n.Lane, Particles.HoldLinear, Particles.Hold)
	case 7:
		spawnWatchNoteParticles(n.Lane, Particles.HoldLinear, Particles.Hold)
	}
	spawnWatchLaneEffect(n.Lane)
}

func (n *WatchBasicNote) replayEventTime() float64 {
	if watch.Replay.IsReplay() {
		return n.ReplayEndTime
	}
	return n.TargetTime
}

func (n *WatchBasicNote) UpdateParallel() {
	y := noteY(n.TargetScaledTime, watch.Time.Scaled())
	switch int(watch.Entity.Key()) {
	case 1:
		drawNoteBody(Skin.Note, n.Lane, y)
	case 2:
		drawFlickNote(Skin.Flick, Skin.FlickArrow, n.Lane, y, watch.Time.Now())
	case 3:
		if displayDirection(n.Direction) > 0 {
			drawDirectionalFlickNote(Skin.RightFlick, Skin.RightFlickArrow, n.Lane, y, n.Direction, watch.Time.Now())
		} else {
			drawDirectionalFlickNote(Skin.LeftFlick, Skin.LeftFlickArrow, n.Lane, y, n.Direction, watch.Time.Now())
		}
	case 4:
		now := watch.Time.Scaled()
		lane := n.Lane
		active := !watch.Replay.IsReplay()
		streamID := watch.CurrentEntityRef[WatchBasicNote]().Index
		stream := Replay.Reserved[int(streamID)]
		key := stream.PreviousKey(watch.Time.Now())
		if watch.Replay.IsReplay() {
			active = stream.Has(key) && stream.Get(key) != 0
		}
		if active && now >= n.TargetScaledTime {
			lane = n.HoldLane
		}
		if y < judgmentLineY {
			y = judgmentLineY
		}
		if active && now >= n.TargetScaledTime {
			Skin.HoldHead.Draw(noteRect(lane, y).ToQuad(), layerZ(layerNoteHead, lane, 0), 1)
		} else {
			drawNoteBody(Skin.HoldHead, lane, y)
		}
	case 5:
		drawNoteBody(Skin.HoldTick, n.Lane, y)
	case 7:
		drawNoteBody(Skin.HoldTail, n.Lane, y)
	}
}

type WatchTapNote struct {
	WatchBasicNote  `archetype:"base"`
	watch.Archetype `archetype:"name=TapNote,key=1"`
}

type WatchAccentTapNote struct {
	WatchTapNote    `archetype:"base"`
	watch.Archetype `archetype:"name=AccentTapNote"`
}

type WatchFlickNote struct {
	WatchBasicNote  `archetype:"base"`
	watch.Archetype `archetype:"name=FlickNote,key=2"`
}

type WatchDirectionalFlickNote struct {
	WatchBasicNote  `archetype:"base"`
	watch.Archetype `archetype:"name=DirectionalFlickNote,key=3"`
}

type WatchHoldHeadNote struct {
	WatchBasicNote  `archetype:"base"`
	watch.Archetype `archetype:"name=HoldHeadNote,key=4"`
}

func (n *WatchBasicNote) currentScaledLane(now float64) float64 {
	previousTime, previousLane := n.TargetScaledTime, n.Lane
	nextRef := n.Next
	for nextRef.Index > 0 {
		next := nextRef.Get()
		if nextRef.Key() == 6 || next.Next.Index <= 0 {
			nextTime := watch.Time.TimeToScaledTime(watch.Time.BeatToTime(next.Beat))
			if now <= nextTime || next.Next.Index <= 0 {
				return holdLane(now, previousTime, nextTime, previousLane, next.Lane)
			}
			previousTime, previousLane = nextTime, next.Lane
		}
		nextRef = next.Next
	}
	return previousLane
}

func (n *WatchBasicNote) currentLane(now float64) float64 {
	previousTime, previousLane := n.TargetTime, n.Lane
	nextRef := n.Next
	for nextRef.Index > 0 {
		next := nextRef.Get()
		if nextRef.Key() == 6 || next.Next.Index <= 0 {
			nextTime := watch.Time.BeatToTime(next.Beat)
			if now <= nextTime || next.Next.Index <= 0 {
				return holdLane(now, previousTime, nextTime, previousLane, next.Lane)
			}
			previousTime, previousLane = nextTime, next.Lane
		}
		nextRef = next.Next
	}
	return previousLane
}

func (n *WatchBasicNote) endBeat() float64 {
	beat := n.Beat
	for nextRef := n.Next; nextRef.Index > 0; {
		next := nextRef.Get()
		beat, nextRef = next.Beat, next.Next
	}
	return beat
}

func (n *WatchBasicNote) endLane() float64 {
	lane := n.Lane
	for nextRef := n.Next; nextRef.Index > 0; {
		next := nextRef.Get()
		lane, nextRef = next.Lane, next.Next
	}
	return lane
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
	n.updateEffects(head.HoldLane)
}

func (n *WatchHoldManager) Terminate() { n.stopEffects() }

func (n *WatchHoldManager) updateEffects(lane float64) {
	if !Config.NoteEffects {
		return
	}
	quad := noteCircularParticleQuad(Layout.Transform, lane)
	if !n.ParticleActive {
		n.ParticleActive = true
		n.ParticleID = Particles.HoldActive.Spawn(quad, 1.5, true).ID
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
	WatchBasicNote  `archetype:"base"`
	watch.Archetype `archetype:"name=HoldAnchorNote,key=6"`
}

type WatchHoldEndNote struct {
	WatchBasicNote  `archetype:"base"`
	watch.Archetype `archetype:"name=HoldEndNote,key=7"`
}

type WatchHoldFlickNote struct {
	WatchBasicNote  `archetype:"base"`
	watch.Archetype `archetype:"name=HoldFlickNote,key=2"`
}

type WatchHoldConnector struct {
	watch.Archetype  `archetype:"name=HoldConnector"`
	First            sonolus.EntityRef[WatchBasicNote] `archetype:"imported,name=first"`
	Second           sonolus.EntityRef[WatchBasicNote] `archetype:"imported,name=second"`
	TargetTime       float64                           `archetype:"data"`
	EndTime          float64                           `archetype:"data"`
	TargetScaledTime float64                           `archetype:"data"`
	EndScaledTime    float64                           `archetype:"data"`
	FirstLane        float64                           `archetype:"data"`
	SecondLane       float64                           `archetype:"data"`
}

func (n *WatchHoldConnector) Preprocess() {
	first, second := n.First.Get(), n.Second.Get()
	n.FirstLane, n.SecondLane = first.Lane, second.Lane
	n.TargetTime = watch.Time.BeatToTime(first.Beat)
	n.EndTime = watch.Time.BeatToTime(second.Beat)
	n.TargetScaledTime = watch.Time.TimeToScaledTime(n.TargetTime)
	n.EndScaledTime = watch.Time.TimeToScaledTime(n.EndTime)
}
func (n *WatchHoldConnector) SpawnTime() float64   { return n.TargetScaledTime - noteTravelTime() }
func (n *WatchHoldConnector) DespawnTime() float64 { return n.EndScaledTime }
func (n *WatchHoldConnector) UpdateSequential() {
	first, second := n.First.Get(), n.Second.Get()
	now := watch.Time.Now()
	headRef := watchHoldHeadRef(n.First)
	head := headRef.Get()
	active := !watch.Replay.IsReplay()
	stream := Replay.Reserved[int(headRef.Index)]
	key := stream.PreviousKey(now)
	if watch.Replay.IsReplay() {
		active = stream.Has(key) && stream.Get(key) != 0
	}
	if now < first.TargetTime || now >= second.TargetTime || !active {
		return
	}
	head.HoldLane = sonolus.Remap(
		noteY(first.TargetScaledTime, watch.Time.Scaled()),
		noteY(second.TargetScaledTime, watch.Time.Scaled()),
		first.Lane,
		second.Lane,
		judgmentLineY,
	)
}
func (n *WatchHoldConnector) UpdateParallel() {
	first, second := n.First.Get(), n.Second.Get()
	firstY := noteY(first.TargetScaledTime, watch.Time.Scaled())
	secondY := noteY(second.TargetScaledTime, watch.Time.Scaled())
	Skin.HoldConnector.Draw(holdConnectorQuad(first.Lane, second.Lane, firstY, secondY), layerZ(layerConnector, math.Min(first.Lane, second.Lane), math.Min(firstY, secondY)), Config.ConnectorAlpha)
}

type WatchSimLine struct {
	watch.Archetype  `archetype:"name=SimLine"`
	First            sonolus.EntityRef[WatchBasicNote] `archetype:"imported,name=first"`
	Second           sonolus.EntityRef[WatchBasicNote] `archetype:"imported,name=second"`
	TargetTime       float64                           `archetype:"data"`
	TargetScaledTime float64                           `archetype:"data"`
}

func (n *WatchSimLine) Preprocess() {
	n.TargetTime = watch.Time.BeatToTime(n.First.Get().Beat)
	n.TargetScaledTime = watch.Time.TimeToScaledTime(n.TargetTime)
}
func (n *WatchSimLine) SpawnTime() float64 {
	return n.TargetScaledTime - noteTravelTime()
}
func (n *WatchSimLine) DespawnTime() float64 {
	first, second := n.First.Get(), n.Second.Get()
	firstTime, secondTime := first.TargetScaledTime, second.TargetScaledTime
	if watch.Replay.IsReplay() {
		firstTime, secondTime = first.ReplayEndScaled, second.ReplayEndScaled
	}
	return math.Min(firstTime, secondTime)
}
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
	alpha := Config.SimLineAlpha * noteAlpha(y)
	if alpha > 0 {
		Skin.SimLine.Draw(simLineQuad(n.First.Get().Lane, n.Second.Get().Lane, y), layerZ(layerSimLine, math.Min(n.First.Get().Lane, n.Second.Get().Lane), y), alpha)
	}
}

type WatchHoldTickNote struct {
	WatchBasicNote  `archetype:"base"`
	watch.Archetype `archetype:"name=HoldTickNote,key=5"`
}

func (n *WatchHoldTickNote) Preprocess() {
	n.WatchBasicNote.Preprocess()
	n.Lane = n.holdHeadRef().Get().currentLane(n.TargetTime)
}

func spawnWatchLaneEffect(lane float64) {
	if Config.LaneEffects {
		Particles.Lane.Spawn(laneParticleQuad(Layout.Transform, lane), 0.2, false)
	}
}

func spawnWatchNoteParticles(lane float64, linear, circular sonolus.Effect) {
	linear.Spawn(noteLinearParticleQuad(Layout.Transform, lane), 0.5, false)
	circular.Spawn(noteCircularParticleQuad(Layout.Transform, lane), 0.5, false)
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
