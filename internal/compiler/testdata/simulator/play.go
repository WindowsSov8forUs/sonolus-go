//go:build play

package main

import (
	"iter"
	"math"

	"github.com/WindowsSov8forUs/sonolus-go/v2/sonolus"
	"github.com/WindowsSov8forUs/sonolus-go/v2/sonolus/play"
)

type Note struct {
	play.Archetype `archetype:"name=Note"`
	Value          float64 `archetype:"memory"`
}

func (note *Note) Preprocess() {
	sonolus.Assert(note.Value >= 0, "note value must be nonnegative")
	note.Value = 2
	play.Debug.Log(note.Value + 3)
}

type MetaNote struct {
	play.Archetype `archetype:"name=MetaNote"`
	Value          float64 `archetype:"memory"`
}

func (note *MetaNote) Preprocess() {
	if sonolus.RuntimeChecksEnabled() {
		note.Value = 1
	} else {
		note.Value = 2
	}
	if false {
		sonolus.Unreachable("constant-pruned unreachable branch was lowered")
	}
}

type ControlNote struct {
	play.Archetype `archetype:"name=ControlNote"`
	Selector       float64 `archetype:"imported,name=selector"`
	Value          float64 `archetype:"memory"`
}

func simulatorLabeledControl(selector int) int {
	result := 0
outer:
	for index := range 3 {
		for inner := 0; inner < 3; inner++ {
			if inner == 1 {
				continue outer
			}
			result += index + inner
		}
	}
choice:
	switch selector {
	case 0:
		result++
		fallthrough
	case 1:
		result += 2
	case 2:
		break choice
	}
done:
	for {
		break done
	}
	return result
}

func (note *ControlNote) Preprocess() {
	note.Value = float64(simulatorLabeledControl(int(note.Selector)))
}

type RangeTarget struct {
	play.Archetype `archetype:"name=RangeTarget"`
	Values         [2]float64 `archetype:"data"`
}

type ViewRangeNote struct {
	play.Archetype `archetype:"name=ViewRangeNote"`
	Target         sonolus.EntityRef[RangeTarget] `archetype:"imported,name=target"`
	Other          sonolus.EntityRef[RangeTarget] `archetype:"imported,name=other"`
	Selector       float64                        `archetype:"imported,name=selector"`
	Value          float64                        `archetype:"memory"`
}

type rangeTargetHolder struct {
	Target *RangeTarget
	Weight float64
}

func (note *ViewRangeNote) Preprocess() {
	holder := rangeTargetHolder{Target: note.Target.Get(), Weight: 2}
	if note.Selector > 0 {
		holder.Target = note.Other.Get()
	}
	for _, value := range holder.Target.Values {
		note.Value += value * holder.Weight
	}
}

type simulatorInt int

type ArithmeticNote struct {
	play.Archetype `archetype:"name=ArithmeticNote"`
	Divisor        float64 `archetype:"imported,name=divisor"`
	Operation      float64 `archetype:"imported,name=operation"`
	Value          float64 `archetype:"memory"`
}

func tracedSimulatorDivisor(value simulatorInt) simulatorInt {
	play.Debug.Log(float64(value))
	return value
}

func (note *ArithmeticNote) Preprocess() {
	divisor := simulatorInt(note.Divisor)
	var result simulatorInt
	switch int(note.Operation) {
	case 0:
		result = 12 / tracedSimulatorDivisor(divisor)
	case 1:
		result = 12 % tracedSimulatorDivisor(divisor)
	case 2:
		result = 12
		result /= tracedSimulatorDivisor(divisor)
	default:
		result = 12
		result %= tracedSimulatorDivisor(divisor)
	}
	note.Value = float64(result)
	play.Debug.Log(note.Value)
}

type simulatorNumber interface{ Number() float64 }

type simulatorPlain struct{ Value float64 }
type simulatorDouble struct{ Value float64 }

