// Package backend lowers source-independent frontend IR into Sonolus engine data.
package backend

import (
	"fmt"

	"github.com/WindowsSov8forUs/sonolus-core-go/core/resource"
	"github.com/WindowsSov8forUs/sonolus-go/internal/compiler/frontend"
	"github.com/WindowsSov8forUs/sonolus-go/internal/compiler/mode"
)

type Artifacts struct {
	Configuration *resource.EngineConfiguration
	ROM           []byte
	Play          *resource.EnginePlayData
	Watch         *resource.EngineWatchData
	Preview       *resource.EnginePreviewData
	Tutorial      *resource.EngineTutorialData
}

func Compile(project *frontend.Project) (*Artifacts, error) {
	if project == nil {
		return nil, fmt.Errorf("backend: project is nil")
	}
	if len(project.ROM)%4 != 0 {
		return nil, fmt.Errorf("backend: ROM length %d is not divisible by 4", len(project.ROM))
	}
	result := &Artifacts{Configuration: project.Configuration, ROM: buildROM(project.ROM)}
	for _, m := range []mode.Mode{mode.ModePlay, mode.ModeWatch, mode.ModePreview, mode.ModeTutorial} {
		declarations := project.Modes[m]
		if declarations == nil {
			continue
		}
		var err error
		switch m {
		case mode.ModePlay:
			result.Play, err = assemblePlay(declarations)
		case mode.ModeWatch:
			result.Watch, err = assembleWatch(declarations)
		case mode.ModePreview:
			result.Preview, err = assemblePreview(declarations)
		case mode.ModeTutorial:
			result.Tutorial, err = assembleTutorial(declarations)
		}
		if err != nil {
			return nil, fmt.Errorf("backend: compile %s mode: %w", m, err)
		}
	}
	return result, nil
}

func compileCallback(m mode.Mode, callback *frontend.CallbackDeclaration) (snode, bool, error) {
	if callback == nil || callback.IR == nil {
		return nil, false, fmt.Errorf("callback has no IR")
	}
	node, err := finalizeFunction(m, callback.IR)
	if err != nil {
		return nil, false, fmt.Errorf("callback %s: %w", callback.Name, err)
	}
	node = simplify(node)
	if value, ok := node.(valueNode); ok {
		return node, omitConstantCallback(m, callback.Name, value), nil
	}
	return node, false, nil
}

func omitConstantCallback(m mode.Mode, callback string, value valueNode) bool {
	switch {
	case m == mode.ModePlay && callback == "spawnOrder":
		return value == 0
	case m == mode.ModePlay && callback == "shouldSpawn":
		return value != 0
	case m == mode.ModeWatch && (callback == "spawnTime" || callback == "despawnTime" || callback == "updateSpawn"):
		return value == 0
	default:
		return true
	}
}

func appendCallback(appender *nodeAppender, m mode.Mode, callback *frontend.CallbackDeclaration) (int, bool, error) {
	node, omit, err := compileCallback(m, callback)
	if err != nil || omit {
		return 0, omit, err
	}
	index, err := appender.append(node)
	return index, false, err
}

func skin(resources frontend.ModeResources) resource.EngineSkinData {
	if resources.Skin == nil {
		return resource.EngineSkinData{Sprites: []resource.EngineSkinDataSprite{}}
	}
	value := *resources.Skin
	value.Sprites = nonNil(value.Sprites)
	return value
}

func effect(resources frontend.ModeResources) resource.EngineEffectData {
	if resources.Effect == nil {
		return resource.EngineEffectData{Clips: []resource.EngineEffectDataClip{}}
	}
	value := *resources.Effect
	value.Clips = nonNil(value.Clips)
	return value
}

func particle(resources frontend.ModeResources) resource.EngineParticleData {
	if resources.Particle == nil {
		return resource.EngineParticleData{Effects: []resource.EngineParticleDataEffect{}}
	}
	value := *resources.Particle
	value.Effects = nonNil(value.Effects)
	return value
}

func instruction(resources frontend.ModeResources) resource.EngineInstructionData {
	if resources.Instruction == nil {
		return resource.EngineInstructionData{Texts: []resource.EngineInstructionDataText{}, Icons: []resource.EngineInstructionDataIcon{}}
	}
	value := *resources.Instruction
	value.Texts = nonNil(value.Texts)
	value.Icons = nonNil(value.Icons)
	return value
}

func nonNil[T any](values []T) []T {
	if values == nil {
		return []T{}
	}
	return append([]T(nil), values...)
}
