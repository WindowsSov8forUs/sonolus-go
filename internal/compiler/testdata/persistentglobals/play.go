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

type persistentEntityPackageHolder struct{ Input shared.AutoInput }

var packageEntityHolder = &persistentEntityPackageHolder{}

func (holder *persistentEntityPackageHolder) Dispatch(value float64) float64 {
	input := holder.Input
	return input.Apply(value)
}

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

type persistentEntityAutoCarrier struct {
	play.Archetype `archetype:"name=PersistentEntityAutoCarrier"`
	Bias           float64 `archetype:"shared"`
}

func (input *persistentEntityAutoCarrier) Apply(value float64) float64 {
	input.Bias += value
	return input.Bias
}

var sharedAutoRoot = &shared.DefaultAutoInput{Bias: 10}
var sharedAutoOtherRoot = &persistentAutoInputOther{Factor: 3}
var packageLifecycleRoot = &persistentUnit{Value: 7}
var packageUpdateOnlyRoot = &persistentUnit{Value: 13}

const (
	minInt32 = -1 << 31
	maxInt32 = 1<<31 - 1
)

func addRuntimeInt32(left, right int) int {
	value := int64(left) + int64(right)
	if value > maxInt32 {
		value -= 1 << 32
	}
	if value < minInt32 {
		value += 1 << 32
	}
	return int(value)
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

type PersistentLifecycleWriter struct {
	play.Archetype `archetype:"name=PersistentLifecycleWriter"`
}

func (*PersistentLifecycleWriter) Preprocess() {
	packageLifecycleRoot.Value = 24
}

type PersistentLifecycleReader struct {
	play.Archetype `archetype:"name=PersistentLifecycleReader"`
}

func (*PersistentLifecycleReader) Preprocess() {
	Persistent.Result = packageLifecycleRoot.Value
}

type PersistentUpdateOnly struct {
	play.Archetype `archetype:"name=PersistentUpdateOnly"`
}

func (*PersistentUpdateOnly) UpdateSequential() {
	Persistent.Result = packageUpdateOnlyRoot.Value
}

type PersistentInterfaceCarrier struct {
	play.Archetype `archetype:"name=PersistentInterfaceCarrier"`
	SharedAuto     shared.AutoInput `archetype:"shared"`
}

type PersistentEntityInterfaceCarrier struct {
	play.Archetype `archetype:"name=PersistentEntityInterfaceCarrier"`
	Target         sonolus.EntityRef[persistentEntityAutoCarrier]            `archetype:"shared"`
	NilTarget      sonolus.EntityRef[persistentEntityAutoCarrier]            `archetype:"shared"`
	Destination    sonolus.EntityRef[PersistentEntityInterfaceTargetCarrier] `archetype:"shared"`
	SharedAuto     shared.AutoInput                                          `archetype:"shared"`
}

type PersistentEntityInterfaceTargetCarrier struct {
	play.Archetype `archetype:"name=PersistentEntityInterfaceTargetCarrier"`
	CopiedAuto     shared.AutoInput `archetype:"shared"`
}

func (carrier *PersistentEntityInterfaceCarrier) Auto() shared.AutoInput {
	return carrier.Target.Get()
}

func (carrier *PersistentEntityInterfaceCarrier) NilAuto() shared.AutoInput {
	if carrier.NilTarget.Index < 0 {
		return nil
	}
	return carrier.NilTarget.Get()
}

type RuntimeInt64Carrier struct {
	play.Archetype `archetype:"name=RuntimeInt64Carrier"`
	Value          int `archetype:"shared"`
}

func (carrier *RuntimeInt64Carrier) Preprocess() {
	Persistent.Result = float64(addRuntimeInt32(carrier.Value, 2000))
}

type NullableEntityCarrier struct {
	play.Archetype `archetype:"name=NullableEntityCarrier"`
	Enabled        bool    `archetype:"shared"`
	Value          float64 `archetype:"shared"`
}

func nullableEntityCarrier(enabled bool) *NullableEntityCarrier {
	if !enabled {
		return nil
	}
	return play.CurrentEntityRef[NullableEntityCarrier]().Get()
}

func (carrier *NullableEntityCarrier) Preprocess() {
	view := nullableEntityCarrier(carrier.Enabled)
	if view == nil {
		Persistent.Result = -1
		return
	}
	view.Value++
	updated := view.Value
	view = nil
	if view != nil {
		Persistent.Result = -2
		return
	}
	Persistent.Result = updated
}

func (carrier *PersistentInterfaceCarrier) Preprocess() {
	if carrier.SharedAuto != nil {
		Persistent.Result = -1
		return
	}
	carrier.SharedAuto = sharedAutoRoot
	if _, ok := carrier.SharedAuto.(*shared.DefaultAutoInput); !ok {
		Persistent.Result = -2
		return
	}
	localAuto := carrier.SharedAuto
	carrier.SharedAuto = nil
	Persistent.Result = localAuto.Apply(1)
	carrier.SharedAuto = sharedAutoOtherRoot
}

func (*PersistentInterfaceCarrier) UpdateSequential() {
	view := play.CurrentEntityRef[PersistentInterfaceCarrier]().Get()
	if _, ok := view.SharedAuto.(*persistentAutoInputOther); !ok {
		Persistent.Result = -3
		return
	}
	direct := view.SharedAuto.Apply(4)
	localAuto := view.SharedAuto
	view.SharedAuto = nil
	copied := localAuto.Apply(5)
	Persistent.Result = direct + copied
}

func (carrier *PersistentEntityInterfaceCarrier) Preprocess() {
	target := carrier.Target.Get()
	target.Bias = 30
	var provider interface {
		Auto() shared.AutoInput
		NilAuto() shared.AutoInput
	} = play.CurrentEntityRef[PersistentEntityInterfaceCarrier]().Get()
	result := provider.Auto().Apply(1)
	if auto := provider.NilAuto(); auto != nil {
		result += auto.Apply(1)
	}
	Persistent.Result = result
	target.Bias = 30
	carrier.SharedAuto = target
	destination := carrier.Destination.Get()
	destination.CopiedAuto = carrier.SharedAuto
	packageEntityHolder.Input = destination.CopiedAuto
	carrier.SharedAuto = carrier.NilTarget.Get()
	if carrier.SharedAuto == nil {
		sonolus.Terminate("typed nil entity interface became nil")
	}
	typedNil, ok := carrier.SharedAuto.(*persistentEntityAutoCarrier)
	if !ok {
		sonolus.Terminate("typed nil entity interface lost its concrete type")
	}
	if (typedNil == nil) != (carrier.NilTarget.Index < 0) {
		sonolus.Terminate("typed nil entity interface nil state diverged")
	}
	carrier.SharedAuto = nil
}

func (carrier *PersistentEntityInterfaceCarrier) UpdateSequential() {
	if carrier.SharedAuto != nil {
		sonolus.Terminate("cleared entity interface was not nil")
	}
	destination := carrier.Destination.Get()
	if _, ok := destination.CopiedAuto.(*persistentEntityAutoCarrier); !ok {
		sonolus.Terminate("entity interface assertion failed")
	}
	local := destination.CopiedAuto
	direct := local.Apply(2)
	nested := packageEntityHolder.Dispatch(3)
	entity, ok := packageEntityHolder.Input.(*persistentEntityAutoCarrier)
	if !ok || entity != carrier.Target.Get() || entity.Bias != 35 {
		sonolus.Terminate("entity interface lost identity")
	}
	Persistent.Result = direct + nested
	packageEntityHolder.Input = nil
	destination.CopiedAuto = packageEntityHolder.Input
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
