package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/WindowsSov8forUs/sonolus-core-go/core/resource"

	"github.com/WindowsSov8forUs/sonolus-go/internal/compiler/build"
	"github.com/WindowsSov8forUs/sonolus-go/internal/compiler/engine"
)

func cmdBuild(srcPaths []string, outDir, modeFlag string, optFlag int, romFlag string, statsFlag bool) error {
	ess, name, err := resolveSourceArg(srcPaths...)
	if err != nil {
		return err
	}

	dir := filepath.Join(outDir, name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("creating %s: %w", dir, err)
	}

	mode, err := ParseMode(modeFlag)
	if err != nil {
		return err
	}

	optLevel, err := parseOptLevel(optFlag)
	if err != nil {
		return err
	}

	cfg := &resource.EngineConfiguration{}
	var playData *resource.EnginePlayData
	var playErr error
	if mode != ModeAll {
		playData, cfg, playErr = engine.CompilePlaySources(ess, buildOpts(nil, optLevel))
	}

	var rom []byte
	if romFlag != "" {
		rom, err = build.BuildROMFromFile(romFlag)
		if err != nil {
			return fmt.Errorf("loading ROM: %w", err)
		}
	} else {
		rom, err = build.DefaultROMBytes()
		if err != nil {
			return fmt.Errorf("building ROM: %w", err)
		}
	}

	modes := mode.Expand()
	if playErr != nil {
		needsPlay := false
		for _, m := range modes {
			if m == ModePlay {
				needsPlay = true
				break
			}
		}
		if !needsPlay {
			fmt.Fprintf(os.Stderr, "warning: play compilation failed (config will be empty): %v\n", playErr)
		}
	}

	if mode == ModeAll {
		c, err := compileAllModes(ess, statsFlag, optLevel)
		if err != nil {
			return fmt.Errorf("compiling: %w", err)
		}
		cfg = &c.Configuration
		writePlay(dir, cfg, &c.PlayData, rom)
		writeNonPlay(dir, cfg, rom, c.WatchData, func(p *build.PackagedEngine, b []byte) { p.WatchData = b }, "watch")
		writeNonPlay(dir, cfg, rom, c.PreviewData, func(p *build.PackagedEngine, b []byte) { p.PreviewData = b }, "preview")
		writeNonPlay(dir, cfg, rom, c.TutorialData, func(p *build.PackagedEngine, b []byte) { p.TutorialData = b }, "tutorial")
	} else {
		for _, m := range modes {
			switch m {
			case ModePlay:
				if playData == nil {
					return fmt.Errorf("compiling play: %w", playErr)
				}
				writePlay(dir, cfg, playData, rom)
			case ModeWatch:
				data, err := engine.CompileWatchSources(ess, buildOpts(nil, optLevel))
				if err != nil {
					return fmt.Errorf("compiling watch: %w", err)
				}
				writeNonPlay(dir, cfg, rom, *data, func(p *build.PackagedEngine, b []byte) { p.WatchData = b }, "watch")
			case ModePreview:
				data, err := engine.CompilePreviewSources(ess, buildOpts(nil, optLevel))
				if err != nil {
					return fmt.Errorf("compiling preview: %w", err)
				}
				writeNonPlay(dir, cfg, rom, *data, func(p *build.PackagedEngine, b []byte) { p.PreviewData = b }, "preview")
			case ModeTutorial:
				data, err := engine.CompileTutorialSources(ess, buildOpts(nil, optLevel))
				if err != nil {
					return fmt.Errorf("compiling tutorial: %w", err)
				}
				writeNonPlay(dir, cfg, rom, *data, func(p *build.PackagedEngine, b []byte) { p.TutorialData = b }, "tutorial")
			}
		}
	}

	if mode == ModeAll {
		fmt.Printf("wrote all (%s) engine to %s/\n", strings.Join(allModeNames(), ", "), dir)
	} else {
		fmt.Printf("wrote %s engine to %s/\n", mode, dir)
	}
	return nil
}

func writePlay(dir string, cfg *resource.EngineConfiguration, data *resource.EnginePlayData, rom []byte) {
	pkg, err := build.PackagePlay(cfg, data, rom)
	if err != nil {
		fatalf("packaging play: %v", err)
	}
	if err := pkg.Write(dir); err != nil {
		fatalf("writing play: %v", err)
	}
}

func writeNonPlay[D any](dir string, cfg *resource.EngineConfiguration,
	rom []byte, data D, setBlob func(*build.PackagedEngine, []byte), name string) {
	pkg, err := build.PackageNonPlay(cfg, rom, &data, setBlob)
	if err != nil {
		fatalf("packaging %s: %v", name, err)
	}
	if err := pkg.Write(dir); err != nil {
		fatalf("writing %s: %v", name, err)
	}
}
