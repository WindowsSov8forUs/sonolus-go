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

// FuzzLevelToBytes is a byte-level fuzz test that CompileLevel must not panic.
func FuzzLevelToBytes(f *testing.F) {
	f.Add([]byte(testLevel))
	f.Fuzz(func(t *testing.T, b []byte) {
		CompileLevel(string(b)) //nolint:errcheck
	})
}
