package main

import (
	"github.com/WindowsSov8forUs/sonolus-go/v2/sonolus"
	"github.com/WindowsSov8forUs/sonolus-go/v2/sonolus/native"
)

type sortItem struct{ Key, Order int }

type identityReader interface{ Read() int }
type identityCounter struct{ Value int }

func (c *identityCounter) Read() int { return c.Value }
func rebindReader(value identityReader) int {
	value = &identityCounter{Value: 9}
	return value.Read()
}
func readFirst(value identityReader, ignored int) int { return value.Read() }
func capturedParameter(value int) int {
	change := func() { value++ }
	change()
	return value
}
func capturedCallable() int {
	value := func() int { return 1 }
	change := func() { value = func() int { return 2 } }
	change()
	return value()
}
func identityValues() {
	native.DebugLog(float64(capturedParameter(1)))
	native.DebugLog(float64(capturedCallable()))
	a, b := identityCounter{Value: 1}, identityCounter{Value: 2}
	var value identityReader = &a
	native.DebugLog(float64(readFirst(value, func() int { value = &b; return 0 }())))
	native.DebugLog(float64(value.Read()))
	native.DebugLog(float64(rebindReader(value)))
	native.DebugLog(float64(value.Read()))
	mutateReader(value)
	native.DebugLog(float64(value.Read()))
}
func mutateReader(value identityReader) { value.(*identityCounter).Value++ }

type sortMemory struct {
	sonolus.LevelMemoryResource
	Values sonolus.VarArray[sortItem]
}

var SortMemory = sortMemory{Values: sonolus.NewVarArray[sortItem](257)}

func sortCollection(count, pattern, descending int, persistent bool) {
	values := sonolus.NewVarArray[sortItem](257)
	if persistent {
		values = SortMemory.Values
	}
	values.Clear()
	for i := 0; i < count; i++ {
		key := (i*37 + 11) % 17
		switch pattern {
		case 0:
			key = count - i
		case 1:
			key = i
		case 2:
			key = i % 5
		case 4:
			key = 1
		}
		values.Append(sortItem{Key: key, Order: i})
	}
	comparisons := 0
	values.SortFunc(func(a, b sortItem) bool {
		comparisons++
		if descending != 0 {
			return a.Key > b.Key
		}
		return a.Key < b.Key
	})
	for value := range values.Values() {
		native.DebugLog(float64(value.Key))
		native.DebugLog(float64(value.Order))
	}
	native.DebugLog(float64(comparisons))
}

type Vec2 struct{ X, Y float64 }
type Touch struct {
	ID, X, Y float64
	Phase    int
}
type Touches struct{ Values [4]Touch }

const One = 1

func multiplyParameter(value float64) float64 { value *= 2; return value }
func countParameter(value float64) float64 {
	for i := 0; i < 3; i++ {
		value *= 2
	}
	return value
}
func conditionalParameter(value float64) float64 {
	count := 0
	for value < 8 && count < 4 {
		value *= 2
		count++
	}
	return value
}
func plainAssignment(value float64) float64 {
	count := 0
	for value < 8 && count < 4 {
		value = value * 2
		count++
	}
	return value
}
func incrementParameter(value int) int {
	count := 0
	for value < 4 && count < 5 {
		value++
		count++
	}
	return value
}
func branchParameter(value float64, condition bool) float64 {
	if condition {
		value *= 2
	}
	return value
}
func localControl(seed float64) float64 {
	value := seed
	return conditionalParameter(value)
}
func genericParameter[T ~int | ~float64](value T) T {
	for i := 0; i < 3; i++ {
		value *= 2
	}
	return value
}
func closureParameter() float64 {
	return func(value float64) float64 {
		for i := 0; i < 3; i++ {
			value *= 2
		}
		return value
	}(1)
}
func sumTouches(values [4]Touch) float64 {
	return values[0].ID + values[1].ID + values[2].ID + values[3].ID
}
func rangeTouches(values [4]Touch) float64 {
	total := 0.0
	for _, v := range values {
		total += v.ID
	}
	return total
}
func arrayParameter(seed float64) float64 {
	var a [4]Touch
	for i := 0; i < 4; i++ {
		a[i].ID = seed + float64(i)
	}
	return sumTouches(a)
}
func arrayRangeParameter(seed float64) float64 {
	var a [4]Touch
	for i := 0; i < 4; i++ {
		a[i].ID = seed + float64(i)
	}
	return rangeTouches(a)
}
func makeTouches(seed float64) [4]Touch {
	var a [4]Touch
	for i := 0; i < 4; i++ {
		a[i].ID = seed + float64(i)
	}
	return a
}
func arrayReturn(seed float64) float64 {
	a := makeTouches(seed)
	return a[0].ID + a[1].ID + a[2].ID + a[3].ID
}
func sumContainer(a Touches) float64 {
	return a.Values[0].ID + a.Values[1].ID + a.Values[2].ID + a.Values[3].ID
}
func structArrayParameter(seed float64) float64 {
	var a Touches
	for i := 0; i < 4; i++ {
		a.Values[i].ID = seed + float64(i)
	}
	return sumContainer(a)
}
func makeContainer(seed float64) Touches {
	var a Touches
	for i := 0; i < 4; i++ {
		a.Values[i].ID = seed + float64(i)
	}
	return a
}
func structArrayReturn(seed float64) float64 {
	a := makeContainer(seed)
	return a.Values[0].ID + a.Values[1].ID + a.Values[2].ID + a.Values[3].ID
}
func makePositions(seed float64) [24]Vec2 {
	var a [24]Vec2
	for i := 0; i < 24; i++ {
		a[i].X = seed + float64(i)
		a[i].Y = seed
	}
	return a
}
func positionReturn(seed float64) float64 {
	a := makePositions(seed)
	sum := 0.0
	for i := 0; i < 24; i++ {
		sum += a[i].X + a[i].Y
	}
	return sum
}
func positionDirect(seed float64) float64 {
	var a [24]Vec2
	for i := 0; i < 24; i++ {
		a[i].X = seed + float64(i)
		a[i].Y = seed
	}
	sum := 0.0
	for i := 0; i < 24; i++ {
		sum += a[i].X + a[i].Y
	}
	return sum
}
func sumPositions(a [24]Vec2) float64 {
	sum := 0.0
	for i := 0; i < 24; i++ {
		sum += a[i].X + a[i].Y
	}
	return sum
}
func positionParameter(seed float64) float64 {
	var a [24]Vec2
	for i := 0; i < 24; i++ {
		a[i].X = seed + float64(i)
		a[i].Y = seed
	}
	return sumPositions(a)
}
func arrayDynamicReturn(seed float64) float64 {
	a := makeTouches(seed)
	sum := 0.0
	for i := 0; i < 4; i++ {
		sum += a[i].ID
	}
	return sum
}
