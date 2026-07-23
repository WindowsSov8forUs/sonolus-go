//go:build play

package main

import (
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
	Unit      persistentUnit
	Pair      persistentPair
	TempPair  *persistentPair
	Selected  *persistentUnit
	AutoValue persistentAutoInputImpl
	AutoOther persistentAutoInputOther
	AutoInput persistentAutoInput
	Result    float64
}

var Persistent = PersistentMemory{}

type PersistentNote struct {
	play.Archetype `archetype:"name=PersistentNote"`
}

func (*PersistentNote) Preprocess() {
	if packageInput.TempPair == nil || packageInput.Auto == nil || packageInput.Units[1].Value != 2 || packageInput.Current != 4 {
		sonolus.Terminate("package pointer-rich graph was not initialized")
	}
	packageInput.TempPair.Set(&packageInput.Units[1], nil)
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
