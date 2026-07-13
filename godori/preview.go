//go:build preview

package main

import (
	"github.com/WindowsSov8forUs/sonolus-go/v2/sonolus"
	"github.com/WindowsSov8forUs/sonolus-go/v2/sonolus/preview"
)

type PreviewSkin struct {
	sonolus.SkinResource
	Lane, Note, BPM sonolus.Sprite
}

var Skin = &PreviewSkin{
	SkinResource: sonolus.SkinResource{RenderMode: sonolus.RenderModeLightweight},
	Lane:         sonolus.SkinSprite(sonolus.StandardSpriteLane), Note: sonolus.SkinSprite(sonolus.StandardSpriteNoteHeadCyan),
	BPM: sonolus.SkinSprite(sonolus.StandardSpriteGridPurple),
}

type PreviewStage struct {
	preview.Archetype `archetype:"name=Stage"`
}

func (*PreviewStage) Preprocess() {
	preview.Canvas.Set(preview.CanvasOptions{Scroll: preview.ScrollTopToBottom, Size: 12})
}
func (*PreviewStage) Render() {
	Skin.Lane.Draw(sonolus.Rect{T: 12, R: stageRight * Config.LaneWidth, B: 0, L: stageLeft * Config.LaneWidth}.ToQuad(), 10, 0.8)
}

type PreviewBPMChange struct {
	preview.Archetype `archetype:"name=#BPM_CHANGE"`
	Beat              float64 `archetype:"imported,name=#BEAT"`
	BPM               float64 `archetype:"imported,name=#BPM"`
	Time              float64 `archetype:"data"`
}

func (b *PreviewBPMChange) Preprocess() { b.Time = b.Beat * 0.5 }
func (b *PreviewBPMChange) Render() {
	Skin.BPM.Draw(sonolus.Rect{T: b.Time + 0.02, R: stageRight, B: b.Time - 0.02, L: stageLeft}.ToQuad(), 20, 0.7)
	preview.Canvas.Print(preview.PrintOptions{
		Value: b.BPM, Format: sonolus.PrintFormatBPM, DecimalPlaces: 0,
		Anchor: sonolus.NewVec2(stageRight+0.2, b.Time), Pivot: sonolus.NewVec2(0, 0.5), Size: sonolus.NewVec2(1, 0.35),
		Alpha: 1, Color: sonolus.PrintColorPurple, HorizontalAlign: sonolus.HorizontalAlignLeft,
	})
}

type PreviewTapNote struct {
	preview.Archetype `archetype:"name=TapNote"`
	Beat              float64 `archetype:"imported,name=#BEAT"`
	Lane              float64 `archetype:"imported,name=lane"`
}

func (n *PreviewTapNote) Render() {
	y := n.Beat * 0.5
	Skin.Note.Draw(noteRect(n.Lane, y).ToQuad(), 30, 1)
}
