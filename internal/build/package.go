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
	"github.com/WindowsSov8forUs/sonolus-go/v2/internal/compiler"
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

// PackageArtifacts serializes a complete compiler snapshot. ROM contains
// final raw float32 bytes and is gzip-compressed without JSON encoding.
func PackageArtifacts(artifacts *compiler.Artifacts) (*PackagedEngine, error) {
	if artifacts == nil {
		return nil, fmt.Errorf("package artifacts: artifacts are nil")
	}
	configuration, err := codec.Compress(artifacts.Configuration)
	if err != nil {
		return nil, fmt.Errorf("package configuration: %w", err)
	}
	var rom []byte
	if artifacts.ROM != nil {
		rom, err = compressBytes(artifacts.ROM)
		if err != nil {
			return nil, fmt.Errorf("package ROM: %w", err)
		}
	}
	result := &PackagedEngine{Configuration: configuration, ROM: rom}
	values := []struct {
		name    string
		value   any
		set     func([]byte)
		present bool
	}{
		{"play data", artifacts.Play, func(value []byte) { result.PlayData = value }, artifacts.Play != nil},
		{"watch data", artifacts.Watch, func(value []byte) { result.WatchData = value }, artifacts.Watch != nil},
		{"preview data", artifacts.Preview, func(value []byte) { result.PreviewData = value }, artifacts.Preview != nil},
		{"tutorial data", artifacts.Tutorial, func(value []byte) { result.TutorialData = value }, artifacts.Tutorial != nil},
	}
	for _, item := range values {
		if !item.present {
			continue
		}
		blob, err := codec.Compress(item.value)
		if err != nil {
			return nil, fmt.Errorf("package %s: %w", item.name, err)
		}
		item.set(blob)
	}
	return result, nil
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

// WriteAtomic writes a complete package through a sibling staging directory
// and swaps it into place only after every file has been written.
func (p *PackagedEngine) WriteAtomic(dir string) error {
	parent, base := filepath.Dir(dir), filepath.Base(dir)
	if err := os.MkdirAll(parent, 0o755); err != nil {
		return err
	}
	stage, err := os.MkdirTemp(parent, "."+base+"-stage-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(stage)
	if err := p.Write(stage); err != nil {
		return err
	}
	backup, err := os.MkdirTemp(parent, "."+base+"-backup-")
	if err != nil {
		return err
	}
	if err := os.Remove(backup); err != nil {
		return err
	}
	defer os.RemoveAll(backup)
	if _, err := os.Stat(dir); err == nil {
		if err := os.Rename(dir, backup); err != nil {
			return err
		}
	} else if !os.IsNotExist(err) {
		return err
	}
	if err := os.Rename(stage, dir); err != nil {
		_ = os.Rename(backup, dir)
		return err
	}
	_ = os.RemoveAll(backup)
	return nil
}
