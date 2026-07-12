//go:build tutorial

package main

import (
	"github.com/WindowsSov8forUs/sonolus-go/sonolus"
	"github.com/WindowsSov8forUs/sonolus-go/sonolus/tutorial"
)

//sonolus:resource instruction
type InstructionData struct {
	Tap sonolus.Text
}

//sonolus:resource instruction
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
