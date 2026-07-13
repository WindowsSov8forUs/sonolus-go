package level

import (
	"testing"

	"github.com/WindowsSov8forUs/sonolus-core-go/codec"
	"github.com/WindowsSov8forUs/sonolus-core-go/core/resource"
	"path/filepath"
	"strings"

	"github.com/WindowsSov8forUs/sonolus-go/v2/internal/compiler"
)

const testLevel = `{
	"bgmOffset": 0.5,
	"entities": [
		{
			"archetype": "TapNote",
			"data": [
				{"name": "beat", "value": 0},
				{"name": "lane", "value": 2}
			]
		}
	]
}`

func TestCompileLevel(t *testing.T) {
	data, err := CompileLevel(testLevel)
	if err != nil {
		t.Fatal(err)
	}
	if data.BGMOffset != 0.5 {
		t.Errorf("bgmOffset=%f", data.BGMOffset)
	}
	if len(data.Entities) != 1 {
		t.Fatalf("entities=%d", len(data.Entities))
	}
	e := data.Entities[0]
	if e.Archetype != "TapNote" {
		t.Errorf("archetype=%q", e.Archetype)
	}
	if len(e.Data) != 2 {
		t.Fatalf("data len=%d", len(e.Data))
	}
	vd, ok := e.Data[0].(resource.LevelDataEntityValueData)
	if !ok || vd.Name != "beat" || vd.Value != 0 {
		t.Errorf("data[0]=%+v", e.Data[0])
	}
}

func TestLevelRoundTrip(t *testing.T) {
	data, err := CompileLevel(testLevel)
	if err != nil {
		t.Fatal(err)
	}
	blob, err := codec.Compress(data)
	if err != nil {
		t.Fatal(err)
	}
	rt, err := codec.Decompress[resource.LevelData](blob)
	if err != nil {
		t.Fatal(err)
	}
	if len(rt.Entities) != 1 {
		t.Fatalf("round trip lost entities: %d", len(rt.Entities))
	}
}

func TestLevel_MalformedJSON(t *testing.T) {
	_, err := CompileLevel("{not json")
	if err == nil {
		t.Error("expected error for malformed JSON")
	}
}

func TestLevel_MissingFields(t *testing.T) {
	if _, err := CompileLevel(`{"bgmOffset": 0}`); err == nil {
		t.Fatal("expected missing entities error")
	}
}

func TestLevel_WrongTypes(t *testing.T) {

	_, err := CompileLevel(`{"bgmOffset": "bad", "entities": []}`)
	if err == nil {
		t.Error("expected error for wrong type")
	}
}

func TestLevel_MultiLevel(t *testing.T) {
	level := `{
		"bgmOffset": 1.0,
		"entities": [
			{"archetype": "Note", "data": [{"name": "beat", "value": 0}]},
			{"archetype": "Note", "data": [{"name": "beat", "value": 1}]}
		]
	}`
	data, err := CompileLevel(level)
	if err != nil {
		t.Fatal(err)
	}
	if len(data.Entities) != 2 {
		t.Errorf("expected 2 entities, got %d", len(data.Entities))
	}
}

func TestLevelBuilder(t *testing.T) {
	b := NewLevelBuilder().
		SetBGMOffset(2.0).
		AddEntity("n0", "TapNote", []resource.LevelDataEntityData{
			resource.LevelDataEntityValueData{Name: "beat", Value: 0},
			resource.LevelDataEntityValueData{Name: "lane", Value: 3},
		}).
		AddEntity("n1", "TapNote", []resource.LevelDataEntityData{
			resource.LevelDataEntityValueData{Name: "beat", Value: 1},
		})

	data := b.Build()
	if data.BGMOffset != 2.0 {
		t.Errorf("BGMOffset = %f, want 2.0", data.BGMOffset)
	}
	if len(data.Entities) != 2 {
		t.Fatalf("entities = %d, want 2", len(data.Entities))
	}
	if data.Entities[0].Name != "n0" || data.Entities[0].Archetype != "TapNote" {
		t.Errorf("entity[0] = %+v", data.Entities[0])
	}
	if data.Entities[1].Name != "n1" {
		t.Errorf("entity[1].Name = %q", data.Entities[1].Name)
	}
}

func TestLevelBuilder_Empty(t *testing.T) {
	data := NewLevelBuilder().Build()
	if data.BGMOffset != 0 {
		t.Errorf("default BGMOffset = %f", data.BGMOffset)
	}
	if data.Entities == nil {
		t.Error("Entities should be non-nil (empty slice)")
	}
	if len(data.Entities) != 0 {
		t.Errorf("Entities = %d", len(data.Entities))
	}
}

