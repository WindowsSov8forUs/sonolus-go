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
