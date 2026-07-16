//go:build preview

package main

import (
	"math"

	"github.com/WindowsSov8forUs/sonolus-go/v2/sonolus"
	"github.com/WindowsSov8forUs/sonolus-go/v2/sonolus/preview"
)

type PreviewChartData struct {
	sonolus.LevelDataResource
	LastTime float64
	LastBeat float64
}

var PreviewData = PreviewChartData{}

type PreviewLayoutData struct {
	sonolus.LevelDataResource
	ColumnCount int
}

var PreviewLayout = PreviewLayoutData{}

func updatePreviewDuration(time, beat float64) {
	if time > PreviewData.LastTime {
		PreviewData.LastTime = time
	}
	if beat > PreviewData.LastBeat {
		PreviewData.LastBeat = beat
	}
}

func previewLaneX(lane float64, column int) float64 {
	return (float64(column)+0.5)*previewColumnWidth + lane*previewLaneWidth - preview.Screen.Rect().Width()/2
}

func previewNoteRect(lane, time float64) sonolus.Rect {
	center := previewLaneX(lane, previewColumn(time))
	halfWidth := previewLaneWidth / 2
	halfHeight := previewLaneWidth / 2
	y := previewTimeY(time)
	return sonolus.Rect{T: y + halfHeight, R: center + halfWidth, B: y - halfHeight, L: center - halfWidth}
}

func drawPreviewDirectionalFlick(body, arrow sonolus.Sprite, lane, time, direction float64) {
	body.Draw(previewNoteRect(lane, time).ToQuad(), layerZ(layerNote, lane, time), 1)
	direction = displayDirection(direction)
	angle := -math.Pi / 2
	sign := 1.0
	if direction < 0 {
		angle = math.Pi / 2
		sign = -1
	}
	for index := range int(math.Abs(direction)) {
		arrowLane := lane + float64(index+1)*sign
		arrow.Draw(previewNoteRect(arrowLane, time).ToQuad().RotateCentered(angle), layerZ(layerArrow, arrowLane, time), 1)
	}
}

func previewBarRect(time float64) sonolus.Rect {
	column := previewColumn(time)
	y := previewTimeY(time)
	return sonolus.Rect{
		T: y + previewBarLineHeight/2,
		R: previewLaneX(stageRight, column),
		B: y - previewBarLineHeight/2,
		L: previewLaneX(stageLeft, column),
	}
}

func previewExtendedBarRect(time float64, left, right bool) sonolus.Rect {
	result := previewBarRect(time)
	if left {
		result.L -= previewBarExtendWidth
	}
	if right {
		result.R += previewBarExtendWidth
	}
	return result
}

func previewLeftOnlyBarRect(time float64) sonolus.Rect {
	result := previewBarRect(time)
	result.R = result.L
	result.L -= previewBarExtendWidth
	return result
}

func previewConnectorQuad(firstLane, secondLane, firstTime, secondTime float64, column int) sonolus.Quad {
	return sonolus.Quad{
		BL: sonolus.NewVec2(previewLaneX(firstLane, column)-previewLaneWidth/2, previewTimeYInColumn(firstTime, column)),
		BR: sonolus.NewVec2(previewLaneX(firstLane, column)+previewLaneWidth/2, previewTimeYInColumn(firstTime, column)),
		TR: sonolus.NewVec2(previewLaneX(secondLane, column)+previewLaneWidth/2, previewTimeYInColumn(secondTime, column)),
		TL: sonolus.NewVec2(previewLaneX(secondLane, column)-previewLaneWidth/2, previewTimeYInColumn(secondTime, column)),
	}
}

type PreviewSkin struct {
	sonolus.SkinResource
	Lane, Note, Flick, FlickArrow, RightFlick, RightFlickArrow, LeftFlick, LeftFlickArrow, BPM, Timescale, Measure, Time sonolus.Sprite
	HoldHead, HoldTail, HoldConnector, SimLine, HoldTick                                                                 sonolus.Sprite
	StageMiddle, LeftBorder, RightBorder, Slot, Cover                                                                    sonolus.Sprite
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
	Time:          sonolus.SkinSprite(sonolus.StandardSpriteGridCyan),
	StageMiddle:   sonolus.SkinSprite(sonolus.StandardSpriteStageMiddle), LeftBorder: sonolus.SkinSprite(sonolus.StandardSpriteStageLeftBorder),
	RightBorder: sonolus.SkinSprite(sonolus.StandardSpriteStageRightBorder), Slot: sonolus.SkinSprite(sonolus.StandardSpriteNoteSlot),
	Cover: sonolus.SkinSprite(sonolus.StandardSpriteStageCover),
}

