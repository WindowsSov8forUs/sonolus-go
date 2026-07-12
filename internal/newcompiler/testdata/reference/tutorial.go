//go:build tutorial

package main

import (
	"github.com/WindowsSov8forUs/sonolus-go/sonolus"
	"github.com/WindowsSov8forUs/sonolus-go/sonolus/tutorial"
)

//sonolus:resource skin
type SkinData struct{ Note sonolus.Sprite }

//sonolus:resource skin
var Skin = &SkinData{Note: sonolus.SkinSprite("note")}

//sonolus:resource effect
type EffectData struct{ Hit sonolus.Clip }

//sonolus:resource effect
var Effects = &EffectData{Hit: sonolus.EffectClip("hit")}

//sonolus:resource particle
type ParticleData struct{ Hit sonolus.Effect }

//sonolus:resource particle
var Particles = &ParticleData{Hit: sonolus.ParticleEffect("hit")}

//sonolus:resource instruction
type TextData struct{ Tap sonolus.Text }

//sonolus:resource instruction
var Texts = &TextData{Tap: sonolus.InstructionText("Tap")}

//sonolus:resource instructionIcon
type IconData struct{ Tap sonolus.Icon }

//sonolus:resource instructionIcon
var Icons = &IconData{Tap: sonolus.InstructionIcon("#HAND")}

type Globals struct{ tutorial.GlobalCallbacks }

var Global Globals

func Preprocess() { tutorial.Debug.Log(1) }
func Navigate()   { tutorial.Instruction.Show(Texts.Tap) }
func Update()     { tutorial.Instruction.Paint(Icons.Tap, sonolus.NewVec2(0, 0), 1, 0, 0, 1) }
