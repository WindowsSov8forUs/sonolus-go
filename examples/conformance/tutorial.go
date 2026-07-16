//go:build tutorial

package main

import (
	"github.com/WindowsSov8forUs/sonolus-go/sonolus"
	"github.com/WindowsSov8forUs/sonolus-go/sonolus/tutorial"
)

type InstructionData struct {
	sonolus.InstructionResource

	Tap sonolus.Text
}

var Instructions = &InstructionData{Tap: sonolus.InstructionText("#TAP")}

type InstructionIconData struct {
	sonolus.InstructionIconResource

	Tap sonolus.Icon
}

var InstructionIcons = &InstructionIconData{Tap: sonolus.InstructionIcon("#HAND")}

type Globals struct{ tutorial.GlobalCallbacks }

var Global Globals

func Preprocess() { tutorial.Debug.Log(sum(1.0, 2.0)) }
func Navigate()   { tutorial.Instruction.Show(Instructions.Tap) }
func Update() {
	tutorial.Instruction.Paint(InstructionIcons.Tap, sonolus.NewVec2(0, 0), 1, 0, 0, 1)
}