type PreviewStage struct {
	preview.Archetype      `archetype:"name=Stage"`
	preview.CallbackOrders `archetype:"preprocess=1"`
}

func (*PreviewStage) Preprocess() {
	columnCount := previewColumnsForDuration(PreviewData.LastTime)
	PreviewLayout.ColumnCount = columnCount
	preview.Canvas.Set(preview.CanvasOptions{Scroll: preview.ScrollLeftToRight, Size: previewColumnWidth * float64(columnCount)})
	screen := preview.Screen.Rect()
	preview.UI.SetMenu(basicMenuLayout(screen, preview.UI.MenuConfiguration()))
	preview.UI.SetProgress(basicProgressLayout(screen, preview.UI.ProgressConfiguration()))
}
func (*PreviewStage) Render() {
	columnCount := PreviewLayout.ColumnCount
	for column := 0; column < columnCount; column++ {
		left := previewLaneX(stageLeft, column)
		right := previewLaneX(stageRight, column)
		for lane := -3; lane <= 3; lane++ {
			center := previewLaneX(float64(lane), column)
			Skin.Lane.Draw(sonolus.Rect{T: 1, R: center + previewLaneWidth/2, B: -1, L: center - previewLaneWidth/2}.ToQuad(), layerLane, 1)
		}
		Skin.LeftBorder.Draw(sonolus.Rect{T: 1, R: left, B: -1, L: left - previewStageBorderWidth}.ToQuad(), layerStage, 1)
		Skin.RightBorder.Draw(sonolus.Rect{T: 1, R: right + previewStageBorderWidth, B: -1, L: right}.ToQuad(), layerStage, 1)
	}
	screen := preview.Screen.Rect()
	coverRight := previewColumnWidth*float64(columnCount) + 1
	Skin.Cover.Draw(sonolus.Rect{T: previewYMin, R: coverRight, B: -1, L: screen.L}.ToQuad(), layerZ(layerPreviewCover, 0, 0), 1)
	Skin.Cover.Draw(sonolus.Rect{T: 1, R: coverRight, B: previewYMax, L: screen.L}.ToQuad(), layerZ(layerPreviewCover, 0, 0), 1)
	for value := 0; value <= int(math.Floor(PreviewData.LastTime)); value++ {
		time := float64(value)
		Skin.Time.Draw(previewLeftOnlyBarRect(time).ToQuad(), layerTimeLine, 0.8)
		preview.Canvas.Print(preview.PrintOptions{
			Value: time, Format: sonolus.PrintFormatTime, DecimalPlaces: 0,
			Anchor: sonolus.NewVec2(previewLaneX(stageLeft, previewColumn(time))-previewTextMarginX, previewTimeY(time)),
			Pivot:  sonolus.NewVec2(1, 0), Size: sonolus.NewVec2(previewTextWidth, previewTextHeight),
			Alpha: 1, Color: sonolus.PrintColorCyan, HorizontalAlign: sonolus.HorizontalAlignRight,
		})
	}
}

type PreviewBPMChange struct {
	preview.Archetype `archetype:"name=#BPM_CHANGE"`
	Beat              float64 `archetype:"imported,name=#BEAT"`
	BPM               float64 `archetype:"imported,name=#BPM"`
	Time              float64 `archetype:"data"`
}

func (b *PreviewBPMChange) Preprocess() {
	b.Time = preview.Time.BeatToTime(b.Beat)
}
func (b *PreviewBPMChange) Render() {
	Skin.BPM.Draw(previewExtendedBarRect(b.Time, false, true).ToQuad(), layerZ(layerBPMChangeLine, 0, 0), 0.8)
	preview.Canvas.Print(preview.PrintOptions{
		Value: b.BPM, Format: sonolus.PrintFormatBPM, DecimalPlaces: -1,
		Anchor: sonolus.NewVec2(previewLaneX(stageRight, previewColumn(b.Time))+previewTextMarginX, previewTimeY(b.Time)), Pivot: sonolus.NewVec2(0, 0), Size: sonolus.NewVec2(previewTextWidth, previewTextHeight),
		Alpha: 1, Color: sonolus.PrintColorPurple, HorizontalAlign: sonolus.HorizontalAlignLeft,
	})
	for beat := b.Beat + 4; preview.Time.BeatToStartingBeat(beat) == b.Beat && beat <= PreviewData.LastBeat; beat += 4 {
		time := preview.Time.BeatToTime(beat)
		Skin.Measure.Draw(previewBarRect(time).ToQuad(), layerMeasureLine-time/100, 0.8)
	}
}

