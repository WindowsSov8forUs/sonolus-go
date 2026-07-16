package main

import (
	_ "embed"

	"github.com/WindowsSov8forUs/sonolus-go/sonolus"
)

//sonolus:level
//go:embed level.json
var DevelopmentLevel sonolus.LevelFile

func main() {}
