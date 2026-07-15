//go:build preview

package main

import (
	"math"

	"github.com/WindowsSov8forUs/sonolus-go/v2/sonolus"
	"github.com/WindowsSov8forUs/sonolus-go/v2/sonolus/native"
	"github.com/WindowsSov8forUs/sonolus-go/v2/sonolus/preview"
)

const (
	previewLastTimeSlot    = 0
	previewColumnCountSlot = 1
)

func updatePreviewLastTime(time float64) {
	if time > preview.LevelData.Get(previewLastTimeSlot) {
		preview.LevelData.Set(previewLastTimeSlot, time)
	}
}

func previewLaneX(lane float64, column int) float64 {
	return (float64(column)+0.5)*previewColumnWidth + laneX(lane)*previewLaneWidth - preview.Screen.Rect().Width()/2
}

func previewNoteRect(lane, time float64) sonolus.Rect {
	center := previewLaneX(lane, previewColumn(time))
	halfWidth := 0.36 * Config.NoteSize * Config.LaneWidth
	halfHeight := 0.06 * Config.NoteSize
	y := previewTimeY(time)
	return sonolus.Rect{T: y + halfHeight, R: center + halfWidth, B: y - halfHeight, L: center - halfWidth}
}

func drawPreviewDirectionalFlick(body, arrow sonolus.Sprite, lane, time, direction float64) {
	body.Draw(previewNoteRect(lane, time).ToQuad(), 30, 1)
	direction = displayDirection(direction)
	sign := 1.0
	if direction < 0 {
		sign = -1
	}
	for i := range int(math.Abs(direction)) {
		arrowLane := lane + float64(i+1)*sign
		arrow.Draw(previewNoteRect(arrowLane, time).Scale(0.7).ToQuad(), 31, 1)
	}
}

func previewBarRect(time float64) sonolus.Rect {
	column := previewColumn(time)
	y := previewTimeY(time)
	return sonolus.Rect{
		T: y + 0.006,
		R: previewLaneX(stageRight, column),
		B: y - 0.006,
		L: previewLaneX(stageLeft, column),
	}
}

func previewConnectorQuad(firstLane, secondLane, firstTime, secondTime float64, column int) sonolus.Quad {
	return sonolus.Quad{
		BL: sonolus.NewVec2(previewLaneX(firstLane, column)-0.36*Config.LaneWidth, previewTimeYInColumn(firstTime, column)),
		BR: sonolus.NewVec2(previewLaneX(firstLane, column)+0.36*Config.LaneWidth, previewTimeYInColumn(firstTime, column)),
		TR: sonolus.NewVec2(previewLaneX(secondLane, column)+0.36*Config.LaneWidth, previewTimeYInColumn(secondTime, column)),
		TL: sonolus.NewVec2(previewLaneX(secondLane, column)-0.36*Config.LaneWidth, previewTimeYInColumn(secondTime, column)),
	}
}

type PreviewSkin struct {
	sonolus.SkinResource
	Lane, Note, Flick, FlickArrow, RightFlick, RightFlickArrow, LeftFlick, LeftFlickArrow, BPM, Timescale, Measure sonolus.Sprite
	HoldHead, HoldTail, HoldConnector, SimLine, HoldTick                                                           sonolus.Sprite
	StageMiddle, LeftBorder, RightBorder, Slot, Cover                                                              sonolus.Sprite
}

