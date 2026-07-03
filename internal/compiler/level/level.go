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
		return nil, fmt.Errorf("level: parse JSON: %w", err)
	}
	var entities []resource.LevelDataEntity
	for _, e := range in.Entities {
		var data []resource.LevelDataEntityData
		for _, raw := range e.Data {
			d, err := resource.DecodeLevelDataEntityData(raw)
			if err != nil {
				return nil, fmt.Errorf("level: entity %q: %w", e.Archetype, err)
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
		return nil, fmt.Errorf("level: read: %w", err)
	}
	level, err := CompileLevel(string(data))
	if err != nil {
		return nil, err
	}
	return codec.Compress(level)
}

// FileName is the canonical on-disk filename for a level.
const FileName = "LevelData"

// ── LevelBuilder ──────────────────────────────────────────────────────────────

// LevelBuilder provides a chainable API for constructing LevelData
// programmatically, matching the builder pattern used by sonolus.py's
// build_level_data() and sonolus.js-compiler's level data assembly.
type LevelBuilder struct {
	bgmOffset float64
	entities  []resource.LevelDataEntity
}

// NewLevelBuilder creates an empty LevelBuilder with BGMOffset defaulting to 0.
func NewLevelBuilder() *LevelBuilder { return &LevelBuilder{} }

// SetBGMOffset sets the BGM offset in seconds.
func (b *LevelBuilder) SetBGMOffset(offset float64) *LevelBuilder {
	b.bgmOffset = offset
	return b
}

// AddEntity appends an entity with the given archetype name, display name, and
// pre-built data entries. The name is used as the entity reference in level data;
// archetype must match an engine archetype name.
func (b *LevelBuilder) AddEntity(name string, archetype string, data []resource.LevelDataEntityData) *LevelBuilder {
	b.entities = append(b.entities, resource.LevelDataEntity{
		Name:      name,
		Archetype: resource.EngineArchetypeName(archetype),
		Data:      data,
	})
	return b
}

// Build returns the assembled LevelData. Returns nil only when there are no
// entities, which is valid (an empty level).
func (b *LevelBuilder) Build() *resource.LevelData {
	if b.entities == nil {
		b.entities = []resource.LevelDataEntity{}
	}
	return &resource.LevelData{
		BGMOffset: b.bgmOffset,
		Entities:  b.entities,
	}
}
