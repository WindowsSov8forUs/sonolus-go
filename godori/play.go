//go:build play

package main

import (
	"math"

	"github.com/WindowsSov8forUs/sonolus-go/v2/sonolus"
	"github.com/WindowsSov8forUs/sonolus-go/v2/sonolus/play"
)

type PlayReplayStreams struct {
	sonolus.StreamResource
	Reserved   [99999]sonolus.Stream[float64]
	EmptyLanes sonolus.Stream[[7]bool]
}

var Replay = PlayReplayStreams{}

type PlayInputMemory struct {
	sonolus.LevelMemoryResource
	ClaimedTouches sonolus.ArraySet[float64]
}

var InputMemory = PlayInputMemory{ClaimedTouches: sonolus.NewArraySet[float64](16)}

type PlayNoteMemory struct {
	sonolus.LevelMemoryResource
	ActiveNotes sonolus.VarArray[sonolus.EntityRef[PlayBasicNote]]
}

var NoteMemory = PlayNoteMemory{ActiveNotes: sonolus.NewVarArray[sonolus.EntityRef[PlayBasicNote]](16)}

type PlayLayoutData struct {
	sonolus.LevelDataResource
	Transform sonolus.Transform2D
}

var Layout = PlayLayoutData{}

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
	play.CallbackOrders `archetype:"updateSequential=-1,touch=2"`
}

func (*PlayStage) SpawnOrder() float64 { return -1e8 }
func (*PlayStage) ShouldSpawn() bool   { return true }
func (*PlayStage) Initialize() {
	sonolus.Assert(Config.LaneWidth > 0, "lane width must be positive")
}
func (*PlayStage) Preprocess() {
	Layout.Transform = stageTransform()
	play.SkinTransform.Set(Layout.Transform)
	play.Score.SetBase(play.BaseScore{Perfect: 1, Great: 0.5, Good: 0.25})
	window := noteBucketWindowMilliseconds()
	Buckets.Tap.SetWindow(window)
	Buckets.HoldHead.SetWindow(window)
	Buckets.HoldEnd.SetWindow(window)
	Buckets.HoldTick.SetWindow(window)
	Buckets.Flick.SetWindow(window)
	Buckets.DirectionalFlick.SetWindow(window)
	configurePlayNoteArchetype[PlayTapNote]()
	configurePlayNoteArchetype[PlayAccentTapNote]()
	configurePlayNoteArchetype[PlayFlickNote]()
	configurePlayNoteArchetype[PlayDirectionalFlickNote]()
	configurePlayNoteArchetype[PlayHoldHeadNote]()
	configurePlayNoteArchetype[PlayHoldAnchorNote]()
	configurePlayNoteArchetype[PlayHoldEndNote]()
	configurePlayNoteArchetype[PlayHoldFlickNote]()
	configurePlayNoteArchetype[PlayHoldTickNote]()
	screen := play.Screen.Rect()
	menuConfig := play.UI.MenuConfiguration()
	comboConfig := play.UI.ComboConfiguration()
	primaryConfig := play.UI.PrimaryMetricConfiguration()
	secondaryConfig := play.UI.SecondaryMetricConfiguration()
	play.UI.SetMenu(menuLayout(screen, menuConfig))
	play.UI.SetJudgment(judgmentLayout(play.UI.JudgmentConfiguration()))
	play.UI.SetComboValue(comboValueLayout(screen, comboConfig))
	play.UI.SetComboText(comboTextLayout(screen, comboConfig))
	play.UI.SetPrimaryMetricBar(primaryMetricBarLayout(screen, primaryConfig))
	play.UI.SetPrimaryMetricValue(primaryMetricValueLayout(screen, primaryConfig))
	play.UI.SetSecondaryMetricBar(secondaryMetricBarLayout(screen, secondaryConfig, menuConfig))
	play.UI.SetSecondaryMetricValue(secondaryMetricValueLayout(screen, secondaryConfig, menuConfig, primaryConfig))
}

func configurePlayNoteArchetype[T any]() {
	archetype := play.ArchetypeID[T]()
	missLife := -100.0
	if play.ArchetypeKey[T]() == 5 {
		missLife = -20
	}
	play.Score.SetArchetype(archetype, play.ArchetypeScore{Multiplier: 1})
	play.Life.SetArchetype(archetype, play.LifeValues{Perfect: 1, Miss: missLife})
}
func (*PlayStage) UpdateParallel() {
	drawPlayStage()
}

func (*PlayStage) UpdateSequential() {
	clearInputState()
}

