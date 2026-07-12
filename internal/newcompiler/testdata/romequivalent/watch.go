//go:build watch

package main

import (
	_ "embed"

	"github.com/WindowsSov8forUs/sonolus-go/sonolus"
)

//go:embed data.bin
var ROM sonolus.ROMFile
