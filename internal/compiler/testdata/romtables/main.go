package main

import "github.com/WindowsSov8forUs/sonolus-go/v2/sonolus"

var ROM = sonolus.ROMValues{9, 8}

// exact and pairs hold binary32-exact, non-arithmetic values and are indexed
// through EngineRom; inexact keeps the exact switch lowering.
var exact = [4]float64{0.5, -3, 2.25, 7}
var pairs = [3]sonolus.Vec2{{X: 1, Y: 2}, {X: 5, Y: -1}, {X: 0.25, Y: 8}}
var inexact = [3]float64{0.1, 0.7, 0.3}
var watchOnly = [3]float64{6, 1, 4}

func main() {}
