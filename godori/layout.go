package main

import (
	"math"

	"github.com/WindowsSov8forUs/sonolus-go/v2/sonolus"
)

func laneSequence(yield func(int) bool) {
	for lane := -3; lane <= 3; lane++ {
		if !yield(lane) {
			return
		}
	}
}

const (
	stageLeft                     = -3.5
	stageRight                    = 3.5
	judgmentLineY                 = 0.0
	judgmentLineScreenY           = -0.5
	vanishingPointY               = 1.35
	baseLaneWidth                 = 0.35
	baseLaneLength                = 8.0
	referenceNoteSpeed            = 6.0
	flickSpeedThreshold           = 6.0
	flickFadeInTime               = 0.1
	flickFadeOutTime              = 0.1
	flickArrowPeriod              = 0.3
	directionalFlickArrowPeriod   = 0.3
	flickArrowInitialOffset       = -0.1
	flickArrowFinalOffset         = 0.4
	directionalFlickArrowOffset   = 0.4
	directionalFlickArrowScale    = 1.5
	previewColumnSeconds          = 2.0
	previewColumnWidth            = 1.004
	previewYMin                   = -0.9
	previewYMax                   = 0.9
	previewLaneWidth              = 0.072
	previewStageBorderWidth       = 0.018
	previewBarLineHeight          = 0.0036
	previewBarExtendWidth         = 3 * previewLaneWidth
	previewTextMarginX            = 0.015
	previewTextWidth              = 0.22
	previewTextHeight             = 0.12
	unsetJudgmentTime             = -1e8
	judgmentWindowGood            = 0.15
	holdPerfectGracePeriod        = 1.0 / 30
	noteFadeLength                = 0.5
	tutorialIntroDuration         = 1.5
	tutorialFallDuration          = 1.5
	tutorialFrozenRepeats         = 4
	tutorialFrozenTapDuration     = 1.0
	tutorialFrozenHoldDuration    = 1.0
	tutorialFrozenFlickDuration   = 0.5
	tutorialFrozenReleaseDuration = 1.0
	tutorialEndDuration           = 1.5
	layerStage                    = 0.0
	layerLane                     = 1.0
	layerJudgmentLine             = 2.0
	layerConnector                = 10.0
	layerPreviewCover             = 20.0
	layerMeasureLine              = 21.0
	layerSimLine                  = 22.0
	layerTimeLine                 = 23.0
	layerBPMChangeLine            = 24.0
	layerTimescaleChangeLine      = 25.0
	layerNoteHead                 = 30.0
	layerNote                     = 31.0
	layerArrow                    = 32.0
)

func layerZ(layer, lane, y float64) float64 {
	return layer*10000 + lane*100 + y
}

func stageTransform() sonolus.Transform2D {
	return sonolus.IdentityTransform2D().PerspectiveY(judgmentLineScreenY, sonolus.NewVec2(0, vanishingPointY))
}

func containsTransformedRect(transform sonolus.Transform2D, rect sonolus.Rect, point sonolus.Vec2) bool {
	return transform.TransformRect(rect).Contains(point)
}

type tutorialPhaseTimeline struct {
	IntroEnd  float64
	FallEnd   float64
	FrozenEnd float64
	Hit       float64
	End       float64
}

func tutorialTimelineFor(phase int) tutorialPhaseTimeline {
	introEnd := tutorialIntroDuration
	if phase == 4 {
		frozenEnd := introEnd + tutorialFrozenHoldDuration*tutorialFrozenRepeats
		fallEnd := frozenEnd + tutorialFallDuration
		return tutorialPhaseTimeline{
			IntroEnd: introEnd, FallEnd: fallEnd, FrozenEnd: frozenEnd,
			Hit: frozenEnd, End: fallEnd + tutorialEndDuration,
		}
	}
	fallEnd := introEnd + tutorialFallDuration
	frozenDuration := tutorialFrozenTapDuration
	if phase == 1 || phase == 2 {
		frozenDuration += tutorialFrozenFlickDuration
	} else if phase == 5 {
		frozenDuration = tutorialFrozenReleaseDuration
	} else if phase == 6 {
		frozenDuration = tutorialFrozenHoldDuration + tutorialFrozenFlickDuration
	}
	frozenEnd := fallEnd + frozenDuration*tutorialFrozenRepeats
	hit := frozenEnd
	if phase == 1 || phase == 2 || phase == 6 {
		hit -= tutorialFrozenFlickDuration
	}
	return tutorialPhaseTimeline{
		IntroEnd: introEnd, FallEnd: fallEnd, FrozenEnd: frozenEnd,
		Hit: hit, End: frozenEnd + tutorialEndDuration,
	}
}

