// Package preview declares Preview-mode markers and runtime facades.
package preview

import "github.com/WindowsSov8forUs/sonolus-go/v2/sonolus"

type Archetype struct{}
type CallbackOrders struct{}

type timeAPI struct{}

func (timeAPI) BeatToBPM(beat float64) float64                 { return 0 }
func (timeAPI) BeatToTime(beat float64) float64                { return 0 }
func (timeAPI) BeatToStartingBeat(beat float64) float64        { return 0 }
func (timeAPI) BeatToStartingTime(beat float64) float64        { return 0 }
func (timeAPI) TimeToScaledTime(value float64) float64         { return 0 }
func (timeAPI) TimeToStartingScaledTime(value float64) float64 { return 0 }
func (timeAPI) TimeToStartingTime(value float64) float64       { return 0 }
func (timeAPI) TimeToTimeScale(value float64) float64          { return 0 }

var Time timeAPI

type EntityInfo struct{ Index, Archetype float64 }
type Scroll int
type CanvasOptions struct {
	Scroll Scroll
	Size   float64
}
type PrintOptions struct {
	Value           float64
	Format          sonolus.PrintFormat
	DecimalPlaces   int
	Anchor, Pivot   sonolus.Vec2
	Size            sonolus.Vec2
	Rotation, Alpha float64
	Color           sonolus.PrintColor
	HorizontalAlign sonolus.HorizontalAlign
	Background      bool
}

const (
	ScrollLeftToRight Scroll = iota
	ScrollTopToBottom
	ScrollRightToLeft
	ScrollBottomToTop
)

type canvasAPI struct{}

func (canvasAPI) Scroll() Scroll             { return ScrollLeftToRight }
func (canvasAPI) Size() float64              { return 0 }
func (canvasAPI) Set(options CanvasOptions)  {}
func (canvasAPI) Print(options PrintOptions) {}

var Canvas canvasAPI

type levelDataAPI struct{}

func (levelDataAPI) Get(index int) float64        { return 0 }
func (levelDataAPI) Set(index int, value float64) {}

var LevelData levelDataAPI

type screenAPI struct{}

func (screenAPI) Rect() sonolus.Rect { return sonolus.Rect{} }

var Screen screenAPI
var SafeArea screenAPI

type transformAPI struct{}

func (transformAPI) Get() sonolus.Transform2D  { return sonolus.Transform2D{} }
func (transformAPI) Set(t sonolus.Transform2D) {}

var SkinTransform transformAPI

type environmentAPI struct{}

func (environmentAPI) Debug() bool          { return false }
func (environmentAPI) AspectRatio() float64 { return 0 }

var Environment environmentAPI

type entityAPI struct{}

func (entityAPI) Info() EntityInfo            { return EntityInfo{} }
func (entityAPI) InfoAt(index int) EntityInfo { return EntityInfo{} }
func (entityAPI) Key() float64                { return 0 }
func ArchetypeID[T any]() int                 { return -1 }
func ArchetypeKey[T any]() float64            { return -1 }
func CurrentEntityRef[T any]() sonolus.EntityRef[T] {
	return sonolus.EntityRef[T]{}
}

var Entity entityAPI

type uiAPI struct{}

func (uiAPI) Menu() sonolus.RuntimeUIBasicLayout             { return sonolus.RuntimeUIBasicLayout{} }
func (uiAPI) SetMenu(value sonolus.RuntimeUIBasicLayout)     {}
func (uiAPI) Progress() sonolus.RuntimeUIBasicLayout         { return sonolus.RuntimeUIBasicLayout{} }
func (uiAPI) SetProgress(value sonolus.RuntimeUIBasicLayout) {}
func (uiAPI) MenuConfiguration() sonolus.RuntimeUIConfiguration {
	return sonolus.RuntimeUIConfiguration{}
}
func (uiAPI) ProgressConfiguration() sonolus.RuntimeUIConfiguration {
	return sonolus.RuntimeUIConfiguration{}
}

var UI uiAPI

type debugAPI struct{}

func (debugAPI) Log(value float64) {}
func (debugAPI) Pause()            {}

var Debug debugAPI
