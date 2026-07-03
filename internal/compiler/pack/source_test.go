package pack

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/WindowsSov8forUs/sonolus-core-go/core/resource"
)

// TestEmitPackSource verifies basic file output for a compiled engine.
func TestEmitPackSource(t *testing.T) {
	dir := t.TempDir()
	c := CompiledEngine{
		Configuration: resource.EngineConfiguration{},
		PlayData: resource.EnginePlayData{
			Archetypes: []resource.EnginePlayDataArchetype{{Name: "Test"}},
		},
		WatchData:    resource.EngineWatchData{},
		PreviewData:  resource.EnginePreviewData{},
		TutorialData: resource.EngineTutorialData{},
	}
	meta := EngineItemMeta{
		Title:  "Test Engine",
		Author: "tester",
	}

	if err := EmitPackSource(dir, "test", c, meta); err != nil {
		t.Fatalf("EmitPackSource: %v", err)
	}

	engineDir := filepath.Join(dir, "engines", "test")

	for _, f := range []string{"thumbnail", "configuration", "playData", "watchData", "previewData", "tutorialData", "item.json"} {
		path := filepath.Join(engineDir, f)
		if _, err := os.Stat(path); err != nil {
			t.Errorf("expected file %s: %v", f, err)
		}
	}

	// rom should NOT exist since nil
	if _, err := os.Stat(filepath.Join(engineDir, "rom")); !os.IsNotExist(err) {
		t.Error("rom should not exist when nil")
	}

	// Verify playData is raw JSON (not gzip) — it should parse as JSON.
	playJSON, err := os.ReadFile(filepath.Join(engineDir, "playData"))
	if err != nil {
		t.Fatal(err)
	}
	var pd resource.EnginePlayData
	if err := json.Unmarshal(playJSON, &pd); err != nil {
		t.Fatalf("playData is not valid JSON: %v", err)
	}
	if len(pd.Archetypes) != 1 || pd.Archetypes[0].Name != "Test" {
		t.Errorf("round-trip lost archetype: %+v", pd.Archetypes)
	}

	// Verify item.json
	itemJSON, err := os.ReadFile(filepath.Join(engineDir, "item.json"))
	if err != nil {
		t.Fatal(err)
	}
	var item map[string]any
	if err := json.Unmarshal(itemJSON, &item); err != nil {
		t.Fatalf("item.json is not valid JSON: %v", err)
	}
	if v, ok := item["version"].(float64); !ok || int(v) != 13 {
		t.Errorf("version = %v, want 13", item["version"])
	}
	title := item["title"].(map[string]any)
	if title["en"] != "Test Engine" {
		t.Errorf("title.en = %q, want Test Engine", title["en"])
	}
	if item["skin"] != "default" {
		t.Errorf("skin = %q, want default", item["skin"])
	}
}

// TestEmitPackSourceWithROM verifies ROM output when provided.
func TestEmitPackSourceWithROM(t *testing.T) {
	dir := t.TempDir()
	c := CompiledEngine{
		Configuration: resource.EngineConfiguration{},
		PlayData:      resource.EnginePlayData{},
		WatchData:     resource.EngineWatchData{},
		PreviewData:   resource.EnginePreviewData{},
		TutorialData:  resource.EngineTutorialData{},
		ROM:           []byte{1, 2, 3, 4},
	}
	if err := EmitPackSource(dir, "withrom", c, EngineItemMeta{Title: "X", Skin: "myskin"}); err != nil {
		t.Fatalf("EmitPackSource: %v", err)
	}
	romPath := filepath.Join(dir, "engines", "withrom", "rom")
	if _, err := os.Stat(romPath); err != nil {
		t.Errorf("rom should exist: %v", err)
	}
	romData, err := os.ReadFile(romPath)
	if err != nil {
		t.Fatal(err)
	}
	if len(romData) != 4 || romData[0] != 1 {
		t.Errorf("rom content mismatch: %v", romData)
	}
}

// TestEmitDefaultItems verifies that default items generate valid item.json
// files for all required resource types.
func TestEmitDefaultItems(t *testing.T) {
	dir := t.TempDir()
	if err := EmitDefaultItems(dir, "test-engine", EngineItemMeta{Skin: "default", Background: "default", Effect: "default", Particle: "default"}); err != nil {
		t.Fatalf("EmitDefaultItems: %v", err)
	}

	checks := []struct {
		path     string
		desc     string
		imgField string
	}{
		{"skins/default", "skin", "thumbnail"},
		{"backgrounds/default", "background", "thumbnail"},
		{"effects/default", "effect", "thumbnail"},
		{"particles/default", "particle", "thumbnail"},
		{"levels/demo", "level", "cover"},
	}
	for _, c := range checks {
		d := filepath.Join(dir, c.path)
		// item.json is always required.
		if _, err := os.Stat(filepath.Join(d, "item.json")); err != nil {
			t.Errorf("%s/item.json: %v", c.desc, err)
		}
		// Check the image file (thumbnail or cover).
		if _, err := os.Stat(filepath.Join(d, c.imgField)); err != nil {
			t.Errorf("%s/%s: %v", c.desc, c.imgField, err)
		}
		// Verify item.json is valid JSON with required fields
		itemJSON, err := os.ReadFile(filepath.Join(d, "item.json"))
		if err != nil {
			t.Errorf("%s item.json read: %v", c.desc, err)
			continue
		}
		var item map[string]any
		if err := json.Unmarshal(itemJSON, &item); err != nil {
			t.Errorf("%s item.json not valid JSON: %v", c.desc, err)
		}
		if _, ok := item["version"]; !ok {
			t.Errorf("%s item.json missing version", c.desc)
		}
	}
}

// TestCDNGeneration tests helper note for CDN/repository path.
func TestCDNGeneration(t *testing.T) {
	// Verify that the emitted default items generate .srl references that
	// pack-go can process.
	dir := t.TempDir()
	if err := EmitDefaultItems(dir, "test-engine", EngineItemMeta{Skin: "default", Background: "default", Effect: "default", Particle: "default"}); err != nil {
		t.Fatalf("EmitDefaultItems: %v", err)
	}

	// Verify skin data is valid EngineSkinData
	data, err := os.ReadFile(filepath.Join(dir, "skins", "default", "data"))
	if err != nil {
		t.Fatal(err)
	}
	var sd resource.EngineSkinData
	if err := json.Unmarshal(data, &sd); err != nil {
		t.Fatalf("skin data not valid EngineSkinData: %v", err)
	}
	if sd.RenderMode != resource.EngineRenderModeDefault {
		t.Errorf("skin renderMode = %v, want default", sd.RenderMode)
	}
	if len(sd.Sprites) < 1 || sd.Sprites[0].Name != "default" {
		t.Error("skin should have at least one 'default' sprite")
	}

	// Verify level references the engine
	itemJSON, err := os.ReadFile(filepath.Join(dir, "levels", "demo", "item.json"))
	if err != nil {
		t.Fatal(err)
	}
	var item map[string]any
	if err := json.Unmarshal(itemJSON, &item); err != nil {
		t.Fatal(err)
	}
	if item["engine"] != "test-engine" {
		t.Errorf("level.engine = %q, want test-engine", item["engine"])
	}
}