func tutorialCrossed(previous, current, instant float64) bool {
	return previous < instant && instant <= current
}

func holdTickResolution(best, target, now float64) (float64, bool) {
	if best <= unsetJudgmentTime {
		return best, now > target+judgmentWindowGood
	}
	canImprove := best < target && now-target < target-best
	if canImprove {
		return best, false
	}
	if best >= target-holdPerfectGracePeriod && best <= target {
		return target, true
	}
	return best, true
}

func noteTravelTime() float64 {
	return referenceNoteSpeed / Config.NoteSpeed
}

func laneX(lane float64) float64 {
	return lane * laneWidth()
}

func laneWidth() float64 { return baseLaneWidth * Config.LaneWidth }

func noteWidth() float64 { return baseLaneWidth * Config.NoteSize }

func displayDirection(direction float64) float64 {
	return direction
}

func directionalFlickSpeedThreshold(direction float64) float64 {
	return (2 + 2*math.Abs(direction)) * laneWidth()
}

func noteBucketWindowMilliseconds() sonolus.JudgmentWindows {
	return sonolus.JudgmentWindows{
		Perfect: sonolus.NewRange(-50, 50),
		Great:   sonolus.NewRange(-100, 100),
		Good:    sonolus.NewRange(-150, 150),
	}
}

func animationAlpha(now, period float64, fadeIn, fadeOut bool) float64 {
	cycle := math.Mod(now, period)
	if fadeIn && cycle < flickFadeInTime {
		return sonolus.Ease(sonolus.EaseOut, sonolus.EaseQuad, cycle/flickFadeInTime)
	}
	if fadeOut && cycle > period-flickFadeOutTime {
		return sonolus.Ease(sonolus.EaseOut, sonolus.EaseQuad, (period-cycle)/flickFadeOutTime)
	}
	return 1
}

func drawNoteBody(sprite sonolus.Sprite, lane, y float64) {
	alpha := noteAlpha(y)
	if alpha > 0 {
		sprite.Draw(noteRect(lane, y).ToQuad(), layerZ(layerNote, lane, y), alpha)
	}
}

func stageProjectY(y float64) (float64, float64) {
	distance := vanishingPointY - judgmentLineScreenY
	offset := y - judgmentLineY
	weight := 1 + offset/distance
	return judgmentLineScreenY + offset/weight, 1 / weight
}

func stageUnprojectPoint(x, y float64) sonolus.Vec2 {
	distance := vanishingPointY - judgmentLineScreenY
	offset := y - judgmentLineScreenY
	internalOffset := offset / (1 - offset/distance)
	weight := 1 + internalOffset/distance
	return sonolus.NewVec2(x*weight, internalOffset+judgmentLineY)
}

func flickArrowQuad(lane, y, progress float64) sonolus.Quad {
	screenY, scale := stageProjectY(y)
	halfWidth := noteWidth() * scale / 2
	centerX := laneX(lane) * scale
	height := noteWidth() * scale
	bottom := screenY + sonolus.Lerp(flickArrowInitialOffset, flickArrowFinalOffset, progress)*height
	top := bottom + height
	return sonolus.Quad{
		BL: stageUnprojectPoint(centerX-halfWidth, bottom),
		BR: stageUnprojectPoint(centerX+halfWidth, bottom),
		TL: stageUnprojectPoint(centerX-halfWidth, top),
		TR: stageUnprojectPoint(centerX+halfWidth, top),
	}
}

