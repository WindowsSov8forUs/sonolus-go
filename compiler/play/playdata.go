// Package play compiles and assembles Play-mode engine data. Play mode
// supports the full set of callbacks (Preprocess, Initialize, Touch, etc.),
// archetype exports, and score/life bindings.
package play

import (
	"github.com/WindowsSov8forUs/sonolus-core-go/core/resource"

	"github.com/WindowsSov8forUs/sonolus-go/compiler/modecompile"
)

// ArchetypeDef is the static metadata for one play-mode archetype, used to seed
// the EnginePlayData skeleton before callbacks are folded in.
type ArchetypeDef struct {
	Name     string
	HasInput bool
	Imports  []resource.EngineDataArchetypeImport
	Exports  []resource.EngineArchetypeDataName
}

// BuildPlayData builds the static EnginePlayData skeleton: skin/effect/particle/
// buckets metadata, archetype metadata with no callbacks yet, and an empty nodes
// array. Mirrors buildEnginePlayData in sonolus.js-compiler.
//
// Imports/exports are normalized to non-nil slices so they serialize as `[]`
// (matching the reference) rather than `null`.
func BuildPlayData(
	skin resource.EngineSkinData,
	effect resource.EngineEffectData,
	particle resource.EngineParticleData,
	buckets []resource.EngineDataBucket,
	archetypes []ArchetypeDef,
) *resource.EnginePlayData {
	arcs := make([]resource.EnginePlayDataArchetype, len(archetypes))
	for i, a := range archetypes {
		arcs[i] = resource.EnginePlayDataArchetype{
			Name:     resource.EngineArchetypeName(a.Name),
			HasInput: a.HasInput,
			Imports:  modecompile.NormalizeSlice(a.Imports),
			Exports:  modecompile.NormalizeSlice(a.Exports),
		}
	}

	return &resource.EnginePlayData{
		Skin:       skin,
		Effect:     effect,
		Particle:   particle,
		Buckets:    modecompile.NormalizeSlice(buckets),
		Archetypes: arcs,
		Nodes:      []resource.EngineDataNode{},
	}
}
