//go:build watch

package main

import (
	"math"
	"math/rand"

	"github.com/WindowsSov8forUs/sonolus-go/v2/sonolus/native"
	"github.com/WindowsSov8forUs/sonolus-go/v2/sonolus/watch"
)

type Draws struct {
	watch.Archetype `archetype:"name=Draws"`
}

// Every Random call draws again: a loop-invariant call stays in its loop and
// equal calls are not shared, as for sonolus.py Op.Random (not pure).
func (*Draws) UpdateSequential() {
	for i := 0; i < 4; i++ {
		native.DebugLog(rand.Float64())
	}
	for i := 0; i < 4; i++ {
		native.DebugLog(float64(rand.Intn(1 << 30)))
	}
	native.DebugLog(native.Random(0, 1))
	native.DebugLog(native.Random(0, 1))
	native.DebugLog(native.RandomInteger(0, 1<<30))
	native.DebugLog(native.RandomInteger(0, 1<<30))
	a := rand.Float64()
	b := rand.Float64()
	native.DebugLog(a)
	native.DebugLog(b)
	var picks [6]float64
	for i := 0; i < 6; i++ {
		picks[i] = math.Floor(rand.Float64() * (1 << 30))
	}
	for i := 0; i < 6; i++ {
		native.DebugLog(picks[i])
	}
}

type Globals struct{ watch.GlobalCallbacks }

var Global Globals
