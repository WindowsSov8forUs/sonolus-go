package watch

import (
	"github.com/WindowsSov8forUs/sonolus-core-go/core/resource"

	"github.com/WindowsSov8forUs/sonolus-go/compiler/modecompile"
)

// ArchetypeDef is the static metadata for one watch-mode archetype.
type ArchetypeDef = modecompile.ArchetypeDef

// BuildWatchData builds the static EngineWatchData skeleton: skin/effect/particle/
// buckets metadata, archetype metadata with no callbacks yet, and an empty nodes
// array.
func BuildWatchData(
	skin resource.EngineSkinData,
	effect resource.EngineEffectData,
	particle resource.EngineParticleData,
	buckets []resource.EngineDataBucket,
	archetypes []ArchetypeDef,
) *resource.EngineWatchData {
	arcs := make([]resource.EngineWatchDataArchetype, len(archetypes))
	for i, a := range archetypes {
		arcs[i] = resource.EngineWatchDataArchetype{
			Name:    resource.EngineArchetypeName(a.Name),
			Imports: modecompile.NormalizeSlice(a.Imports),
		}
	}

	return &resource.EngineWatchData{
		Skin:       skin,
		Effect:     effect,
		Particle:   particle,
		Buckets:    modecompile.NormalizeSlice(buckets),
		Archetypes: arcs,
		Nodes:      []resource.EngineDataNode{},
	}
}
