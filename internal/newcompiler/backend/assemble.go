package backend

import (
	"fmt"

	"github.com/WindowsSov8forUs/sonolus-core-go/core/resource"
	"github.com/WindowsSov8forUs/sonolus-go/internal/newcompiler/frontend"
	"github.com/WindowsSov8forUs/sonolus-go/internal/newcompiler/mode"
)

func assemblePlay(declarations *frontend.ModeDeclarations) (*resource.EnginePlayData, error) {
	appender := newNodeAppender()
	archetypes := make([]resource.EnginePlayDataArchetype, len(declarations.Archetypes))
	for i, declaration := range declarations.Archetypes {
		archetype := resource.EnginePlayDataArchetype{
			Name: resource.EngineArchetypeName(declaration.Name), HasInput: declaration.HasInput,
			Imports: nonNil(declaration.Imports), Exports: nonNil(declaration.Exports),
		}
		for _, callback := range declaration.Callbacks {
			index, omit, err := appendCallback(appender, mode.ModePlay, callback)
			if err != nil {
				return nil, fmt.Errorf("archetype %s: %w", declaration.Name, err)
			}
			if omit {
				continue
			}
			value := &resource.EnginePlayDataArchetypeCallback{Index: index, Order: callback.Order}
			switch callback.Name {
			case "preprocess":
				archetype.Preprocess = value
			case "spawnOrder":
				archetype.SpawnOrder = value
			case "shouldSpawn":
				archetype.ShouldSpawn = value
			case "initialize":
				archetype.Initialize = value
			case "updateSequential":
				archetype.UpdateSequential = value
			case "touch":
				archetype.Touch = value
			case "updateParallel":
				archetype.UpdateParallel = value
			case "terminate":
				archetype.Terminate = value
			default:
				return nil, fmt.Errorf("archetype %s has unknown callback %q", declaration.Name, callback.Name)
			}
		}
		archetypes[i] = archetype
	}
	return &resource.EnginePlayData{
		Skin: skin(declarations.Resources), Effect: effect(declarations.Resources), Particle: particle(declarations.Resources),
		Buckets: nonNil(declarations.Resources.Buckets), Archetypes: archetypes, Nodes: nonNil(appender.nodes),
	}, nil
}

func assembleWatch(declarations *frontend.ModeDeclarations) (*resource.EngineWatchData, error) {
	appender := newNodeAppender()
	archetypes := make([]resource.EngineWatchDataArchetype, len(declarations.Archetypes))
	for i, declaration := range declarations.Archetypes {
		archetype := resource.EngineWatchDataArchetype{
			Name: resource.EngineArchetypeName(declaration.Name), HasInput: declaration.HasInput, Imports: nonNil(declaration.Imports),
		}
		for _, callback := range declaration.Callbacks {
			index, omit, err := appendCallback(appender, mode.ModeWatch, callback)
			if err != nil {
				return nil, fmt.Errorf("archetype %s: %w", declaration.Name, err)
			}
			if omit {
				continue
			}
			value := &resource.EngineWatchDataArchetypeCallback{Index: index, Order: callback.Order}
			switch callback.Name {
			case "preprocess":
				archetype.Preprocess = value
			case "spawnTime":
				archetype.SpawnTime = value
			case "despawnTime":
				archetype.DespawnTime = value
			case "initialize":
				archetype.Initialize = value
			case "updateSequential":
				archetype.UpdateSequential = value
			case "updateParallel":
				archetype.UpdateParallel = value
			case "terminate":
				archetype.Terminate = value
			default:
				return nil, fmt.Errorf("archetype %s has unknown callback %q", declaration.Name, callback.Name)
			}
		}
		archetypes[i] = archetype
	}
	result := &resource.EngineWatchData{
		Skin: skin(declarations.Resources), Effect: effect(declarations.Resources), Particle: particle(declarations.Resources),
		Buckets: nonNil(declarations.Resources.Buckets), Archetypes: archetypes,
	}
	for _, callback := range declarations.Globals {
		if callback.Name != "updateSpawn" {
			return nil, fmt.Errorf("unknown global callback %q", callback.Name)
		}
		index, omit, err := appendCallback(appender, mode.ModeWatch, callback)
		if err != nil {
			return nil, err
		}
		if !omit {
			result.UpdateSpawn = index
		}
	}
	result.Nodes = nonNil(appender.nodes)
	return result, nil
}

func assemblePreview(declarations *frontend.ModeDeclarations) (*resource.EnginePreviewData, error) {
	appender := newNodeAppender()
	archetypes := make([]resource.EnginePreviewDataArchetype, len(declarations.Archetypes))
	for i, declaration := range declarations.Archetypes {
		archetype := resource.EnginePreviewDataArchetype{Name: resource.EngineArchetypeName(declaration.Name), Imports: nonNil(declaration.Imports)}
		for _, callback := range declaration.Callbacks {
			index, omit, err := appendCallback(appender, mode.ModePreview, callback)
			if err != nil {
				return nil, fmt.Errorf("archetype %s: %w", declaration.Name, err)
			}
			if omit {
				continue
			}
			value := &resource.EnginePreviewDataArchetypeCallback{Index: index, Order: callback.Order}
			switch callback.Name {
			case "preprocess":
				archetype.Preprocess = value
			case "render":
				archetype.Render = value
			default:
				return nil, fmt.Errorf("archetype %s has unknown callback %q", declaration.Name, callback.Name)
			}
		}
		archetypes[i] = archetype
	}
	return &resource.EnginePreviewData{Skin: skin(declarations.Resources), Archetypes: archetypes, Nodes: nonNil(appender.nodes)}, nil
}

func assembleTutorial(declarations *frontend.ModeDeclarations) (*resource.EngineTutorialData, error) {
	appender := newNodeAppender()
	result := &resource.EngineTutorialData{
		Skin: skin(declarations.Resources), Effect: effect(declarations.Resources), Particle: particle(declarations.Resources),
		Instruction: instruction(declarations.Resources),
	}
	groups := map[string][]snode{}
	for _, callback := range declarations.Globals {
		node, omit, err := compileCallback(mode.ModeTutorial, callback)
		if err != nil {
			return nil, err
		}
		if !omit {
			groups[callback.Name] = append(groups[callback.Name], node)
		}
	}
	for _, name := range []string{"preprocess", "navigate", "update"} {
		nodes := groups[name]
		if len(nodes) == 0 {
			continue
		}
		index, err := appender.append(execute(nodes...))
		if err != nil {
			return nil, err
		}
		switch name {
		case "preprocess":
			result.Preprocess = index
		case "navigate":
			result.Navigate = index
		case "update":
			result.Update = index
		}
	}
	for name := range groups {
		if name != "preprocess" && name != "navigate" && name != "update" {
			return nil, fmt.Errorf("unknown global callback %q", name)
		}
	}
	result.Nodes = nonNil(appender.nodes)
	return result, nil
}