type PreviewTimescaleChange struct {
	preview.Archetype `archetype:"name=#TIMESCALE_CHANGE"`
	Beat              float64 `archetype:"imported,name=#BEAT"`
	Timescale         float64 `archetype:"imported,name=#TIMESCALE"`
	Time              float64 `archetype:"data"`
}

func (t *PreviewTimescaleChange) Preprocess() {
	t.Time = preview.Time.BeatToTime(t.Beat)
}
func (t *PreviewTimescaleChange) Render() {
	Skin.Timescale.Draw(previewExtendedBarRect(t.Time, true, false).ToQuad(), layerZ(layerTimescaleChangeLine, 0, 0), 0.8)
	preview.Canvas.Print(preview.PrintOptions{
		Value: t.Timescale, Format: sonolus.PrintFormatTimeScale, DecimalPlaces: -1,
		Anchor: sonolus.NewVec2(previewLaneX(stageLeft, previewColumn(t.Time))-previewTextMarginX, previewTimeY(t.Time)), Pivot: sonolus.NewVec2(1, 0), Size: sonolus.NewVec2(previewTextWidth, previewTextHeight),
		Alpha: 1, Color: sonolus.PrintColorYellow, HorizontalAlign: sonolus.HorizontalAlignRight,
	})
}

type PreviewBasicNote struct {
	preview.Archetype `archetype:"abstract"`
	Beat              float64                             `archetype:"imported,name=#BEAT"`
	Lane              float64                             `archetype:"imported,name=lane"`
	Direction         float64                             `archetype:"imported,name=direction"`
	Previous          sonolus.EntityRef[PreviewBasicNote] `archetype:"imported,name=prev"`
	Next              sonolus.EntityRef[PreviewBasicNote] `archetype:"imported,name=next"`
	Time              float64                             `archetype:"data"`
}

func (n *PreviewBasicNote) Preprocess() {
	if Config.Mirror {
		n.Lane = -n.Lane
		n.Direction = -n.Direction
	}
	n.Time = preview.Time.BeatToTime(n.Beat)
	updatePreviewDuration(n.Time, n.Beat)
}

func (n *PreviewBasicNote) Render() {
	switch int(preview.Entity.Key()) {
	case 1:
		Skin.Note.Draw(previewNoteRect(n.Lane, n.Time).ToQuad(), layerZ(layerNote, n.Lane, n.Time), 1)
	case 2:
		Skin.Flick.Draw(previewNoteRect(n.Lane, n.Time).ToQuad(), layerZ(layerNote, n.Lane, n.Time), 1)
		Skin.FlickArrow.Draw(previewNoteRect(n.Lane, n.Time).Translate(sonolus.NewVec2(0, 0.9*previewLaneWidth)).ToQuad(), layerZ(layerArrow, n.Lane, n.Time), 1)
	case 3:
		if displayDirection(n.Direction) > 0 {
			drawPreviewDirectionalFlick(Skin.RightFlick, Skin.RightFlickArrow, n.Lane, n.Time, n.Direction)
		} else {
			drawPreviewDirectionalFlick(Skin.LeftFlick, Skin.LeftFlickArrow, n.Lane, n.Time, n.Direction)
		}
	case 4:
		Skin.HoldHead.Draw(previewNoteRect(n.Lane, n.Time).ToQuad(), layerZ(layerNote, n.Lane, n.Time), 1)
	case 5:
		Skin.HoldTick.Draw(previewNoteRect(n.Lane, n.Time).ToQuad(), layerZ(layerNote, n.Lane, n.Time), 1)
	case 7:
		Skin.HoldTail.Draw(previewNoteRect(n.Lane, n.Time).ToQuad(), layerZ(layerNote, n.Lane, n.Time), 1)
	}
}

func (n *PreviewBasicNote) holdHeadRef() sonolus.EntityRef[PreviewHoldHeadNote] {
	ref := preview.CurrentEntityRef[PreviewBasicNote]()
	for previous := n.Previous; previous.Index > 0; previous = previous.Get().Previous {
		ref = previous
	}
	return sonolus.EntityRefAs[PreviewHoldHeadNote](ref)
}

type PreviewTapNote struct {
	PreviewBasicNote  `archetype:"base"`
	preview.Archetype `archetype:"name=TapNote,key=1"`
}

type PreviewAccentTapNote struct {
	PreviewTapNote    `archetype:"base"`
	preview.Archetype `archetype:"name=AccentTapNote"`
}

