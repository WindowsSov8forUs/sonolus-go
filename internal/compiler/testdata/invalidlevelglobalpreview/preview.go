//go:build preview

package main

import "github.com/WindowsSov8forUs/sonolus-go/v2/sonolus"

type PreviewMemory struct {
	sonolus.LevelMemoryResource
	Value float64
}

var Memory = PreviewMemory{}
