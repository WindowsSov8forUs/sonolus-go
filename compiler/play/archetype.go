package play

import "github.com/WindowsSov8forUs/sonolus-core-go/core/resource"

type Archetype struct {
	Name     string
	Index    int
	HasInput bool

	entityImports []resource.EngineDataArchetypeImport
	ExportKeys    []string
	Exports       []resource.EngineArchetypeDataName

	PreprocessOrder       int
	SpawnOrderOrder       int
	ShouldSpawnOrder      int
	UpdateSequentialOrder int
	TouchOrder            int
}

func NewArchetype(name string, index int) *Archetype {
	return &Archetype{
		Name:  name,
		Index: index,
	}
}

func (a *Archetype) DefineImport(name resource.EngineArchetypeDataName, def ...float64) int {
	index := len(a.entityImports)

	entry := resource.EngineDataArchetypeImport{
		Name:  name,
		Index: index,
	}
	if len(def) > 0 {
		entry.Def = def[0]
	}

	a.entityImports = append(a.entityImports, entry)
	return index
}
