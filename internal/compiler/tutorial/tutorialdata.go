package tutorial

import "github.com/WindowsSov8forUs/sonolus-core-go/core/resource"

// BuildTutorialData creates the skeleton EngineTutorialData with resource info
// and an empty node list. Callers then append compiled callback nodes and wire
// their indices.
func BuildTutorialData(
	skin resource.EngineSkinData,
	effect resource.EngineEffectData,
	particle resource.EngineParticleData,
	instruction resource.EngineInstructionData,
) *resource.EngineTutorialData {
	return &resource.EngineTutorialData{
		Skin:        skin,
		Effect:      effect,
		Particle:    particle,
		Instruction: instruction,
		Nodes:       []resource.EngineDataNode{},
	}
}
