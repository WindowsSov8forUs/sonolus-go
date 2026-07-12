// Package pack provides the adapter layer between sonolus-go's compiler output
// and sonolus-pack-go's source tree input. It writes compiled Engine*Data as
// raw JSON files into the directory layout that pack-go expects, and generates
// minimal default non-engine items so an engine can pass pack-go's reference
// validation.
package pack

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/WindowsSov8forUs/sonolus-core-go/core"
	"github.com/WindowsSov8forUs/sonolus-core-go/core/resource"
	"github.com/WindowsSov8forUs/sonolus-core-go/database"
	"github.com/WindowsSov8forUs/sonolus-go/internal/newcompiler"
)

// EngineItemMeta carries the metadata that goes into item.json for an engine.
type EngineItemMeta struct {
	Title      string // e.g. "My Engine"
	Subtitle   string
	Author     string
	Skin       string // name of the skin item (e.g. "default")
	Background string // name of the background item
	Effect     string // name of the effect item
	Particle   string // name of the particle item
}

// CompiledEngine bundles the outputs of compiling one engine for all four modes.
type CompiledEngine struct {
	Configuration resource.EngineConfiguration
	PlayData      resource.EnginePlayData
	WatchData     resource.EngineWatchData
	PreviewData   resource.EnginePreviewData
	TutorialData  resource.EngineTutorialData
	ROM           []byte // optional ROM data, nil to omit
}

func EmitArtifactsSource(dir, name string, artifacts *newcompiler.Artifacts, meta EngineItemMeta) error {
	if artifacts == nil || artifacts.Configuration == nil || artifacts.Play == nil || artifacts.Watch == nil || artifacts.Preview == nil || artifacts.Tutorial == nil {
		return fmt.Errorf("pack: complete four-mode artifacts are required")
	}
	return EmitPackSource(dir, name, CompiledEngine{
		Configuration: *artifacts.Configuration,
		PlayData:      *artifacts.Play,
		WatchData:     *artifacts.Watch,
		PreviewData:   *artifacts.Preview,
		TutorialData:  *artifacts.Tutorial,
		ROM:           append([]byte(nil), artifacts.ROM...),
	}, meta)
}

// EmitPackSource writes a pack-go-compatible source/engines/<name>/ directory
// tree under dir. The Engine*Data values are written as raw JSON (NOT gzip),
// matching what pack-go's Pack with type "json" expects.
func EmitPackSource(dir string, name string, c CompiledEngine, meta EngineItemMeta) error {
	engineDir := filepath.Join(dir, "engines", name)
	if err := os.MkdirAll(engineDir, defaultDirMode); err != nil {
		return fmt.Errorf("pack: create engine dir: %w", err)
	}

	// Write thumbnail (required by sonolus-pack-go; a minimal 1x1 PNG).
	if err := os.WriteFile(filepath.Join(engineDir, "thumbnail"), minPNG, defaultFileMode); err != nil {
		return fmt.Errorf("pack: write engine thumbnail: %w", err)
	}

	// Write raw JSON data files (pack-go gzips them itself).
	writeJSON := func(filename string, v any) error {
		data, err := json.MarshalIndent(v, "", "\t")
		if err != nil {
			return err
		}
		return os.WriteFile(filepath.Join(engineDir, filename), data, defaultFileMode)
	}
	if err := writeJSON("configuration", c.Configuration); err != nil {
		return err
	}
	if err := writeJSON("playData", c.PlayData); err != nil {
		return err
	}
	if err := writeJSON("watchData", c.WatchData); err != nil {
		return err
	}
	if err := writeJSON("previewData", c.PreviewData); err != nil {
		return err
	}
	if err := writeJSON("tutorialData", c.TutorialData); err != nil {
		return err
	}

	// ROM is optional binary. If the ROM bytes are gzip-compressed (as produced
	// by build.BuildROM), decompress them so sonolus-pack-go can gzip them once.
	// Raw (uncompressed) ROM bytes pass through unchanged.
	if len(c.ROM) > 0 {
		romData := c.ROM
		if len(romData) >= 2 && romData[0] == 0x1f && romData[1] == 0x8b {
			reader, err := gzip.NewReader(bytes.NewReader(romData))
			if err != nil {
				return fmt.Errorf("pack: decompress ROM: %w", err)
			}
			romData, err = io.ReadAll(reader)
			closeErr := reader.Close()
			if err != nil {
				return fmt.Errorf("pack: decompress ROM: %w", err)
			}
			if closeErr != nil {
				return fmt.Errorf("pack: close ROM gzip: %w", closeErr)
			}
		}
		if err := os.WriteFile(filepath.Join(engineDir, "rom"), romData, 0644); err != nil {
			return err
		}
	}

	// Write item.json.
	if err := writeEngineItem(engineDir, meta); err != nil {
		return err
	}

	// Write info.json (required by pack-go; title is mandatory).
	if err := writeInfoJSON(dir, meta); err != nil {
		return err
	}

	return nil
}

// writeEngineItem writes the item.json metadata that pack-go's Schema parser
// requires for an engine.
func writeEngineItem(dir string, meta EngineItemMeta) error {
	item := map[string]any{
		"version": 13,
		"title": map[string]string{
			"en": strRef(meta.Title),
		},
		"subtitle": map[string]string{
			"en": strRef(meta.Subtitle),
		},
		"author": map[string]string{
			"en": strRef(meta.Author),
		},
		"tags":       []any{},
		"skin":       strRef(meta.Skin),
		"background": strRef(meta.Background),
		"effect":     strRef(meta.Effect),
		"particle":   strRef(meta.Particle),
	}
	data, err := json.MarshalIndent(item, "", "\t")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, "item.json"), data, 0644)
}

// strRef returns s, or "default" when s is empty (sonolus-pack convention).
func strRef(s string) string {
	if s == "" {
		return "default"
	}
	return s
}

// writeInfoJSON writes the top-level info.json file that pack-go requires.
// It must contain at least a "title" field in localization-text format.
func writeInfoJSON(dir string, meta EngineItemMeta) error {
	info := map[string]any{
		"title": database.LocalizationText{"en": core.Text(meta.Title)},
	}
	if meta.Subtitle != "" {
		info["subtitle"] = database.LocalizationText{"en": core.Text(meta.Subtitle)}
	}
	if meta.Author != "" {
		info["author"] = database.LocalizationText{"en": core.Text(meta.Author)}
	}
	data, err := json.MarshalIndent(info, "", "\t")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, "info.json"), data, 0644)
}