func drawFlickNote(body, arrow sonolus.Sprite, lane, y, now float64) {
	drawNoteBody(body, lane, y)
	progress := math.Mod(now, flickArrowPeriod) / flickArrowPeriod
	alpha := noteAlpha(y) * animationAlpha(now, flickArrowPeriod, true, true)
	if alpha > 0 {
		arrow.Draw(flickArrowQuad(lane, y, progress), layerZ(layerArrow, lane, y), alpha)
	}
}

func directionalFlickArrowQuad(lane, y, direction float64, number int, progress float64) sonolus.Quad {
	sign := 1.0
	angle := -math.Pi / 2
	if direction < 0 {
		sign = -1
		angle = math.Pi / 2
	}
	centerLane := lane + sign*(directionalFlickArrowOffset+float64(number)+progress)
	return noteRect(centerLane, y).Scale(directionalFlickArrowScale).ToQuad().RotateCentered(angle)
}

func drawDirectionalFlickNote(body, arrow sonolus.Sprite, lane, y, direction, now float64) {
	drawNoteBody(body, lane, y)
	direction = displayDirection(direction)
	count := int(math.Abs(direction))
	progress := math.Mod(now, directionalFlickArrowPeriod) / directionalFlickArrowPeriod
	for number := range count {
		alpha := noteAlpha(y) * animationAlpha(now, directionalFlickArrowPeriod, number == 0, number == count-1)
		if alpha > 0 {
			arrow.Draw(directionalFlickArrowQuad(lane, y, direction, number, progress), layerZ(layerArrow, lane, y), alpha)
		}
	}
}

func previewColumn(time float64) int {
	return int(time / previewColumnSeconds)
}

func previewColumnsForDuration(duration float64) int {
	return previewColumn(duration) + 1
}

func previewTimeY(time float64) float64 {
	remainder := math.Mod(time, previewColumnSeconds)
	if remainder < 0 {
		remainder += previewColumnSeconds
	}
	return previewYMin + remainder/previewColumnSeconds*(previewYMax-previewYMin)
}

func previewTimeYInColumn(time float64, column int) float64 {
	return previewYMin + (time-float64(column)*previewColumnSeconds)/previewColumnSeconds*(previewYMax-previewYMin)
}

func noteY(targetTime, now float64) float64 {
	return judgmentLineY + (targetTime-now)/noteTravelTime()*baseLaneLength*Config.LaneLength
}

func laneTopY() float64 { return judgmentLineY + baseLaneLength*Config.LaneLength }

func noteFadeProgress(y float64) float64 {
	alpha := 1.0
	if y < -1+noteFadeLength {
		alpha = (y + 1) / noteFadeLength
	} else if y > laneTopY()-noteFadeLength {
		alpha = (laneTopY() - y) / noteFadeLength
	}
	return math.Max(0, math.Min(1, alpha))
}

func noteAlpha(y float64) float64 {
	return sonolus.Ease(sonolus.EaseOut, sonolus.EaseQuad, noteFadeProgress(y))
}

func noteRect(lane, y float64) sonolus.Rect {
	halfWidth := noteWidth() / 2
	halfHeight := noteWidth() / 2
	center := laneX(lane)
	return sonolus.Rect{T: y + halfHeight, R: center + halfWidth, B: y - halfHeight, L: center - halfWidth}
}

func laneParticleQuad(transform sonolus.Transform2D, lane float64) sonolus.Quad {
	return transform.TransformRect(laneRect(lane))
}

func noteLinearParticleQuad(transform sonolus.Transform2D, lane float64) sonolus.Quad {
	left := transform.TransformVec(sonolus.NewVec2(laneX(lane)-noteWidth()/2, judgmentLineY))
	right := transform.TransformVec(sonolus.NewVec2(laneX(lane)+noteWidth()/2, judgmentLineY))
	height := right.Sub(left).Magnitude()
	up := sonolus.NewVec2(0, height)
	return sonolus.Quad{BL: left, BR: right, TL: left.Add(up), TR: right.Add(up)}
}

