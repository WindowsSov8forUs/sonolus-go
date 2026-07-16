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

var tutorialDurations = [2]float64{1.5, 2.5}

func Preprocess() {
	skinTransform := tutorial.SkinTransform.Get()
	tutorial.SkinTransform.Set(skinTransform)
	particleTransform := tutorial.ParticleTransform.Get()
	tutorial.ParticleTransform.Set(particleTransform)
	tutorial.TutorialData.Set(1, tutorial.TutorialData.Get(0))
	tutorial.TutorialMemory.Set(0, tutorial.TutorialData.Get(0))
	value := tutorial.TutorialMemory.Get(0)
	value += tutorial.Time.Now() + tutorial.Time.Delta() + tutorial.Time.Scaled() + tutorial.Time.Previous() + tutorial.Time.OffsetAdjusted()
	value += tutorial.Time.BeatToBPM(1) + tutorial.Time.BeatToTime(1)
	value += tutorial.Time.BeatToStartingBeat(1) + tutorial.Time.BeatToStartingTime(1)
	value += tutorial.Time.TimeToScaledTime(1) + tutorial.Time.TimeToStartingScaledTime(1)
	value += tutorial.Time.TimeToStartingTime(1) + tutorial.Time.TimeToTimeScale(1)
	value += tutorial.Screen.Rect().Width() + tutorial.SafeArea.Rect().Width() + tutorial.Audio.Offset()
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
	tutorial.Debug.Log(tutorialDurations[int(tutorial.Time.Now())%len(tutorialDurations)])
	tutorial.Instruction.Paint(Icons.Tap, sonolus.NewVec2(0, 0), 1, 0, 0, 1)
}

func main() {}
