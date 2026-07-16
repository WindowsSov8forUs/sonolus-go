//go:build tutorial

package main

import (
	"math"

	"github.com/WindowsSov8forUs/sonolus-go/v2/sonolus"
	"github.com/WindowsSov8forUs/sonolus-go/v2/sonolus/tutorial"
)

type TutorialSkin struct {
	sonolus.SkinResource
	Lane, JudgmentLine, Note, Flick, FlickArrow, RightFlick, RightFlickArrow, LeftFlick, LeftFlickArrow sonolus.Sprite
	HoldHead, HoldTail, HoldConnector, HoldTick                                                         sonolus.Sprite
	StageMiddle, LeftBorder, RightBorder, Slot, Cover                                                   sonolus.Sprite
}

var Skin = &TutorialSkin{
	SkinResource: sonolus.SkinResource{RenderMode: sonolus.RenderModeLightweight},
	Lane:         sonolus.SkinSprite(sonolus.StandardSpriteLane), JudgmentLine: sonolus.SkinSprite(sonolus.StandardSpriteJudgmentLine),
	Note: sonolus.SkinSprite(sonolus.StandardSpriteNoteHeadCyan), Flick: sonolus.SkinSprite(sonolus.StandardSpriteNoteHeadRed),
	FlickArrow: sonolus.SkinSprite(sonolus.StandardSpriteDirectionalMarkerRed),
	RightFlick: sonolus.SkinSprite(sonolus.StandardSpriteNoteHeadYellow), RightFlickArrow: sonolus.SkinSprite(sonolus.StandardSpriteDirectionalMarkerYellow),
	LeftFlick: sonolus.SkinSprite(sonolus.StandardSpriteNoteHeadPurple), LeftFlickArrow: sonolus.SkinSprite(sonolus.StandardSpriteDirectionalMarkerPurple),
	HoldHead: sonolus.SkinSprite(sonolus.StandardSpriteNoteHeadGreen), HoldTail: sonolus.SkinSprite(sonolus.StandardSpriteNoteTailGreen),
	HoldConnector: sonolus.SkinSprite(sonolus.StandardSpriteNoteConnectionGreenSeamless),
	HoldTick:      sonolus.SkinSprite(sonolus.StandardSpriteNoteTickGreen),
	StageMiddle:   sonolus.SkinSprite(sonolus.StandardSpriteStageMiddle), LeftBorder: sonolus.SkinSprite(sonolus.StandardSpriteStageLeftBorder),
	RightBorder: sonolus.SkinSprite(sonolus.StandardSpriteStageRightBorder), Slot: sonolus.SkinSprite(sonolus.StandardSpriteNoteSlot),
	Cover: sonolus.SkinSprite(sonolus.StandardSpriteStageCover),
}

type TutorialEffects struct {
	sonolus.EffectResource
	Stage, Perfect, Great, Good                           sonolus.Clip
	PerfectAlternative, GreatAlternative, GoodAlternative sonolus.Clip
	Hold                                                  sonolus.Clip
}

var Effects = &TutorialEffects{
	Stage: sonolus.EffectClip(sonolus.StandardClipStage), Perfect: sonolus.EffectClip(sonolus.StandardClipPerfect),
	Great: sonolus.EffectClip(sonolus.StandardClipGreat), Good: sonolus.EffectClip(sonolus.StandardClipGood),
	PerfectAlternative: sonolus.EffectClip(sonolus.StandardClipPerfectAlternative),
	GreatAlternative:   sonolus.EffectClip(sonolus.StandardClipGreatAlternative),
	GoodAlternative:    sonolus.EffectClip(sonolus.StandardClipGoodAlternative),
	Hold:               sonolus.EffectClip(sonolus.StandardClipHold),
}