func noteCircularParticleQuad(transform sonolus.Transform2D, lane float64) sonolus.Quad {
	projected := transform.TransformRect(noteRect(lane, judgmentLineY).Scale(1.8))
	meanWidth := (projected.TR.X - projected.TL.X + projected.BR.X - projected.BL.X) / 2
	meanTopX := (projected.TL.X + projected.TR.X) / 2
	meanBottomX := (projected.BL.X + projected.BR.X) / 2
	return sonolus.Quad{
		BL: sonolus.NewVec2(meanBottomX-meanWidth/2, projected.BL.Y),
		BR: sonolus.NewVec2(meanBottomX+meanWidth/2, projected.BR.Y),
		TL: sonolus.NewVec2(meanTopX-meanWidth/2, projected.TL.Y),
		TR: sonolus.NewVec2(meanTopX+meanWidth/2, projected.TR.Y),
	}
}

func hitbox(lane float64) sonolus.Rect {
	center := laneX(lane)
	halfWidth := 1.25 * laneWidth()
	return sonolus.Rect{T: laneTopY(), R: center + halfWidth, B: -1, L: center - halfWidth}
}

func noteHitbox(lane, direction float64) sonolus.Rect {
	box := hitbox(lane)
	if direction > 0 {
		box.R += direction * laneWidth()
	} else if direction < 0 {
		box.L += direction * laneWidth()
	}
	return box
}

func hitboxOverlap(lane, otherLane float64, box, other sonolus.Rect) (float64, float64) {
	if otherLane > lane {
		return 0, box.R - other.L
	}
	if otherLane < lane {
		return other.R - box.L, 0
	}
	return 0, 0
}

var noteJudgmentWindows = sonolus.JudgmentWindows{
	Perfect: sonolus.Range{Min: -0.05, Max: 0.05},
	Great:   sonolus.Range{Min: -0.1, Max: 0.1},
	Good:    sonolus.Range{Min: -0.15, Max: 0.15},
}

func withinJudgmentWindow(value, target float64) bool {
	return noteJudgmentWindows.Good.Add(target).Contains(value)
}

func holdProgress(now, start, end float64) float64 {
	return sonolus.NewRange(start, end).UnlerpClamped(now)
}

func holdLane(now, start, end, startLane, endLane float64) float64 {
	return startLane + (endLane-startLane)*holdProgress(now, start, end)
}

func stageRect() sonolus.Rect {
	return sonolus.Rect{T: laneTopY(), R: stageRight * laneWidth(), B: -1, L: stageLeft * laneWidth()}
}

func laneRect(lane float64) sonolus.Rect {
	center := laneX(lane)
	halfWidth := laneWidth() / 2
	return sonolus.Rect{T: laneTopY(), R: center + halfWidth, B: -1, L: center - halfWidth}
}

func leftBorderRect() sonolus.Rect {
	x := laneX(stageLeft)
	width := baseLaneWidth * 0.25
	return sonolus.Rect{T: laneTopY(), R: x, B: -1, L: x - width}
}

func rightBorderRect() sonolus.Rect {
	x := laneX(stageRight)
	width := baseLaneWidth * 0.25
	return sonolus.Rect{T: laneTopY(), R: x + width, B: -1, L: x}
}

func stageCoverRect() sonolus.Rect {
	return sonolus.Rect{T: laneTopY() + noteWidth(), R: laneX(stageRight), B: laneTopY(), L: laneX(stageLeft)}
}

func judgmentRect() sonolus.Rect {
	return sonolus.Rect{T: judgmentLineY + noteWidth()/2, R: stageRight * laneWidth(), B: judgmentLineY - noteWidth()/2, L: stageLeft * laneWidth()}
}

func holdConnectorQuad(startLane, endLane, startY, endY float64) sonolus.Quad {
	adjustedStartY := sonolus.Clamp(startY, judgmentLineY, laneTopY())
	adjustedEndY := sonolus.Clamp(endY, judgmentLineY, laneTopY())
	adjustedStartLane := sonolus.Remap(startY, endY, startLane, endLane, adjustedStartY)
	adjustedEndLane := sonolus.Remap(startY, endY, startLane, endLane, adjustedEndY)
	halfWidth := noteWidth() / 2
	startX := laneX(adjustedStartLane)
	endX := laneX(adjustedEndLane)
	return sonolus.Quad{
		BL: sonolus.NewVec2(startX-halfWidth, adjustedStartY), TL: sonolus.NewVec2(endX-halfWidth, adjustedEndY),
		TR: sonolus.NewVec2(endX+halfWidth, adjustedEndY), BR: sonolus.NewVec2(startX+halfWidth, adjustedStartY),
	}
}