func (value simulatorPlain) Number() float64  { return value.Value }
func (value simulatorDouble) Number() float64 { return value.Value * 2 }

func chooseSimulatorNumber(doubled bool, value float64) simulatorNumber {
	if doubled {
		return simulatorDouble{Value: value}
	}
	return simulatorPlain{Value: value}
}

func simulatorIncrement(value float64) float64 { return value + 1 }
func simulatorDecrement(value float64) float64 { return value - 1 }

func chooseSimulatorCallable(increment bool) func(float64) float64 {
	if increment {
		return simulatorIncrement
	}
	return simulatorDecrement
}

func simulatorPointer(selector bool, values *[2]float64) *float64 {
	if selector {
		return &values[1]
	}
	return &values[0]
}

func simulatorNamedNilPointer() (pointer *int) { return }

func simulatorNewPointerValue(selector int) float64 {
	values := new([2]int)
	values[0], values[1] = 4, 6
	selected := simulatorNamedNilPointer()
	if selected != (*int)(nil) {
		return -1
	}
	if selector != 0 {
		selected = &values[selector-1]
	}
	if selected == nil {
		return 1
	}
	*selected += 2
	return float64(*selected + values[0] + values[1])
}

func simulatorAdder(delta float64) func(float64) float64 {
	return func(value float64) float64 { return value + delta }
}

func simulatorContainerValue() float64 {
	values := sonolus.NewVarArray[int](6)
	values.Append(3)
	values.Append(1)
	values.Extend(func(yield func(int) bool) {
		if !yield(2) {
			return
		}
		yield(4)
	})
	less := func(left, right int) bool { return left < right }
	last := values.LastIndex(1)
	minimumIndex := values.IndexMinFunc(less)
	maximumIndex := values.IndexMaxFunc(less)
	minimum := values.MinFunc(less)
	maximum := values.MaxFunc(less)
	result := last + minimumIndex + maximumIndex
	result += minimum + maximum
	unchecked := sonolus.NewVarArray[int](3)
	unchecked.AppendUnchecked(5)
	unchecked.AppendUnchecked(6)
	unchecked.AppendUnchecked(7)
	unchecked.SetUnchecked(1, 8)
	unchecked.SwapUnchecked(0, 2)
	result += unchecked.GetUnchecked(0)
	for index, value := range unchecked.Items() {
		result += index + value
	}
	for value := range unchecked.ValuesReversed() {
		result += value
	}
	mapping := sonolus.NewArrayMap[int, int](1)
	mapping.Set(1, 2)
	if mapping.IsFull() {
		result++
	}
	set := sonolus.NewArraySet[int](1)
	set.Add(1)
	if set.IsFull() {
		result++
	}
	return float64(result)
}

func chooseSimulatorContainer(selector bool, left, right sonolus.VarArray[int]) sonolus.VarArray[int] {
	if selector {
		return right
	}
	return left
}

func chooseSimulatorMap(selector bool, left, right sonolus.ArrayMap[int, int]) sonolus.ArrayMap[int, int] {
	if selector {
		return right
	}
	return left
}

func chooseSimulatorSet(selector bool, left, right sonolus.ArraySet[int]) sonolus.ArraySet[int] {
	if selector {
		return right
	}
	return left
}