type TutorialParticles struct {
	sonolus.ParticleResource
	Lane, TapLinear, Tap, HoldLinear, Hold, HoldActive sonolus.Effect
	FlickLinear, Flick                                 sonolus.Effect
	RightFlickLinear, RightFlick                       sonolus.Effect
	LeftFlickLinear, LeftFlick                         sonolus.Effect
}

var Particles = &TutorialParticles{
	Lane:      sonolus.ParticleEffect(sonolus.StandardEffectLaneLinear),
	TapLinear: sonolus.ParticleEffect(sonolus.StandardEffectNoteLinearTapCyan), Tap: sonolus.ParticleEffect(sonolus.StandardEffectNoteCircularTapCyan),
	HoldLinear: sonolus.ParticleEffect(sonolus.StandardEffectNoteLinearTapGreen), Hold: sonolus.ParticleEffect(sonolus.StandardEffectNoteCircularTapGreen),
	HoldActive:  sonolus.ParticleEffect(sonolus.StandardEffectNoteCircularHoldGreen),
	FlickLinear: sonolus.ParticleEffect(sonolus.StandardEffectNoteLinearAlternativeRed), Flick: sonolus.ParticleEffect(sonolus.StandardEffectNoteCircularAlternativeRed),
	RightFlickLinear: sonolus.ParticleEffect(sonolus.StandardEffectNoteLinearAlternativeYellow), RightFlick: sonolus.ParticleEffect(sonolus.StandardEffectNoteCircularAlternativeYellow),
	LeftFlickLinear: sonolus.ParticleEffect(sonolus.StandardEffectNoteLinearAlternativePurple), LeftFlick: sonolus.ParticleEffect(sonolus.StandardEffectNoteCircularAlternativePurple),
}

type TutorialInstructions struct {
	sonolus.InstructionResource
	Tap, TapFlick, TapHold, HoldFollow, Release, HoldFlick sonolus.Text
}

var Instructions = &TutorialInstructions{
	Tap:        sonolus.InstructionText("#TAP"),
	TapFlick:   sonolus.InstructionText("#TAP_FLICK"),
	TapHold:    sonolus.InstructionText("#TAP_HOLD"),
	HoldFollow: sonolus.InstructionText("#HOLD_FOLLOW"),
	Release:    sonolus.InstructionText("#RELEASE"),
	HoldFlick:  sonolus.InstructionText("#FLICK"),
}

type TutorialIcons struct {
	sonolus.InstructionIconResource
	Hand sonolus.Icon
}

var InstructionIcons = &TutorialIcons{Hand: sonolus.InstructionIcon("#HAND")}

type TutorialMemoryState struct {
	sonolus.LevelMemoryResource
	Transform    sonolus.Transform2D
	Phase        int
	PhaseStart   float64
	PreviousTime float64
	HoldParticle sonolus.ParticleHandle
	HoldLoop     sonolus.LoopedEffectHandle
	HoldActive   bool
	HoldAccessed bool
}

var TutorialState = TutorialMemoryState{}

type TutorialGlobals struct{ tutorial.GlobalCallbacks }

var Global TutorialGlobals

type tutorialNoteKind int

const (
	tutorialTap tutorialNoteKind = iota + 1
	tutorialFlick
	tutorialDirectionalFlick
	tutorialHoldHead
	tutorialHoldTick
	tutorialHoldEnd
)

var tutorialTickLanes = [7]float64{0, 0, 2, 0, -2, 0, 0}
var tutorialTickTargets = [7]float64{-999, 0.2, 0.4, 0.6, 0.8, 1, 999}

