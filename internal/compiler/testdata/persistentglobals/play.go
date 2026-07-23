//go:build play

package main

import (
	"github.com/WindowsSov8forUs/sonolus-go/v2/internal/compiler/testdata/persistentglobals/shared"
	"github.com/WindowsSov8forUs/sonolus-go/v2/sonolus"
	"github.com/WindowsSov8forUs/sonolus-go/v2/sonolus/play"
)

type persistentUnit struct {
	Value float64
}

type persistentPair struct {
	Unit  *persistentUnit
	Other *persistentUnit
}

var packageTempPair = new(persistentPair)
var packageInitialPair = &persistentPair{Unit: &persistentUnit{Value: 7}}

type persistentPackageHolder struct{ Input persistentAutoInput }

var packageHolder = &persistentPackageHolder{Input: &persistentAutoInputImpl{Bias: 5}}

type persistentPackageInput struct {
	TempPair *persistentPair
	Units    [2]persistentUnit
	Auto     persistentAutoInput
	Current  int
}

var packageInput = &persistentPackageInput{
	TempPair: &persistentPair{},
	Units:    [2]persistentUnit{{Value: 1}, {Value: 2}},
	Auto:     &persistentAutoInputImpl{Bias: 3},
	Current:  4,
}

func setPackagePair(index int) {
	packageInput.TempPair.Set(&packageInput.Units[index], nil)
}

func packageInputRoot() *persistentPackageInput { return packageInput }

type persistentWrapper struct {
	Input persistentPackageInput
	Ref   *persistentPackageInput
}

var packageWrapper = &persistentWrapper{}

func bindWrapperInput() {
	packageWrapper.Ref = &packageWrapper.Input
}

func (pair *persistentPair) Set(unit, other *persistentUnit) *persistentPair {
	pair.Unit = unit
	pair.Other = other
	return pair
}

func (pair *persistentPair) Clear() {
	pair.Unit = nil
	pair.Other = nil
}

type persistentAutoInput interface {
	Apply(float64) float64
}

type persistentAutoInputImpl struct {
	Bias float64
	Last float64
}

func (input *persistentAutoInputImpl) Apply(value float64) float64 {
	input.Last = value
	input.Bias += value
	return input.Bias
}

type persistentAutoInputOther struct {
	Factor float64
}

func (input *persistentAutoInputOther) Apply(value float64) float64 {
	return input.Factor * value
}

type PersistentMemory struct {
	sonolus.LevelMemoryResource
	Unit                persistentUnit
	Pair                persistentPair
	TempPair            *persistentPair
	Selected            *persistentUnit
	AutoValue           persistentAutoInputImpl
	AutoOther           persistentAutoInputOther
	AutoInput           persistentAutoInput
	Result              float64
	ProviderValue       shared.AggregateProvider
	Provider            *shared.AggregateProvider
	AggregateInputValue shared.AggregateInput
	AggregateInput      *shared.AggregateInput
}

type PersistentWrapperMemory struct {
	sonolus.LevelMemoryResource
	Input persistentPackageInput
	Ref   *persistentPackageInput
}

var PersistentWrapper = PersistentWrapperMemory{}

var Persistent = PersistentMemory{}

type PersistentNote struct {
	play.Archetype `archetype:"name=PersistentNote"`
}

