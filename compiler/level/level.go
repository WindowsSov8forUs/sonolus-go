// Package level compiles and packages Sonolus levels (charts). Unlike engines,
// levels are pure data — there is no code generation, only validation and
// packaging of externally-authored chart definitions.
package level

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/WindowsSov8forUs/sonolus-core-go/codec"
	"github.com/WindowsSov8forUs/sonolus-core-go/core/resource"
)

// InputLevel is the JSON-parsed intermediate form of a level before packaging.
type InputLevel struct {
	BGMOffset float64       `json:"bgmOffset"`
	Entities  []LevelEntity `json:"entities"`
}

// LevelEntity is a single chart entity.
type LevelEntity struct {
	Name      string            `json:"name,omitempty"`
	Archetype string            `json:"archetype"`
	Data      []json.RawMessage `json:"data"`
}

// CompileLevel parses a JSON level definition and builds LevelData.
func CompileLevel(src string) (*resource.LevelData, error) {
	var in InputLevel
	if err := json.Unmarshal([]byte(src), &in); err != nil {
		return nil, fmt.Errorf("parse level JSON: %w", err)
	}
	var entities []resource.LevelDataEntity
	for _, e := range in.Entities {
		var data []resource.LevelDataEntityData
		for _, raw := range e.Data {
			d, err := resource.DecodeLevelDataEntityData(raw)
			if err != nil {
				return nil, fmt.Errorf("entity %q: %w", e.Archetype, err)
			}
			data = append(data, d)
		}
		entities = append(entities, resource.LevelDataEntity{
			Name:      e.Name,
			Archetype: resource.EngineArchetypeName(e.Archetype),
			Data:      data,
		})
	}
	return &resource.LevelData{
		BGMOffset: in.BGMOffset,
		Entities:  entities,
	}, nil
}

// PackageLevel reads a JSON level file, compiles it, and returns the gzipped
// LevelData bytes ready for writing to disk.
func PackageLevel(path string) ([]byte, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading level: %w", err)
	}
	level, err := CompileLevel(string(data))
	if err != nil {
		return nil, err
	}
	return codec.Compress(level)
}

// FileName is the canonical on-disk filename for a level.
const FileName = "LevelData"
