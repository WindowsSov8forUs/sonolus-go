package shared

import "github.com/WindowsSov8forUs/sonolus-go/v2/sonolus"

type CallbackAggregate struct {
	Started     bool
	Ended       bool
	Time        float64
	Position    sonolus.Vec2
	Delta       sonolus.Vec2
	DeltaTime   float64
	FingerIndex int
}

type AggregateProvider struct {
	LaneCount          int
	FlickDistance      float64
	Positions          [24]sonolus.Vec2
	PositionCount      int
	JudgementMaxOffset sonolus.Vec2
}

type AggregateInput struct {
	CurrentMusicTimeMs int
}

func (AggregateProvider) Update(input *AggregateInput, values [4]CallbackAggregate, count int) {
	for index := 0; index < count; index++ {
		input.CurrentMusicTimeMs = values[index].FingerIndex
	}
}

type Unit struct {
	Value float64
}

type Pair struct {
	Unit *Unit
}

var Root = &Pair{Unit: &Unit{Value: 9}}

const InputCapacity = 24

type InputUnit struct {
	UnitIndex      int
	InputState     int
	ScreenPosition sonolus.Vec2
	Lane           float64
	InputTimeMs    int
	FingerIndex    int
}

func (unit *InputUnit) Clear() {
	unit.InputState = 0
	unit.ScreenPosition = sonolus.Vec2{}
	unit.Lane = -1
	unit.InputTimeMs = -1
	unit.FingerIndex = -1
}

type FlickInputUnit struct {
	InputUnitIndex        int
	IsFlickedCurrentFrame bool
	MoveVector            sonolus.Vec2
	BeginLane             float64
	FingerIndex           int
}

func (unit *FlickInputUnit) Clear() {
	unit.IsFlickedCurrentFrame = false
	unit.MoveVector = sonolus.Vec2{}
	unit.BeginLane = -1
	unit.FingerIndex = -1
}

type InputPair struct {
	InputUnit      *InputUnit
	FlickInputUnit *FlickInputUnit
}

func (pair *InputPair) Set(inputUnit *InputUnit, flickInputUnit *FlickInputUnit) {
	pair.InputUnit = inputUnit
	pair.FlickInputUnit = flickInputUnit
}

type AutoInput interface {
	Apply(float64) float64
}

type DefaultAutoInput struct {
	Bias float64
	Last float64
}

func (input *DefaultAutoInput) Apply(value float64) float64 {
	input.Last = value
	return input.Bias + value
}

type Input struct {
	TempPair                           *InputPair
	DirectionFlickAngle                float64
	LaneCount                          int
	CurrentMusicTimeMs                 int
	CurrentFrameRealtimeSinceStartupMs int
	InputStateArray                    [InputCapacity]InputUnit
	CurrentFrameFlickStateArray        [InputCapacity]FlickInputUnit
	CurrentFrameChanceFlickStateArray  [InputCapacity]FlickInputUnit
	AutoInput                          AutoInput
}

var InputRoot = &Input{
	TempPair:  &InputPair{},
	LaneCount: InputCapacity,
	AutoInput: &DefaultAutoInput{Bias: 3},
}

func GetInputRoot() *Input { return InputRoot }

type InputWrapperMemory struct {
	sonolus.LevelMemoryResource
	Input Input
	Ref   *Input
}

var InputWrapper = InputWrapperMemory{}

func GetWrappedInput() *Input { return &InputWrapper.Input }
