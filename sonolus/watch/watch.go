// Package watch declares Watch-mode markers and runtime facades.
package watch

import "github.com/WindowsSov8forUs/sonolus-go/sonolus"

type Archetype struct{}
type CallbackOrders struct{}
type GlobalCallbacks struct{}

type EntityInfo struct {
	Index, Archetype float64
	State            sonolus.EntityState
}
type InputResult struct {
	TargetTime  float64
	Bucket      sonolus.Bucket
	BucketValue float64
}
type LifeValues struct{ Perfect, Great, Good, Miss float64 }
type ArchetypeScore struct{ Multiplier float64 }
type BaseScore struct{ Perfect, Great, Good float64 }
type ConsecutiveScore struct{ Multiplier, Step, Cap float64 }
type ConsecutiveLife struct{ Increment, Step float64 }

type timeAPI struct{}

func (timeAPI) Now() float64                    { return 0 }
func (timeAPI) Delta() float64                  { return 0 }
func (timeAPI) Scaled() float64                 { return 0 }
func (timeAPI) Previous() float64               { return 0 }
func (timeAPI) BeatToTime(beat float64) float64 { return 0 }
func (timeAPI) Skip() bool                      { return false }

var Time timeAPI

type screenAPI struct{}

func (screenAPI) Rect() sonolus.Rect { return sonolus.Rect{} }

var Screen screenAPI
var SafeArea screenAPI

type audioAPI struct{}

func (audioAPI) Offset() float64                                       { return 0 }
func (audioAPI) Play(clip sonolus.Clip, distance float64)              {}
func (audioAPI) PlayScheduled(clip sonolus.Clip, at, distance float64) {}

var Audio audioAPI

type backgroundAPI struct{}

func (backgroundAPI) Get() sonolus.Quad  { return sonolus.Quad{} }
func (backgroundAPI) Set(q sonolus.Quad) {}

var Background backgroundAPI

type entityAPI struct{}

func (entityAPI) Info() EntityInfo            { return EntityInfo{} }
func (entityAPI) InfoAt(index int) EntityInfo { return EntityInfo{} }
func (entityAPI) Result() InputResult         { return InputResult{} }

var Entity entityAPI

type replayAPI struct{}

func (replayAPI) IsReplay() bool                      { return false }
func (replayAPI) Judgment(index int) sonolus.Judgment { return sonolus.JudgmentMiss }
func (replayAPI) Accuracy(index int) float64          { return 0 }

var Replay replayAPI

type scoreAPI struct{}

func (scoreAPI) Archetype(index int) ArchetypeScore                               { return ArchetypeScore{} }
func (scoreAPI) SetArchetype(index int, value ArchetypeScore)                     {}
func (scoreAPI) Base() BaseScore                                                  { return BaseScore{} }
func (scoreAPI) SetBase(value BaseScore)                                          {}
func (scoreAPI) Consecutive(judgment sonolus.Judgment) ConsecutiveScore           { return ConsecutiveScore{} }
func (scoreAPI) SetConsecutive(judgment sonolus.Judgment, value ConsecutiveScore) {}

var Score scoreAPI

type lifeAPI struct{}

func (lifeAPI) Initial() float64                                                { return 0 }
func (lifeAPI) SetInitial(value float64)                                        {}
func (lifeAPI) Max() float64                                                    { return 0 }
func (lifeAPI) SetMax(value float64)                                            {}
func (lifeAPI) Archetype(index int) LifeValues                                  { return LifeValues{} }
func (lifeAPI) SetArchetype(index int, value LifeValues)                        {}
func (lifeAPI) Consecutive(judgment sonolus.Judgment) ConsecutiveLife           { return ConsecutiveLife{} }
func (lifeAPI) SetConsecutive(judgment sonolus.Judgment, value ConsecutiveLife) {}
func (lifeAPI) AddScheduled(value, at float64)                                  {}

var Life lifeAPI

type uiAPI struct{}

func (uiAPI) Configure(config sonolus.UIConfig) {}

var UI uiAPI

type streamsAPI struct{}

func (streamsAPI) Has(id, key float64) bool            { return false }
func (streamsAPI) PreviousKey(id, key float64) float64 { return 0 }
func (streamsAPI) NextKey(id, key float64) float64     { return 0 }
func (streamsAPI) Value(id, key float64) float64       { return 0 }

var Streams streamsAPI

type inputAPI struct{}

func (inputAPI) Offset() float64 { return 0 }
func (inputAPI) Judge(hitTime, targetTime float64, windows sonolus.JudgmentWindows) sonolus.Judgment {
	return sonolus.JudgmentMiss
}

var Input inputAPI

type debugAPI struct{}

func (debugAPI) Log(value float64) {}
func (debugAPI) Pause()            {}

var Debug debugAPI