func Preprocess() {
	now := tutorial.Time.Now()
	TutorialState.Transform = stageTransform()
	tutorial.SkinTransform.Set(TutorialState.Transform)
	TutorialState.Phase = 0
	TutorialState.PhaseStart = now
	TutorialState.PreviousTime = now
	TutorialState.HoldParticle = sonolus.ParticleHandle{}
	TutorialState.HoldLoop = sonolus.LoopedEffectHandle{}
	TutorialState.HoldActive = false
	TutorialState.HoldAccessed = false
	screen := tutorial.Screen.Rect()
	tutorial.UI.SetMenu(basicMenuLayout(screen, tutorial.UI.MenuConfiguration()))
	navigationConfig := tutorial.UI.NavigationConfiguration()
	tutorial.UI.SetPrevious(tutorialPreviousLayout(screen, navigationConfig))
	tutorial.UI.SetNext(tutorialNextLayout(screen, navigationConfig))
	tutorial.UI.SetInstruction(tutorialInstructionLayout(tutorial.UI.InstructionConfiguration()))
}

func Navigate() {
	phase := TutorialState.Phase
	direction := tutorial.Navigation.Direction()
	if direction > 0 {
		phase += 1
	} else {
		phase -= 1
	}
	setTutorialPhase(phase)
}

func Update() {
	tutorialUpdateStart()
	drawTutorialStage()
	phase := TutorialState.Phase
	now, previous := currentTutorialPhaseTime()
	if runTutorialPhase(phase, now, previous) {
		setTutorialPhase(phase + 1)
	}
	tutorialUpdateEnd()
}

func setTutorialPhase(phase int) {
	if phase < 0 {
		phase = len(tutorialPhases) - 1
	} else if phase >= len(tutorialPhases) {
		phase = 0
	}
	TutorialState.Phase = phase
	TutorialState.PhaseStart = tutorial.Time.Now()
}

func tutorialUpdateStart() {
	TutorialState.HoldAccessed = false
	tutorial.Instruction.Clear()
}

func tutorialUpdateEnd() {
	if !TutorialState.HoldAccessed {
		stopTutorialHold()
	}
	TutorialState.PreviousTime = tutorial.Time.Now()
}

func currentTutorialPhaseTime() (float64, float64) {
	return tutorial.Time.Now() - TutorialState.PhaseStart, TutorialState.PreviousTime - TutorialState.PhaseStart
}

var tutorialPhases = [7]func(float64, float64) bool{
	runTutorialTap,
	runTutorialFlick,
	runTutorialDirectionalFlick,
	runTutorialHoldHead,
	runTutorialHoldTick,
	runTutorialHoldEnd,
	runTutorialHoldFlick,
}

func runTutorialPhase(phase int, now, previous float64) bool {
	return tutorialPhases[phase](now, previous)
}

func runTutorialTap(now, previous float64) bool {
	return runTutorialStandardPhase(0, now, previous)
}

func runTutorialFlick(now, previous float64) bool {
	return runTutorialStandardPhase(1, now, previous)
}

func runTutorialDirectionalFlick(now, previous float64) bool {
	return runTutorialStandardPhase(2, now, previous)
}

func runTutorialHoldHead(now, previous float64) bool {
	return runTutorialStandardPhase(3, now, previous)
}

func runTutorialHoldEnd(now, previous float64) bool {
	return runTutorialStandardPhase(5, now, previous)
}

func runTutorialHoldFlick(now, previous float64) bool {
	return runTutorialStandardPhase(6, now, previous)
}

func runTutorialStandardPhase(phase int, now, previous float64) bool {
	timeline := tutorialTimelineFor(phase)
	introEnd := timeline.IntroEnd
	fallEnd := timeline.FallEnd
	frozenEnd := timeline.FrozenEnd
	hit := timeline.Hit
	end := timeline.End
	if now < introEnd {
		drawTutorialPhaseIntro(phase)
	} else if now < fallEnd {
		drawTutorialPhaseFall(phase, tutorialProgressY(sonolus.Remap(introEnd, fallEnd, 0, 1, now)))
	} else if now < frozenEnd {
		progress := tutorialRepeatedProgress(now, fallEnd, frozenEnd, tutorialFrozenRepeats)
		drawTutorialPhaseFrozen(phase, progress, now < hit)
	} else if phase == 3 && now < end {
		updateTutorialHold(0, true)
		paintTutorialHold(tutorialLanePosition(0), 1)
		Skin.HoldConnector.Draw(holdConnectorQuad(0, 0, judgmentLineY, 99), layerZ(layerConnector, 0, judgmentLineY), Config.ConnectorAlpha)
		Skin.HoldHead.Draw(noteRect(0, judgmentLineY).ToQuad(), layerZ(layerNoteHead, 0, 0), 1)
		tutorial.Instruction.Show(Instructions.TapHold)
	}
	if tutorialCrossed(previous, now, hit) {
		playTutorialPhaseHit(phase)
	}
	return now >= end
}

