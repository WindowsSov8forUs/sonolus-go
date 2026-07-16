//go:build watch

package main

import (
	"github.com/WindowsSov8forUs/sonolus-go/v2/sonolus"
	"github.com/WindowsSov8forUs/sonolus-go/v2/sonolus/watch"
)

type ReplayStreams struct {
	sonolus.StreamResource
	Notes   [2]sonolus.Stream[StreamState]
	Summary sonolus.StreamData[sonolus.Vec2]
}

var Replay = ReplayStreams{}

type Note struct {
	watch.Archetype `archetype:"name=Note"`
}

func (note *Note) Preprocess() {
	if Replay.Notes[0].Has(1) {
		value := Replay.Notes[0].Get(1)
		watch.Debug.Log(value.Value + value.Point.X)
	}
	watch.Debug.Log(Replay.Notes[0].PreviousKey(2))
	watch.Debug.Log(Replay.Notes[0].NextKey(0))
	watch.Debug.Log(Replay.Notes[0].PreviousKeyOrDefault(0, -1))
	watch.Debug.Log(Replay.Notes[0].NextKeyOrDefault(2, -1))
	watch.Debug.Log(Replay.Notes[0].PreviousKeyInclusive(2))
	watch.Debug.Log(Replay.Notes[0].NextKeyInclusive(0))
	if Replay.Notes[0].HasPreviousKey(2) {
		watch.Debug.Log(Replay.Notes[0].GetPrevious(2).Value)
	}
	if Replay.Notes[0].HasNextKey(0) {
		watch.Debug.Log(Replay.Notes[0].GetNext(0).Value)
	}
	watch.Debug.Log(Replay.Notes[0].GetPreviousInclusive(2).Value)
	watch.Debug.Log(Replay.Notes[0].GetNextInclusive(0).Value)
	for key, value := range Replay.Notes[0].ItemsFrom(0) {
		watch.Debug.Log(key + value.Value)
	}
	for key, value := range Replay.Notes[0].ItemsFromDescending(2) {
		watch.Debug.Log(key + value.Value)
	}
	for key, value := range Replay.Notes[0].ItemsSincePreviousFrame() {
		watch.Debug.Log(key + value.Value)
	}
	for key := range Replay.Notes[0].KeysFrom(0) {
		watch.Debug.Log(key)
	}
	for key := range Replay.Notes[0].KeysFromDescending(2) {
		watch.Debug.Log(key)
	}
	for key := range Replay.Notes[0].KeysSincePreviousFrame() {
		watch.Debug.Log(key)
	}
	for value := range Replay.Notes[0].ValuesFrom(0) {
		watch.Debug.Log(value.Value)
	}
	for value := range Replay.Notes[0].ValuesFromDescending(2) {
		watch.Debug.Log(value.Value)
	}
	for value := range Replay.Notes[0].ValuesSincePreviousFrame() {
		watch.Debug.Log(value.Value)
	}
	watch.Debug.Log(Replay.Summary.Get().X)
}