func simulatorContainerVariantValue(selector int) float64 {
	left := sonolus.NewVarArray[int](4)
	left.Append(10)
	right := sonolus.NewVarArray[int](6)
	right.Append(20)
	right.Append(30)
	selected := chooseSimulatorContainer(selector != 0, left, right)
	selected.Append(1)
	selected.SortFunc(func(left, right int) bool { return left < right })
	selected.Extend(func(yield func(int) bool) { yield(3) })
	alias := left
	if selector != 0 {
		alias = right
	}
	alias.Set(0, 5)
	result := selected.Len() + selected.Capacity()
	for _, value := range selected {
		result += value
	}
	for value := range selected.ValuesReversed() {
		result += value
	}
	for index, value := range selected.Items() {
		result += index + value
	}
	less := func(left, right int) bool { return left < right }
	result += selected.MinFunc(less) + selected.MaxFunc(less)
	if selector == 0 {
		result += right.Get(0)
	} else {
		result += left.Get(0)
	}
	sequence := selected.Values()
	ranged := selected
	replacement := right
	if selector != 0 {
		replacement = left
	}
	selected = replacement
	for value := range sequence {
		result += value
	}
	for _, value := range ranged {
		ranged = replacement
		result += value
	}
	leftMap := sonolus.NewArrayMap[int, int](2)
	leftMap.Set(1, 10)
	rightMap := sonolus.NewArrayMap[int, int](3)
	rightMap.Set(2, 20)
	selectedMap := chooseSimulatorMap(selector != 0, leftMap, rightMap)
	selectedMap.Set(3, 30)
	mapAlias := leftMap
	if selector != 0 {
		mapAlias = rightMap
	}
	mapAlias.Set(1+selector, 5)
	result += selectedMap.Len() + selectedMap.Capacity()
	for key, value := range selectedMap.Items() {
		result += key + value
	}
	if selector == 0 {
		result += rightMap.Get(2)
	} else {
		result += leftMap.Get(1)
	}
	leftSet := sonolus.NewArraySet[int](3)
	leftSet.Add(1)
	rightSet := sonolus.NewArraySet[int](4)
	rightSet.Add(2)
	selectedSet := chooseSimulatorSet(selector != 0, leftSet, rightSet)
	selectedSet.Add(3)
	setAlias := leftSet
	if selector != 0 {
		setAlias = rightSet
	}
	setAlias.Add(4)
	result += selectedSet.Len() + selectedSet.Capacity()
	for value := range selectedSet.Values() {
		result += value
	}
	if selector == 0 && rightSet.Contains(2) || selector != 0 && leftSet.Contains(1) {
		result += 10
	}
	captureLeft := sonolus.NewVarArray[int](3)
	captureLeft.Append(1)
	captureRight := sonolus.NewVarArray[int](3)
	captureRight.Append(2)
	captured := captureLeft
	appendCaptured := func() { captured.Append(4) }
	if selector != 0 {
		captured = captureRight
	}
	appendCaptured()
	result += captureLeft.Len()*10 + captureRight.Len()
	leftPointer, rightPointer := 0, 0
	capturedPointer := &leftPointer
	writeCapturedPointer := func() { *capturedPointer = 3 }
	if selector != 0 {
		capturedPointer = &rightPointer
	}
	writeCapturedPointer()
	result += leftPointer*10 + rightPointer
	boundTarget := captureLeft
	boundAppend := boundTarget.Append
	boundTarget = captureRight
	boundAppend(6)
	result += captureLeft.Len()*10 + captureRight.Len()
	clamp := sonolus.Clamp
	result += int(clamp(3, 0, 2))
	abs := math.Abs
	result += int(abs(-3))
	var extreme func(float64, float64) float64
	if selector == 0 {
		extreme = math.Min
	} else {
		extreme = math.Max
	}
	result += int(extreme(2, 5))
	magnitude := sonolus.NewVec2(3, 4).Magnitude
	result += int(magnitude())
	return float64(result)
}

type methodExpressionState struct {
	Value int
}

type methodExpressionOperation func(methodExpressionState, int) int

type emptyComparable struct{}

func emptyComparableValue(calls *int) emptyComparable {
	(*calls)++
	return emptyComparable{}
}

func pointerRangeValue(values *[3]int, calls *int) *[3]int {
	(*calls)++
	return values
}

func genericCallableIncrement[T int | float64](value T) T {
	return value + 1
}

type genericAggregate struct {
	Left  int
	Right int
}

type genericBox[T any] struct {
	Value T
}