type PreviewFlickNote struct {
	PreviewBasicNote  `archetype:"base"`
	preview.Archetype `archetype:"name=FlickNote,key=2"`
}

type PreviewDirectionalFlickNote struct {
	PreviewBasicNote  `archetype:"base"`
	preview.Archetype `archetype:"name=DirectionalFlickNote,key=3"`
}

type PreviewHoldHeadNote struct {
	PreviewBasicNote  `archetype:"base"`
	preview.Archetype `archetype:"name=HoldHeadNote,key=4"`
}

func (n *PreviewBasicNote) endBeat() float64 {
	beat := n.Beat
	for nextRef := n.Next; nextRef.Index > 0; {
		next := nextRef.Get()
		beat, nextRef = next.Beat, next.Next
	}
	return beat
}

func (n *PreviewBasicNote) endLane() float64 {
	lane := n.Lane
	for nextRef := n.Next; nextRef.Index > 0; {
		next := nextRef.Get()
		lane, nextRef = next.Lane, next.Next
	}
	return lane
}

func (n *PreviewBasicNote) laneAtBeat(beat float64) float64 {
	previousBeat, previousLane := n.Beat, n.Lane
	nextRef := n.Next
	for nextRef.Index > 0 {
		next := nextRef.Get()
		if nextRef.Key() == 6 || next.Next.Index <= 0 {
			if beat <= next.Beat || next.Next.Index <= 0 {
				return holdLane(beat, previousBeat, next.Beat, previousLane, next.Lane)
			}
			previousBeat, previousLane = next.Beat, next.Lane
		}
		nextRef = next.Next
	}
	return previousLane
}

type PreviewHoldAnchorNote struct {
	PreviewBasicNote  `archetype:"base"`
	preview.Archetype `archetype:"name=HoldAnchorNote,key=6"`
}

type PreviewHoldEndNote struct {
	PreviewBasicNote  `archetype:"base"`
	preview.Archetype `archetype:"name=HoldEndNote,key=7"`
}

type PreviewHoldFlickNote struct {
	PreviewBasicNote  `archetype:"base"`
	preview.Archetype `archetype:"name=HoldFlickNote,key=2"`
}

type PreviewHoldConnector struct {
	preview.Archetype `archetype:"name=HoldConnector"`
	First             sonolus.EntityRef[PreviewBasicNote] `archetype:"imported,name=first"`
	Second            sonolus.EntityRef[PreviewBasicNote] `archetype:"imported,name=second"`
}

func (n *PreviewHoldConnector) Render() {
	first, second := n.First.Get(), n.Second.Get()
	startTime := preview.Time.BeatToTime(first.Beat)
	endTime := preview.Time.BeatToTime(second.Beat)
	startColumn := previewColumn(startTime)
	endColumn := previewColumn(endTime)
	for column := startColumn; column <= endColumn; column++ {
		Skin.HoldConnector.Draw(previewConnectorQuad(first.Lane, second.Lane, startTime, endTime, column), layerZ(layerConnector, math.Min(first.Lane, second.Lane), math.Min(startTime, endTime)), Config.ConnectorAlpha)
	}
}

type PreviewSimLine struct {
	preview.Archetype `archetype:"name=SimLine"`
	First             sonolus.EntityRef[PreviewBasicNote] `archetype:"imported,name=first"`
	Second            sonolus.EntityRef[PreviewBasicNote] `archetype:"imported,name=second"`
}

func (n *PreviewSimLine) Render() {
	if !Config.SimLines {
		return
	}
	time := preview.Time.BeatToTime(n.First.Get().Beat)
	column := previewColumn(time)
	firstX := previewLaneX(n.First.Get().Lane, column)
	secondX := previewLaneX(n.Second.Get().Lane, column)
	y := previewTimeY(time)
	Skin.SimLine.Draw(sonolus.Rect{T: y + previewLaneWidth/8, R: secondX, B: y - previewLaneWidth/8, L: firstX}.ToQuad(), layerZ(layerSimLine, math.Min(n.First.Get().Lane, n.Second.Get().Lane), time), Config.SimLineAlpha)
}

type PreviewHoldTickNote struct {
	PreviewBasicNote  `archetype:"base"`
	preview.Archetype `archetype:"name=HoldTickNote,key=5"`
}

func (n *PreviewHoldTickNote) Preprocess() {
	n.PreviewBasicNote.Preprocess()
	n.Lane = n.holdHeadRef().Get().laneAtBeat(n.Beat)
}
