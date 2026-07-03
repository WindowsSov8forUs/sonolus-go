// Package engine is the top-level integration layer: it compiles a Go-authored
// engine description end-to-end into Sonolus EngineData. It wires together the
// front end (go/ast tracer), the optimizer, finalization, and mode-specific
// assembly.
//
// The primary entry points are CompilePlayFile, CompileWatchFile,
// CompilePreviewFile, and CompileTutorialFile. Each parses a Go source file
// containing archetype structs (tagged with sonolus:"imported"/"memory"/etc.)
// and callback methods, then compiles them to the corresponding Engine*Data
// output.
//
// For compilation timing, use the *WithStats variants (e.g.
// CompilePlayFileWithStats) which populate a CompileStats with per-callback
// durations.
package engine

import (
	"context"

	"github.com/WindowsSov8forUs/sonolus-go/internal/compiler/ir"
	"github.com/WindowsSov8forUs/sonolus-go/internal/compiler/ir/optimize"
)

// CompileOptions holds optional compilation parameters. nil is equivalent to
// default options (no stats collection, standard optimization level, no
// cancellation).
type CompileOptions struct {
	Context context.Context // if non-nil, checked between optimization passes for cancellation
	Stats   *CompileStats   // if non-nil, per-callback compilation timing is recorded
	Opt     optimize.Level  // optimization level (0 = default → LevelStandard)
}

// optsCtx extracts the context from opts, returning nil if opts is nil (safe
// for callers that pass nil CompileOptions).
func optsCtx(opts *CompileOptions) context.Context {
	if opts == nil {
		return nil
	}
	return opts.Context
}

// optsLevel extracts the optimization level from opts. Returns LevelStandard
// when opts is nil or Opt is unset (zero-valued, the default).
// Level constant values start at iota+1, so zero unambiguously means "not set".
func optsLevel(opts *CompileOptions) optimize.Level {
	if opts == nil || opts.Opt <= 0 {
		return optimize.LevelStandard
	}
	return opts.Opt
}

// ImportedField describes one imported field of an archetype. Name is the Go
// field name; Def is an optional default value (0.0 when absent).
type ImportedField struct {
	Name string
	Def  float64
}

// ExportedBlock is the sentinel Block value for archetype-exported fields.
// Fields with this block id are emitted as ExportValue nodes rather than
// SetPlace (see emitBindingStore in compiler/frontend/trace_stmt.go).
const ExportedBlock = -1

// Entity memory block IDs — aliases of canonical ir.BlockEntity* constants.
const (
	entityMemoryBlock  = ir.BlockEntityMemory
	entityDataBlock    = ir.BlockEntityData
	entitySharedBlock  = ir.BlockEntityShared
	entityInfoBlock    = ir.BlockEntityInfo
	entityDespawnBlock = ir.BlockEntityDespawn
	entityInputBlock   = ir.BlockEntityInput
)