func (box genericBox[T]) Get() T {
	return box.Value
}

func genericArraySecond[T any](values [2]T) T {
	copy := values
	return copy[1]
}

func genericZeroValue[T any]() T {
	return sonolus.Zero[T]()
}

func genericBoxValue[T any](value T) T {
	box := genericBox[T]{Value: value}
	return box.Value
}

func genericPointerValue[T any](value *T) *T {
	return value
}

func genericNewValue[T any]() *T {
	return new(T)
}

func genericContainerValue[T any](value T) T {
	values := sonolus.NewVarArray[T](2)
	values.Append(value)
	return values.Get(0)
}

func genericVariadicSecond[T any](values ...T) T {
	return values[1]
}

func genericSlots[T any]() int {
	return sonolus.SlotsOf[T]()
}

func nestedGenericSlots[T any]() int {
	return genericSlots[T]()
}

func genericSlotsClosure[T any]() func() int {
	return func() int { return sonolus.SlotsOf[T]() }
}

func genericValueClosure[T any](value T) func() T {
	return func() T { return value }
}

func (state methodExpressionState) Add(value int) int {
	return state.Value + value
}

func (state methodExpressionState) Multiply(value int) int {
	return state.Value * value
}

func (state *methodExpressionState) Accumulate(value int) int {
	state.Value += value
	return state.Value
}

func methodExpressionReceiver(state *methodExpressionState, calls *int) *methodExpressionState {
	(*calls)++
	return state
}

type StaticLanguageNote struct {
	play.Archetype `archetype:"name=StaticLanguageNote"`
	Selector       float64 `archetype:"imported,name=selector"`
	Value          float64 `archetype:"memory"`
}

