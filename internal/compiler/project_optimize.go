package compiler

import (
	"fmt"
	"strings"
	"sync"

	"github.com/WindowsSov8forUs/sonolus-go/internal/compiler/frontend"
	"github.com/WindowsSov8forUs/sonolus-go/internal/compiler/mode"
	"github.com/WindowsSov8forUs/sonolus-go/internal/compiler/optimize"
)

type callbackOptimization struct {
	mode     mode.Mode
	callback *frontend.CallbackDeclaration
	result   *frontend.CallbackDeclaration
	err      error
}

func optimizeProject(optimizer *optimize.Optimizer, project *frontend.Project) (*frontend.Project, error) {
	if err := optimizer.Validate(); err != nil {
		return nil, fmt.Errorf("compiler: optimize project: %w", err)
	}
	if project == nil {
		return nil, fmt.Errorf("compiler: optimize project: project is nil")
	}
	result := &frontend.Project{
		Configuration: project.Configuration,
		ROM:           append([]byte(nil), project.ROM...),
		Modes:         make(map[mode.Mode]*frontend.ModeDeclarations, len(project.Modes)),
	}
	var jobs []*callbackOptimization
	for _, m := range orderedModes {
		declarations := project.Modes[m]
		if declarations == nil {
			continue
		}
		cloned := *declarations
		cloned.Archetypes = make([]*frontend.ArchetypeDeclaration, len(declarations.Archetypes))
		for i, archetype := range declarations.Archetypes {
			if archetype == nil {
				continue
			}
			copyArchetype := *archetype
			copyArchetype.Callbacks = make([]*frontend.CallbackDeclaration, len(archetype.Callbacks))
			for j, callback := range archetype.Callbacks {
				job := &callbackOptimization{mode: m, callback: callback}
				jobs = append(jobs, job)
				copyArchetype.Callbacks[j] = nil
			}
			cloned.Archetypes[i] = &copyArchetype
		}
		cloned.Globals = make([]*frontend.CallbackDeclaration, len(declarations.Globals))
		for _, callback := range declarations.Globals {
			jobs = append(jobs, &callbackOptimization{mode: m, callback: callback})
		}
		result.Modes[m] = &cloned
	}

	var wg sync.WaitGroup
	for _, job := range jobs {
		wg.Add(1)
		go func(job *callbackOptimization) {
			defer wg.Done()
			if job.callback == nil {
				job.err = fmt.Errorf("callback declaration is nil")
				return
			}
			function, err := optimizer.Optimize(optimize.Context{Mode: job.mode, Callback: job.callback.Name}, job.callback.IR)
			if err != nil {
				job.err = err
				return
			}
			cloned := *job.callback
			cloned.IR = function
			job.result = &cloned
		}(job)
	}
	wg.Wait()

	var errors []string
	jobIndex := 0
	for _, m := range orderedModes {
		declarations := result.Modes[m]
		if declarations == nil {
			continue
		}
		for _, archetype := range declarations.Archetypes {
			if archetype == nil {
				continue
			}
			for i := range archetype.Callbacks {
				job := jobs[jobIndex]
				jobIndex++
				if job.err != nil {
					errors = append(errors, job.err.Error())
				} else {
					archetype.Callbacks[i] = job.result
				}
			}
		}
		for i := range declarations.Globals {
			job := jobs[jobIndex]
			jobIndex++
			if job.err != nil {
				errors = append(errors, job.err.Error())
			} else {
				declarations.Globals[i] = job.result
			}
		}
	}
	if len(errors) != 0 {
		return nil, fmt.Errorf("compiler: optimize callbacks:\n%s", strings.Join(errors, "\n"))
	}
	return result, nil
}
