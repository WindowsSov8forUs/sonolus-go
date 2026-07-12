package main

import (
	_ "embed"

	"github.com/WindowsSov8forUs/sonolus-go/sonolus"
)

//go:embed rom.bin
var ROM sonolus.ROMFile
