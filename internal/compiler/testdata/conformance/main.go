package main

import (
	"iter"

	"github.com/WindowsSov8forUs/sonolus-go/v2/sonolus"
)

type staticNumber interface{ Number() float64 }

type concreteNumber struct{ Value float64 }
type doubledNumber struct{ Value float64 }

var packageCallable = incrementValue

func (value concreteNumber) Number() float64 { return value.Value }
func (value doubledNumber) Number() float64  { return value.Value * 2 }

func interfaceNumber(value staticNumber) float64 { return value.Number() }

func returnInterface(value concreteNumber) staticNumber { return value }

func forwardInterface(value staticNumber) staticNumber { return value }

func chooseInterface(doubled bool, value float64) staticNumber {
	if doubled {
		return doubledNumber{Value: value}
	}
	return concreteNumber{Value: value}
}

func inspectInterface(value staticNumber) float64 {
	result := 0.0
	if doubled, ok := value.(doubledNumber); ok {
		result += doubled.Value
	}
	switch concrete := value.(type) {
	case concreteNumber:
		result += concrete.Value
	case doubledNumber:
		result += concrete.Value * 2
	}
	return result
}

func genericTypeNumber[T int | float64](value T) float64 {
	switch any(value).(type) {
	case int:
		return 1
	case float64:
		return 2
	default:
		return 0
	}
}

func genericAssertion[T int | float64](value T) float64 {
	integer, ok := any(value).(int)
	if ok {
		return float64(integer)
	}
	return 0
}

func identityPointer(value *float64) *float64 { return value }

func pointerEnhancements(index int) float64 {
	values := [2]float64{1, 2}
	pointer := identityPointer(&values[index])
	*pointer += 3
	return *pointer
}

func choosePointer(values *[2]float64, index int) *float64 {
	if index == 0 {
		return &values[0]
	}
	return &values[1]
}

func dynamicPointerEnhancements(index int) float64 {
	values := [2]float64{1, 2}
	pointer := &values[0]
	if index != 0 {
		pointer = &values[1]
	}
	*pointer += 4
	selected := choosePointer(&values, index)
	if selected == pointer {
		return *selected
	}
	return 0
}

func chooseArrayPointer(left, right *[2]float64, chooseRight bool) *[2]float64 {
	if chooseRight {
		return right
	}
	return left
}

func dynamicPointerArrayIndex(index int) float64 {
	left := [2]float64{1, 2}
	right := [2]float64{3, 4}
	pointer := chooseArrayPointer(&left, &right, index != 0)
	pointer[index] += 5
	return pointer[index]
}

func labeledControlFlow(selector int) int {
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

func explicitDerefPointerArrayIndex(index int) float64 {
	left := [2]float64{1, 2}
	right := [2]float64{3, 4}
	pointer := chooseArrayPointer(&left, &right, index != 0)
	(*pointer)[index] += 6
	return (*pointer)[index]
}

func mergePointerSets(leftA, leftB, rightA, rightB *[2]float64, chooseLeft, chooseSecond bool) *[2]float64 {
	if chooseLeft {
		return chooseArrayPointer(leftA, leftB, chooseSecond)
	}
	return chooseArrayPointer(rightA, rightB, chooseSecond)
}

func independentPointerSets(index int) float64 {
	a := [2]float64{1, 2}
	b := [2]float64{3, 4}
	c := [2]float64{5, 6}
	d := [2]float64{7, 8}
	pointer := mergePointerSets(&a, &b, &c, &d, index == 0, index != 0)
	return pointer[index]
}

func containerEnhancements() float64 {
	values := sonolus.NewVarArray[int](8)
	values.Append(3)
	values.Append(1)
	values.Append(2)
	values.Swap(0, 1)
	values.Reverse()
	values.Shuffle()
	values.SortFunc(func(left, right int) bool { return left < right })
	values.Extend(func(yield func(int) bool) {
		for _, value := range [2]int{5, 4} {
			if !yield(value) {
				return
			}
		}
	})
	less := func(left, right int) bool { return left < right }
	result := float64(values.Count(2) + values.Index(3) + values.LastIndex(2))
	result += float64(values.IndexMinFunc(less) + values.IndexMaxFunc(less))
	result += float64(values.MinFunc(less) + values.MaxFunc(less))
	for value := range values.Values() {
		result += float64(value)
	}
	values.RemoveAt(0)
	values.Remove(3)
	unchecked := sonolus.NewVarArray[int](4)
	unchecked.AppendUnchecked(6)
	unchecked.AppendUnchecked(7)
	unchecked.SetUnchecked(0, 8)
	unchecked.SwapUnchecked(0, 1)
	result += float64(unchecked.GetUnchecked(0))
	for index, value := range unchecked.Items() {
		result += float64(index + value)
	}
	for value := range unchecked.ValuesReversed() {
		result += float64(value)
	}

	mapping := sonolus.NewArrayMap[int, float64](2)
	mapping.Set(1, 4)
	if mapping.IsFull() {
		result++
	}
	if value, ok := mapping.GetOK(1); ok {
		result += value
	}
	for key, value := range mapping.Items() {
		result += float64(key) + value
	}
	for key := range mapping.Keys() {
		result += float64(key)
	}
	for value := range mapping.Values() {
		result += value
	}
	if value, ok := mapping.Pop(1); ok {
		result += value
	}
	set := sonolus.NewArraySet[int](2)
	set.Add(2)
	if set.IsFull() {
		result++
	}
	for value := range set.Values() {
		result += float64(value)
	}
	return result
}

type GameConfiguration struct {
	sonolus.Configuration
	Speed float64
	Auto  bool
}

var Configuration = GameConfiguration{
	Speed: sonolus.SliderOption(sonolus.SliderOptionConfig{Name: "Speed", Default: 1, Min: 0.5, Max: 2, Step: 0.1}),
	Auto:  sonolus.ToggleOption(sonolus.ToggleOptionConfig{Name: "Auto"}),
}
var ROM = sonolus.ROMValues{1, 2, 3}

func sum[T ~float64](values ...T) float64 {
	result := 0.0
	for _, value := range values {
		result += float64(value)
	}
	return result
}

func makeAdder(base float64) func(float64) float64 {
	return func(value float64) float64 { return base + value }
}

type accumulator struct{ Value float64 }

func (a *accumulator) Add(value float64) float64 {
	a.Value += value
	return a.Value
}

func sequence(base float64) iter.Seq[float64] {
	return func(yield func(float64) bool) {
		for i := range 3 {
			if !yield(base + float64(i)) {
				return
			}
		}
	}
}

func firstSequenceValue(base float64) float64 {
	for value := range sequence(base) {
		return value
	}
	return 0
}

func triangular(value int) float64 {
	if value <= 0 {
		return 0
	}
	return float64(value) + triangular(value-1)
}

func closureTriangular(value int) float64 {
	var visit func(int) float64
	visit = func(current int) float64 {
		if current <= 0 {
			return 0
		}
		return float64(current) + visit(current-1)
	}
	return visit(value)
}

func incrementValue(value float64) float64 { return value + 1 }
func decrementValue(value float64) float64 { return value - 1 }

func chooseCallable(increment bool) func(float64) float64 {
	if increment {
		return incrementValue
	}
	return decrementValue
}

func applyVariadicCallables(index int, value float64, callables ...func(float64) float64) float64 {
	result := callables[index](value)
	for _, callable := range callables {
		result = callable(result)
	}
	return forwardVariadicCallables(result, callables...)
}

func forwardVariadicCallables(value float64, callables ...func(float64) float64) float64 {
	return callables[0](value)
}

func main() {}
