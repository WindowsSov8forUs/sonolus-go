package level

import (
	"testing"

	"github.com/WindowsSov8forUs/sonolus-core-go/codec"
	"github.com/WindowsSov8forUs/sonolus-core-go/core/resource"
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
	// Missing "entities" key defaults to empty.
	data, err := CompileLevel(`{"bgmOffset": 0}`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(data.Entities) != 0 {
		t.Errorf("expected 0 entities for missing key, got %d", len(data.Entities))
	}
}

func TestLevel_WrongTypes(t *testing.T) {
	// bgmOffset as string instead of number.
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
		CompileLevel(string(b)) //nolint:errcheck
	})
}