var Skin = &PreviewSkin{
	SkinResource: sonolus.SkinResource{RenderMode: sonolus.RenderModeLightweight},
	Lane:         sonolus.SkinSprite(sonolus.StandardSpriteLane), Note: sonolus.SkinSprite(sonolus.StandardSpriteNoteHeadCyan),
	Flick: sonolus.SkinSprite(sonolus.StandardSpriteNoteHeadRed), FlickArrow: sonolus.SkinSprite(sonolus.StandardSpriteDirectionalMarkerRed),
	RightFlick: sonolus.SkinSprite(sonolus.StandardSpriteNoteHeadYellow), RightFlickArrow: sonolus.SkinSprite(sonolus.StandardSpriteDirectionalMarkerYellow),
	LeftFlick: sonolus.SkinSprite(sonolus.StandardSpriteNoteHeadPurple), LeftFlickArrow: sonolus.SkinSprite(sonolus.StandardSpriteDirectionalMarkerPurple),
	HoldHead: sonolus.SkinSprite(sonolus.StandardSpriteNoteHeadGreen), HoldTail: sonolus.SkinSprite(sonolus.StandardSpriteNoteTailGreen),
	HoldConnector: sonolus.SkinSprite(sonolus.StandardSpriteNoteConnectionGreenSeamless),
	SimLine:       sonolus.SkinSprite(sonolus.StandardSpriteSimultaneousConnectionNeutralSeamless),
	HoldTick:      sonolus.SkinSprite(sonolus.StandardSpriteNoteTickGreen),
	BPM:           sonolus.SkinSprite(sonolus.StandardSpriteGridPurple),
	Timescale:     sonolus.SkinSprite(sonolus.StandardSpriteGridYellow),
	Measure:       sonolus.SkinSprite(sonolus.StandardSpriteGridNeutral),
	StageMiddle:   sonolus.SkinSprite(sonolus.StandardSpriteStageMiddle), LeftBorder: sonolus.SkinSprite(sonolus.StandardSpriteStageLeftBorder),
	RightBorder: sonolus.SkinSprite(sonolus.StandardSpriteStageRightBorder), Slot: sonolus.SkinSprite(sonolus.StandardSpriteNoteSlot),
	Cover: sonolus.SkinSprite(sonolus.StandardSpriteStageCover),
}

type PreviewStage struct {
	preview.Archetype      `archetype:"name=Stage"`
	preview.CallbackOrders `archetype:"preprocess=1"`
}

func (*PreviewStage) Preprocess() {
	columnCount := previewColumnsForDuration(preview.LevelData.Get(previewLastTimeSlot))
	preview.LevelData.Set(previewColumnCountSlot, float64(columnCount))
	preview.Canvas.Set(preview.CanvasOptions{Scroll: preview.ScrollLeftToRight, Size: previewColumnWidth * float64(columnCount)})
	preview.UI.SetMenu(basicMenuLayout())
	preview.UI.SetProgress(basicProgressLayout())
}
func (*PreviewStage) Render() {
	columnCount := int(preview.LevelData.Get(previewColumnCountSlot))
	for column := 0; column < columnCount; column++ {
		left := previewLaneX(stageLeft, column)
		right := previewLaneX(stageRight, column)
		Skin.StageMiddle.Draw(sonolus.Rect{T: 1, R: right, B: -1, L: left}.ToQuad(), 5, 1)
		for lane := -3; lane <= 3; lane++ {
			center := previewLaneX(float64(lane), column)
			Skin.Lane.Draw(sonolus.Rect{T: 1, R: center + 0.36*Config.LaneWidth, B: -1, L: center - 0.36*Config.LaneWidth}.ToQuad(), 10, 0.8)
		}
		Skin.LeftBorder.Draw(sonolus.Rect{T: 1, R: left + 0.04, B: -1, L: left - 0.04}.ToQuad(), 12, 1)
		Skin.RightBorder.Draw(sonolus.Rect{T: 1, R: right + 0.04, B: -1, L: right - 0.04}.ToQuad(), 12, 1)
		Skin.Cover.Draw(sonolus.Rect{T: previewYMin, R: right, B: -1, L: left}.ToQuad(), 40, 1)
		Skin.Cover.Draw(sonolus.Rect{T: 1, R: right, B: previewYMax, L: left}.ToQuad(), 40, 1)
	}
	for beat := 0; beat <= 24; beat += 4 {
		y := native.BeatToTime(float64(beat))
		Skin.Measure.Draw(previewBarRect(y).ToQuad(), 18, 0.35)
	}
}

type PreviewBPMChange struct {
	preview.Archetype `archetype:"name=#BPM_CHANGE"`
	Beat              float64 `archetype:"imported,name=#BEAT"`
	BPM               float64 `archetype:"imported,name=#BPM"`
	Time              float64 `archetype:"data"`
}

