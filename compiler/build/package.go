// Package build packages compiled engine data into the on-disk Sonolus file
// layout: each datum is gzip-compressed, mirroring sonolus.py's
// package_data / PackagedEngine.write.
package build

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/WindowsSov8forUs/sonolus-core-go/codec"
	"github.com/WindowsSov8forUs/sonolus-core-go/core/resource"
)

// Canonical engine file names (no extension), matching PackagedEngine.write.
const (
	FileConfiguration = "EngineConfiguration"
	FilePlayData      = "EnginePlayData"
	FileWatchData     = "EngineWatchData"
	FilePreviewData   = "EnginePreviewData"
	FileTutorialData  = "EngineTutorialData"
	FileROM           = "EngineRom"
)

// PackagedEngine holds the gzipped blobs for all engine modes.
type PackagedEngine struct {
	Configuration []byte
	ROM           []byte
	PlayData      []byte
	WatchData     []byte
	PreviewData   []byte
	TutorialData  []byte
}

// PackageAny gzip-compresses any JSON-serializable value.
func PackageAny(data any) ([]byte, error) { return codec.Compress(data) }

// PackagePlay builds the play-mode packaged engine from its configuration, play
// data, and ROM.
func PackagePlay(cfg *resource.EngineConfiguration, data *resource.EnginePlayData, rom []byte) (*PackagedEngine, error) {
	configuration, err := codec.Compress(cfg)
	if err != nil {
		return nil, fmt.Errorf("package configuration: %w", err)
	}
	playData, err := codec.Compress(data)
	if err != nil {
		return nil, fmt.Errorf("package play data: %w", err)
	}
	return &PackagedEngine{Configuration: configuration, PlayData: playData, ROM: rom}, nil
}

// PackageNonPlay builds a packaged engine for a non-play mode (watch, preview,
// or tutorial). cfg and rom are shared with Play; data is the mode-specific
// compiled data. setBlob stores the compressed blob into the correct field
// on the PackagedEngine (e.g. p.WatchData = blob).
func PackageNonPlay(cfg *resource.EngineConfiguration, rom []byte, data any, setBlob func(*PackagedEngine, []byte)) (*PackagedEngine, error) {
	configuration, err := codec.Compress(cfg)
	if err != nil {
		return nil, fmt.Errorf("package configuration: %w", err)
	}
	blob, err := codec.Compress(data)
	if err != nil {
		return nil, fmt.Errorf("package data: %w", err)
	}
	p := &PackagedEngine{Configuration: configuration, ROM: rom}
	setBlob(p, blob)
	return p, nil
}

// Write writes the non-nil packaged engine files into dir, creating it if needed.
func (p *PackagedEngine) Write(dir string) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	files := map[string][]byte{
		FileConfiguration: p.Configuration,
		FileROM:           p.ROM,
		FilePlayData:      p.PlayData,
		FileWatchData:     p.WatchData,
		FilePreviewData:   p.PreviewData,
		FileTutorialData:  p.TutorialData,
	}
	for name, blob := range files {
		if blob == nil {
			continue
		}
		if err := os.WriteFile(filepath.Join(dir, name), blob, 0o644); err != nil {
			return err
		}
	}
	return nil
}
