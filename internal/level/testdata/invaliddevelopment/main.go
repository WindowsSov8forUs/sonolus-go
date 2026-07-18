package main

import (
	_ "embed"

	"github.com/WindowsSov8forUs/sonolus-go/v2/sonolus"
)

//sonolus:level
//go:embed first.json
var FirstLevel sonolus.LevelFile

//sonolus:level
//go:embed second.json
var SecondLevel sonolus.LevelFile

func main() {}
