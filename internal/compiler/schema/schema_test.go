package schema

import (
	"reflect"
	"testing"

	"github.com/WindowsSov8forUs/sonolus-go/internal/compiler/mode"
)

func TestBuildMatchesPythonFieldUnion(t *testing.T) {
	contract := Build(
		[]ModeArchetype{{Name: "Note", Exports: []string{"result", "shared"}, Imports: []string{"beat", "shared"}}},
		[]ModeArchetype{{Name: "Note", Imports: []string{"beat", "watchOnly", "#ACCURACY", "#JUDGMENT"}}, {Name: "WatchOnly", Imports: []string{"time"}}},
		[]ModeArchetype{{Name: "Note", Imports: []string{"beat", "previewOnly"}}, {Name: "Alpha", Imports: []string{"x"}}},
	)
	want := Project{Archetypes: []Archetype{
		{Name: "Alpha", Fields: []string{"x"}},
		{Name: "Note", Fields: []string{"result", "shared", "beat", "watchOnly", "previewOnly"}},
		{Name: "WatchOnly", Fields: []string{"time"}},
	}}
	if !reflect.DeepEqual(contract.Project, want) {
		t.Fatalf("schema = %#v, want %#v", contract.Project, want)
	}
	if !contract.HasArchetype(mode.ModeWatch, "WatchOnly") || contract.HasArchetype(mode.ModePlay, "WatchOnly") {
		t.Fatal("mode archetype provenance was lost")
	}
	if contract.IsImportedByAll("Note", "watchOnly") || !contract.IsImportedByAll("Note", "beat") {
		t.Fatal("field import provenance was lost")
	}
	if !contract.IsImported(mode.ModeWatch, "Note", "#ACCURACY") {
		t.Fatal("hidden Watch field import provenance was lost")
	}
}
