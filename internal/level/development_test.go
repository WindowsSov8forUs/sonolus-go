package level

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/WindowsSov8forUs/sonolus-core-go/core/resource"
	"github.com/WindowsSov8forUs/sonolus-go/v2/internal/compiler"
)

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
