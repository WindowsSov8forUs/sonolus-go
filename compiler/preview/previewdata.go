package preview

import (
	"github.com/WindowsSov8forUs/sonolus-core-go/core/resource"

	"github.com/WindowsSov8forUs/sonolus-go/compiler/modecompile"
)

// ArchetypeDef is the static metadata for one preview-mode archetype.
type ArchetypeDef struct {
	Name    string
	Imports []resource.EngineDataArchetypeImport
}

// BuildPreviewData builds the static EnginePreviewData skeleton.
func BuildPreviewData(
	skin resource.EngineSkinData,
	archetypes []ArchetypeDef,
) *resource.EnginePreviewData {
	arcs := make([]resource.EnginePreviewDataArchetype, len(archetypes))
	for i, a := range archetypes {
		arcs[i] = resource.EnginePreviewDataArchetype{
			Name:    resource.EngineArchetypeName(a.Name),
			Imports: modecompile.NormalizeSlice(a.Imports),
		}
	}

	return &resource.EnginePreviewData{
		Skin:       skin,
		Archetypes: arcs,
		Nodes:      []resource.EngineDataNode{},
	}
}
