// Package optimize provides source-independent optimization passes for the
// strongly typed compiler IR.
package optimize

import (
	"fmt"

	"github.com/WindowsSov8forUs/sonolus-go/v2/internal/compiler/ir"
	"github.com/WindowsSov8forUs/sonolus-go/v2/internal/compiler/mode"
)

type Level int

const (
	LevelMinimal Level = iota + 1
	LevelFast
	LevelStandard
)

func (level Level) String() string {
	switch level {
	case LevelMinimal:
		return "minimal"
	case LevelFast:
		return "fast"
	case LevelStandard:
		return "standard"
	default:
		return fmt.Sprintf("level-%d", level)
	}
}

type Context struct {
	Mode     mode.Mode
	Callback string
	analyses *analysisManager
}

type analysisManager struct {
	values map[Analysis]any
}

func newAnalysisManager() *analysisManager { return &analysisManager{values: map[Analysis]any{}} }

func (m *analysisManager) ensure(analysis Analysis, function *ir.Function) error {
	if _, ok := m.values[analysis]; ok {
		return nil
	}
	switch analysis {
	case AnalysisDominance:
		m.values[analysis] = ComputeDominance(function)
	case AnalysisLiveness:
		m.values[analysis] = localInterference(function)
	case AnalysisSSA:
		m.values[analysis] = true
	default:
		return fmt.Errorf("unknown analysis %q", analysis)
	}
	return nil
}

func (m *analysisManager) invalidateExcept(preserved []Analysis) {
	keep := make(map[Analysis]bool, len(preserved))
	for _, analysis := range preserved {
		keep[analysis] = true
	}
	for analysis := range m.values {
		if !keep[analysis] {
			delete(m.values, analysis)
		}
	}
}

func dominanceFor(context Context, function *ir.Function) *Dominance {
	if context.analyses != nil {
		if value, ok := context.analyses.values[AnalysisDominance].(*Dominance); ok {
			return value
		}
	}
	return ComputeDominance(function)
}

func interferenceFor(context Context, function *ir.Function) interferenceGraph {
	if context.analyses != nil {
		if value, ok := context.analyses.values[AnalysisLiveness].(interferenceGraph); ok {
			return value
		}
	}
	return localInterference(function)
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

type conservativeManagedPass struct{ Pass }

func (conservativeManagedPass) Requires() []Analysis  { return nil }
func (conservativeManagedPass) Preserves() []Analysis { return nil }
func (conservativeManagedPass) Destroys() []Analysis {
	return []Analysis{AnalysisDominance, AnalysisSSA, AnalysisLiveness}
}

type Optimizer struct {
	level  Level
	passes []Pass
}

func NewOptimizer(level Level) *Optimizer {
	if level == 0 {
		level = LevelStandard
	}
	optimizer := &Optimizer{level: level}
	switch level {
	case LevelMinimal:
		optimizer.passes = []Pass{
			FoldConstantControl{},
			RemoveUnreachable{},
			CoalesceFlow{},
			RemoveNoOps{},
			RemoveUnreachable{},
			AllocateBasic{},
			RenumberBlocks{},
		}
	case LevelFast:
		optimizer.passes = []Pass{CoalesceFlow{}, RemoveUnreachable{}, TryAllocateBasic{}, CoalesceFlow{}, RenumberBlocks{}}
	case LevelStandard:
		optimizer.passes = []Pass{
			CoalesceFlow{}, RemoveUnreachable{}, DeadCodeElimination{}, CoalesceSmallConditionalBlocks{},
			ToSSA{}, SparseConditionalConstantPropagation{}, RemoveUnreachable{}, DeadCodeElimination{}, CoalesceFlow{},
			NormalizeBlocks{}, InlineVars{}, DeadCodeElimination{}, InlineVars{}, CoalesceFlow{},
			SparseConditionalConstantPropagation{}, FlattenAssociativeOps{}, RemoveRedundantArguments{}, DeadCodeElimination{}, CoalesceFlow{},
			RewriteToSwitch{}, InlineVars{Aggressive: true}, UnflattenAssociativeOps{}, LoopInvariantCodeMotion{}, CommonSubexpressionElimination{}, NormalizeBlocks{},
			FlattenAssociativeOps{}, InlineVars{}, DeadCodeElimination{}, FlattenAssociativeOps{}, RemoveRedundantArguments{},
			FromSSA{}, CoalesceFlow{}, CopyCoalesce{}, AdvancedDeadCodeElimination{}, CoalesceFlow{}, NormalizeSwitch{}, CombineExitBlocks{},
			Allocate{}, RenumberBlocks{},
		}
	}
	for index, pass := range optimizer.passes {
		if _, managed := pass.(ManagedPass); !managed {
			optimizer.passes[index] = conservativeManagedPass{Pass: pass}
		}
	}
	return optimizer
}

func (o *Optimizer) Validate() error {
	if o == nil {
		return fmt.Errorf("optimizer is nil")
	}
	if o.level < LevelMinimal || o.level > LevelStandard {
		return fmt.Errorf("unknown optimization level %d", o.level)
	}
	return nil
}

func (o *Optimizer) Optimize(context Context, function *ir.Function) (*ir.Function, error) {
	if err := o.Validate(); err != nil {
		return nil, fmt.Errorf("optimize %s: %w", contextLabel(context, function), err)
	}
	validator := new(ir.Validator)
	if err := validator.Validate(function); err != nil {
		return nil, fmt.Errorf("optimize %s input: %w", contextLabel(context, function), err)
	}
	result := CloneFunction(function)
	context.analyses = newAnalysisManager()
	for passIndex, pass := range o.passes {
		if managed, ok := pass.(ManagedPass); ok {
			for _, required := range managed.Requires() {
				if err := context.analyses.ensure(required, result); err != nil {
					return nil, fmt.Errorf("optimize %s pass %s: %w", contextLabel(context, function), pass.Name(), err)
				}
			}
		}
		if err := pass.Run(context, result); err != nil {
			return nil, fmt.Errorf("optimize %s pass %s: %w", contextLabel(context, function), pass.Name(), err)
		}
		var validationErr error
		if passIndex == len(o.passes)-1 {
			validationErr = validator.ValidateFinal(result)
		} else {
			validationErr = validator.Validate(result)
		}
		if validationErr != nil {
			return nil, fmt.Errorf("optimize %s pass %s produced invalid IR: %w", contextLabel(context, function), pass.Name(), validationErr)
		}
		if managed, ok := pass.(ManagedPass); ok {
			context.analyses.invalidateExcept(managed.Preserves())
			for _, destroyed := range managed.Destroys() {
				delete(context.analyses.values, destroyed)
			}
		} else {
			context.analyses.invalidateExcept(nil)
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
