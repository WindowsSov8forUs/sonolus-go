package compiler

import (
	"github.com/WindowsSov8forUs/sonolus-go/internal/compiler/mode"
	"github.com/WindowsSov8forUs/sonolus-go/internal/compiler/optimize"
)

// Mode identifies a Sonolus engine compilation mode.
type Mode = mode.Mode

const (
	ModePlay     = mode.ModePlay
	ModeWatch    = mode.ModeWatch
	ModePreview  = mode.ModePreview
	ModeTutorial = mode.ModeTutorial
)

// OptimizationLevel selects the compiler optimization pipeline.
type OptimizationLevel = optimize.Level

const (
	OptimizationMinimal  = optimize.LevelMinimal
	OptimizationFast     = optimize.LevelFast
	OptimizationStandard = optimize.LevelStandard
)
