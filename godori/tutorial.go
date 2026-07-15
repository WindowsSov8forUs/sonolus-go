//go:build tutorial

package main

import (
	"math"

	"github.com/WindowsSov8forUs/sonolus-go/v2/sonolus"
	"github.com/WindowsSov8forUs/sonolus-go/v2/sonolus/tutorial"
)

type TutorialSkin struct {
	sonolus.SkinResource
	Lane, JudgmentLine, Note, Flick, FlickArrow, RightFlick, RightFlickArrow, LeftFlick, LeftFlickArrow sonolus.Sprite
	HoldHead, HoldTail, HoldConnector, HoldTick                                                         sonolus.Sprite
	StageMiddle, LeftBorder, RightBorder, Slot, Cover                                                   sonolus.Sprite
}

var Skin = &TutorialSkin{
	SkinResource: sonolus.SkinResource{RenderMode: sonolus.RenderModeLightweight},
	Lane:         sonolus.SkinSprite(sonolus.StandardSpriteLane), JudgmentLine: sonolus.SkinSprite(sonolus.StandardSpriteJudgmentLine),
	Note: sonolus.SkinSprite(sonolus.StandardSpriteNoteHeadCyan), Flick: sonolus.SkinSprite(sonolus.StandardSpriteNoteHeadRed),
	FlickArrow: sonolus.SkinSprite(sonolus.StandardSpriteDirectionalMarkerRed),
	RightFlick: sonolus.SkinSprite(sonolus.StandardSpriteNoteHeadYellow), RightFlickArrow: sonolus.SkinSprite(sonolus.StandardSpriteDirectionalMarkerYellow),
	LeftFlick: sonolus.SkinSprite(sonolus.StandardSpriteNoteHeadPurple), LeftFlickArrow: sonolus.SkinSprite(sonolus.StandardSpriteDirectionalMarkerPurple),
	HoldHead: sonolus.SkinSprite(sonolus.StandardSpriteNoteHeadGreen), HoldTail: sonolus.SkinSprite(sonolus.StandardSpriteNoteTailGreen),
	HoldConnector: sonolus.SkinSprite(sonolus.StandardSpriteNoteConnectionGreenSeamless),
	HoldTick:      sonolus.SkinSprite(sonolus.StandardSpriteNoteTickGreen),
	StageMiddle:   sonolus.SkinSprite(sonolus.StandardSpriteStageMiddle), LeftBorder: sonolus.SkinSprite(sonolus.StandardSpriteStageLeftBorder),
	RightBorder: sonolus.SkinSprite(sonolus.StandardSpriteStageRightBorder), Slot: sonolus.SkinSprite(sonolus.StandardSpriteNoteSlot),
	Cover: sonolus.SkinSprite(sonolus.StandardSpriteStageCover),
}

type TutorialEffects struct {
	sonolus.EffectResource
	Stage, Perfect, Great, Good                           sonolus.Clip
	PerfectAlternative, GreatAlternative, GoodAlternative sonolus.Clip
	Hold                                                  sonolus.Clip
}

var Effects = &TutorialEffects{
	Stage: sonolus.EffectClip(sonolus.StandardClipStage), Perfect: sonolus.EffectClip(sonolus.StandardClipPerfect),
	Great: sonolus.EffectClip(sonolus.StandardClipGreat), Good: sonolus.EffectClip(sonolus.StandardClipGood),
	PerfectAlternative: sonolus.EffectClip(sonolus.StandardClipPerfectAlternative),
	GreatAlternative:   sonolus.EffectClip(sonolus.StandardClipGreatAlternative),
	GoodAlternative:    sonolus.EffectClip(sonolus.StandardClipGoodAlternative),
	Hold:               sonolus.EffectClip(sonolus.StandardClipHold),
}

type TutorialParticles struct {
	sonolus.ParticleResource
	Lane, TapLinear, Tap, HoldLinear, Hold, HoldActive sonolus.Effect
	FlickLinear, Flick                                 sonolus.Effect
	RightFlickLinear, RightFlick                       sonolus.Effect
	LeftFlickLinear, LeftFlick                         sonolus.Effect
}

