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
	stageLeft              = -3.5
	stageRight             = 3.5
	judgmentLineY          = -0.75
	referenceNoteSpeed     = 6.0
	flickSpeedThreshold    = 6.0
	previewColumnSeconds   = 3.0
	previewColumnWidth     = 8.0
	previewYMin            = -0.9
	previewYMax            = 0.9
	previewLaneWidth       = 0.8
	unsetJudgmentTime      = -1e8
	judgmentWindowGood     = 0.15
	holdPerfectGracePeriod = 1.0 / 30
)

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
	if Config.Mirror {
		lane = -lane
	}
	return lane * Config.LaneWidth
}

func displayDirection(direction float64) float64 {
	if Config.Mirror {
		return -direction
	}
	return direction
}

func directionalFlickSpeedThreshold(direction float64) float64 {
	return (2 + 2*math.Abs(direction)) * Config.LaneWidth
}

func drawDirectionalFlickNote(body, arrow sonolus.Sprite, lane, y, direction float64) {
	body.Draw(noteRect(lane, y).ToQuad(), 30, 1)
	direction = displayDirection(direction)
	sign := 1.0
	if direction < 0 {
		sign = -1
	}
	for i := range int(math.Abs(direction)) {
		arrowLane := lane + float64(i+1)*sign
		arrow.Draw(noteRect(arrowLane, y).Scale(0.7).ToQuad(), 31, 1)
	}
}

func previewColumn(time float64) int {
	return int(math.Floor(time / previewColumnSeconds))
}

func previewColumnsForDuration(duration float64) int {
	return previewColumn(duration) + 1
}

func previewTimeY(time float64) float64 {
	return previewYMin + math.Mod(time, previewColumnSeconds)/previewColumnSeconds*(previewYMax-previewYMin)
}

func previewTimeYInColumn(time float64, column int) float64 {
	return previewYMin + (time-float64(column)*previewColumnSeconds)/previewColumnSeconds*(previewYMax-previewYMin)
}

func noteY(targetTime, now float64) float64 {
	return judgmentLineY + (targetTime-now)/noteTravelTime()*2*Config.LaneLength
}

func laneTopY() float64 { return judgmentLineY + 2*Config.LaneLength }

func noteRect(lane, y float64) sonolus.Rect {
	halfWidth := 0.42 * Config.NoteSize * Config.LaneWidth
	halfHeight := 0.12 * Config.NoteSize
	center := laneX(lane)
	return sonolus.Rect{T: y + halfHeight, R: center + halfWidth, B: y - halfHeight, L: center - halfWidth}
}

func hitbox(lane float64) sonolus.Rect {
	center := laneX(lane)
	halfWidth := 1.25 * Config.LaneWidth
	return sonolus.Rect{T: judgmentLineY + 0.35, R: center + halfWidth, B: judgmentLineY - 0.35, L: center - halfWidth}
}

