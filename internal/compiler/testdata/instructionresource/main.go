package main

import "github.com/WindowsSov8forUs/sonolus-go/sonolus"

type InstructionTexts struct {
	sonolus.InstructionResource

	Tap sonolus.Text
}

var Texts = &InstructionTexts{
	Tap: sonolus.InstructionText("Tap"),
}

type InstructionIcons struct {
	sonolus.InstructionIconResource

	Tap sonolus.Icon
}

var Icons = &InstructionIcons{
	Tap: sonolus.InstructionIcon("#HAND"),
}

func main() {}
