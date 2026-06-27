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