// FuzzLevelToBytes is a byte-level fuzz test that CompileLevel must not panic.
func FuzzLevelToBytes(f *testing.F) {
	f.Add([]byte(testLevel))
	f.Fuzz(func(t *testing.T, b []byte) {
		CompileLevel(string(b))
	})
}
func TestLoadDevelopment(t *testing.T) {
	development, err := LoadDevelopment("./testdata/development")
	if err != nil {
		t.Fatal(err)
	}
	if development.File == "" || filepath.Base(development.File) != "level.json" {
		t.Fatalf("file = %q", development.File)
	}
	if development.Data.Entities == nil || len(development.Files) == 0 {
		t.Fatalf("development = %#v", development)
	}
}

func TestValidateDevelopment(t *testing.T) {
	imports := []resource.EngineDataArchetypeImport{{Name: "beat", Index: 0}}
	artifacts := &compiler.Artifacts{
		Play:    &resource.EnginePlayData{Archetypes: []resource.EnginePlayDataArchetype{{Name: "Note", Imports: imports}}},
		Watch:   &resource.EngineWatchData{Archetypes: []resource.EngineWatchDataArchetype{{Name: "Note", Imports: imports}}},
		Preview: &resource.EnginePreviewData{Archetypes: []resource.EnginePreviewDataArchetype{{Name: "Note", Imports: imports}}},
	}
	valid := &resource.LevelData{Entities: []resource.LevelDataEntity{
		{Name: "a", Archetype: "Note", Data: []resource.LevelDataEntityData{resource.LevelDataEntityValueData{Name: "beat", Value: 1}}},
		{Name: "b", Archetype: "Note", Data: []resource.LevelDataEntityData{resource.LevelDataEntityRefData{Name: "beat", Ref: "a"}}},
	}}
	if err := Validate(valid, artifacts); err != nil {
		t.Fatal(err)
	}
	invalid := &resource.LevelData{Entities: []resource.LevelDataEntity{{Archetype: "Note", Data: []resource.LevelDataEntityData{resource.LevelDataEntityRefData{Name: "beat", Ref: "missing"}}}}}
	if err := Validate(invalid, artifacts); err == nil || !strings.Contains(err.Error(), "unknown entity") {
		t.Fatalf("error = %v", err)
	}
}

func TestValidateDevelopmentDoesNotTreatSchemaUnionAsSharedImports(t *testing.T) {
	artifacts := &compiler.Artifacts{
		Play: &resource.EnginePlayData{Archetypes: []resource.EnginePlayDataArchetype{{
			Name: "Note", Exports: []resource.EngineArchetypeDataName{"result"},
		}}},
		Watch:   &resource.EngineWatchData{Archetypes: []resource.EngineWatchDataArchetype{{Name: "Note", Imports: []resource.EngineDataArchetypeImport{{Name: "watchOnly"}}}}},
		Preview: &resource.EnginePreviewData{Archetypes: []resource.EnginePreviewDataArchetype{{Name: "Note", Imports: []resource.EngineDataArchetypeImport{{Name: "previewOnly"}}}}},
	}
	for _, field := range []string{"result", "watchOnly", "previewOnly"} {
		data := &resource.LevelData{Entities: []resource.LevelDataEntity{{Archetype: "Note", Data: []resource.LevelDataEntityData{
			resource.LevelDataEntityValueData{Name: resource.EngineArchetypeDataName(field), Value: 1},
		}}}}
		if err := Validate(data, artifacts); err == nil || !strings.Contains(err.Error(), "is not imported") {
			t.Errorf("field %q error = %v", field, err)
		}
	}
}

func TestCompileLevelStrictSchema(t *testing.T) {
	for _, source := range []string{
		`{"bgmOffset":0}`,
		`{"bgmOffset":0,"entities":null}`,
		`{"bgmOffset":0,"entities":[],"unknown":true}`,
		`{"bgmOffset":0,"entities":[]} {}`,
		`{"bgmOffset":0,"entities":[{"archetype":"Note"}]}`,
		`{"bgmOffset":0,"entities":[{"archetype":"Note","data":[{"value":1}]}]}`,
	} {
		if _, err := CompileLevel(source); err == nil {
			t.Errorf("CompileLevel(%q) succeeded", source)
		}
	}
}