func (*PersistentNote) Preprocess() {
	Persistent.Provider = &Persistent.ProviderValue
	Persistent.AggregateInput = &Persistent.AggregateInputValue
	shared.InputWrapper.Ref = &shared.InputWrapper.Input
	wrappedInput := shared.GetWrappedInput()
	wrappedInput.LaneCount = 6
	if shared.InputWrapper.Ref != wrappedInput || shared.InputWrapper.Ref.LaneCount != 6 {
		sonolus.Terminate("nested persistent input address lost identity")
	}
	packagePairBeforeReset := packageInput.TempPair
	packageAutoBeforeReset := packageInput.Auto
	*packageInput = persistentPackageInput{
		TempPair: packagePairBeforeReset,
		Units:    [2]persistentUnit{{Value: 1}, {Value: 2}},
		Auto:     packageAutoBeforeReset,
		Current:  4,
	}
	sharedInput := shared.GetInputRoot()
	sharedIndex := sharedInput.LaneCount - 1
	sharedInput.InputStateArray[sharedIndex].Lane = 7
	sharedInput.CurrentFrameFlickStateArray[sharedIndex].BeginLane = 8
	unit := &sharedInput.InputStateArray[sharedIndex]
	flickUnit := &sharedInput.CurrentFrameFlickStateArray[sharedIndex]
	unit.Clear()
	flickUnit.Clear()
	if unit.Lane != -1 || flickUnit.BeginLane != -1 {
		sonolus.Terminate("dynamic input pointer receiver lost identity")
	}
	unit.Lane = 7
	flickUnit.BeginLane = 8
	sharedInput.InputStateArray[sharedIndex].Clear()
	sharedInput.CurrentFrameFlickStateArray[sharedIndex].Clear()
	if sharedInput.InputStateArray[sharedIndex].Lane != -1 || sharedInput.CurrentFrameFlickStateArray[sharedIndex].BeginLane != -1 {
		sonolus.Terminate("dynamic indexed input method receiver lost identity")
	}
	unit.Lane = 7
	flickUnit.BeginLane = 8
	sharedInput.TempPair.Set(
		unit,
		flickUnit,
	)
	if sharedInput.TempPair.InputUnit.Lane != 7 || sharedInput.TempPair.FlickInputUnit.BeginLane != 8 {
		sonolus.Terminate("dynamic persistent input address lost identity")
	}
	sharedInput.CurrentMusicTimeMs = int(sharedInput.AutoInput.Apply(2))
	if sharedInput.CurrentMusicTimeMs != 5 {
		sonolus.Terminate("cross-package persistent input interface dispatch failed")
	}
	PersistentWrapper.Ref = &PersistentWrapper.Input
	bindWrapperInput()
	if shared.Root == nil || shared.Root.Unit == nil || shared.Root.Unit.Value != 9 {
		sonolus.Terminate("cross-package persistent graph was not initialized")
	}
	if packageInputRoot() != packageInput {
		sonolus.Terminate("package root helper identity changed")
	}
	if packageInput.TempPair == nil || packageInput.Auto == nil || packageInput.Units[1].Value != 2 || packageInput.Current != 4 {
		sonolus.Terminate("package pointer-rich graph was not initialized")
	}
	setPackagePair(1)
	if packageInput.Auto.Apply(packageInput.TempPair.Unit.Value) != 5 {
		sonolus.Terminate("package pointer-rich graph dispatch failed")
	}
	if packageHolder.Input == nil || packageHolder.Input.Apply(1) != 6 {
		sonolus.Terminate("package initial interface graph was not initialized")
	}
	if packageInitialPair.Unit == nil || packageInitialPair.Unit.Value != 7 {
		sonolus.Terminate("package initial pointer graph was not initialized")
	}
	packageTempPair.Set(&Persistent.Unit, nil)
	if Persistent.AutoInput != nil {
		sonolus.Terminate("persistent interface zero value is not nil")
	}
	if _, ok := Persistent.AutoInput.(*persistentAutoInputImpl); ok {
		sonolus.Terminate("nil persistent interface assertion succeeded")
	}
	switch Persistent.AutoInput.(type) {
	case *persistentAutoInputImpl:
		sonolus.Terminate("nil persistent interface type switch matched")
	default:
	}
	Persistent.Unit.Value = 3
	Persistent.TempPair = &Persistent.Pair
	Persistent.TempPair.Set(&Persistent.Unit, nil)
	Persistent.AutoOther.Factor = 2
	Persistent.AutoInput = &Persistent.AutoValue
}

func (*PersistentNote) UpdateSequential() {
	var callbackValues [4]shared.CallbackAggregate
	callbackValues[0].FingerIndex = 11
	Persistent.Provider.Update(Persistent.AggregateInput, callbackValues, 1)
	if Persistent.AggregateInput.CurrentMusicTimeMs != 11 {
		sonolus.Terminate("callback aggregate parameter lost")
	}
	packageInitialPair.Unit.Value++
	packagePair := packageTempPair
	packagePair.Set(packagePair.Unit, packagePair.Other)
	if packagePair != packageTempPair {
		sonolus.Terminate("package persistent pair identity changed")
	}
	pair := Persistent.TempPair
	if pair != Persistent.TempPair {
		sonolus.Terminate("persistent pair identity changed")
	}
	pair.Clear()
	pair.Set(&Persistent.Unit, nil)
	pair.Unit.Value++
	Persistent.Selected = pair.Unit
	Persistent.Result = Persistent.AutoInput.Apply(Persistent.Selected.Value)
	Persistent.AutoInput = &Persistent.AutoOther
	if _, ok := Persistent.AutoInput.(*persistentAutoInputOther); !ok {
		sonolus.Terminate("persistent interface assertion failed")
	}
	Persistent.Result += Persistent.AutoInput.Apply(Persistent.Selected.Value)
}