func drawTutorialPhaseIntro(phase int) {
	if phase == 0 {
		drawTutorialIntroNote(tutorialTap, 0, 0, false)
	} else if phase == 1 {
		drawTutorialIntroNote(tutorialFlick, 0, 0, false)
	} else if phase == 2 {
		drawTutorialIntroNote(tutorialDirectionalFlick, -0.55, -1, false)
		drawTutorialIntroNote(tutorialDirectionalFlick, 0.55, 1, false)
	} else if phase == 3 {
		drawTutorialIntroNote(tutorialHoldHead, 0, 0, false)
	} else if phase == 5 {
		drawTutorialIntroNote(tutorialHoldEnd, 0, 0, false)
	} else {
		drawTutorialIntroNote(tutorialFlick, 0, 0, true)
	}
}

func drawTutorialPhaseFall(phase int, y float64) {
	if phase == 0 {
		drawTutorialNote(tutorialTap, 0, y, 0)
	} else if phase == 1 {
		drawTutorialNote(tutorialFlick, 0, y, 0)
	} else if phase == 2 {
		drawTutorialNote(tutorialDirectionalFlick, -1, y, -1)
		drawTutorialNote(tutorialDirectionalFlick, 1, y, 1)
	} else if phase == 3 {
		drawTutorialNote(tutorialHoldHead, 0, y, 0)
		Skin.HoldConnector.Draw(holdConnectorQuad(0, 0, y, 99), layerZ(layerConnector, 0, y), Config.ConnectorAlpha)
	} else {
		updateTutorialHold(0, true)
		kind := tutorialHoldEnd
		if phase == 6 {
			kind = tutorialFlick
		}
		drawTutorialNote(kind, 0, y, 0)
		Skin.HoldHead.Draw(noteRect(0, judgmentLineY).ToQuad(), layerZ(layerNoteHead, 0, 0), 1)
		Skin.HoldConnector.Draw(holdConnectorQuad(0, 0, judgmentLineY, y), layerZ(layerConnector, 0, math.Min(judgmentLineY, y)), Config.ConnectorAlpha)
		paintTutorialHold(tutorialLanePosition(0), 1)
	}
}

