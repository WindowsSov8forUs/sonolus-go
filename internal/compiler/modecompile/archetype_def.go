package modecompile

import "github.com/WindowsSov8forUs/sonolus-core-go/core/resource"

// ArchetypeDef is the static metadata for one mode archetype, shared across
// play, watch, and preview modes. Fields that are mode-specific (HasInput,
// Exports — used only by Play mode) are zero-valued in other modes.
type ArchetypeDef struct {
	Name     string
	HasInput bool // Play mode only
	Imports  []resource.EngineDataArchetypeImport
	Exports  []resource.EngineArchetypeDataName // Play mode only
}
