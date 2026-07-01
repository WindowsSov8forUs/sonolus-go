package pack

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/WindowsSov8forUs/sonolus-core-go/core"
	"github.com/WindowsSov8forUs/sonolus-core-go/core/resource"
	"github.com/WindowsSov8forUs/sonolus-core-go/database"
)

const (
	defaultFileMode = 0644
	defaultDirMode  = 0755
)

// minPNG is a 1x1 transparent PNG pixel (the smallest valid PNG).
var minPNG = []byte{
	0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A,
	0x00, 0x00, 0x00, 0x0D, 0x49, 0x48, 0x44, 0x52,
	0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01,
	0x08, 0x06, 0x00, 0x00, 0x00, 0x1F, 0x15, 0xC4,
	0x89, 0x00, 0x00, 0x00, 0x0A, 0x49, 0x44, 0x41,
	0x54, 0x78, 0x9C, 0x62, 0x00, 0x00, 0x00, 0x02,
	0x00, 0x01, 0xE5, 0x27, 0xDE, 0xFC, 0x00, 0x00,
	0x00, 0x00, 0x49, 0x45, 0x4E, 0x44, 0xAE, 0x42,
	0x60, 0x82,
}

// minMP3 is a minimal silent MP3 frame (~1 second of silence at 128kbps).
// It is just enough for a valid mp3 header + silent audio.
var minMP3 = []byte{
	0xFF, 0xFB, 0x90, 0x00, 0x00, 0x00, 0x00, 0x00,
	0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
}

func loc(s string) database.LocalizationText {
	return database.LocalizationText{"en": core.Text(s)}
}

// writeItemJSON writes a minimal item.json for a non-engine resource.
func writeItemJSON(dir string, item map[string]any) error {
	data, err := json.MarshalIndent(item, "", "\t")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, "item.json"), data, defaultFileMode)
}

// EmitDefaultItems writes minimal placeholder items for skin, background, effect,
// particle, and a demo level so that an engine referencing them passes pack-go's
// reference validation. All assets are minimal placeholders (1x1 PNG, silent MP3,
// empty resource data).
func EmitDefaultItems(dir string, engineName string) error {
	skinName := "default"
	bgName := "default"
	effectName := "default"
	particleName := "default"
	levelName := "demo"

	if err := emitDefaultSkin(dir, skinName); err != nil {
		return fmt.Errorf("pack: default skin: %w", err)
	}
	if err := emitDefaultBackground(dir, bgName); err != nil {
		return fmt.Errorf("pack: default background: %w", err)
	}
	if err := emitDefaultEffect(dir, effectName); err != nil {
		return fmt.Errorf("pack: default effect: %w", err)
	}
	if err := emitDefaultParticle(dir, particleName); err != nil {
		return fmt.Errorf("pack: default particle: %w", err)
	}
	if err := emitDefaultLevel(dir, levelName, engineName, skinName, bgName, effectName, particleName); err != nil {
		return fmt.Errorf("pack: default level: %w", err)
	}
	return nil
}

func emitDefaultSkin(dir, name string) error {
	d := filepath.Join(dir, "skins", name)
	if err := os.MkdirAll(d, defaultDirMode); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(d, "thumbnail"), minPNG, defaultFileMode); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(d, "texture"), minPNG, defaultFileMode); err != nil {
		return err
	}
	data, err := json.Marshal(resource.EngineSkinData{
		RenderMode: resource.EngineRenderModeDefault,
		Sprites: []resource.EngineSkinDataSprite{
			{Name: "default", ID: 0},
		},
	})
	if err != nil {
		return fmt.Errorf("pack: marshal skin data: %w", err)
	}
	if err := os.WriteFile(filepath.Join(d, "data"), data, defaultFileMode); err != nil {
		return err
	}
	return writeItemJSON(d, map[string]any{
		"version":  4,
		"title":    loc(name),
		"subtitle": loc(""),
		"author":   loc("sonolus-go"),
		"tags":     []any{},
	})
}

