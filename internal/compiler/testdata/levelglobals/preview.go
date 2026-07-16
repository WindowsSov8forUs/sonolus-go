//go:build preview

package main

import (
	"github.com/WindowsSov8forUs/sonolus-go/v2/sonolus"
	"github.com/WindowsSov8forUs/sonolus-go/v2/sonolus/preview"
)

type PreviewFirstData struct {
	sonolus.LevelDataResource
	LastTime float64
}

var FirstData = PreviewFirstData{}

type PreviewLayoutData struct {
	sonolus.LevelDataResource
	Columns int
}

var LayoutData = PreviewLayoutData{}

type PreviewGlobalNote struct {
	preview.Archetype `archetype:"name=GlobalNote"`
}

func (*PreviewGlobalNote) Preprocess() {
	FirstData.LastTime = 10
	LayoutData.Columns = 2
}

func (*PreviewGlobalNote) Render() { _ = FirstData.LastTime + float64(LayoutData.Columns) }
