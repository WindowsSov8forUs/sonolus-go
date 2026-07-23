package shared

import "github.com/WindowsSov8forUs/sonolus-go/v2/sonolus"

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

type FlickInputUnit struct {
	InputUnitIndex        int
	IsFlickedCurrentFrame bool
	MoveVector            sonolus.Vec2
	BeginLane             float64
	FingerIndex           int
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