var Particles = &TutorialParticles{
	Lane:      sonolus.ParticleEffect(sonolus.StandardEffectLaneLinear),
	TapLinear: sonolus.ParticleEffect(sonolus.StandardEffectNoteLinearTapCyan), Tap: sonolus.ParticleEffect(sonolus.StandardEffectNoteCircularTapCyan),
	HoldLinear: sonolus.ParticleEffect(sonolus.StandardEffectNoteLinearTapGreen), Hold: sonolus.ParticleEffect(sonolus.StandardEffectNoteCircularTapGreen),
	HoldActive:  sonolus.ParticleEffect(sonolus.StandardEffectNoteCircularHoldGreen),
	FlickLinear: sonolus.ParticleEffect(sonolus.StandardEffectNoteLinearAlternativeRed), Flick: sonolus.ParticleEffect(sonolus.StandardEffectNoteCircularAlternativeRed),
	RightFlickLinear: sonolus.ParticleEffect(sonolus.StandardEffectNoteLinearAlternativeYellow), RightFlick: sonolus.ParticleEffect(sonolus.StandardEffectNoteCircularAlternativeYellow),
	LeftFlickLinear: sonolus.ParticleEffect(sonolus.StandardEffectNoteLinearAlternativePurple), LeftFlick: sonolus.ParticleEffect(sonolus.StandardEffectNoteCircularAlternativePurple),
}

type TutorialInstructions struct {
	sonolus.InstructionResource
	Tap, TapFlick, TapHold, HoldFollow, Release, HoldFlick sonolus.Text
}

var Instructions = &TutorialInstructions{
	Tap:        sonolus.InstructionText("#TAP"),
	TapFlick:   sonolus.InstructionText("#TAP_FLICK"),
	TapHold:    sonolus.InstructionText("#TAP_HOLD"),
	HoldFollow: sonolus.InstructionText("#HOLD_FOLLOW"),
	Release:    sonolus.InstructionText("#RELEASE"),
	HoldFlick:  sonolus.InstructionText("#FLICK"),
}

type TutorialIcons struct {
	sonolus.InstructionIconResource
	Hand sonolus.Icon
}

var InstructionIcons = &TutorialIcons{Hand: sonolus.InstructionIcon("#HAND")}

type TutorialGlobals struct{ tutorial.GlobalCallbacks }

var Global TutorialGlobals

func Preprocess() {
	tutorial.TutorialMemory.Set(0, 0)
	tutorial.TutorialMemory.Set(1, -1)
	tutorial.TutorialMemory.Set(2, 0)
	tutorial.TutorialMemory.Set(3, 0)
	tutorial.TutorialMemory.Set(4, 0)
	tutorial.UI.SetMenu(basicMenuLayout())
	tutorial.UI.SetPrevious(tutorialPreviousLayout())
	tutorial.UI.SetNext(tutorialNextLayout())
	tutorial.UI.SetInstruction(tutorialInstructionLayout())
	showTutorialInstruction(0)
}

func Navigate() {
	stopTutorialHold()
	phase := tutorial.TutorialMemory.Get(0)
	if tutorial.Navigation.Direction() == tutorial.NavigationNext {
		phase += 1
	} else if tutorial.Navigation.Direction() == tutorial.NavigationPrevious {
		phase -= 1
	}
	if phase < 0 {
		phase = 0
	} else if phase > 6 {
		phase = 6
	}
	tutorial.TutorialMemory.Set(0, phase)
	tutorial.TutorialMemory.Set(1, -1)
	showTutorialInstruction(phase)
}

