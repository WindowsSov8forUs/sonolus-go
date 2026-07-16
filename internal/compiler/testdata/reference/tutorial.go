//go:build tutorial

package main

import (
	"github.com/WindowsSov8forUs/sonolus-go/sonolus"
	"github.com/WindowsSov8forUs/sonolus-go/sonolus/tutorial"
)

type SkinData struct {
	sonolus.SkinResource
	Note sonolus.Sprite
}

var Skin = &SkinData{Note: sonolus.SkinSprite("note")}

type EffectData struct {
	sonolus.EffectResource
	Hit sonolus.Clip
}

var Effects = &EffectData{Hit: sonolus.EffectClip("hit")}

type ParticleData struct {
	sonolus.ParticleResource
	Hit sonolus.Effect
}

var Particles = &ParticleData{Hit: sonolus.ParticleEffect("hit")}

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

type Globals struct{ tutorial.GlobalCallbacks }

var Global Globals

func Preprocess() { tutorial.Debug.Log(1) }
func Navigate()   { tutorial.Instruction.Show(Texts.Tap) }
func Update()     { tutorial.Instruction.Paint(Icons.Tap, sonolus.NewVec2(0, 0), 1, 0, 0, 1) }