func drawTutorialPhaseFrozen(phase int, progress float64, beforeHit bool) {
	position := tutorialLanePosition(0)
	if phase == 0 {
		drawTutorialNote(tutorialTap, 0, judgmentLineY, 0)
		paintTutorialTap(position, progress, true)
		tutorial.Instruction.Show(Instructions.Tap)
	} else if phase == 1 {
		if beforeHit {
			drawTutorialNote(tutorialFlick, 0, judgmentLineY, 0)
		}
		paintTutorialTapFlick(position, math.Pi/2, progress, tutorialFrozenTapDuration, tutorialFrozenFlickDuration)
		tutorial.Instruction.Show(Instructions.TapFlick)
	} else if phase == 2 {
		if beforeHit {
			drawTutorialNote(tutorialDirectionalFlick, -1, judgmentLineY, -1)
			drawTutorialNote(tutorialDirectionalFlick, 1, judgmentLineY, 1)
		}
		paintTutorialTapFlick(tutorialLanePosition(-1), math.Pi, progress, tutorialFrozenTapDuration, tutorialFrozenFlickDuration)
		paintTutorialTapFlick(tutorialLanePosition(1), 0, progress, tutorialFrozenTapDuration, tutorialFrozenFlickDuration)
		tutorial.Instruction.Show(Instructions.TapFlick)
	} else if phase == 3 {
		drawTutorialNote(tutorialHoldHead, 0, judgmentLineY, 0)
		Skin.HoldConnector.Draw(holdConnectorQuad(0, 0, judgmentLineY, 99), layerZ(layerConnector, 0, judgmentLineY), Config.ConnectorAlpha)
		paintTutorialTap(position, progress, false)
		tutorial.Instruction.Show(Instructions.TapHold)
	} else if phase == 5 {
		drawTutorialNote(tutorialHoldEnd, 0, judgmentLineY, 0)
		paintTutorialRelease(position, progress)
		tutorial.Instruction.Show(Instructions.Release)
	} else {
		if beforeHit {
			drawTutorialNote(tutorialFlick, 0, judgmentLineY, 0)
		}
		paintTutorialHoldFlick(position, math.Pi/2, progress, tutorialFrozenHoldDuration, tutorialFrozenFlickDuration)
		tutorial.Instruction.Show(Instructions.HoldFlick)
	}
}

func playTutorialPhaseHit(phase int) {
	if phase == 0 {
		playTutorialHit(tutorialTap, 0, 0)
	} else if phase == 1 || phase == 6 {
		playTutorialHit(tutorialFlick, 0, 0)
	} else if phase == 2 {
		playTutorialHit(tutorialDirectionalFlick, -1, -1)
		playTutorialHit(tutorialDirectionalFlick, 1, 1)
	} else if phase == 3 {
		playTutorialHit(tutorialHoldHead, 0, 0)
	} else {
		playTutorialHit(tutorialHoldEnd, 0, 0)
	}
}

func runTutorialHoldTick(now, previous float64) bool {
	timeline := tutorialTimelineFor(4)
	introEnd := timeline.IntroEnd
	frozenEnd := timeline.FrozenEnd
	fallEnd := timeline.FallEnd
	end := timeline.End
	fallProgress := sonolus.Remap(frozenEnd, fallEnd, 0, 1, now)
	if now < introEnd {
		drawTutorialIntroNote(tutorialHoldTick, 0, 0, false)
	} else {
		holdLane := 0.0
		for i := 1; i < len(tutorialTickLanes); i++ {
			y := tutorialTickY(i, fallProgress)
			previousY := tutorialTickY(i-1, fallProgress)
			if y < judgmentLineY {
				continue
			}
			if previousY < judgmentLineY {
				holdLane = sonolus.Remap(previousY, y, tutorialTickLanes[i-1], tutorialTickLanes[i], judgmentLineY)
				break
			}
		}
		updateTutorialHold(holdLane, now >= frozenEnd)
		Skin.HoldHead.Draw(noteRect(holdLane, judgmentLineY).ToQuad(), layerZ(layerNoteHead, holdLane, 0), 1)
		paintTutorialHold(tutorialLanePosition(holdLane), 1)
		for i := 1; i < len(tutorialTickLanes); i++ {
			Skin.HoldConnector.Draw(holdConnectorQuad(tutorialTickLanes[i-1], tutorialTickLanes[i], tutorialTickY(i-1, fallProgress), tutorialTickY(i, fallProgress)), layerZ(layerConnector, math.Min(tutorialTickLanes[i-1], tutorialTickLanes[i]), math.Min(tutorialTickY(i-1, fallProgress), tutorialTickY(i, fallProgress))), Config.ConnectorAlpha)
		}
		for i := 0; i < len(tutorialTickLanes); i++ {
			lane := tutorialTickLanes[i]
			hit := frozenEnd + tutorialTickTargets[i]*tutorialFallDuration
			if now < hit {
				drawTutorialNote(tutorialHoldTick, lane, tutorialTickY(i, fallProgress), 0)
			}
			if tutorialCrossed(previous, now, hit) {
				playTutorialHit(tutorialHoldTick, lane, 0)
			}
		}
	}
	if introEnd <= now && now < frozenEnd {
		paintTutorialHold(tutorialLanePosition(tutorialTickLanes[0]), 1)
		tutorial.Instruction.Show(Instructions.HoldFollow)
	}
	return now >= end
}

