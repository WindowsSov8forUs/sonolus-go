//go:build tutorial

package main

import "github.com/WindowsSov8forUs/sonolus-go/v2/sonolus/tutorial"

type state struct{ Value float64 }

var root = &state{}

type Globals struct{ tutorial.GlobalCallbacks }

var Global = Globals{}

func Navigate() {
	root.Value = 1
}
