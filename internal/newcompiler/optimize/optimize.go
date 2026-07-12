// Package optimize provides source-independent optimization passes for the
// strongly typed newcompiler IR.
package optimize

import (
	"fmt"

	"github.com/WindowsSov8forUs/sonolus-go/internal/newcompiler/ir"
	"github.com/WindowsSov8forUs/sonolus-go/internal/newcompiler/mode"
)

type Level int

const (
	LevelMinimal Level = iota + 1
	LevelFast
	LevelStandard
)

type Context struct {
	Mode     mode.Mode
	Callback string
}

type Analysis string

const (
	AnalysisDominance Analysis = "dominance"
	AnalysisSSA       Analysis = "ssa"
	AnalysisLiveness  Analysis = "liveness"
)

type Pass interface {
	Name() string
	Run(Context, *ir.Function) error
}

type ManagedPass interface {
	Pass
	Requires() []Analysis
	Preserves() []Analysis
	Destroys() []Analysis
}

type Optimizer struct {
	level  Level
	passes []Pass
}

func NewOptimizer(level Level) *Optimizer {
	if level == 0 {
		level = LevelMinimal
	}
	optimizer := &Optimizer{level: level}
	if level == LevelMinimal {
		optimizer.passes = []Pass{
			FoldConstantControl{},
			RemoveUnreachable{},
			CoalesceFlow{},
			RemoveNoOps{},
			RemoveUnreachable{},
			RenumberBlocks{},
		}
	}
	return optimizer
}

func (o *Optimizer) Validate() error {
	if o == nil {
		return fmt.Errorf("optimizer is nil")
	}
	if o.level != LevelMinimal {
		return fmt.Errorf("optimization level %d is not implemented; only minimal is currently available", o.level)
	}
	return nil
}

func (o *Optimizer) Optimize(context Context, function *ir.Function) (*ir.Function, error) {
	if err := o.Validate(); err != nil {
		return nil, fmt.Errorf("optimize %s: %w", contextLabel(context, function), err)
	}
	if err := ir.Validate(function); err != nil {
		return nil, fmt.Errorf("optimize %s input: %w", contextLabel(context, function), err)
	}
	result := CloneFunction(function)
	available := map[Analysis]bool{}
	for _, pass := range o.passes {
		if managed, ok := pass.(ManagedPass); ok {
			for _, required := range managed.Requires() {
				if !available[required] {
					return nil, fmt.Errorf("optimize %s pass %s: required analysis %q is unavailable", contextLabel(context, function), pass.Name(), required)
				}
			}
		}
		if err := pass.Run(context, result); err != nil {
			return nil, fmt.Errorf("optimize %s pass %s: %w", contextLabel(context, function), pass.Name(), err)
		}
		if err := ir.Validate(result); err != nil {
			return nil, fmt.Errorf("optimize %s pass %s produced invalid IR: %w", contextLabel(context, function), pass.Name(), err)
		}
		if managed, ok := pass.(ManagedPass); ok {
			for _, destroyed := range managed.Destroys() {
				delete(available, destroyed)
			}
		}
	}
	return result, nil
}

func contextLabel(context Context, function *ir.Function) string {
	name := "<nil>"
	if function != nil && function.Name != "" {
		name = function.Name
	}
	return fmt.Sprintf("%s/%s (%s)", context.Mode, context.Callback, name)
}
