//go:build tutorial

package main

import (
	"github.com/WindowsSov8forUs/sonolus-go/v2/sonolus"
	"github.com/WindowsSov8forUs/sonolus-go/v2/sonolus/tutorial"
)

type TutorialMemoryState struct {
	sonolus.LevelMemoryResource
	Phase  int
	Active bool
}

var TutorialMemory = TutorialMemoryState{}

type TutorialDataState struct {
	sonolus.LevelDataResource
	Limit int
}

var TutorialData = TutorialDataState{}

type TutorialGlobals struct{ tutorial.GlobalCallbacks }

var Global TutorialGlobals

func Preprocess() {
	TutorialMemory.Phase = 0
	TutorialMemory.Active = true
	TutorialData.Limit = 4
}

func Navigate() { TutorialMemory.Phase++ }
func Update()   { _ = TutorialMemory.Phase < TutorialData.Limit }
