package main

import "github.com/WindowsSov8forUs/sonolus-go/sonolus"

//sonolus:resource instruction
type InstructionTexts struct {
	Tap sonolus.Text
}

//sonolus:resource instruction
var Texts = &InstructionTexts{
	Tap: sonolus.InstructionText("Tap"),
}

//sonolus:resource instructionIcon
type InstructionIcons struct {
	Tap sonolus.Icon
}

//sonolus:resource instructionIcon
var Icons = &InstructionIcons{
	Tap: sonolus.InstructionIcon("#HAND"),
}

func main() {}
