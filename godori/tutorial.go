//go:build tutorial

package main

import (
	"math"

	"github.com/WindowsSov8forUs/sonolus-go/v2/sonolus"
	"github.com/WindowsSov8forUs/sonolus-go/v2/sonolus/tutorial"
)

type TutorialSkin struct {
	sonolus.SkinResource
	Lane, JudgmentLine, Note sonolus.Sprite
}

var Skin = &TutorialSkin{
	SkinResource: sonolus.SkinResource{RenderMode: sonolus.RenderModeLightweight},
	Lane:         sonolus.SkinSprite(sonolus.StandardSpriteLane), JudgmentLine: sonolus.SkinSprite(sonolus.StandardSpriteJudgmentLine),
	Note: sonolus.SkinSprite(sonolus.StandardSpriteNoteHeadCyan),
}

type TutorialEffects struct {
	sonolus.EffectResource
	Perfect sonolus.Clip
}

var Effects = &TutorialEffects{Perfect: sonolus.EffectClip(sonolus.StandardClipPerfect)}

type TutorialParticles struct {
	sonolus.ParticleResource
	Tap sonolus.Effect
}

var Particles = &TutorialParticles{Tap: sonolus.ParticleEffect(sonolus.StandardEffectNoteCircularTapCyan)}

type TutorialInstructions struct {
	sonolus.InstructionResource
	Tap sonolus.Text
}

var Instructions = &TutorialInstructions{Tap: sonolus.InstructionText("#TAP")}

type TutorialIcons struct {
	sonolus.InstructionIconResource
	Hand sonolus.Icon
}

var InstructionIcons = &TutorialIcons{Hand: sonolus.InstructionIcon("#HAND")}

type TutorialGlobals struct{ tutorial.GlobalCallbacks }

var Global TutorialGlobals

func Preprocess() {
	tutorial.TutorialMemory.Set(0, 0)
	tutorial.Instruction.Show(Instructions.Tap)
}

func Navigate() {
	phase := tutorial.TutorialMemory.Get(0)
	if tutorial.Navigation.Direction() == tutorial.NavigationNext {
		phase += 1
	} else if tutorial.Navigation.Direction() == tutorial.NavigationPrevious {
		phase -= 1
	}
	if phase < 0 {
		phase = 0
	}
	tutorial.TutorialMemory.Set(0, phase)
	tutorial.Instruction.Show(Instructions.Tap)
}

func Update() {
	progress := math.Mod(tutorial.Time.Now(), 2) / 2
	y := judgmentLineY + (1-progress)*1.6
	Skin.Lane.Draw(stageRect().ToQuad(), 10, 0.8)
	Skin.JudgmentLine.Draw(judgmentRect().ToQuad(), 20, 1)
	Skin.Note.Draw(noteRect(0, y).ToQuad(), 30, 1)
	if progress > 0.8 {
		tutorial.Instruction.Paint(InstructionIcons.Hand, sonolus.NewVec2(0, judgmentLineY), 0.5, 0, 40, 1)
	}
}