func emitDefaultBackground(dir, name string) error {
	d := filepath.Join(dir, "backgrounds", name)
	if err := os.MkdirAll(d, defaultDirMode); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(d, "thumbnail"), minPNG, defaultFileMode); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(d, "image"), minPNG, defaultFileMode); err != nil {
		return err
	}
	data, err := json.Marshal(resource.BackgroundData{Fit: resource.FitWidth, Color: "#000000"})
	if err != nil {
		return fmt.Errorf("pack: marshal background data: %w", err)
	}
	if err := os.WriteFile(filepath.Join(d, "data"), data, defaultFileMode); err != nil {
		return err
	}
	cfg, err := json.Marshal(resource.BackgroundConfiguration{Blur: 0, Mask: "#000000"})
	if err != nil {
		return fmt.Errorf("pack: marshal background config: %w", err)
	}
	if err := os.WriteFile(filepath.Join(d, "configuration"), cfg, defaultFileMode); err != nil {
		return err
	}
	return writeItemJSON(d, map[string]any{
		"version":  2,
		"title":    loc(name),
		"subtitle": loc(""),
		"author":   loc("sonolus-go"),
		"tags":     []any{},
	})
}

func emitDefaultEffect(dir, name string) error {
	d := filepath.Join(dir, "effects", name)
	if err := os.MkdirAll(d, defaultDirMode); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(d, "thumbnail"), minPNG, defaultFileMode); err != nil {
		return err
	}
	// Audio is required by pack-go for effect items (type "zip").
	// Write a minimal silent MP3 as a placeholder, matching the level BGM pattern.
	if err := os.WriteFile(filepath.Join(d, "audio"), minMP3, defaultFileMode); err != nil {
		return err
	}
	data, err := json.Marshal(resource.EngineEffectData{})
	if err != nil {
		return fmt.Errorf("pack: marshal effect data: %w", err)
	}
	if err := os.WriteFile(filepath.Join(d, "data"), data, defaultFileMode); err != nil {
		return err
	}
	return writeItemJSON(d, map[string]any{
		"version":  5,
		"title":    loc(name),
		"subtitle": loc(""),
		"author":   loc("sonolus-go"),
		"tags":     []any{},
	})
}

func emitDefaultParticle(dir, name string) error {
	d := filepath.Join(dir, "particles", name)
	if err := os.MkdirAll(d, defaultDirMode); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(d, "thumbnail"), minPNG, defaultFileMode); err != nil {
		return err
	}
	data, err := json.Marshal(resource.EngineParticleData{
		Effects: []resource.EngineParticleDataEffect{
			{Name: "default", ID: 0},
		},
	})
	if err != nil {
		return fmt.Errorf("pack: marshal particle data: %w", err)
	}
	if err := os.WriteFile(filepath.Join(d, "data"), data, defaultFileMode); err != nil {
		return err
	}
	return writeItemJSON(d, map[string]any{
		"version":  3,
		"title":    loc(name),
		"subtitle": loc(""),
		"author":   loc("sonolus-go"),
		"tags":     []any{},
	})
}

func emitDefaultLevel(dir, name, engineName, skin, bg, effect, particle string) error {
	d := filepath.Join(dir, "levels", name)
	if err := os.MkdirAll(d, defaultDirMode); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(d, "cover"), minPNG, defaultFileMode); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(d, "bgm"), minMP3, defaultFileMode); err != nil {
		return err
	}
	data, err := json.Marshal(resource.LevelData{})
	if err != nil {
		return fmt.Errorf("pack: marshal level data: %w", err)
	}
	if err := os.WriteFile(filepath.Join(d, "data"), data, defaultFileMode); err != nil {
		return err
	}
	return writeItemJSON(d, map[string]any{
		"version":       1,
		"rating":        1,
		"title":         loc("Demo Chart"),
		"artists":       loc("sonolus-go"),
		"author":        loc("sonolus-go"),
		"tags":          []any{},
		"engine":        engineName,
		"useSkin":       map[string]any{"useDefault": true},
		"useBackground": map[string]any{"useDefault": true},
		"useEffect":     map[string]any{"useDefault": true},
		"useParticle":   map[string]any{"useDefault": true},
	})
}