func (b *PreviewBPMChange) Preprocess() {
	b.Time = native.BeatToTime(b.Beat)
	updatePreviewLastTime(b.Time)
}
func (b *PreviewBPMChange) Render() {
	Skin.BPM.Draw(previewBarRect(b.Time).ToQuad(), 20, 0.7)
	preview.Canvas.Print(preview.PrintOptions{
		Value: b.BPM, Format: sonolus.PrintFormatBPM, DecimalPlaces: 0,
		Anchor: sonolus.NewVec2(previewLaneX(stageRight, previewColumn(b.Time))+0.2, previewTimeY(b.Time)), Pivot: sonolus.NewVec2(0, 0.5), Size: sonolus.NewVec2(1, 0.35),
		Alpha: 1, Color: sonolus.PrintColorPurple, HorizontalAlign: sonolus.HorizontalAlignLeft,
	})
}

type PreviewTimescaleChange struct {
	preview.Archetype `archetype:"name=#TIMESCALE_CHANGE"`
	Beat              float64 `archetype:"imported,name=#BEAT"`
	Timescale         float64 `archetype:"imported,name=#TIMESCALE"`
	Time              float64 `archetype:"data"`
}

func (t *PreviewTimescaleChange) Preprocess() {
	t.Time = native.BeatToTime(t.Beat)
	updatePreviewLastTime(t.Time)
}
func (t *PreviewTimescaleChange) Render() {
	Skin.Timescale.Draw(previewBarRect(t.Time).ToQuad(), 20, 0.7)
	preview.Canvas.Print(preview.PrintOptions{
		Value: t.Timescale, Format: sonolus.PrintFormatTimeScale, DecimalPlaces: 2,
		Anchor: sonolus.NewVec2(previewLaneX(stageLeft, previewColumn(t.Time))-0.2, previewTimeY(t.Time)), Pivot: sonolus.NewVec2(1, 0.5), Size: sonolus.NewVec2(1, 0.35),
		Alpha: 1, Color: sonolus.PrintColorYellow, HorizontalAlign: sonolus.HorizontalAlignRight,
	})
}

type PreviewTapNote struct {
	preview.Archetype `archetype:"name=TapNote"`
	Beat              float64 `archetype:"imported,name=#BEAT"`
	Lane              float64 `archetype:"imported,name=lane"`
	Time              float64 `archetype:"data"`
}

type PreviewAccentTapNote struct {
	PreviewTapNote    `archetype:"base"`
	preview.Archetype `archetype:"name=AccentTapNote"`
}

func (n *PreviewTapNote) Preprocess() {
	n.Time = native.BeatToTime(n.Beat)
	updatePreviewLastTime(n.Time)
}
func (n *PreviewTapNote) Render() {
	Skin.Note.Draw(previewNoteRect(n.Lane, n.Time).ToQuad(), 30, 1)
}

type PreviewFlickNote struct {
	preview.Archetype `archetype:"name=FlickNote"`
	Beat              float64 `archetype:"imported,name=#BEAT"`
	Lane              float64 `archetype:"imported,name=lane"`
	Time              float64 `archetype:"data"`
}

func (n *PreviewFlickNote) Preprocess() {
	n.Time = native.BeatToTime(n.Beat)
	updatePreviewLastTime(n.Time)
}
func (n *PreviewFlickNote) Render() {
	Skin.Flick.Draw(previewNoteRect(n.Lane, n.Time).ToQuad(), 30, 1)
	Skin.FlickArrow.Draw(previewNoteRect(n.Lane, n.Time).Translate(sonolus.NewVec2(0, 0.08)).Scale(0.7).ToQuad(), 31, 1)
}

type PreviewDirectionalFlickNote struct {
	preview.Archetype `archetype:"name=DirectionalFlickNote"`
	Beat              float64 `archetype:"imported,name=#BEAT"`
	Lane              float64 `archetype:"imported,name=lane"`
	Direction         float64 `archetype:"imported,name=direction"`
	Time              float64 `archetype:"data"`
}