func tutorialRepeatedProgress(now, start, end float64, repeats int) float64 {
	return math.Mod(sonolus.Clamp(sonolus.Remap(start, end, 0, 1, now), 0, 1)*float64(repeats), 1)
}

func tutorialTickY(index int, fallProgress float64) float64 {
	progress := sonolus.Remap(tutorialTickTargets[index]-1, tutorialTickTargets[index], 0, 1, math.Max(0, fallProgress))
	return tutorialProgressY(progress)
}

func tutorialProgressY(progress float64) float64 {
	return sonolus.Lerp(laneTopY(), judgmentLineY, progress)
}

func drawTutorialNote(kind tutorialNoteKind, lane, y, direction float64) {
	if kind == tutorialTap {
		drawNoteBody(Skin.Note, lane, y)
	} else if kind == tutorialFlick {
		drawFlickNote(Skin.Flick, Skin.FlickArrow, lane, y, tutorial.Time.Now())
	} else if kind == tutorialDirectionalFlick {
		if displayDirection(direction) > 0 {
			drawDirectionalFlickNote(Skin.RightFlick, Skin.RightFlickArrow, lane, y, direction, tutorial.Time.Now())
		} else {
			drawDirectionalFlickNote(Skin.LeftFlick, Skin.LeftFlickArrow, lane, y, direction, tutorial.Time.Now())
		}
	} else if kind == tutorialHoldHead {
		drawNoteBody(Skin.HoldHead, lane, y)
	} else if kind == tutorialHoldTick {
		drawNoteBody(Skin.HoldTick, lane, y)
	} else {
		drawNoteBody(Skin.HoldTail, lane, y)
	}
}

func tutorialLanePosition(lane float64) sonolus.Vec2 {
	return TutorialState.Transform.TransformVec(sonolus.NewVec2(laneX(lane), judgmentLineY))
}

func tutorialIntroQuad(quad sonolus.Quad) sonolus.Quad {
	center := TutorialState.Transform.TransformVec(sonolus.NewVec2(0, judgmentLineY+1))
	transform := func(point sonolus.Vec2) sonolus.Vec2 {
		screen := TutorialState.Transform.TransformVec(point)
		screen = screen.Sub(center).Mul(3)
		return stageUnprojectPoint(screen.X, screen.Y)
	}
	return sonolus.Quad{BL: transform(quad.BL), BR: transform(quad.BR), TL: transform(quad.TL), TR: transform(quad.TR)}
}

