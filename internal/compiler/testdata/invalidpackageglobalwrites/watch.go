//go:build watch

package main

import "github.com/WindowsSov8forUs/sonolus-go/v2/sonolus/watch"

type state struct{ Value float64 }

var zeroRoot = &state{}
var initializedRoot = &state{Value: 7}

type Globals struct{ watch.GlobalCallbacks }

var Global = Globals{}

func UpdateSpawn() float64 {
	zeroRoot.Value = 1
	return initializedRoot.Value
}