func (n *PreviewDirectionalFlickNote) Preprocess() {
	n.Time = native.BeatToTime(n.Beat)
	updatePreviewLastTime(n.Time)
}
func (n *PreviewDirectionalFlickNote) Render() {
	if displayDirection(n.Direction) > 0 {
		drawPreviewDirectionalFlick(Skin.RightFlick, Skin.RightFlickArrow, n.Lane, n.Time, n.Direction)
	} else {
		drawPreviewDirectionalFlick(Skin.LeftFlick, Skin.LeftFlickArrow, n.Lane, n.Time, n.Direction)
	}
}

type PreviewHoldHeadNote struct {
	preview.Archetype `archetype:"name=HoldHeadNote"`
	Beat              float64                                  `archetype:"imported,name=#BEAT"`
	Lane              float64                                  `archetype:"imported,name=lane"`
	Anchor            sonolus.EntityRef[PreviewHoldAnchorNote] `archetype:"imported,name=anchor"`
	End               sonolus.EntityRef[PreviewHoldEndNote]    `archetype:"imported,name=end"`
	FlickEnd          sonolus.EntityRef[PreviewHoldFlickNote]  `archetype:"imported,name=flickEnd"`
	Time              float64                                  `archetype:"data"`
}

func (n *PreviewHoldHeadNote) Preprocess() {
	n.Time = native.BeatToTime(n.Beat)
	updatePreviewLastTime(n.Time)
}
func (n *PreviewHoldHeadNote) Render() {
	Skin.HoldHead.Draw(previewNoteRect(n.Lane, n.Time).ToQuad(), 30, 1)
}

func (n *PreviewHoldHeadNote) endBeat() float64 {
	if n.FlickEnd.Index > 0 {
		return n.FlickEnd.Get().Beat
	}
	return n.End.Get().Beat
}

func (n *PreviewHoldHeadNote) endLane() float64 {
	if n.FlickEnd.Index > 0 {
		return n.FlickEnd.Get().Lane
	}
	return n.End.Get().Lane
}

type PreviewHoldAnchorNote struct {
	preview.Archetype `archetype:"name=HoldAnchorNote"`
	Beat              float64 `archetype:"imported,name=#BEAT"`
	Lane              float64 `archetype:"imported,name=lane"`
	Time              float64 `archetype:"data"`
}

func (n *PreviewHoldAnchorNote) Preprocess() {
	n.Time = native.BeatToTime(n.Beat)
	updatePreviewLastTime(n.Time)
}

type PreviewHoldEndNote struct {
	preview.Archetype `archetype:"name=HoldEndNote"`
	Head              sonolus.EntityRef[PreviewHoldHeadNote] `archetype:"imported,name=head"`
	Beat              float64                                `archetype:"imported,name=#BEAT"`
	Lane              float64                                `archetype:"imported,name=lane"`
	Time              float64                                `archetype:"data"`
}

func (n *PreviewHoldEndNote) Preprocess() {
	n.Time = native.BeatToTime(n.Beat)
	updatePreviewLastTime(n.Time)
}
func (n *PreviewHoldEndNote) Render() {
	Skin.HoldTail.Draw(previewNoteRect(n.Lane, n.Time).ToQuad(), 30, 1)
}

type PreviewHoldFlickNote struct {
	preview.Archetype `archetype:"name=HoldFlickNote"`
	Head              sonolus.EntityRef[PreviewHoldHeadNote] `archetype:"imported,name=head"`
	Beat              float64                                `archetype:"imported,name=#BEAT"`
	Lane              float64                                `archetype:"imported,name=lane"`
	Time              float64                                `archetype:"data"`
}

func (n *PreviewHoldFlickNote) Preprocess() {
	n.Time = native.BeatToTime(n.Beat)
	updatePreviewLastTime(n.Time)
}
func (n *PreviewHoldFlickNote) Render() {
	quad := previewNoteRect(n.Lane, n.Time)
	Skin.Flick.Draw(quad.ToQuad(), 30, 1)
	Skin.FlickArrow.Draw(quad.Translate(sonolus.NewVec2(0, 0.08)).Scale(0.7).ToQuad(), 31, 1)
}

