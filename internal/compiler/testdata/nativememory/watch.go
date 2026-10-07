//go:build watch

package main

import (
	"github.com/WindowsSov8forUs/sonolus-go/v2/sonolus/native"
	"github.com/WindowsSov8forUs/sonolus-go/v2/sonolus/watch"
)

type Frame struct {
	watch.Archetype `archetype:"name=Frame"`
	Kind            int `archetype:"imported,name=kind"`
}

func add() { native.Set(4102, 7, native.Get(4102, 7)+1) }

func (f *Frame) UpdateSequential() {
	before := int(native.Get(4102, 7))
	if f.Kind == 0 {
		native.Set(4102, 7, native.Get(4102, 7)+1)
	} else if f.Kind == 1 {
		add()
	} else {
		v := int(native.Get(4102, 7))
		for i := 0; i < 16; i++ {
			if native.Get(4102, float64(100+i)) == 0 {
				v++
			}
		}
		native.Set(4102, 7, float64(v))
	}
	after := int(native.Get(4102, 7))
	count := 0
	for i := before; i < after; i++ {
		count++
	}
	native.DebugLog(float64(before))
	native.DebugLog(float64(after))
	native.DebugLog(float64(count))
}

type Globals struct{ watch.GlobalCallbacks }

type Reads struct {
	watch.Archetype `archetype:"name=Reads"`
}

func (*Reads) UpdateSequential() {
	native.Set(4102, 20, 4102)
	native.Set(4102, 21, 7)
	native.Set(4102, 7, 3)
	a := native.GetPointed(4102, 20, 0)
	b := native.GetShifted(4102, 5, 1, 2)
	native.SetPointed(4102, 20, 0, 8)
	native.DebugLog(a)
	native.DebugLog(b)
	native.DebugLog(native.GetPointed(4102, 20, 0))
	native.DebugLog(native.GetShifted(4102, 5, 1, 2))
	// Both loop-carried mutation and argument evaluation must keep reads fresh.
	sum := 0.0
	for i := 0; i < 3; i++ {
		sum += native.Get(4102, 7)
		native.Set(4102, 7, native.Get(4102, 7)+1)
	}
	native.DebugLog(sum)
	native.DebugLog(native.Add(native.Get(4102, 7), native.Set(4102, 7, 40), native.Get(4102, 7)))
	// Rebinding a pointed address also invalidates its value.
	native.Set(4102, 8, 50)
	x := native.GetPointed(4102, 20, 0)
	native.Set(4102, 21, 8)
	native.DebugLog(x)
	native.DebugLog(native.GetPointed(4102, 20, 0))
}

var Global Globals

func UpdateSpawn() float64 { return watch.Time.Scaled() }
