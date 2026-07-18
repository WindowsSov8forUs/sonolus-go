package main

import (
	_ "embed"

	"github.com/WindowsSov8forUs/sonolus-go/v2/sonolus"
)

//sonolus:level basic
//go:embed level.json
var BasicLevel sonolus.LevelFile

//sonolus:level alternate
//go:embed alternate.json
var AlternateLevel sonolus.LevelFile

func main() {}
