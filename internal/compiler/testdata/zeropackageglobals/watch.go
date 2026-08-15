//go:build watch

package main

import "github.com/WindowsSov8forUs/sonolus-go/v2/sonolus/watch"

type zeroState struct{ Value float64 }

var zeroRoot = &zeroState{}

type Globals struct{ watch.GlobalCallbacks }

var Global = Globals{}

func UpdateSpawn() float64 {
	return zeroRoot.Value
}