func simLineQuad(firstLane, secondLane, y float64) sonolus.Quad {
	left := laneX(firstLane)
	right := laneX(secondLane)
	return sonolus.Rect{T: y + noteWidth()/2, R: right, B: y - noteWidth()/2, L: left}.ToQuad()
}

func menuLayout(screen sonolus.Rect, config sonolus.RuntimeUIConfiguration) sonolus.RuntimeUILayout {
	return sonolus.RuntimeUILayout{
		Anchor: screen.TR().Add(sonolus.NewVec2(-0.05, -0.05)), Pivot: sonolus.NewVec2(1, 1),
		Size: sonolus.NewVec2(0.15, 0.15).Mul(config.Scale), Alpha: config.Alpha, HorizontalAlign: sonolus.HorizontalAlignCenter, Background: true,
	}
}

func judgmentLayout(config sonolus.RuntimeUIConfiguration) sonolus.RuntimeUILayout {
	return sonolus.RuntimeUILayout{
		Anchor: sonolus.NewVec2(0, -0.25), Pivot: sonolus.NewVec2(0.5, 0),
		Size: sonolus.NewVec2(0, 0.15).Mul(config.Scale), Alpha: config.Alpha, HorizontalAlign: sonolus.HorizontalAlignCenter,
	}
}

func comboValueLayout(screen sonolus.Rect, config sonolus.RuntimeUIConfiguration) sonolus.RuntimeUILayout {
	return sonolus.RuntimeUILayout{
		Anchor: sonolus.NewVec2(screen.R*0.7, 0), Pivot: sonolus.NewVec2(0.5, 0),
		Size: sonolus.NewVec2(0, 0.2).Mul(config.Scale), Alpha: config.Alpha, HorizontalAlign: sonolus.HorizontalAlignCenter,
	}
}

func comboTextLayout(screen sonolus.Rect, config sonolus.RuntimeUIConfiguration) sonolus.RuntimeUILayout {
	return sonolus.RuntimeUILayout{
		Anchor: sonolus.NewVec2(screen.R*0.7, 0), Pivot: sonolus.NewVec2(0.5, 1),
		Size: sonolus.NewVec2(0, 0.12).Mul(config.Scale), Alpha: config.Alpha, HorizontalAlign: sonolus.HorizontalAlignCenter,
	}
}

func primaryMetricBarLayout(screen sonolus.Rect, config sonolus.RuntimeUIConfiguration) sonolus.RuntimeUILayout {
	return sonolus.RuntimeUILayout{
		Anchor: screen.TL().Add(sonolus.NewVec2(0.05, -0.05)), Pivot: sonolus.NewVec2(0, 1),
		Size: sonolus.NewVec2(0.75, 0.15).Mul(config.Scale), Alpha: config.Alpha, HorizontalAlign: sonolus.HorizontalAlignLeft, Background: true,
	}
}

func primaryMetricValueLayout(screen sonolus.Rect, config sonolus.RuntimeUIConfiguration) sonolus.RuntimeUILayout {
	return sonolus.RuntimeUILayout{
		Anchor: screen.TL().Add(sonolus.NewVec2(0.05, -0.05)).Add(sonolus.NewVec2(0.715, -0.035).Mul(config.Scale)), Pivot: sonolus.NewVec2(0, 1),
		Size: sonolus.NewVec2(0, 0.08).Mul(config.Scale), Alpha: config.Alpha, HorizontalAlign: sonolus.HorizontalAlignRight,
	}
}

