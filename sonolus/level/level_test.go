package level

import (
	"math"
	"reflect"
	"strings"
	"testing"

	"github.com/WindowsSov8forUs/sonolus-core-go/core/resource"
	"github.com/WindowsSov8forUs/sonolus-go/v2/sonolus"
)

type testSingle struct{ Value float64 }

type testNoteData struct {
	Beat     float64           `level:"#BEAT"`
	Position sonolus.Vec2      `level:"position"`
	Samples  [2]float64        `level:"samples"`
	Single   testSingle        `level:"single"`
	Next     Ref[testNoteData] `level:"next,omitempty"`
	Optional float64           `level:"optional,omitempty"`
}

func TestBuilderConstructsTypedLevelData(t *testing.T) {
	note := MustDefine[testNoteData]("Note")
	first := note.New(testNoteData{Beat: 1, Position: sonolus.Vec2{X: 2, Y: 3}, Samples: [2]float64{4, 5}, Single: testSingle{Value: 6}})
	second := note.New(testNoteData{Beat: 2}).Named("tail")
	first.Data.Next = second.Ref()
	data, err := NewBuilder().SetBGMOffset(0.25).Add(first, second).Build()
	if err != nil {
		t.Fatal(err)
	}
	if data.BGMOffset != 0.25 || len(data.Entities) != 2 || data.Entities[0].Name != "0_Note" || data.Entities[1].Name != "tail" {
		t.Fatalf("level = %#v", data)
	}
	want := []resource.LevelDataEntityData{
		resource.LevelDataEntityValueData{Name: "#BEAT", Value: 1},
		resource.LevelDataEntityValueData{Name: "position.x", Value: 2},
		resource.LevelDataEntityValueData{Name: "position.y", Value: 3},
		resource.LevelDataEntityValueData{Name: "samples[0]", Value: 4},
		resource.LevelDataEntityValueData{Name: "samples[1]", Value: 5},
		resource.LevelDataEntityValueData{Name: "single", Value: 6},
		resource.LevelDataEntityRefData{Name: "next", Ref: "tail"},
	}
	if !reflect.DeepEqual(data.Entities[0].Data, want) {
		t.Fatalf("data = %#v, want %#v", data.Entities[0].Data, want)
	}
	encoded, err := Marshal(data)
	if err != nil || !strings.HasSuffix(string(encoded), "\n") || !strings.Contains(string(encoded), `"ref": "tail"`) {
		t.Fatalf("encoded = %s, err = %v", encoded, err)
	}
}

func TestBuilderRejectsInvalidDeclarationsAndReferences(t *testing.T) {
	if _, err := Define[int]("Note"); err == nil || !strings.Contains(err.Error(), "must be a struct") {
		t.Fatalf("non-struct error = %v", err)
	}
	type duplicate struct {
		Position sonolus.Vec2 `level:"value"`
		X        float64      `level:"value.x"`
	}
	if _, err := Define[duplicate]("Duplicate"); err == nil || !strings.Contains(err.Error(), "duplicate flattened") {
		t.Fatalf("duplicate error = %v", err)
	}
	note := MustDefine[testNoteData]("Note")
	foreign := note.New(testNoteData{})
	entity := note.New(testNoteData{Next: foreign.Ref()})
	if _, err := NewBuilder().Add(entity).Build(); err == nil || !strings.Contains(err.Error(), "not part of this level") {
		t.Fatalf("foreign reference error = %v", err)
	}
	entity.Data.Next = Ref[testNoteData]{}
	entity.Data.Beat = math.Inf(1)
	if _, err := NewBuilder().Add(entity).Build(); err == nil || !strings.Contains(err.Error(), "must be finite") {
		t.Fatalf("non-finite error = %v", err)
	}
}
