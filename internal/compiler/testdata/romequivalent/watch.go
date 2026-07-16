//go:build watch

package main

import (
	_ "embed"

	"github.com/WindowsSov8forUs/sonolus-go/v2/sonolus"
)

//go:embed data.bin
var ROM sonolus.ROMFile