func noteHitbox(lane, direction float64) sonolus.Rect {
	box := hitbox(lane)
	if direction > 0 {
		box.R += direction * Config.LaneWidth
	} else if direction < 0 {
		box.L += direction * Config.LaneWidth
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

func judgmentWindows() sonolus.JudgmentWindows {
	return sonolus.JudgmentWindows{
		Perfect: sonolus.Range{Min: -0.05, Max: 0.05},
		Great:   sonolus.Range{Min: -0.1, Max: 0.1},
		Good:    sonolus.Range{Min: -0.15, Max: 0.15},
	}
}

func withinJudgmentWindow(value, target float64) bool {
	return value >= target-0.15 && value <= target+0.15
}

func holdProgress(now, start, end float64) float64 {
	progress := (now - start) / (end - start)
	if progress < 0 {
		return 0
	}
	if progress > 1 {
		return 1
	}
	return progress
}

func holdLane(now, start, end, startLane, endLane float64) float64 {
	return startLane + (endLane-startLane)*holdProgress(now, start, end)
}

func holdChainLane(now, start, anchor, end, startLane, anchorLane, endLane float64) float64 {
	if now <= anchor {
		return holdLane(now, start, anchor, startLane, anchorLane)
	}
	return holdLane(now, anchor, end, anchorLane, endLane)
}

func stageRect() sonolus.Rect {
	return sonolus.Rect{T: laneTopY() + 0.15, R: stageRight * Config.LaneWidth, B: -1, L: stageLeft * Config.LaneWidth}
}

func laneRect(lane float64) sonolus.Rect {
	center := laneX(lane)
	halfWidth := 0.45 * Config.LaneWidth
	return sonolus.Rect{T: laneTopY() + 0.15, R: center + halfWidth, B: -1, L: center - halfWidth}
}

func leftBorderRect() sonolus.Rect {
	x := laneX(stageLeft)
	return sonolus.Rect{T: laneTopY() + 0.15, R: x + 0.04, B: -1, L: x - 0.04}
}

func rightBorderRect() sonolus.Rect {
	x := laneX(stageRight)
	return sonolus.Rect{T: laneTopY() + 0.15, R: x + 0.04, B: -1, L: x - 0.04}
}

func stageCoverRect() sonolus.Rect {
	return sonolus.Rect{T: laneTopY() + 0.3, R: laneX(stageRight), B: laneTopY() + 0.07, L: laneX(stageLeft)}
}

func judgmentRect() sonolus.Rect {
	return sonolus.Rect{T: judgmentLineY + 0.025, R: stageRight * Config.LaneWidth, B: judgmentLineY - 0.025, L: stageLeft * Config.LaneWidth}
}

func holdConnectorQuad(startLane, endLane, startY, endY float64) sonolus.Quad {
	halfWidth := 0.28 * Config.NoteSize * Config.LaneWidth
	startX := laneX(startLane)
	endX := laneX(endLane)
	return sonolus.Quad{
		BL: sonolus.NewVec2(startX-halfWidth, startY), TL: sonolus.NewVec2(endX-halfWidth, endY),
		TR: sonolus.NewVec2(endX+halfWidth, endY), BR: sonolus.NewVec2(startX+halfWidth, startY),
	}
}

func simLineQuad(firstLane, secondLane, y float64) sonolus.Quad {
	left := laneX(firstLane)
	right := laneX(secondLane)
	if left > right {
		left, right = right, left
	}
	return sonolus.Rect{T: y + 0.025, R: right, B: y - 0.025, L: left}.ToQuad()
}

func menuLayout() sonolus.RuntimeUILayout {
	return sonolus.RuntimeUILayout{
		Anchor: sonolus.NewVec2(0.95, 0.9), Pivot: sonolus.NewVec2(1, 0.5), Size: sonolus.NewVec2(0.15, 0.15), Alpha: 1,
	}
}

func judgmentLayout() sonolus.RuntimeUILayout {
	return sonolus.RuntimeUILayout{
		Anchor: sonolus.NewVec2(0, -0.35), Pivot: sonolus.NewVec2(0.5, 0.5), Size: sonolus.NewVec2(0.8, 0.12), Alpha: 1,
	}
}

func comboValueLayout() sonolus.RuntimeUILayout {
	return sonolus.RuntimeUILayout{
		Anchor: sonolus.NewVec2(0, 0.25), Pivot: sonolus.NewVec2(0.5, 0), Size: sonolus.NewVec2(0.5, 0.18), Alpha: 1,
	}
}

func comboTextLayout() sonolus.RuntimeUILayout {
	return sonolus.RuntimeUILayout{
		Anchor: sonolus.NewVec2(0, 0.25), Pivot: sonolus.NewVec2(0.5, 1), Size: sonolus.NewVec2(0.5, 0.1), Alpha: 1,
	}
}

func primaryMetricBarLayout() sonolus.RuntimeUILayout {
	return sonolus.RuntimeUILayout{
		Anchor: sonolus.NewVec2(-0.95, 0.9), Pivot: sonolus.NewVec2(0, 0.5), Size: sonolus.NewVec2(0.8, 0.08), Alpha: 1,
	}
}

func primaryMetricValueLayout() sonolus.RuntimeUILayout {
	return sonolus.RuntimeUILayout{
		Anchor: sonolus.NewVec2(-0.95, 0.82), Pivot: sonolus.NewVec2(0, 0.5), Size: sonolus.NewVec2(0.8, 0.1), Alpha: 1,
	}
}

func secondaryMetricBarLayout() sonolus.RuntimeUILayout {
	return sonolus.RuntimeUILayout{
		Anchor: sonolus.NewVec2(0.75, 0.9), Pivot: sonolus.NewVec2(1, 0.5), Size: sonolus.NewVec2(0.8, 0.08), Alpha: 1,
	}
}

func secondaryMetricValueLayout() sonolus.RuntimeUILayout {
	return sonolus.RuntimeUILayout{
		Anchor: sonolus.NewVec2(0.75, 0.82), Pivot: sonolus.NewVec2(1, 0.5), Size: sonolus.NewVec2(0.8, 0.1), Alpha: 1,
	}
}

func progressLayout() sonolus.RuntimeUILayout {
	return sonolus.RuntimeUILayout{
		Anchor: sonolus.NewVec2(0, -0.95), Pivot: sonolus.NewVec2(0.5, 0), Size: sonolus.NewVec2(1.6, 0.08), Alpha: 1,
	}
}

func basicMenuLayout() sonolus.RuntimeUIBasicLayout {
	return sonolus.RuntimeUIBasicLayout{
		Anchor: sonolus.NewVec2(0.95, 0.9), Pivot: sonolus.NewVec2(1, 0.5), Size: sonolus.NewVec2(0.15, 0.15), Alpha: 1, Background: true,
	}
}

func basicProgressLayout() sonolus.RuntimeUIBasicLayout {
	return sonolus.RuntimeUIBasicLayout{
		Anchor: sonolus.NewVec2(0, -0.95), Pivot: sonolus.NewVec2(0.5, 0), Size: sonolus.NewVec2(1.6, 0.08), Alpha: 1, Background: true,
	}
}

func tutorialPreviousLayout() sonolus.RuntimeUIBasicLayout {
	return sonolus.RuntimeUIBasicLayout{
		Anchor: sonolus.NewVec2(-0.9, 0), Pivot: sonolus.NewVec2(0, 0.5), Size: sonolus.NewVec2(0.15, 0.15), Alpha: 1, Background: true,
	}
}

func tutorialNextLayout() sonolus.RuntimeUIBasicLayout {
	return sonolus.RuntimeUIBasicLayout{
		Anchor: sonolus.NewVec2(0.9, 0), Pivot: sonolus.NewVec2(1, 0.5), Size: sonolus.NewVec2(0.15, 0.15), Alpha: 1, Background: true,
	}
}

func tutorialInstructionLayout() sonolus.RuntimeUIBasicLayout {
	return sonolus.RuntimeUIBasicLayout{
		Anchor: sonolus.NewVec2(0, 0.3), Pivot: sonolus.NewVec2(0.5, 0.5), Size: sonolus.NewVec2(1.2, 0.15), Alpha: 1, Background: true,
	}
}