func (*PlayStage) Touch() {
	effectLanes := [7]bool{}
	hasEffectLane := false
	for touch := range play.Touches.Values() {
		if !touch.Started || isTouchClaimed(touch.ID) {
			continue
		}
		lanes := sonolus.NewVarArray[int](7)
		for lane := range laneSequence {
			lanes.Append(lane)
		}
		for lane := range lanes.Values() {
			if !containsTransformedRect(Layout.Transform, laneRect(float64(lane)), touch.Position) {
				continue
			}
			if Config.SFX {
				play.Audio.Play(Effects.Stage, 0.02)
			}
			if Config.LaneEffects {
				Particles.Lane.Spawn(laneParticleQuad(Layout.Transform, float64(lane)), 0.2, false)
			}
			effectLanes[lane+3] = true
			hasEffectLane = true
		}
	}
	if hasEffectLane {
		Replay.EmptyLanes.Set(play.Time.Now(), effectLanes)
	}
}

func drawPlayStage() {
	for lane := -3; lane <= 3; lane++ {
		Skin.Lane.Draw(laneRect(float64(lane)).ToQuad(), layerLane, 1)
	}
	Skin.LeftBorder.Draw(leftBorderRect().ToQuad(), layerLane, 1)
	Skin.RightBorder.Draw(rightBorderRect().ToQuad(), layerLane, 1)
	Skin.JudgmentLine.Draw(judgmentRect().ToQuad(), layerJudgmentLine, 1)
}

type PlayBasicNote struct {
	play.Archetype   `archetype:"abstract"`
	Beat             float64                          `archetype:"imported,name=#BEAT"`
	Lane             float64                          `archetype:"imported,name=lane"`
	Direction        float64                          `archetype:"imported,name=direction"`
	Previous         sonolus.EntityRef[PlayBasicNote] `archetype:"imported,name=prev"`
	Next             sonolus.EntityRef[PlayBasicNote] `archetype:"imported,name=next"`
	ReplayEndTime    float64                          `archetype:"exported,name=end_time"`
	TargetTime       float64                          `archetype:"data"`
	TargetScaledTime float64                          `archetype:"data"`
	EndTime          float64                          `archetype:"data"`
	EndScaledTime    float64                          `archetype:"data"`
	BestJudgmentTime float64                          `archetype:"memory"`
	Resolved         bool                             `archetype:"memory"`
	TouchID          float64                          `archetype:"shared"`
	HoldLane         float64                          `archetype:"shared"`
	Judged           bool                             `archetype:"shared"`
}

func (n *PlayBasicNote) SpawnOrder() float64 { return n.TargetScaledTime - noteTravelTime() }
func (n *PlayBasicNote) ShouldSpawn() bool {
	return play.Time.Scaled() >= n.TargetScaledTime-noteTravelTime()
}

func (n *PlayBasicNote) holdHeadRef() sonolus.EntityRef[PlayHoldHeadNote] {
	return playHoldHeadRef(play.CurrentEntityRef[PlayBasicNote]())
}

func playHoldHeadRef(ref sonolus.EntityRef[PlayBasicNote]) sonolus.EntityRef[PlayHoldHeadNote] {
	for previous := ref.Get().Previous; previous.Index > 0; previous = previous.Get().Previous {
		ref = previous
	}
	return sonolus.EntityRefAs[PlayHoldHeadNote](ref)
}

func (n *PlayBasicNote) Preprocess() {
	if Config.Mirror {
		n.Lane = -n.Lane
		n.Direction = -n.Direction
	}
	n.TargetTime = play.Time.BeatToTime(n.Beat)
	n.TargetScaledTime = play.Time.TimeToScaledTime(n.TargetTime)
	n.EndTime = play.Time.BeatToTime(n.endBeat())
	n.EndScaledTime = play.Time.TimeToScaledTime(n.EndTime)
	key := int(play.Entity.Key())
	if key == 6 {
		return
	}
	if key == 2 || key == 3 || key == 5 {
		n.BestJudgmentTime = unsetJudgmentTime
	}
	schedulePlaySFX(n.TargetTime, key == 2 || key == 3)
	result := play.Entity.Result()
	result.Accuracy = 1
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
	play.Entity.SetResult(result)
}

func (n *PlayBasicNote) Initialize() {
	if int(play.Entity.Key()) == 4 {
		play.Spawn(PlayHoldManager{Head: sonolus.EntityRefAs[PlayHoldHeadNote](play.CurrentEntityRef[PlayBasicNote]())})
	}
}