func drawTutorialIntroNote(kind tutorialNoteKind, lane, direction float64, holdFlick bool) {
	y := judgmentLineY + 1
	body := Skin.Note
	if kind == tutorialFlick {
		body = Skin.Flick
	} else if kind == tutorialDirectionalFlick && displayDirection(direction) > 0 {
		body = Skin.RightFlick
	} else if kind == tutorialDirectionalFlick {
		body = Skin.LeftFlick
	} else if kind == tutorialHoldHead {
		body = Skin.HoldHead
	} else if kind == tutorialHoldTick {
		body = Skin.HoldTick
	} else if kind == tutorialHoldEnd {
		body = Skin.HoldTail
	}
	body.Draw(tutorialIntroQuad(noteRect(lane, y).ToQuad()), layerZ(layerNote, 0, 0), 1)
	if kind == tutorialFlick {
		Skin.FlickArrow.Draw(tutorialIntroQuad(flickArrowQuad(lane, y, 0.5)), layerZ(layerArrow, 0, 0), 1)
	} else if kind == tutorialDirectionalFlick {
		arrow := Skin.RightFlickArrow
		if displayDirection(direction) < 0 {
			arrow = Skin.LeftFlickArrow
		}
		for number := range int(math.Abs(direction)) {
			arrow.Draw(tutorialIntroQuad(directionalFlickArrowQuad(lane, y, displayDirection(direction), number, 0.5)), layerZ(layerArrow, 0, 0), 1)
		}
	}
	if kind == tutorialHoldHead {
		Skin.HoldConnector.Draw(tutorialIntroQuad(holdConnectorQuad(lane, lane, y, y+1)), layerZ(layerConnector, 0, 0), 1)
	} else if kind == tutorialHoldEnd || holdFlick {
		Skin.HoldConnector.Draw(tutorialIntroQuad(holdConnectorQuad(lane, lane, y-1, y)), layerZ(layerConnector, 0, 0), 1)
	} else if kind == tutorialHoldTick {
		Skin.HoldConnector.Draw(tutorialIntroQuad(holdConnectorQuad(lane, lane, y-1, y+1)), layerZ(layerConnector, 0, 0), 1)
	}
}

func tutorialInstructionScale() float64 {
	return tutorial.UI.InstructionConfiguration().Scale
}

func tutorialInstructionAlpha() float64 {
	return tutorial.UI.InstructionConfiguration().Alpha
}

func tutorialPiecewise(start, end, from, to, value float64) float64 {
	return sonolus.LerpClamped(from, to, sonolus.Remap(start, end, 0, 1, value))
}

func paintTutorialTap(position sonolus.Vec2, progress float64, fade bool) {
	alpha := tutorialInstructionAlpha()
	if fade {
		if progress < 0.05 {
			alpha *= tutorialPiecewise(0, 0.05, 0, 1, progress)
		} else if progress > 0.75 {
			alpha *= tutorialPiecewise(0.75, 0.95, 1, 0, progress)
		}
	} else {
		alpha *= tutorialPiecewise(0, 0.25, 0, 1, progress)
	}
	tapProgress := tutorialPiecewise(0.25, 0.75, 0, 1, progress)
	paintTutorialTapPose(position, tapProgress, alpha)
}

func paintTutorialTapPose(position sonolus.Vec2, progress, alpha float64) {
	scale := tutorialInstructionScale()
	angle := sonolus.LerpClamped(math.Pi/6, math.Pi/3, progress)
	base := sonolus.NewVec2(0, -1).Rotate(math.Pi / 3).Mul(0.25 * scale).Add(position)
	hand := sonolus.NewVec2(0, 1).Rotate(angle).Mul(0.25 * scale).Add(base)
	tutorial.Instruction.Paint(InstructionIcons.Hand, hand, 0.25*scale, 180*angle/math.Pi, 0, alpha)
}

func paintTutorialRelease(position sonolus.Vec2, progress float64) {
	alpha := tutorialPiecewise(0.25, 0.75, 1, 0, progress) * tutorialInstructionAlpha()
	tapProgress := tutorialPiecewise(0.25, 0.75, 1, 0, progress)
	paintTutorialTapPose(position, tapProgress, alpha)
}

func paintTutorialHold(position sonolus.Vec2, alpha float64) {
	paintTutorialTapPose(position, 1, alpha*tutorialInstructionAlpha())
}

func paintTutorialFlick(position sonolus.Vec2, angle, progress float64) {
	destination := position.Add(sonolus.UnitVec2(angle).Mul(0.75))
	alpha := tutorialPiecewise(0.25, 0.75, 1, 0, progress)
	current := position.LerpClamped(destination, progress)
	paintTutorialHold(current, alpha)
}

func paintTutorialTapFlick(position sonolus.Vec2, angle, progress, tapDuration, flickDuration float64) {
	flickStart := tapDuration / (tapDuration + flickDuration)
	if progress < flickStart {
		paintTutorialTap(position, sonolus.Remap(0, flickStart, 0, 1, progress), false)
	} else {
		paintTutorialFlick(position, angle, sonolus.Remap(flickStart, 1, 0, 1, progress))
	}
}

