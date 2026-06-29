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
package engine

import "github.com/WindowsSov8forUs/sonolus-go/compiler/ir"

// ImportedField describes one imported field of an archetype. Name is the Go
// field name; Def is an optional default value (0.0 when absent).
type ImportedField struct {
	Name string
	Def  float64
}

// Entity memory block IDs — aliases of canonical ir.BlockEntity* constants.
const (
	entityMemoryBlock  = ir.BlockEntityMemory
	entityDataBlock    = ir.BlockEntityData
	entitySharedBlock  = ir.BlockEntityShared
	entityInfoBlock    = ir.BlockEntityInfo
	entityDespawnBlock = ir.BlockEntityDespawn
	entityInputBlock   = ir.BlockEntityInput
)
