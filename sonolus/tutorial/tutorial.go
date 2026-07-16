// Package tutorial declares Tutorial-mode markers and runtime facades.
package tutorial

import "github.com/WindowsSov8forUs/sonolus-go/v2/sonolus"

type GlobalCallbacks struct{}

type timeAPI struct{}

func (timeAPI) Now() float64                                   { return 0 }
func (timeAPI) Delta() float64                                 { return 0 }
func (timeAPI) Scaled() float64                                { return 0 }
func (timeAPI) Previous() float64                              { return 0 }
func (timeAPI) OffsetAdjusted() float64                        { return 0 }
func (timeAPI) BeatToBPM(beat float64) float64                 { return 0 }
func (timeAPI) BeatToTime(beat float64) float64                { return 0 }
func (timeAPI) BeatToStartingBeat(beat float64) float64        { return 0 }
func (timeAPI) BeatToStartingTime(beat float64) float64        { return 0 }
func (timeAPI) TimeToScaledTime(value float64) float64         { return 0 }
func (timeAPI) TimeToStartingScaledTime(value float64) float64 { return 0 }
func (timeAPI) TimeToStartingTime(value float64) float64       { return 0 }
func (timeAPI) TimeToTimeScale(value float64) float64          { return 0 }

var Time timeAPI

type screenAPI struct{}

func (screenAPI) Rect() sonolus.Rect { return sonolus.Rect{} }

var Screen screenAPI
var SafeArea screenAPI

type environmentAPI struct{}

func (environmentAPI) Debug() bool          { return false }
func (environmentAPI) AspectRatio() float64 { return 0 }

var Environment environmentAPI

type audioAPI struct{}

func (audioAPI) Offset() float64                                       { return 0 }
func (audioAPI) Play(clip sonolus.Clip, distance float64)              {}
func (audioAPI) PlayScheduled(clip sonolus.Clip, at, distance float64) {}

var Audio audioAPI

type backgroundAPI struct{}

func (backgroundAPI) Get() sonolus.Quad  { return sonolus.Quad{} }
func (backgroundAPI) Set(q sonolus.Quad) {}

var Background backgroundAPI

type transformAPI struct{}

func (transformAPI) Get() sonolus.Transform2D  { return sonolus.Transform2D{} }
func (transformAPI) Set(t sonolus.Transform2D) {}

var SkinTransform transformAPI
var ParticleTransform transformAPI

type navigationAPI struct{}
type NavigationDirection int

const (
	NavigationPrevious NavigationDirection = -1
	NavigationNone     NavigationDirection = 0
	NavigationNext     NavigationDirection = 1
)

func (navigationAPI) Direction() NavigationDirection { return NavigationNone }

var Navigation navigationAPI

type instructionAPI struct{}

func (instructionAPI) Show(instruction sonolus.Text) {}
func (instructionAPI) Clear()                        {}
func (instructionAPI) Paint(icon sonolus.Icon, position sonolus.Vec2, size, rotation, z, alpha float64) {
}

var Instruction instructionAPI

type uiAPI struct{}

func (uiAPI) Menu() sonolus.RuntimeUIBasicLayout                { return sonolus.RuntimeUIBasicLayout{} }
func (uiAPI) SetMenu(value sonolus.RuntimeUIBasicLayout)        {}
func (uiAPI) Previous() sonolus.RuntimeUIBasicLayout            { return sonolus.RuntimeUIBasicLayout{} }
func (uiAPI) SetPrevious(value sonolus.RuntimeUIBasicLayout)    {}
func (uiAPI) Next() sonolus.RuntimeUIBasicLayout                { return sonolus.RuntimeUIBasicLayout{} }
func (uiAPI) SetNext(value sonolus.RuntimeUIBasicLayout)        {}
func (uiAPI) Instruction() sonolus.RuntimeUIBasicLayout         { return sonolus.RuntimeUIBasicLayout{} }
func (uiAPI) SetInstruction(value sonolus.RuntimeUIBasicLayout) {}
func (uiAPI) MenuConfiguration() sonolus.RuntimeUIConfiguration {
	return sonolus.RuntimeUIConfiguration{}
}
func (uiAPI) NavigationConfiguration() sonolus.RuntimeUIConfiguration {
	return sonolus.RuntimeUIConfiguration{}
}
func (uiAPI) InstructionConfiguration() sonolus.RuntimeUIConfiguration {
	return sonolus.RuntimeUIConfiguration{}
}

var UI uiAPI

type memoryAPI struct{}
type dataAPI struct{}

func (memoryAPI) Get(index int) float64        { return 0 }
func (memoryAPI) Set(index int, value float64) {}
func (dataAPI) Get(index int) float64          { return 0 }
func (dataAPI) Set(index int, value float64)   {}

var LevelData dataAPI
var LevelMemory memoryAPI
var TutorialData dataAPI
var TutorialMemory memoryAPI

type debugAPI struct{}

func (debugAPI) Log(value float64) {}
func (debugAPI) Pause()            {}

var Debug debugAPI