func paintTutorialHoldFlick(position sonolus.Vec2, angle, progress, holdDuration, flickDuration float64) {
	flickStart := holdDuration / (holdDuration + flickDuration)
	if progress < flickStart {
		paintTutorialHold(position, 1)
	} else {
		paintTutorialFlick(position, angle, sonolus.Remap(flickStart, 1, 0, 1, progress))
	}
}

func playTutorialHit(kind tutorialNoteKind, lane, direction float64) {
	if Config.SFX {
		if kind == tutorialFlick || kind == tutorialDirectionalFlick {
			tutorial.Audio.Play(Effects.PerfectAlternative, 0.02)
		} else {
			tutorial.Audio.Play(Effects.Perfect, 0.02)
		}
	}
	if Config.NoteEffects {
		if kind == tutorialTap {
			spawnTutorialNoteParticles(lane, Particles.TapLinear, Particles.Tap)
		} else if kind == tutorialFlick {
			spawnTutorialNoteParticles(lane, Particles.FlickLinear, Particles.Flick)
		} else if kind == tutorialDirectionalFlick && displayDirection(direction) > 0 {
			spawnTutorialNoteParticles(lane, Particles.RightFlickLinear, Particles.RightFlick)
		} else if kind == tutorialDirectionalFlick {
			spawnTutorialNoteParticles(lane, Particles.LeftFlickLinear, Particles.LeftFlick)
		} else {
			spawnTutorialNoteParticles(lane, Particles.HoldLinear, Particles.Hold)
		}
	}
	if Config.LaneEffects {
		Particles.Lane.Spawn(laneParticleQuad(TutorialState.Transform, lane), 0.2, false)
	}
}

func updateTutorialHold(lane float64, sound bool) {
	TutorialState.HoldAccessed = true
	quad := noteCircularParticleQuad(TutorialState.Transform, lane)
	if !TutorialState.HoldActive {
		if Config.NoteEffects {
			particle := Particles.HoldActive.Spawn(quad, 1.5, true)
			TutorialState.HoldParticle = particle
		}
		TutorialState.HoldActive = true
	} else if Config.NoteEffects {
		TutorialState.HoldParticle.Move(quad)
	}
	if sound && Config.SFX && TutorialState.HoldLoop.ID == 0 {
		TutorialState.HoldLoop = Effects.Hold.PlayLooped()
	}
}

func spawnTutorialNoteParticles(lane float64, linear, circular sonolus.Effect) {
	linear.Spawn(noteLinearParticleQuad(TutorialState.Transform, lane), 0.5, false)
	circular.Spawn(noteCircularParticleQuad(TutorialState.Transform, lane), 0.5, false)
}

func stopTutorialHold() {
	if !TutorialState.HoldActive {
		return
	}
	if Config.NoteEffects && TutorialState.HoldParticle.ID != 0 {
		TutorialState.HoldParticle.Destroy()
	}
	if Config.SFX && TutorialState.HoldLoop.ID != 0 {
		TutorialState.HoldLoop.Stop()
	}
	TutorialState.HoldParticle = sonolus.ParticleHandle{}
	TutorialState.HoldLoop = sonolus.LoopedEffectHandle{}
	TutorialState.HoldActive = false
}

func drawTutorialStage() {
	for lane := -3; lane <= 3; lane++ {
		Skin.Lane.Draw(laneRect(float64(lane)).ToQuad(), layerLane, 1)
	}
	Skin.LeftBorder.Draw(leftBorderRect().ToQuad(), layerLane, 1)
	Skin.RightBorder.Draw(rightBorderRect().ToQuad(), layerLane, 1)
	Skin.JudgmentLine.Draw(judgmentRect().ToQuad(), layerJudgmentLine, 1)
}
