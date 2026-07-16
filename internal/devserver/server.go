// Package devserver assembles a complete, in-memory Sonolus development server.
package devserver

import (
	"embed"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"sync"

	"github.com/WindowsSov8forUs/sonolus-core-go/core"
	"github.com/WindowsSov8forUs/sonolus-core-go/database"
	"github.com/WindowsSov8forUs/sonolus-go/internal/build"
	"github.com/WindowsSov8forUs/sonolus-go/internal/compiler"
	sonolusserver "github.com/WindowsSov8forUs/sonolus-server-go"
	"github.com/gin-gonic/gin"
)

//go:embed assets/free-pack/pack assets/silence.mp3
var assets embed.FS

var (
	packOnce sync.Once
	packDir  string
	packErr  error
)

func init() {
	gin.SetMode(gin.ReleaseMode)
}

// New creates a complete Sonolus handler backed by one immutable compile snapshot.
func New(name string, artifacts *compiler.Artifacts, packaged *build.PackagedEngine, levelData []byte) (http.Handler, error) {
	if artifacts == nil || packaged == nil || len(levelData) == 0 {
		return nil, fmt.Errorf("dev server: complete engine and level artifacts are required")
	}
	dir, err := embeddedPackDir()
	if err != nil {
		return nil, err
	}
	server := sonolusserver.New(sonolusserver.Options{})
	if err := server.Load(dir); err != nil {
		return nil, fmt.Errorf("dev server: load embedded free pack: %w", err)
	}
	server.Title = text("sonolus-go Dev Server")

	thumbnail, err := assets.ReadFile("assets/free-pack/pack/repository/17bb17c86fe270da637693ee36e26c40e947bd12")
	if err != nil {
		return nil, fmt.Errorf("dev server: read thumbnail: %w", err)
	}
	bgm, err := assets.ReadFile("assets/silence.mp3")
	if err != nil {
		return nil, fmt.Errorf("dev server: read BGM: %w", err)
	}
	thumbnailSRL := server.AddBytes(thumbnail, "")
	bgmSRL := server.AddBytes(bgm, "")
	configuration := server.AddBytes(packaged.Configuration, "")
	playData := server.AddBytes(packaged.PlayData, "")
	watchData := server.AddBytes(packaged.WatchData, "")
	previewData := server.AddBytes(packaged.PreviewData, "")
	tutorialData := server.AddBytes(packaged.TutorialData, "")
	rom := server.AddBytes(packaged.ROM, "")
	level := server.AddBytes(levelData, "")

	server.Engine.Items = append(server.Engine.Items, sonolusserver.EngineItemModel{DatabaseEngineItem: database.DatabaseEngineItem{
		Name: name, Version: database.DatabaseEngineItemVersion,
		Title: text(name), Subtitle: text("Development Engine"), Author: text("sonolus-go"), Tags: []database.DatabaseTag{},
		Skin: "pixel", Background: "darkblue", Effect: "8bit", Particle: "pixel", Thumbnail: thumbnailSRL,
		PlayData: playData, WatchData: watchData, PreviewData: previewData, TutorialData: tutorialData, ROM: &rom, Configuration: configuration,
	}})
	server.Level.Items = append(server.Level.Items, sonolusserver.LevelItemModel{DatabaseLevelItem: database.DatabaseLevelItem{
		Name: "dev", Version: database.DatabaseLevelItemVersion, Rating: 0,
		Title: text("Dev Level"), Artists: text("Unknown"), Author: text("sonolus-go"), Tags: []database.DatabaseTag{}, Engine: name,
		UseSkin: database.DatabaseUseItem{UseDefault: true}, UseBackground: database.DatabaseUseItem{UseDefault: true},
		UseEffect: database.DatabaseUseItem{UseDefault: true}, UseParticle: database.DatabaseUseItem{UseDefault: true},
		Cover: thumbnailSRL, BGM: bgmSRL, Data: level,
	}})
	return server.Handler(), nil
}

func text(value string) database.LocalizationText {
	return database.LocalizationText{"en": core.Text(value)}
}

func embeddedPackDir() (string, error) {
	packOnce.Do(func() {
		root, err := os.MkdirTemp("", "sonolus-go-free-pack-")
		if err != nil {
			packErr = err
			return
		}
		source, err := fs.Sub(assets, "assets/free-pack/pack")
		if err != nil {
			packErr = err
			return
		}
		packErr = fs.WalkDir(source, ".", func(path string, entry fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			target := filepath.Join(root, filepath.FromSlash(path))
			if entry.IsDir() {
				return os.MkdirAll(target, 0o755)
			}
			data, err := fs.ReadFile(source, path)
			if err != nil {
				return err
			}
			return os.WriteFile(target, data, 0o644)
		})
		if packErr == nil {
			packDir = root
		}
	})
	if packErr != nil {
		return "", fmt.Errorf("dev server: materialize embedded free pack: %w", packErr)
	}
	return packDir, nil
}