func (note *StaticLanguageNote) Preprocess() {
	selector := int(note.Selector) % 2
	state := methodExpressionState{Value: 4}
	var operation func(methodExpressionState, int) int
	if selector == 0 {
		operation = methodExpressionState.Add
	} else {
		operation = methodExpressionState.Multiply
	}
	result := operation(state, 3)
	result += len("static")
	result += methodExpressionState.Add(state, 1)
	converted := methodExpressionOperation(methodExpressionState.Add)
	result += converted(state, 2)
	explicitGeneric := genericCallableIncrement[int]
	result += explicitGeneric(2)
	var inferredGeneric func(int) int = genericCallableIncrement
	result += inferredGeneric(2)
	aggregate := genericArraySecond([2]genericAggregate{{Left: 1, Right: 2}, {Left: 3, Right: 4}})
	result += aggregate.Left + aggregate.Right
	zeroAggregate := genericZeroValue[genericAggregate]()
	result += zeroAggregate.Left + zeroAggregate.Right
	zeroPointer := sonolus.Zero[*genericAggregate]()
	if zeroPointer == nil {
		result++
	}
	boxedAggregate := genericBoxValue(genericAggregate{Left: 3, Right: 4})
	result += boxedAggregate.Left + boxedAggregate.Right
	getBoxedAggregate := genericBox[genericAggregate].Get
	methodAggregate := getBoxedAggregate(genericBox[genericAggregate]{Value: genericAggregate{Left: 3, Right: 4}})
	result += methodAggregate.Left + methodAggregate.Right
	pointerAggregate := genericAggregate{Left: 3, Right: 4}
	returnedPointer := genericPointerValue(&pointerAggregate)
	result += returnedPointer.Left + returnedPointer.Right
	newAggregate := genericNewValue[genericAggregate]()
	newAggregate.Left = 3
	newAggregate.Right = 4
	result += newAggregate.Left + newAggregate.Right
	containerAggregate := genericContainerValue(genericAggregate{Left: 3, Right: 4})
	result += containerAggregate.Left + containerAggregate.Right
	variadicAggregate := genericVariadicSecond(genericAggregate{Left: 1, Right: 2}, genericAggregate{Left: 3, Right: 4})
	result += variadicAggregate.Left + variadicAggregate.Right
	result += genericSlots[genericAggregate]()
	result += nestedGenericSlots[genericAggregate]()
	storedGenericSlots := genericSlots[genericAggregate]
	result += storedGenericSlots()
	storedGenericClosure := genericSlotsClosure[genericAggregate]()
	result += storedGenericClosure()
	var selectedGenericSlots func() int
	if selector == 0 {
		selectedGenericSlots = genericSlots[genericAggregate]
	} else {
		selectedGenericSlots = genericSlots[[3]int]
	}
	result += selectedGenericSlots()
	storedValueClosure := genericValueClosure(genericAggregate{Left: 3, Right: 4})
	closureAggregate := storedValueClosure()
	result += closureAggregate.Left + closureAggregate.Right
	sequence := iter.Seq[int](func(yield func(int) bool) {
		yield(2)
		yield(3)
	})
	for value := range sequence {
		result += value
	}
	emptyCalls := 0
	if emptyComparableValue(&emptyCalls) == emptyComparableValue(&emptyCalls) {
		result += emptyCalls
	}
	if [0]int{} == [0]int{} {
		result++
	}
	for _, value := range [2]emptyComparable{} {
		if value == (emptyComparable{}) {
			result++
		}
	}
	emptyValues := sonolus.NewVarArray[emptyComparable](3)
	emptyValues.Append(emptyComparable{})
	emptyValues.Append(emptyComparable{})
	result += emptyValues.Len() + emptyValues.Count(emptyComparable{})
	rangedValues := [3]int{1, 2, 3}
	for index, value := range rangedValues {
		if index == 0 {
			rangedValues[1] = 9
		}
		result += value
	}
	rangedValues = [3]int{1, 2, 3}
	rangeCalls := 0
	for index, value := range pointerRangeValue(&rangedValues, &rangeCalls) {
		if index == 0 {
			rangedValues[1] = 9
		}
		result += value
	}
	result += rangeCalls
	var nilRange *[3]int
	result += len(nilRange) + cap(nilRange)
	for index := range nilRange {
		result += index
	}

	accumulate := (*methodExpressionState).Accumulate
	receiverCalls := 0
	result += accumulate(methodExpressionReceiver(&state, &receiverCalls), 2)*10 + receiverCalls

	magnitude := sonolus.Vec2.Magnitude
	result += int(magnitude(sonolus.NewVec2(6, 8))) * 100
	var vectorMetric func(sonolus.Vec2) float64
	if selector == 0 {
		vectorMetric = sonolus.Vec2.Magnitude
	} else {
		vectorMetric = sonolus.Vec2.MagnitudeSquared
	}
	result += int(vectorMetric(sonolus.NewVec2(3, 4)))
	number := simulatorNumber.Number
	result += int(number(chooseSimulatorNumber(selector != 0, 3)))
	result += int(simulatorNumber.Number(simulatorPlain{Value: 1}))
	result += int(sonolus.Vec2.Magnitude(sonolus.NewVec2(0, 2)))

	values := sonolus.NewVarArray[int](4)
	appendValue := sonolus.VarArray[int].Append
	appendValue(values, 5)
	appendValue(values, 2)
	appendValue(values, 1)
	sortValues := sonolus.VarArray[int].SortFunc
	sortValues(values, func(left, right int) bool { return left < right })
	result += values.Get(0)*10_000 + values.Get(2)*1_000

	note.Value = float64(result)
}

type VariantNote struct {
	play.Archetype        `archetype:"name=VariantNote"`
	Selector              float64 `archetype:"imported,name=selector"`
	InterfaceValue        float64 `archetype:"memory"`
	PointerValue          float64 `archetype:"memory"`
	ContainerValue        float64 `archetype:"memory"`
	ContainerVariantValue float64 `archetype:"memory"`
	NewPointerValue       float64 `archetype:"memory"`
	Value                 float64 `archetype:"memory"`
}