func Update() {
	now := tutorial.Time.Now()
	progress := math.Mod(now, 2) / 2
	y := judgmentLineY + (1-progress)*1.6
	phase := tutorial.TutorialMemory.Get(0)
	if progress < 0.05 {
		stopTutorialHold()
	}
	drawTutorialStage()
	if phase == 0 {
		Skin.Note.Draw(noteRect(0, y).ToQuad(), 30, 1)
		if progress > 0.8 {
			tutorial.Instruction.Paint(InstructionIcons.Hand, sonolus.NewVec2(0, judgmentLineY), 0.5, 0, 40, 1)
		}
	} else if phase == 1 {
		Skin.Flick.Draw(noteRect(0, y).ToQuad(), 30, 1)
		Skin.FlickArrow.Draw(noteRect(0, y).Scale(0.7).ToQuad(), 31, 1)
		if progress > 0.8 {
			x := (progress-0.8)/0.2*0.8 - 0.4
			tutorial.Instruction.Paint(InstructionIcons.Hand, sonolus.NewVec2(x, judgmentLineY), 0.5, 0, 40, 1)
		}
	} else if phase == 2 {
		direction := displayDirection(1)
		if direction > 0 {
			Skin.RightFlick.Draw(noteRect(0, y).ToQuad(), 30, 1)
			Skin.RightFlickArrow.Draw(noteRect(0, y).Scale(0.7).ToQuad(), 31, 1)
		} else {
			Skin.LeftFlick.Draw(noteRect(0, y).ToQuad(), 30, 1)
			Skin.LeftFlickArrow.Draw(noteRect(0, y).Scale(0.7).ToQuad(), 31, 1)
		}
		if progress > 0.8 {
			x := ((progress-0.8)/0.2*0.8 - 0.4) * direction
			tutorial.Instruction.Paint(InstructionIcons.Hand, sonolus.NewVec2(x, judgmentLineY), 0.5, 0, 40, 1)
		}
	} else if phase == 3 {
		startY := y
		if startY < judgmentLineY {
			startY = judgmentLineY
		}
		Skin.HoldConnector.Draw(holdConnectorQuad(0, 0, startY, 1.1), 25, Config.ConnectorAlpha)
		Skin.HoldHead.Draw(noteRect(0, startY).ToQuad(), 30, 1)
		if progress > 0.5 {
			updateTutorialHold(0)
			tutorial.Instruction.Paint(InstructionIcons.Hand, sonolus.NewVec2(0, judgmentLineY), 0.5, 0, 40, 1)
		}
	} else if phase == 4 {
		lane := math.Sin(progress*2*math.Pi) * 2
		updateTutorialHold(lane)
		Skin.HoldHead.Draw(noteRect(lane, judgmentLineY).ToQuad(), 30, 1)
		for i := 0; i < 4; i++ {
			tickProgress := math.Mod(progress+float64(i)*0.25, 1)
			tickLane := math.Sin(tickProgress*2*math.Pi) * 2
			tickY := judgmentLineY + (1-tickProgress)*1.6
			Skin.HoldTick.Draw(noteRect(tickLane, tickY).Scale(0.7).ToQuad(), 29, 1)
		}
		Skin.HoldConnector.Draw(holdConnectorQuad(lane, -lane, judgmentLineY, 1.1), 25, Config.ConnectorAlpha)
		tutorial.Instruction.Paint(InstructionIcons.Hand, sonolus.NewVec2(laneX(lane), judgmentLineY), 0.5, 0, 40, 1)
	} else if phase == 5 {
		endY := y
		if progress < 0.8 {
			updateTutorialHold(0)
		} else {
			stopTutorialHold()
			endY = judgmentLineY
		}
		Skin.HoldHead.Draw(noteRect(0, judgmentLineY).ToQuad(), 30, 1)
		Skin.HoldTail.Draw(noteRect(0, endY).ToQuad(), 30, 1)
		Skin.HoldConnector.Draw(holdConnectorQuad(0, 0, judgmentLineY, endY), 25, Config.ConnectorAlpha)
		tutorial.Instruction.Paint(InstructionIcons.Hand, sonolus.NewVec2(0, judgmentLineY+(progress-0.5)*0.8), 0.5, 0, 40, 1)
	} else {
		if progress < 0.75 {
			updateTutorialHold(0)
		} else {
			stopTutorialHold()
		}
		Skin.Flick.Draw(noteRect(0, judgmentLineY).ToQuad(), 30, 1)
		Skin.FlickArrow.Draw(noteRect(0, judgmentLineY).Scale(0.7).ToQuad(), 31, 1)
		Skin.HoldConnector.Draw(holdConnectorQuad(0, 0, judgmentLineY, 1.1), 25, Config.ConnectorAlpha)
		x := 0.0
		if progress > 0.75 {
			x = (progress - 0.75) / 0.25 * 0.8
		}
		tutorial.Instruction.Paint(InstructionIcons.Hand, sonolus.NewVec2(x, judgmentLineY), 0.5, 0, 40, 1)
	}
	if progress > 0.95 {
		if tutorial.TutorialMemory.Get(1) == 0 {
			tutorial.TutorialMemory.Set(1, 1)
			if Config.SFX {
				if phase == 1 || phase == 2 || phase == 6 {
					tutorial.Audio.Play(Effects.PerfectAlternative, 0.02)
				} else {
					tutorial.Audio.Play(Effects.Perfect, 0.02)
				}
			}
			if Config.NoteEffects {
				if phase == 0 {
					spawnTutorialNoteParticles(0, Particles.TapLinear, Particles.Tap)
				} else if phase == 1 {
					spawnTutorialNoteParticles(0, Particles.FlickLinear, Particles.Flick)
				} else if phase == 2 {
					if displayDirection(1) > 0 {
						spawnTutorialNoteParticles(0, Particles.RightFlickLinear, Particles.RightFlick)
					} else {
						spawnTutorialNoteParticles(0, Particles.LeftFlickLinear, Particles.LeftFlick)
					}
				} else if phase == 3 {
					spawnTutorialNoteParticles(1, Particles.HoldLinear, Particles.Hold)
				} else if phase == 4 {
					spawnTutorialNoteParticles(0, Particles.HoldLinear, Particles.Hold)
				} else if phase == 5 {
					spawnTutorialNoteParticles(0, Particles.HoldLinear, Particles.Hold)
				} else {
					spawnTutorialNoteParticles(0, Particles.FlickLinear, Particles.Flick)
				}
			}
			if Config.LaneEffects {
				lane := 0.0
				if phase >= 3 {
					lane = 1
				}
				Particles.Lane.Spawn(laneRect(lane).ToQuad(), 0.2, false)
			}
		}
	} else {
		tutorial.TutorialMemory.Set(1, 0)
	}
}

