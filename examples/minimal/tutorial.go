//go:build tutorial

package main

import (
	"github.com/WindowsSov8forUs/sonolus-go/v2/sonolus"
	"github.com/WindowsSov8forUs/sonolus-go/v2/sonolus/tutorial"
)

type InstructionData struct {
	sonolus.InstructionResource

	Tap sonolus.Text
}

var Instructions = &InstructionData{
	Tap: sonolus.InstructionText("#TAP"),
}

type Globals struct {
	tutorial.GlobalCallbacks
}

var Global Globals

func Preprocess() {}

func Navigate() {
	tutorial.Instruction.Show(Instructions.Tap)
}

func Update() {}