type PreviewHoldConnector struct {
	preview.Archetype `archetype:"name=HoldConnector"`
	Head              sonolus.EntityRef[PreviewHoldHeadNote]   `archetype:"imported,name=head"`
	Anchor            sonolus.EntityRef[PreviewHoldAnchorNote] `archetype:"imported,name=anchor"`
	End               sonolus.EntityRef[PreviewHoldEndNote]    `archetype:"imported,name=end"`
	FlickEnd          sonolus.EntityRef[PreviewHoldFlickNote]  `archetype:"imported,name=flickEnd"`
	Segment           float64                                  `archetype:"imported,name=segment"`
}

func (n *PreviewHoldConnector) Render() {
	startBeat := n.Head.Get().Beat
	endBeat := n.Anchor.Get().Beat
	startLane := n.Head.Get().Lane
	endLane := n.Anchor.Get().Lane
	if n.Segment != 0 {
		startBeat = n.Anchor.Get().Beat
		endBeat = n.Head.Get().endBeat()
		startLane = n.Anchor.Get().Lane
		endLane = n.Head.Get().endLane()
	}
	startTime := native.BeatToTime(startBeat)
	endTime := native.BeatToTime(endBeat)
	startColumn := previewColumn(startTime)
	endColumn := previewColumn(endTime)
	for column := startColumn; column <= endColumn; column++ {
		columnStart := float64(column) * previewColumnSeconds
		columnEnd := columnStart + previewColumnSeconds
		segmentStart := math.Max(startTime, columnStart)
		segmentEnd := math.Min(endTime, columnEnd)
		segmentStartLane := holdLane(segmentStart, startTime, endTime, startLane, endLane)
		segmentEndLane := holdLane(segmentEnd, startTime, endTime, startLane, endLane)
		Skin.HoldConnector.Draw(previewConnectorQuad(segmentStartLane, segmentEndLane, segmentStart, segmentEnd, column), 25, Config.ConnectorAlpha)
	}
}

type PreviewSimLine struct {
	preview.Archetype `archetype:"name=SimLine"`
	First             sonolus.EntityRef[PreviewTapNote] `archetype:"imported,name=first"`
	Second            sonolus.EntityRef[PreviewTapNote] `archetype:"imported,name=second"`
}

func (n *PreviewSimLine) Render() {
	if !Config.SimLines {
		return
	}
	time := native.BeatToTime(n.First.Get().Beat)
	column := previewColumn(time)
	firstX := previewLaneX(n.First.Get().Lane, column)
	secondX := previewLaneX(n.Second.Get().Lane, column)
	y := previewTimeY(time)
	Skin.SimLine.Draw(sonolus.Rect{T: y + 0.01, R: math.Max(firstX, secondX), B: y - 0.01, L: math.Min(firstX, secondX)}.ToQuad(), 24, Config.ConnectorAlpha)
}

type PreviewHoldTickNote struct {
	preview.Archetype `archetype:"name=HoldTickNote"`
	Head              sonolus.EntityRef[PreviewHoldHeadNote] `archetype:"imported,name=head"`
	Beat              float64                                `archetype:"imported,name=#BEAT"`
	Lane              float64                                `archetype:"data"`
	Time              float64                                `archetype:"data"`
}

func (n *PreviewHoldTickNote) Preprocess() {
	n.Time = native.BeatToTime(n.Beat)
	updatePreviewLastTime(n.Time)
	n.Lane = holdChainLane(
		n.Beat,
		n.Head.Get().Beat,
		n.Head.Get().Anchor.Get().Beat,
		n.Head.Get().endBeat(),
		n.Head.Get().Lane,
		n.Head.Get().Anchor.Get().Lane,
		n.Head.Get().endLane(),
	)
}
func (n *PreviewHoldTickNote) Render() {
	Skin.HoldTick.Draw(previewNoteRect(n.Lane, n.Time).Scale(0.7).ToQuad(), 29, 1)
}
