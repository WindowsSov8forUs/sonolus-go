//go:build play

package main

import (
	"github.com/WindowsSov8forUs/sonolus-go/v2/sonolus"
	"github.com/WindowsSov8forUs/sonolus-go/v2/sonolus/play"
)

type SkinData struct {
	sonolus.SkinResource

	Notes [2]sonolus.Sprite
}

var Skin = &SkinData{
	SkinResource: sonolus.SkinResource{RenderMode: sonolus.RenderModeStandard},
	Notes: [2]sonolus.Sprite{
		sonolus.SkinSprite("#NOTE_HEAD_CYAN"),
		sonolus.SkinSprite("conformance.note"),
	},
}

type EffectData struct {
	sonolus.EffectResource

	Hit sonolus.Clip
}

var Effects = &EffectData{
	Hit: sonolus.EffectClip("#PERFECT"),
}

type ParticleData struct {
	sonolus.ParticleResource

	Hit sonolus.Effect
}

var Particles = &ParticleData{
	Hit: sonolus.ParticleEffect("#NOTE_CIRCULAR_TAP_CYAN"),
}

type BucketData struct {
	sonolus.BucketsResource

	Tap sonolus.Bucket
}

var Buckets = &BucketData{
	Tap: sonolus.JudgmentBucket("#MILLISECONDS", sonolus.JudgmentBucketSprite(Skin.Notes[0], 0, 0, 1, 1, 0)),
}

type Note struct {
	play.Archetype      `archetype:"name=ConformanceNote,hasInput=true"`
	play.CallbackOrders `archetype:"preprocess=-10"`

	Beat     float64                 `archetype:"imported,name=#BEAT,default=0"`
	Value    float64                 `archetype:"memory"`
	Result   float64                 `archetype:"exported,name=result"`
	Next     sonolus.EntityRef[Note] `archetype:"shared"`
	Previous sonolus.EntityRef[Note] `archetype:"shared"`
}

func noteLess(left, right *Note) bool                              { return left.Beat < right.Beat }
func noteNext(note *Note) sonolus.EntityRef[Note]                  { return note.Next }
func noteSetNext(note *Note, next sonolus.EntityRef[Note])         { note.Next = next }
func noteSetPrevious(note *Note, previous sonolus.EntityRef[Note]) { note.Previous = previous }

type conformanceInt int

func tracedDivisor(value conformanceInt) conformanceInt {
	play.Debug.Log(float64(value))
	return value
}

func integerArithmetic(value, divisor conformanceInt) conformanceInt {
	quotient := value / tracedDivisor(divisor)
	remainder := value % tracedDivisor(divisor)
	quotient /= tracedDivisor(divisor)
	remainder %= tracedDivisor(divisor)
	return quotient + remainder
}

func (n *Note) Preprocess() {
	result := sum(1.0, 2.0, 3.0)
	result += interfaceNumber(concreteNumber{Value: 3})
	result += forwardInterface(returnInterface(concreteNumber{Value: 4})).Number()
	result += chooseInterface(n.Beat >= 0, 5).Number()
	result += inspectInterface(chooseInterface(n.Beat >= 0, 6))
	result += genericTypeNumber(1)
	result += genericTypeNumber(1.0)
	result += genericAssertion(4)
	result += genericAssertion(4.0)
	result += pointerEnhancements(int(n.Beat) % 2)
	result += dynamicPointerEnhancements(int(n.Beat) % 2)
	result += dynamicPointerArrayIndex(int(n.Beat) % 2)
	result += explicitDerefPointerArrayIndex(int(n.Beat) % 2)
	result += independentPointerSets(int(n.Beat) % 2)
	result += float64(labeledControlFlow(int(n.Beat) % 3))
	result = packageCallable(result)
	result += containerEnhancements()
	result += triangular(4)
	result += closureTriangular(4)
	result += float64(integerArithmetic(
		conformanceInt(int(n.Beat)+4),
		conformanceInt(int(n.Beat)+1),
	))
	result += firstSequenceValue(3)
	_ = sonolus.SortLinkedEntities(n.Next, noteLess, noteNext, noteSetNext)
	_ = sonolus.SortDoublyLinkedEntities(n.Next, noteLess, noteNext, noteSetNext, noteSetPrevious)
	offset := 1.0
	transform := func(value float64) float64 {
		return value + offset
	}
	offset = 2
	result = transform(result)
	result = makeAdder(result)(1)
	accumulator := accumulator{Value: result}
	add := accumulator.Add
	result = add(2)
	var selected func(float64) float64
	if n.Beat >= 0 {
		selected = incrementValue
	} else {
		selected = decrementValue
	}
	result = selected(result)
	result = chooseCallable(n.Beat >= 0)(result)
	result = applyVariadicCallables(int(n.Beat)%2, result, incrementValue, decrementValue)
	leftOffset, rightOffset := 10.0, 20.0
	var capturedSelection func(float64) float64
	if n.Beat >= 0 {
		capturedSelection = func(value float64) float64 { return value + leftOffset }
	} else {
		capturedSelection = func(value float64) float64 { return value + rightOffset }
	}
	result = capturedSelection(result)
	for value := range sequence(result) {
		if value < 0 {
			continue
		}
		result += value
		if result > 1000 {
			break
		}
	}

	values := [3]float64{n.Beat, 2, 4}
	for _, value := range values {
		result += value
	}
	for i := range 3 {
		result += float64(i)
	}
	switch int(result) % 2 {
	case 0:
		result += 1
	default:
		result -= 1
	}
	n.Value = result
	n.Result = result
	play.Debug.Log(result)
}

func (*Note) ShouldSpawn() bool {
	return true
}
