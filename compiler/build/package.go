// Package build packages compiled engine data into the on-disk Sonolus file
// layout: each datum is gzip-compressed, mirroring sonolus.py's
// package_data / PackagedEngine.write.
package build

import (
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
	FileRom           = "EngineRom"
)

// PackagedPlayEngine holds the gzipped blobs for the play-mode slice of an engine.
type PackagedPlayEngine struct {
	Configuration []byte
	PlayData      []byte
	Rom           []byte
}

// PackageAny gzip-compresses any JSON-serializable value.
func PackageAny(data any) ([]byte, error) { return codec.Compress(data) }

// PackagePlayData gzip-compresses a JSON EnginePlayData.
func PackagePlayData(data *resource.EnginePlayData) ([]byte, error) {
	return codec.Compress(data)
}

// PackageConfiguration gzip-compresses a JSON EngineConfiguration.
func PackageConfiguration(cfg *resource.EngineConfiguration) ([]byte, error) {
	return codec.Compress(cfg)
}

// PackagePlay builds the play-mode packaged engine from its configuration, play
// data, and ROM.
func PackagePlay(cfg *resource.EngineConfiguration, data *resource.EnginePlayData, rom []byte) (*PackagedPlayEngine, error) {
	configuration, err := PackageConfiguration(cfg)
	if err != nil {
		return nil, err
	}
	playData, err := PackagePlayData(data)
	if err != nil {
		return nil, err
	}
	return &PackagedPlayEngine{Configuration: configuration, PlayData: playData, Rom: rom}, nil
}

// Write writes the packaged engine files into dir, creating it if needed.
func (p *PackagedPlayEngine) Write(dir string) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	files := map[string][]byte{
		FileConfiguration: p.Configuration,
		FilePlayData:      p.PlayData,
		FileRom:           p.Rom,
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
