// Package sim compiles and executes final Sonolus EngineData callbacks.
package sim

import (
	"fmt"

	"github.com/WindowsSov8forUs/sonolus-core-go/core/resource"
	"github.com/WindowsSov8forUs/sonolus-go/v2/internal/compiler"
	"github.com/WindowsSov8forUs/sonolus-go/v2/internal/simexec"
)

type Options = compiler.Options
type Mode = compiler.Mode
type OptimizationLevel = compiler.OptimizationLevel
type RuntimeChecks = compiler.RuntimeChecks

const (
	ModePlay               = compiler.ModePlay
	ModeWatch              = compiler.ModeWatch
	ModePreview            = compiler.ModePreview
	ModeTutorial           = compiler.ModeTutorial
	OptimizationMinimal    = compiler.OptimizationMinimal
	OptimizationFast       = compiler.OptimizationFast
	OptimizationStandard   = compiler.OptimizationStandard
	RuntimeChecksNone      = compiler.RuntimeChecksNone
	RuntimeChecksTerminate = compiler.RuntimeChecksTerminate
	RuntimeChecksNotify    = compiler.RuntimeChecksNotify
)

type Handler = simexec.Handler
type StreamEntry = simexec.StreamEntry
type SideEffect = simexec.SideEffect
type Result = simexec.Result
type ExecutionErrorKind = simexec.ExecutionErrorKind
type ExecutionError = simexec.ExecutionError

const (
	ExecutionErrorInvalidRequest  = simexec.ExecutionErrorInvalidRequest
	ExecutionErrorInvalidNode     = simexec.ExecutionErrorInvalidNode
	ExecutionErrorInvalidArity    = simexec.ExecutionErrorInvalidArity
	ExecutionErrorInvalidArgument = simexec.ExecutionErrorInvalidArgument
	ExecutionErrorInvalidState    = simexec.ExecutionErrorInvalidState
	ExecutionErrorMissingHandler  = simexec.ExecutionErrorMissingHandler
	ExecutionErrorStepLimit       = simexec.ExecutionErrorStepLimit
)

type Request struct {
	Mode       Mode
	Archetype  string
	Callback   string
	Memory     map[int][]float64
	Streams    map[int][]StreamEntry
	RandomSeed int64
	StepLimit  int
	Handler    Handler
}

type Engine struct{ artifacts *compiler.Artifacts }

func Compile(options Options, patterns ...string) (*Engine, error) {
	artifacts, err := compiler.NewCompiler(options, patterns...).CompileAll()
	if err != nil {
		return nil, err
	}
	return &Engine{artifacts: artifacts}, nil
}

func (e *Engine) Run(request Request) (Result, error) {
	if e == nil || e.artifacts == nil {
		return Result{}, invalidRequest("engine is nil")
	}
	nodes, root, err := callback(e.artifacts, request)
	if err != nil {
		return Result{}, err
	}
	return simexec.Execute(nodes, root, simexec.Request{
		Memory: request.Memory, Streams: request.Streams, ROM: e.artifacts.ROM,
		RandomSeed: request.RandomSeed, StepLimit: request.StepLimit, Handler: request.Handler,
	})
}

func invalidRequest(format string, arguments ...any) error {
	return &ExecutionError{Kind: ExecutionErrorInvalidRequest, NodeIndex: -1, ArgumentIndex: -1, Message: fmt.Sprintf(format, arguments...)}
}

func callback(artifacts *compiler.Artifacts, request Request) ([]resource.EngineDataNode, int, error) {
	missing := func() ([]resource.EngineDataNode, int, error) {
		return nil, 0, invalidRequest("callback %s/%s/%s not found", request.Mode, request.Archetype, request.Callback)
	}
	switch request.Mode {
	case ModePlay:
		if artifacts.Play == nil {
			return missing()
		}
		for _, archetype := range artifacts.Play.Archetypes {
			if string(archetype.Name) != request.Archetype {
				continue
			}
			var item *resource.EnginePlayDataArchetypeCallback
			switch request.Callback {
			case "preprocess":
				item = archetype.Preprocess
			case "spawnOrder":
				item = archetype.SpawnOrder
			case "shouldSpawn":
				item = archetype.ShouldSpawn
			case "initialize":
				item = archetype.Initialize
			case "updateSequential":
				item = archetype.UpdateSequential
			case "touch":
				item = archetype.Touch
			case "updateParallel":
				item = archetype.UpdateParallel
			case "terminate":
				item = archetype.Terminate
			}
			if item == nil {
				return missing()
			}
			return artifacts.Play.Nodes, item.Index, nil
		}
	case ModeWatch:
		if artifacts.Watch == nil {
			return missing()
		}
		if request.Archetype == "" && request.Callback == "updateSpawn" {
			return artifacts.Watch.Nodes, artifacts.Watch.UpdateSpawn, nil
		}
		for _, archetype := range artifacts.Watch.Archetypes {
			if string(archetype.Name) != request.Archetype {
				continue
			}
			var item *resource.EngineWatchDataArchetypeCallback
			switch request.Callback {
			case "preprocess":
				item = archetype.Preprocess
			case "spawnTime":
				item = archetype.SpawnTime
			case "despawnTime":
				item = archetype.DespawnTime
			case "initialize":
				item = archetype.Initialize
			case "updateSequential":
				item = archetype.UpdateSequential
			case "updateParallel":
				item = archetype.UpdateParallel
			case "terminate":
				item = archetype.Terminate
			}
			if item == nil {
				return missing()
			}
			return artifacts.Watch.Nodes, item.Index, nil
		}
	case ModePreview:
		if artifacts.Preview == nil {
			return missing()
		}
		for _, archetype := range artifacts.Preview.Archetypes {
			if string(archetype.Name) != request.Archetype {
				continue
			}
			var item *resource.EnginePreviewDataArchetypeCallback
			switch request.Callback {
			case "preprocess":
				item = archetype.Preprocess
			case "render":
				item = archetype.Render
			}
			if item == nil {
				return missing()
			}
			return artifacts.Preview.Nodes, item.Index, nil
		}
	case ModeTutorial:
		if artifacts.Tutorial == nil {
			return missing()
		}
		index := 0
		switch request.Callback {
		case "preprocess":
			index = artifacts.Tutorial.Preprocess
		case "navigate":
			index = artifacts.Tutorial.Navigate
		case "update":
			index = artifacts.Tutorial.Update
		default:
			return missing()
		}
		return artifacts.Tutorial.Nodes, index, nil
	default:
		return nil, 0, invalidRequest("invalid mode %q", request.Mode)
	}
	return missing()
}
