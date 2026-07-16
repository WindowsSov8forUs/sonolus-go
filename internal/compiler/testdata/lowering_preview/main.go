package main

import (
	"github.com/WindowsSov8forUs/sonolus-go/sonolus"
	"github.com/WindowsSov8forUs/sonolus-go/sonolus/preview"
)

type Note struct {
	preview.Archetype `archetype:"name=Note"`
}

type SkinData struct {
	sonolus.SkinResource
	Note sonolus.Sprite
}

var Skin = &SkinData{Note: sonolus.SkinSprite("note")}

func (*Note) Preprocess() {
	preview.Canvas.Set(preview.CanvasOptions{Scroll: preview.ScrollTopToBottom, Size: 10})
	preview.UI.SetMenu(preview.UI.Menu())
	preview.UI.SetProgress(preview.UI.Progress())
	value := preview.Canvas.Size() + preview.Screen.Rect().Width() + preview.SafeArea.Rect().Width()
	value += preview.UI.MenuConfiguration().Scale + preview.UI.ProgressConfiguration().Scale
	_, _ = preview.Entity.Info(), preview.Entity.InfoAt(0)
	_, _, _ = preview.Canvas.Scroll(), preview.Environment.Debug(), preview.Environment.AspectRatio()
	preview.Debug.Log(value)
	preview.Debug.Pause()
}

func (*Note) Render() {
	preview.Canvas.Print(preview.PrintOptions{
		Value: 1, Format: sonolus.PrintFormatNumber, DecimalPlaces: 2,
		Anchor: sonolus.NewVec2(0, 0), Pivot: sonolus.NewVec2(0.5, 0.5), Size: sonolus.NewVec2(1, 1),
		Alpha: 1,
	})
	Skin.Note.Draw(sonolus.Rect{T: 1, R: 1, B: -1, L: -1}.ToQuad(), 0, 1)
}

func main() {}