func (n *PlayBasicNote) UpdateSequential() {
	switch int(play.Entity.Key()) {
	case 1:
		n.updateTap()
	case 2:
		if n.Previous.Index > 0 {
			n.updateHoldFlick()
		} else {
			n.updateFlick(0)
		}
	case 3:
		n.updateFlick(displayDirection(n.Direction))
	case 4:
		n.updateHoldHead()
	case 5:
		n.updateHoldTick()
	case 6:
		play.Entity.SetDespawn(true)
	case 7:
		n.updateHoldEnd()
	}
}

func (n *PlayBasicNote) Touch() {
	switch int(play.Entity.Key()) {
	case 1:
		n.touchTap()
	case 2:
		if n.Previous.Index > 0 {
			n.touchHoldFlick()
		} else {
			n.touchFlick(0)
		}
	case 3:
		n.touchFlick(displayDirection(n.Direction))
	case 4:
		n.touchHoldHead()
	case 5:
		n.touchHoldTick()
	case 7:
		n.touchHoldEnd()
	}
}

func (n *PlayBasicNote) Terminate() {
	n.ReplayEndTime = play.Time.Now()
}

func (n *PlayBasicNote) UpdateParallel() {
	y := noteY(n.TargetScaledTime, play.Time.Scaled())
	switch int(play.Entity.Key()) {
	case 1:
		drawNoteBody(Skin.Note, n.Lane, y)
	case 2:
		drawFlickNote(Skin.Flick, Skin.FlickArrow, n.Lane, y, play.Time.Now())
	case 3:
		if displayDirection(n.Direction) > 0 {
			drawDirectionalFlickNote(Skin.RightFlick, Skin.RightFlickArrow, n.Lane, y, n.Direction, play.Time.Now())
		} else {
			drawDirectionalFlickNote(Skin.LeftFlick, Skin.LeftFlickArrow, n.Lane, y, n.Direction, play.Time.Now())
		}
	case 4:
		lane := n.Lane
		if y < judgmentLineY {
			y = judgmentLineY
		}
		if n.hasActiveTouch() && play.Time.Scaled() >= n.TargetScaledTime {
			lane = n.HoldLane
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

type PlayTapNote struct {
	PlayBasicNote  `archetype:"base"`
	play.Archetype `archetype:"name=TapNote,hasInput=true,key=1"`
}

type PlayAccentTapNote struct {
	PlayTapNote    `archetype:"base"`
	play.Archetype `archetype:"name=AccentTapNote"`
}

func (n *PlayBasicNote) updateTap() {
	if play.Time.Now() > n.TargetTime+0.15+play.Input.Offset() {
		failPlayHold(Buckets.Tap)
		return
	}
	registerActiveNote(0)
}

func (n *PlayBasicNote) touchTap() {
	if !withinJudgmentWindow(play.Time.Now()-play.Input.Offset(), n.TargetTime) {
		return
	}
	for touch := range play.Touches.Values() {
		if !touch.Started || !containsTransformedRect(Layout.Transform, simultaneousHitbox(n.TargetTime, n.Lane, 0), touch.Position) {
			continue
		}
		if isTouchClaimed(touch.ID) {
			continue
		}
		claimTouch(touch.ID)
		finishPlayNote(n.Lane, n.TargetTime, touch.StartTime, Buckets.Tap, Particles.TapLinear, Particles.Tap, false)
		return
	}
}

type PlayFlickNote struct {
	PlayBasicNote  `archetype:"base"`
	play.Archetype `archetype:"name=FlickNote,hasInput=true,key=2"`
}

func (n *PlayBasicNote) updateFlick(direction float64) {
	offsetAdjustedTime := play.Time.Now() - play.Input.Offset()
	hitTime, ready := holdTickResolution(n.BestJudgmentTime, n.TargetTime, offsetAdjustedTime)
	if ready {
		if hitTime > unsetJudgmentTime {
			n.resolveFlick(direction, hitTime)
		} else if direction == 0 {
			failPlayHold(Buckets.Flick)
		} else {
			failPlayHold(Buckets.DirectionalFlick)
		}
		return
	}
	activeTouchID := 0.0
	if n.hasActiveTouch() {
		activeTouchID = n.TouchID
	}
	registerActiveNote(activeTouchID)
}

func (n *PlayBasicNote) touchFlick(direction float64) {
	offsetAdjustedTime := play.Time.Now() - play.Input.Offset()
	if !withinJudgmentWindow(offsetAdjustedTime, n.TargetTime) {
		return
	}
	if !n.hasActiveTouch() {
		for touch := range play.Touches.Values() {
			if touch.Started && !isTouchClaimed(touch.ID) && containsTransformedRect(Layout.Transform, simultaneousHitbox(n.TargetTime, n.Lane, direction), touch.Position) {
				claimTouch(touch.ID)
				n.TouchID = touch.ID
				break
			}
		}
	}
	if !n.hasActiveTouch() {
		return
	}
	for touch := range play.Touches.Values() {
		if touch.ID != n.TouchID {
			continue
		}
		hitTime := offsetAdjustedTime
		meetsSpeed := direction == 0 && touch.Speed >= flickSpeedThreshold*laneWidth() || direction != 0 && touch.Speed >= directionalFlickSpeedThreshold(n.Direction)
		meetsDirection := direction == 0 || touch.Velocity.X*direction > 0
		if containsTransformedRect(Layout.Transform, simultaneousHitbox(n.TargetTime, n.Lane, direction), touch.Position) && meetsSpeed && meetsDirection {
			if n.BestJudgmentTime <= unsetJudgmentTime || math.Abs(hitTime-n.TargetTime) < math.Abs(n.BestJudgmentTime-n.TargetTime) {
				n.BestJudgmentTime = hitTime
			}
		}
		if touch.Ended || n.BestJudgmentTime >= n.TargetTime {
			n.resolveFlick(direction, n.BestJudgmentTime)
		}
		return
	}
}

func (n *PlayBasicNote) resolveFlick(direction, hitTime float64) {
	if direction == 0 {
		finishPlayNote(n.Lane, n.TargetTime, hitTime, Buckets.Flick, Particles.FlickLinear, Particles.Flick, true)
	} else if direction > 0 {
		finishPlayNote(n.Lane, n.TargetTime, hitTime, Buckets.DirectionalFlick, Particles.RightFlickLinear, Particles.RightFlick, true)
	} else {
		finishPlayNote(n.Lane, n.TargetTime, hitTime, Buckets.DirectionalFlick, Particles.LeftFlickLinear, Particles.LeftFlick, true)
	}
}

type PlayDirectionalFlickNote struct {
	PlayBasicNote  `archetype:"base"`
	play.Archetype `archetype:"name=DirectionalFlickNote,hasInput=true,key=3"`
}

type PlayHoldHeadNote struct {
	PlayBasicNote  `archetype:"base"`
	play.Archetype `archetype:"name=HoldHeadNote,hasInput=true,key=4"`
}

func (n *PlayBasicNote) updateHoldHead() {
	now := play.Time.Now() - play.Input.Offset()
	if !n.Judged && now > n.TargetTime+0.15 {
		n.Judged = true
		failPlayHold(Buckets.HoldHead)
		return
	}
	if !n.Judged {
		activeTouchID := 0.0
		if n.hasActiveTouch() {
			activeTouchID = n.TouchID
		}
		registerActiveNote(activeTouchID)
	}
}

func (n *PlayBasicNote) touchHoldHead() {
	if n.Judged {
		return
	}
	if !withinJudgmentWindow(play.Time.Now()-play.Input.Offset(), n.TargetTime) {
		return
	}
	for touch := range play.Touches.Values() {
		if !touch.Started || isTouchClaimed(touch.ID) || !containsTransformedRect(Layout.Transform, simultaneousHitbox(n.TargetTime, n.Lane, 0), touch.Position) {
			continue
		}
		claimTouch(touch.ID)
		n.TouchID = touch.ID
		n.Judged = true
		Replay.Reserved[int(play.CurrentEntityRef[PlayBasicNote]().Index)].Set(play.Time.Now(), 1)
		finishPlayNote(n.Lane, n.TargetTime, touch.StartTime, Buckets.HoldHead, Particles.HoldLinear, Particles.Hold, false)
		return
	}
}

func (n *PlayBasicNote) currentLane(now float64) float64 {
	previousTime, previousLane := n.TargetTime, n.Lane
	nextRef := n.Next
	for nextRef.Index > 0 {
		next := nextRef.Get()
		if nextRef.Key() == 6 || next.Next.Index <= 0 {
			nextTime := play.Time.BeatToTime(next.Beat)
			if now <= nextTime || next.Next.Index <= 0 {
				return holdLane(now, previousTime, nextTime, previousLane, next.Lane)
			}
			previousTime, previousLane = nextTime, next.Lane
		}
		nextRef = next.Next
	}
	return previousLane
}

func (n *PlayBasicNote) hasActiveTouch() bool { return n.TouchID > 0 }

func (n *PlayBasicNote) currentScaledLane(now float64) float64 {
	previousTime, previousLane := n.TargetScaledTime, n.Lane
	nextRef := n.Next
	for nextRef.Index > 0 {
		next := nextRef.Get()
		if nextRef.Key() == 6 || next.Next.Index <= 0 {
			nextTime := play.Time.TimeToScaledTime(play.Time.BeatToTime(next.Beat))
			if now <= nextTime || next.Next.Index <= 0 {
				return holdLane(now, previousTime, nextTime, previousLane, next.Lane)
			}
			previousTime, previousLane = nextTime, next.Lane
		}
		nextRef = next.Next
	}
	return previousLane
}

func (n *PlayBasicNote) endBeat() float64 {
	beat := n.Beat
	for nextRef := n.Next; nextRef.Index > 0; {
		next := nextRef.Get()
		beat, nextRef = next.Beat, next.Next
	}
	return beat
}

func (n *PlayBasicNote) endLane() float64 {
	lane := n.Lane
	for nextRef := n.Next; nextRef.Index > 0; {
		next := nextRef.Get()
		lane, nextRef = next.Lane, next.Next
	}
	return lane
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
	if play.Time.Now() > head.EndTime {
		n.stopEffects()
		play.Entity.SetDespawn(true)
		return
	}
	if play.Time.Now() < head.TargetTime {
		return
	}
	if !head.hasActiveTouch() {
		n.stopEffects()
		return
	}
	lane := head.HoldLane
	Skin.HoldHead.Draw(noteRect(lane, judgmentLineY).ToQuad(), layerZ(layerNoteHead, lane, 0), 1)
	n.updateEffects(lane)
}

func (n *PlayHoldManager) Terminate() { n.stopEffects() }

func (n *PlayHoldManager) Touch() {
	head := n.Head.Get()
	if !head.hasActiveTouch() {
		return
	}
	for touch := range play.Touches.Values() {
		if touch.ID != head.TouchID {
			continue
		}
		if touch.Ended {
			head.TouchID = 0
			Replay.Reserved[int(n.Head.Index)].Set(play.Time.Now(), 0)
		}
		return
	}
}

func (n *PlayHoldManager) updateEffects(lane float64) {
	quad := noteCircularParticleQuad(Layout.Transform, lane)
	if !n.EffectsActive {
		n.EffectsActive = true
		if Config.SFX {
			n.LoopID = Effects.Hold.PlayLooped().ID
		}
		if Config.NoteEffects {
			n.ParticleID = Particles.HoldActive.Spawn(quad, 1.5, true).ID
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
	PlayBasicNote  `archetype:"base"`
	play.Archetype `archetype:"name=HoldAnchorNote,key=6"`
}

type PlayHoldEndNote struct {
	PlayBasicNote  `archetype:"base"`
	play.Archetype `archetype:"name=HoldEndNote,hasInput=true,key=7"`
}

func (n *PlayBasicNote) updateHoldEnd() {
	if n.Resolved {
		return
	}
	headRef := n.holdHeadRef()
	head := headRef.Get()
	if play.Time.Now()-play.Input.Offset() > n.TargetTime+0.15 {
		n.Resolved = true
		head.TouchID = 0
		Replay.Reserved[int(headRef.Index)].Set(play.Time.Now(), 0)
		failPlayHold(Buckets.HoldEnd)
		return
	}
	activeTouchID := 0.0
	if head.hasActiveTouch() {
		activeTouchID = head.TouchID
	}
	registerActiveNote(activeTouchID)
}
func (n *PlayBasicNote) touchHoldEnd() {
	if n.Resolved {
		return
	}
	hitTime := play.Time.Now() - play.Input.Offset()
	if !withinJudgmentWindow(hitTime, n.TargetTime) {
		return
	}
	headRef := n.holdHeadRef()
	head := headRef.Get()
	quad := simultaneousHitbox(n.TargetTime, n.Lane, 0)
	n.captureHoldTouchIfNeeded(headRef, quad)
	if !head.hasActiveTouch() {
		return
	}
	for touch := range play.Touches.Values() {
		if touch.ID != head.TouchID || !touch.Ended {
			continue
		}
		n.Resolved = true
		head.TouchID = 0
		Replay.Reserved[int(headRef.Index)].Set(play.Time.Now(), 0)
		if containsTransformedRect(Layout.Transform, quad, touch.Position) {
			finishPlayNote(n.Lane, n.TargetTime, hitTime, Buckets.HoldEnd, Particles.HoldLinear, Particles.Hold, false)
		} else {
			failPlayHold(Buckets.HoldEnd)
		}
		return
	}
}

type PlayHoldFlickNote struct {
	PlayBasicNote  `archetype:"base"`
	play.Archetype `archetype:"name=HoldFlickNote,hasInput=true,key=2"`
}

func (n *PlayBasicNote) updateHoldFlick() {
	if n.Resolved {
		return
	}
	hitTime, ready := holdTickResolution(n.BestJudgmentTime, n.TargetTime, play.Time.Now()-play.Input.Offset())
	if ready {
		n.resolveHoldFlick(hitTime)
		return
	}
	head := n.holdHeadRef().Get()
	activeTouchID := 0.0
	if head.hasActiveTouch() {
		activeTouchID = head.TouchID
	}
	registerActiveNote(activeTouchID)
}

func (n *PlayBasicNote) touchHoldFlick() {
	if n.Resolved {
		return
	}
	hitTime := play.Time.Now() - play.Input.Offset()
	if !withinJudgmentWindow(hitTime, n.TargetTime) {
		return
	}
	head := n.holdHeadRef().Get()
	headRef := n.holdHeadRef()
	n.captureHoldTouchIfNeeded(headRef, simultaneousHitbox(n.TargetTime, n.Lane, 0))
	if !head.hasActiveTouch() {
		return
	}
	for touch := range play.Touches.Values() {
		if touch.ID != head.TouchID {
			continue
		}
		if touch.Ended {
			n.resolveHoldFlick(n.BestJudgmentTime)
			return
		}
		if touch.Speed < flickSpeedThreshold*laneWidth() || !containsTransformedRect(Layout.Transform, simultaneousHitbox(n.TargetTime, n.Lane, 0), touch.Position) {
			return
		}
		if n.BestJudgmentTime <= unsetJudgmentTime || math.Abs(hitTime-n.TargetTime) < math.Abs(n.BestJudgmentTime-n.TargetTime) {
			n.BestJudgmentTime = hitTime
		}
		if n.BestJudgmentTime >= n.TargetTime {
			n.resolveHoldFlick(n.BestJudgmentTime)
		}
		return
	}
}

func (n *PlayBasicNote) resolveHoldFlick(hitTime float64) {
	n.Resolved = true
	headRef := n.holdHeadRef()
	head := headRef.Get()
	head.TouchID = 0
	Replay.Reserved[int(headRef.Index)].Set(play.Time.Now(), 0)
	if hitTime > unsetJudgmentTime {
		finishPlayNote(n.Lane, n.TargetTime, hitTime, Buckets.Flick, Particles.FlickLinear, Particles.Flick, true)
	} else {
		failPlayHold(Buckets.Flick)
	}
}

type PlayHoldConnector struct {
	play.Archetype   `archetype:"name=HoldConnector"`
	First            sonolus.EntityRef[PlayBasicNote] `archetype:"imported,name=first"`
	Second           sonolus.EntityRef[PlayBasicNote] `archetype:"imported,name=second"`
	TargetScaledTime float64                          `archetype:"data"`
	EndScaledTime    float64                          `archetype:"data"`
	FirstLane        float64                          `archetype:"data"`
	SecondLane       float64                          `archetype:"data"`
}

func (n *PlayHoldConnector) Preprocess() {
	first, second := n.First.Get(), n.Second.Get()
	n.FirstLane, n.SecondLane = first.Lane, second.Lane
	targetTime := play.Time.BeatToTime(first.Beat)
	endTime := play.Time.BeatToTime(second.Beat)
	n.TargetScaledTime = play.Time.TimeToScaledTime(targetTime)
	n.EndScaledTime = play.Time.TimeToScaledTime(endTime)
}
func (n *PlayHoldConnector) SpawnOrder() float64 { return n.TargetScaledTime - noteTravelTime() }
func (n *PlayHoldConnector) ShouldSpawn() bool {
	return play.Time.Scaled() >= n.TargetScaledTime-noteTravelTime()
}
func (n *PlayHoldConnector) UpdateSequential() {
	first, second := n.First.Get(), n.Second.Get()
	now := play.Time.Now()
	if now >= second.TargetTime {
		play.Entity.SetDespawn(true)
		return
	}
	if now < first.TargetTime {
		return
	}
	head := playHoldHeadRef(n.First).Get()
	head.HoldLane = sonolus.Remap(
		noteY(first.TargetScaledTime, play.Time.Scaled()),
		noteY(second.TargetScaledTime, play.Time.Scaled()),
		first.Lane,
		second.Lane,
		judgmentLineY,
	)
}
func (n *PlayHoldConnector) UpdateParallel() {
	first, second := n.First.Get(), n.Second.Get()
	firstY := noteY(first.TargetScaledTime, play.Time.Scaled())
	secondY := noteY(second.TargetScaledTime, play.Time.Scaled())
	Skin.HoldConnector.Draw(holdConnectorQuad(first.Lane, second.Lane, firstY, secondY), layerZ(layerConnector, math.Min(first.Lane, second.Lane), math.Min(firstY, secondY)), Config.ConnectorAlpha)
}

type PlaySimLine struct {
	play.Archetype   `archetype:"name=SimLine"`
	First            sonolus.EntityRef[PlayBasicNote] `archetype:"imported,name=first"`
	Second           sonolus.EntityRef[PlayBasicNote] `archetype:"imported,name=second"`
	TargetTime       float64                          `archetype:"data"`
	TargetScaledTime float64                          `archetype:"data"`
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
	alpha := Config.SimLineAlpha * noteAlpha(y)
	if alpha > 0 {
		Skin.SimLine.Draw(simLineQuad(n.First.Get().Lane, n.Second.Get().Lane, y), layerZ(layerSimLine, math.Min(n.First.Get().Lane, n.Second.Get().Lane), y), alpha)
	}
}

type PlayHoldTickNote struct {
	PlayBasicNote  `archetype:"base"`
	play.Archetype `archetype:"name=HoldTickNote,hasInput=true,key=5"`
}

func (n *PlayHoldTickNote) Preprocess() {
	n.PlayBasicNote.Preprocess()
	n.Lane = n.holdHeadRef().Get().currentLane(n.TargetTime)
}

func (n *PlayBasicNote) updateHoldTick() {
	if n.Resolved {
		return
	}
	offsetAdjustedTime := play.Time.Now() - play.Input.Offset()
	hitTime, ready := holdTickResolution(n.BestJudgmentTime, n.TargetTime, offsetAdjustedTime)
	if ready {
		n.resolveHoldTick(hitTime)
		return
	}
	head := n.holdHeadRef().Get()
	activeTouchID := 0.0
	if head.hasActiveTouch() {
		activeTouchID = head.TouchID
	}
	registerActiveNote(activeTouchID)
}

func (n *PlayBasicNote) touchHoldTick() {
	if n.Resolved {
		return
	}
	offsetAdjustedTime := play.Time.Now() - play.Input.Offset()
	if !withinJudgmentWindow(offsetAdjustedTime, n.TargetTime) {
		return
	}
	headRef := n.holdHeadRef()
	head := headRef.Get()
	n.captureHoldTouchIfNeeded(headRef, simultaneousHitbox(n.TargetTime, n.Lane, 0))
	if !head.hasActiveTouch() {
		return
	}
	for touch := range play.Touches.Values() {
		if touch.ID != head.TouchID || touch.Ended || !containsTransformedRect(Layout.Transform, simultaneousHitbox(n.TargetTime, n.Lane, 0), touch.Position) {
			continue
		}
		if n.BestJudgmentTime <= unsetJudgmentTime || math.Abs(offsetAdjustedTime-n.TargetTime) < math.Abs(n.BestJudgmentTime-n.TargetTime) {
			n.BestJudgmentTime = offsetAdjustedTime
		}
		if n.BestJudgmentTime >= n.TargetTime {
			n.resolveHoldTick(n.BestJudgmentTime)
		}
		return
	}
}

func (n *PlayBasicNote) captureHoldTouchIfNeeded(headRef sonolus.EntityRef[PlayHoldHeadNote], quad sonolus.Rect) {
	head := headRef.Get()
	if head.hasActiveTouch() || !head.Judged {
		return
	}
	for touch := range play.Touches.Values() {
		if touch.Ended || isTouchClaimed(touch.ID) || !containsTransformedRect(Layout.Transform, quad, touch.Position) {
			continue
		}
		claimTouch(touch.ID)
		head.TouchID = touch.ID
		Replay.Reserved[int(headRef.Index)].Set(play.Time.Now(), 1)
		return
	}
}

func (n *PlayBasicNote) resolveHoldTick(hitTime float64) {
	n.Resolved = true
	result := play.Entity.Result()
	result.Bucket = Buckets.HoldTick
	if hitTime > unsetJudgmentTime {
		judgment := play.Input.Judge(hitTime, n.TargetTime, noteJudgmentWindows)
		result.Judgment = judgment
		result.Accuracy = hitTime - n.TargetTime
		result.BucketValue = result.Accuracy * 1000
		if Config.SFX && !Config.AutoSFX {
			playJudgmentSFX(judgment, false)
		}
		if Config.NoteEffects && judgment != sonolus.JudgmentMiss {
			spawnPlayNoteParticles(n.Lane, Particles.HoldLinear, Particles.Hold)
		}
		if Config.LaneEffects && judgment != sonolus.JudgmentMiss {
			Particles.Lane.Spawn(laneParticleQuad(Layout.Transform, n.Lane), 0.2, false)
		}
	} else {
		result.Judgment = sonolus.JudgmentMiss
		result.Accuracy = 1
		result.BucketValue = 1000
	}
	play.Entity.SetResult(result)
	play.Entity.SetDespawn(true)
}
func failPlayHold(bucket sonolus.Bucket) {
	result := play.Entity.Result()
	result.Judgment = sonolus.JudgmentMiss
	result.Accuracy = 1
	result.Bucket = bucket
	result.BucketValue = 1000
	play.Entity.SetResult(result)
	play.Entity.SetDespawn(true)
}

func finishPlayNote(lane, targetTime, hitTime float64, bucket sonolus.Bucket, linear, circular sonolus.Effect, alternative bool) {
	accuracy := sonolus.Clamp(hitTime-targetTime, -1, 1)
	judgment := play.Input.Judge(hitTime, targetTime, noteJudgmentWindows)
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
			spawnPlayNoteParticles(lane, linear, circular)
		}
	}
	if Config.LaneEffects && judgment != sonolus.JudgmentMiss {
		Particles.Lane.Spawn(laneParticleQuad(Layout.Transform, lane), 0.2, false)
	}
	play.Entity.SetDespawn(true)
}

func spawnPlayNoteParticles(lane float64, linear, circular sonolus.Effect) {
	linear.Spawn(noteLinearParticleQuad(Layout.Transform, lane), 0.5, false)
	circular.Spawn(noteCircularParticleQuad(Layout.Transform, lane), 0.5, false)
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

func clearInputState() {
	InputMemory.ClaimedTouches.Clear()
	NoteMemory.ActiveNotes.Clear()
}

func isTouchClaimed(id float64) bool {
	return InputMemory.ClaimedTouches.Contains(id)
}

func claimTouch(id float64) {
	if isTouchClaimed(id) || InputMemory.ClaimedTouches.IsFull() {
		return
	}
	InputMemory.ClaimedTouches.Add(id)
}

func registerActiveNote(activeTouchID float64) {
	if activeTouchID > 0 {
		claimTouch(activeTouchID)
	}
	ref := play.CurrentEntityRef[PlayBasicNote]()
	note := ref.Get()
	now := play.Time.Now() - play.Input.Offset()
	if !withinJudgmentWindow(now, note.TargetTime) {
		return
	}
	if NoteMemory.ActiveNotes.IsFull() {
		return
	}
	NoteMemory.ActiveNotes.Append(ref)
}

func simultaneousHitbox(targetTime, lane, direction float64) sonolus.Rect {
	entityIndex := play.CurrentEntityRef[PlayBasicNote]().Index
	box := noteHitbox(lane, direction)
	leftOverlap := 0.0
	rightOverlap := 0.0
	for i := range NoteMemory.ActiveNotes.Len() {
		stateRef := NoteMemory.ActiveNotes.Get(i)
		if stateRef.Index != entityIndex {
			continue
		}
		for j := 0; j < NoteMemory.ActiveNotes.Len(); j++ {
			otherRef := NoteMemory.ActiveNotes.Get(j)
			if j == i {
				continue
			}
			other := otherRef.Get()
			otherTouchID := other.TouchID
			if other.Previous.Index > 0 {
				otherTouchID = playHoldHeadRef(otherRef).Get().TouchID
			}
			if other.Judged || otherTouchID > 0 || math.Abs(other.TargetTime-targetTime) > 0.005 {
				continue
			}
			otherDirection := 0.0
			if otherRef.Key() == 3 {
				otherDirection = displayDirection(other.Direction)
			}
			otherBox := noteHitbox(other.Lane, otherDirection)
			left, right := hitboxOverlap(lane, other.Lane, box, otherBox)
			if left > leftOverlap {
				leftOverlap = left
			}
			if right > rightOverlap {
				rightOverlap = right
			}
		}
		return sonolus.Rect{T: laneTopY(), R: box.R - rightOverlap/2, B: -1, L: box.L + leftOverlap/2}
	}
	return box
}