func showTutorialInstruction(phase float64) {
	if phase == 0 {
		tutorial.Instruction.Show(Instructions.Tap)
	} else if phase <= 2 {
		tutorial.Instruction.Show(Instructions.TapFlick)
	} else if phase == 3 {
		tutorial.Instruction.Show(Instructions.TapHold)
	} else if phase == 4 {
		tutorial.Instruction.Show(Instructions.HoldFollow)
	} else if phase == 5 {
		tutorial.Instruction.Show(Instructions.Release)
	} else {
		tutorial.Instruction.Show(Instructions.HoldFlick)
	}
}

func updateTutorialHold(lane float64) {
	quad := noteRect(lane, judgmentLineY).Scale(1.5).ToQuad()
	if tutorial.TutorialMemory.Get(4) == 0 {
		if Config.NoteEffects {
			particle := Particles.HoldActive.Spawn(quad, 0.3, true)
			tutorial.TutorialMemory.Set(2, particle.ID)
		}
		if Config.SFX {
			loop := Effects.Hold.PlayLooped()
			tutorial.TutorialMemory.Set(3, loop.ID)
		}
		tutorial.TutorialMemory.Set(4, 1)
	} else if Config.NoteEffects {
		particle := sonolus.ParticleHandle{ID: tutorial.TutorialMemory.Get(2)}
		particle.Move(quad)
	}
}

func spawnTutorialNoteParticles(lane float64, linear, circular sonolus.Effect) {
	particleRect := noteRect(lane, judgmentLineY).Scale(1.5)
	width := particleRect.R - particleRect.L
	linear.Spawn(sonolus.Rect{T: judgmentLineY + width, R: particleRect.R, B: judgmentLineY, L: particleRect.L}.ToQuad(), 0.3, false)
	circular.Spawn(particleRect.ToQuad(), 0.3, false)
}

func stopTutorialHold() {
	if tutorial.TutorialMemory.Get(4) == 0 {
		return
	}
	if Config.NoteEffects {
		particle := sonolus.ParticleHandle{ID: tutorial.TutorialMemory.Get(2)}
		particle.Destroy()
	}
	if Config.SFX {
		loop := sonolus.LoopedEffectHandle{ID: tutorial.TutorialMemory.Get(3)}
		loop.Stop()
	}
	tutorial.TutorialMemory.Set(2, 0)
	tutorial.TutorialMemory.Set(3, 0)
	tutorial.TutorialMemory.Set(4, 0)
}

func drawTutorialStage() {
	Skin.StageMiddle.Draw(stageRect().ToQuad(), 5, 1)
	for lane := -3; lane <= 3; lane++ {
		Skin.Lane.Draw(laneRect(float64(lane)).ToQuad(), 10, 0.8)
		Skin.Slot.Draw(noteRect(float64(lane), judgmentLineY).Scale(0.65).ToQuad(), 15, 0.7)
	}
	Skin.LeftBorder.Draw(leftBorderRect().ToQuad(), 12, 1)
	Skin.RightBorder.Draw(rightBorderRect().ToQuad(), 12, 1)
	Skin.JudgmentLine.Draw(judgmentRect().ToQuad(), 20, 1)
	Skin.Cover.Draw(stageCoverRect().ToQuad(), 40, 1)
}
