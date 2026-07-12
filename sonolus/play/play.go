// Package play declares Play-mode markers and runtime facades.
package play

import "github.com/WindowsSov8forUs/sonolus-go/sonolus"

type Archetype struct{}
type CallbackOrders struct{}

type EntityInfo struct {
	Index, Archetype float64
	State            sonolus.EntityState
}
type InputResult struct {
	Judgment    sonolus.Judgment
	Accuracy    float64
	Bucket      sonolus.Bucket
	BucketValue float64
	Haptic      sonolus.HapticType
}
type LifeValues struct{ Perfect, Great, Good, Miss float64 }
type ArchetypeScore struct{ Multiplier float64 }
type BaseScore struct{ Perfect, Great, Good float64 }
type ConsecutiveScore struct{ Multiplier, Step, Cap float64 }
type ConsecutiveLife struct{ Increment, Step float64 }

type timeAPI struct{}

func (timeAPI) Now() float64                          { return 0 }
func (timeAPI) Delta() float64                        { return 0 }
func (timeAPI) Scaled() float64                       { return 0 }
func (timeAPI) Previous() float64                     { return 0 }
func (timeAPI) OffsetAdjusted() float64               { return 0 }
func (timeAPI) BeatToTime(beat float64) float64       { return 0 }
func (timeAPI) TimeToScaledTime(time float64) float64 { return 0 }

var Time timeAPI

type screenAPI struct{}

func (screenAPI) Rect() sonolus.Rect { return sonolus.Rect{} }

var Screen screenAPI
var SafeArea screenAPI

type audioAPI struct{}

func (audioAPI) Offset() float64                                       { return 0 }
func (audioAPI) Play(clip sonolus.Clip, distance float64)              {}
func (audioAPI) PlayScheduled(clip sonolus.Clip, at, distance float64) {}
func (audioAPI) PlayLooped(clip sonolus.Clip) sonolus.LoopedEffectHandle {
	return sonolus.LoopedEffectHandle{}
}

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

type entityAPI struct{}

func (entityAPI) Info() EntityInfo            { return EntityInfo{} }
func (entityAPI) InfoAt(index int) EntityInfo { return EntityInfo{} }
func (entityAPI) Despawn() bool               { return false }
func (entityAPI) SetDespawn(value bool)       {}
func (entityAPI) Result() InputResult         { return InputResult{} }
func (entityAPI) SetResult(value InputResult) {}
func Spawn[T any](data T)                     {}

var Entity entityAPI

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

type touchesAPI struct{}
type Touch struct {
	ID                      float64
	Started, Ended          bool
	Time, StartTime         float64
	Position, StartPosition Vec2
	Delta, Velocity         Vec2
	Speed, Angle            float64
}
type Vec2 = sonolus.Vec2

func (touchesAPI) Count() int          { return 0 }
func (touchesAPI) Get(index int) Touch { return Touch{} }

var Touches touchesAPI

type inputAPI struct{}

func (inputAPI) Offset() float64 { return 0 }
func (inputAPI) Judge(hitTime, targetTime float64, windows sonolus.JudgmentWindows) sonolus.Judgment {
	return sonolus.JudgmentMiss
}

var Input inputAPI

type multiplayerAPI struct{}

func (multiplayerAPI) IsMultiplayer() bool { return false }

var Multiplayer multiplayerAPI

type uiAPI struct{}

func (uiAPI) Menu() sonolus.RuntimeUILayout                         { return sonolus.RuntimeUILayout{} }
func (uiAPI) SetMenu(value sonolus.RuntimeUILayout)                 {}
func (uiAPI) Judgment() sonolus.RuntimeUILayout                     { return sonolus.RuntimeUILayout{} }
func (uiAPI) SetJudgment(value sonolus.RuntimeUILayout)             {}
func (uiAPI) ComboValue() sonolus.RuntimeUILayout                   { return sonolus.RuntimeUILayout{} }
func (uiAPI) SetComboValue(value sonolus.RuntimeUILayout)           {}
func (uiAPI) ComboText() sonolus.RuntimeUILayout                    { return sonolus.RuntimeUILayout{} }
func (uiAPI) SetComboText(value sonolus.RuntimeUILayout)            {}
func (uiAPI) PrimaryMetricBar() sonolus.RuntimeUILayout             { return sonolus.RuntimeUILayout{} }
func (uiAPI) SetPrimaryMetricBar(value sonolus.RuntimeUILayout)     {}
func (uiAPI) PrimaryMetricValue() sonolus.RuntimeUILayout           { return sonolus.RuntimeUILayout{} }
func (uiAPI) SetPrimaryMetricValue(value sonolus.RuntimeUILayout)   {}
func (uiAPI) SecondaryMetricBar() sonolus.RuntimeUILayout           { return sonolus.RuntimeUILayout{} }
func (uiAPI) SetSecondaryMetricBar(value sonolus.RuntimeUILayout)   {}
func (uiAPI) SecondaryMetricValue() sonolus.RuntimeUILayout         { return sonolus.RuntimeUILayout{} }
func (uiAPI) SetSecondaryMetricValue(value sonolus.RuntimeUILayout) {}
func (uiAPI) MenuConfiguration() sonolus.RuntimeUIConfiguration {
	return sonolus.RuntimeUIConfiguration{}
}
func (uiAPI) JudgmentConfiguration() sonolus.RuntimeUIConfiguration {
	return sonolus.RuntimeUIConfiguration{}
}
func (uiAPI) ComboConfiguration() sonolus.RuntimeUIConfiguration {
	return sonolus.RuntimeUIConfiguration{}
}
func (uiAPI) PrimaryMetricConfiguration() sonolus.RuntimeUIConfiguration {
	return sonolus.RuntimeUIConfiguration{}
}
func (uiAPI) SecondaryMetricConfiguration() sonolus.RuntimeUIConfiguration {
	return sonolus.RuntimeUIConfiguration{}
}

var UI uiAPI

type environmentAPI struct{}

func (environmentAPI) Debug() bool          { return false }
func (environmentAPI) Multiplayer() bool    { return false }
func (environmentAPI) AspectRatio() float64 { return 0 }

var Environment environmentAPI

type streamsAPI struct{}

func (streamsAPI) Set(id, key, value float64) {}

var Streams streamsAPI

type debugAPI struct{}

func (debugAPI) Log(value float64) {}
func (debugAPI) Pause()            {}
func (debugAPI) Terminate()        {}

var Debug debugAPI