func (note *VariantNote) Preprocess() {
	selector := int(note.Selector) % 2
	selected := chooseSimulatorNumber(selector != 0, 3)
	result := selected.Number()
	var direct simulatorNumber
	if selector == 0 {
		direct = simulatorPlain{Value: 2}
	} else {
		direct = simulatorDouble{Value: 2}
	}
	result += direct.Number()
	if plain, ok := direct.(simulatorPlain); ok {
		result += plain.Value
	}
	switch concrete := direct.(type) {
	case simulatorPlain:
		result += concrete.Value
	case simulatorDouble:
		result += concrete.Value * 2
	}
	result = chooseSimulatorCallable(selector == 0)(result)
	note.InterfaceValue = result
	values := [2]float64{5, 7}
	pointer := simulatorPointer(selector != 0, &values)
	*pointer += 2
	pointerAlias := &values[0]
	if selector != 0 {
		pointerAlias = &values[1]
	}
	*pointerAlias++
	note.PointerValue = *pointer + *pointerAlias
	addOne := simulatorAdder(1)
	addTwo := simulatorAdder(2)
	result = addOne(result) + addTwo(result)
	note.ContainerValue = simulatorContainerValue()
	note.ContainerVariantValue = simulatorContainerVariantValue(selector)
	note.NewPointerValue = simulatorNewPointerValue(selector)
	result += note.PointerValue + note.ContainerValue
	note.Value = result
}

type NilPointerNote struct {
	play.Archetype `archetype:"name=NilPointerNote"`
	Selector       float64 `archetype:"imported,name=selector"`
	Value          float64 `archetype:"memory"`
}

func (note *NilPointerNote) Preprocess() {
	var pointer *float64
	if note.Selector == 0 && pointer == nil {
		note.Value = 1
		return
	}
	*pointer = 2
	note.Value = 3
}

type DiagnosticControlNote struct {
	play.Archetype `archetype:"name=DiagnosticControlNote"`
	Selector       float64 `archetype:"imported,name=selector"`
	Value          float64 `archetype:"memory"`
}

func terminateDiagnosticHelper() { sonolus.Terminate("helper termination") }

func (note *DiagnosticControlNote) Preprocess() {
	sonolus.Notify("diagnostic notification")
	if note.Selector == 0 {
		note.Value = 1
		return
	}
	if note.Selector == 1 {
		sonolus.Require(false, "constant require failure")
		note.Value = 2
		return
	}
	terminateDiagnosticHelper()
	note.Value = 3
}

type LinkedNote struct {
	play.Archetype `archetype:"name=LinkedNote"`
	Key            float64                       `archetype:"imported,name=key"`
	Head           sonolus.EntityRef[LinkedNote] `archetype:"memory"`
	Next           sonolus.EntityRef[LinkedNote] `archetype:"shared"`
	Previous       sonolus.EntityRef[LinkedNote] `archetype:"shared"`
}

func linkedNoteLess(left, right *LinkedNote) bool { return left.Key < right.Key }
func linkedNoteNext(note *LinkedNote) sonolus.EntityRef[LinkedNote] {
	return note.Next
}
func linkedNoteSetNext(note *LinkedNote, next sonolus.EntityRef[LinkedNote]) {
	note.Next = next
}
func linkedNoteSetPrevious(note *LinkedNote, previous sonolus.EntityRef[LinkedNote]) {
	note.Previous = previous
}

func (note *LinkedNote) Preprocess() {
	note.Head = sonolus.SortLinkedEntities(note.Head, linkedNoteLess, linkedNoteNext, linkedNoteSetNext)
	note.Head = sonolus.SortDoublyLinkedEntities(note.Head, linkedNoteLess, linkedNoteNext, linkedNoteSetNext, linkedNoteSetPrevious)
}
