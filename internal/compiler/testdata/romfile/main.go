package main

import (
	_ "embed"

	"github.com/WindowsSov8forUs/sonolus-go/v2/sonolus"
)

//go:embed rom.bin
var ROM sonolus.ROMFile