func secondaryMetricBarLayout(screen sonolus.Rect, config, menuConfig sonolus.RuntimeUIConfiguration) sonolus.RuntimeUILayout {
	return sonolus.RuntimeUILayout{
		Anchor: screen.TR().Sub(sonolus.NewVec2(0.1, 0.05)).Sub(sonolus.NewVec2(0.15, 0).Mul(menuConfig.Scale)), Pivot: sonolus.NewVec2(1, 1),
		Size: sonolus.NewVec2(0.75, 0.15).Mul(config.Scale), Alpha: config.Alpha, HorizontalAlign: sonolus.HorizontalAlignLeft, Background: true,
	}
}

func secondaryMetricValueLayout(screen sonolus.Rect, config, menuConfig, primaryMetricConfig sonolus.RuntimeUIConfiguration) sonolus.RuntimeUILayout {
	return sonolus.RuntimeUILayout{
		Anchor: screen.TR().Sub(sonolus.NewVec2(0.1, 0.05)).Sub(sonolus.NewVec2(0.15, 0).Mul(menuConfig.Scale)).Sub(sonolus.NewVec2(0.035, 0.035).Mul(primaryMetricConfig.Scale)), Pivot: sonolus.NewVec2(1, 1),
		Size: sonolus.NewVec2(0, 0.08).Mul(config.Scale), Alpha: config.Alpha, HorizontalAlign: sonolus.HorizontalAlignRight,
	}
}

func progressLayout(screen sonolus.Rect, config sonolus.RuntimeUIConfiguration) sonolus.RuntimeUILayout {
	return sonolus.RuntimeUILayout{
		Anchor: screen.BL().Add(sonolus.NewVec2(0.05, 0.05)), Pivot: sonolus.NewVec2(0, 0),
		Size: sonolus.NewVec2(screen.Width()-0.1, 0.15*config.Scale), Alpha: config.Alpha, HorizontalAlign: sonolus.HorizontalAlignCenter, Background: true,
	}
}

func basicMenuLayout(screen sonolus.Rect, config sonolus.RuntimeUIConfiguration) sonolus.RuntimeUIBasicLayout {
	return sonolus.RuntimeUIBasicLayout{
		Anchor: screen.TR().Add(sonolus.NewVec2(-0.05, -0.05)), Pivot: sonolus.NewVec2(1, 1),
		Size: sonolus.NewVec2(0.15, 0.15).Mul(config.Scale), Alpha: config.Alpha, Background: true,
	}
}

func basicProgressLayout(screen sonolus.Rect, config sonolus.RuntimeUIConfiguration) sonolus.RuntimeUIBasicLayout {
	return sonolus.RuntimeUIBasicLayout{
		Anchor: screen.BL().Add(sonolus.NewVec2(0.05, 0.05)), Pivot: sonolus.NewVec2(0, 0),
		Size: sonolus.NewVec2(screen.Width()-0.1, 0.15*config.Scale), Alpha: config.Alpha, Background: true,
	}
}

func tutorialPreviousLayout(screen sonolus.Rect, config sonolus.RuntimeUIConfiguration) sonolus.RuntimeUIBasicLayout {
	return sonolus.RuntimeUIBasicLayout{
		Anchor: sonolus.NewVec2(screen.L+0.05, 0), Pivot: sonolus.NewVec2(0, 0.5),
		Size: sonolus.NewVec2(0.15, 0.15).Mul(config.Scale), Alpha: config.Alpha, Background: true,
	}
}

func tutorialNextLayout(screen sonolus.Rect, config sonolus.RuntimeUIConfiguration) sonolus.RuntimeUIBasicLayout {
	return sonolus.RuntimeUIBasicLayout{
		Anchor: sonolus.NewVec2(screen.R-0.05, 0), Pivot: sonolus.NewVec2(1, 0.5),
		Size: sonolus.NewVec2(0.15, 0.15).Mul(config.Scale), Alpha: config.Alpha, Background: true,
	}
}

func tutorialInstructionLayout(config sonolus.RuntimeUIConfiguration) sonolus.RuntimeUIBasicLayout {
	return sonolus.RuntimeUIBasicLayout{
		Anchor: sonolus.NewVec2(0, 0.2), Pivot: sonolus.NewVec2(0.5, 0.5),
		Size: sonolus.NewVec2(1.2, 0.15).Mul(config.Scale), Alpha: config.Alpha, Background: true,
	}
}
