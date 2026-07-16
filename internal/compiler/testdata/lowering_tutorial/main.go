package main

import (
	"github.com/WindowsSov8forUs/sonolus-go/v2/sonolus"
	"github.com/WindowsSov8forUs/sonolus-go/v2/sonolus/tutorial"
)

type Callbacks struct{ tutorial.GlobalCallbacks }

var Globals Callbacks

type TextData struct {
	sonolus.InstructionResource
	Tap sonolus.Text
}

var Texts = &TextData{Tap: sonolus.InstructionText("Tap")}

type IconData struct {
	sonolus.InstructionIconResource
	Tap sonolus.Icon
}

var Icons = &IconData{Tap: sonolus.InstructionIcon("#HAND")}

type EffectsData struct {
	sonolus.EffectResource
	Hit sonolus.Clip
}

var Effects = &EffectsData{Hit: sonolus.EffectClip("hit")}

func Preprocess() {
	tutorial.TutorialMemory.Set(0, tutorial.TutorialData.Get(0))
	value := tutorial.TutorialMemory.Get(0)
	value += tutorial.Time.Now() + tutorial.Time.Delta()
	value += tutorial.Screen.Rect().Width() + tutorial.SafeArea.Rect().Width()
	tutorial.Audio.Play(Effects.Hit, 0)
	tutorial.Audio.PlayScheduled(Effects.Hit, 1, 0)
	tutorial.UI.SetMenu(tutorial.UI.Menu())
	tutorial.UI.SetPrevious(tutorial.UI.Previous())
	tutorial.UI.SetNext(tutorial.UI.Next())
	tutorial.UI.SetInstruction(tutorial.UI.Instruction())
	value += tutorial.UI.MenuConfiguration().Scale + tutorial.UI.NavigationConfiguration().Scale
	value += tutorial.UI.InstructionConfiguration().Scale
	background := tutorial.Background.Get()
	tutorial.Background.Set(background)
	_, _ = tutorial.Environment.Debug(), tutorial.Environment.AspectRatio()
	tutorial.Debug.Log(value)
	tutorial.Debug.Pause()
}

func Navigate() {
	if tutorial.Navigation.Direction() == tutorial.NavigationNext {
		tutorial.Instruction.Show(Texts.Tap)
	} else {
		tutorial.Instruction.Clear()
	}
}

func Update() {
	tutorial.Instruction.Paint(Icons.Tap, sonolus.NewVec2(0, 0), 1, 0, 0, 1)
}

func main() {}
