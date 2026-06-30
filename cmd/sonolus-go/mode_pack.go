package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/WindowsSov8forUs/sonolus-core-go/core/resource"

	"github.com/WindowsSov8forUs/sonolus-go/compiler/build"
	"github.com/WindowsSov8forUs/sonolus-go/compiler/engine"
	"github.com/WindowsSov8forUs/sonolus-go/compiler/pack"
	"github.com/WindowsSov8forUs/sonolus-pack-go/packer"
)

// compileAllModes compiles the engine source for all four modes in parallel,
// returning the CompiledEngine bundle with the shared configuration.
// Go has no GIL, so this scales to all available cores — a structural advantage
// over the Python reference which requires no-GIL builds for parallelism.
// If stats is true, per-mode compilation times are printed to stdout.
func compileAllModes(src string, stats bool) (pack.CompiledEngine, error) {
	var c pack.CompiledEngine

	var (
		playData     *resource.EnginePlayData
		playCfg      *resource.EngineConfiguration
		watchData    *resource.EngineWatchData
		previewData  *resource.EnginePreviewData
		tutorialData *resource.EngineTutorialData
		playErr      error
		watchErr     error
		previewErr   error
		tutorialErr  error
		playDur      time.Duration
		watchDur     time.Duration
		previewDur   time.Duration
		tutorialDur  time.Duration
	)

	var wg sync.WaitGroup
	wg.Add(4)

	go func() {
		defer wg.Done()
		t0 := time.Now()
		if stats {
			stats := &engine.CompileStats{}
			opts := &engine.CompileOptions{Stats: stats}
			playData, playCfg, playErr = engine.CompilePlayFileWithStats(src, opts)
			playDur = time.Since(t0)
			stats.WriteSummary(os.Stderr, "play")
		} else {
			playData, playCfg, playErr = engine.CompilePlayFile(src)
			playDur = time.Since(t0)
		}
	}()
	go func() {
		defer wg.Done()
		t0 := time.Now()
		if stats {
			stats := &engine.CompileStats{}
			opts := &engine.CompileOptions{Stats: stats}
			watchData, watchErr = engine.CompileWatchFileWithStats(src, opts)
			watchDur = time.Since(t0)
			stats.WriteSummary(os.Stderr, "watch")
		} else {
			watchData, watchErr = engine.CompileWatchFile(src)
			watchDur = time.Since(t0)
		}
	}()
	go func() {
		defer wg.Done()
		t0 := time.Now()
		if stats {
			stats := &engine.CompileStats{}
			opts := &engine.CompileOptions{Stats: stats}
			previewData, previewErr = engine.CompilePreviewFileWithStats(src, opts)
			previewDur = time.Since(t0)
			stats.WriteSummary(os.Stderr, "preview")
		} else {
			previewData, previewErr = engine.CompilePreviewFile(src)
			previewDur = time.Since(t0)
		}
	}()
	go func() {
		defer wg.Done()
		t0 := time.Now()
		if stats {
			stats := &engine.CompileStats{}
			opts := &engine.CompileOptions{Stats: stats}
			tutorialData, tutorialErr = engine.CompileTutorialFileWithStats(src, opts)
			tutorialDur = time.Since(t0)
			stats.WriteSummary(os.Stderr, "tutorial")
		} else {
			tutorialData, tutorialErr = engine.CompileTutorialFile(src)
			tutorialDur = time.Since(t0)
		}
	}()

	wg.Wait()

	if stats {
		fmt.Printf("Compilation times (4-mode parallel):\n")
		fmt.Printf("  play:     %s\n", playDur.Round(time.Millisecond))
		fmt.Printf("  watch:    %s\n", watchDur.Round(time.Millisecond))
		fmt.Printf("  preview:  %s\n", previewDur.Round(time.Millisecond))
		fmt.Printf("  tutorial: %s\n", tutorialDur.Round(time.Millisecond))
		fmt.Printf("  total:    %s\n", (playDur + watchDur + previewDur + tutorialDur).Round(time.Millisecond))
	}

	if playErr != nil {
		return c, fmt.Errorf("play: %w", playErr)
	}
	if watchErr != nil {
		return c, fmt.Errorf("watch: %w", watchErr)
	}
	if previewErr != nil {
		return c, fmt.Errorf("preview: %w", previewErr)
	}
	if tutorialErr != nil {
		return c, fmt.Errorf("tutorial: %w", tutorialErr)
	}

	c.PlayData = *playData
	c.Configuration = *playCfg
	c.WatchData = *watchData
	c.PreviewData = *previewData
	c.TutorialData = *tutorialData

	return c, nil
}

func runPack(srcPath string, author string) error {
	src, err := os.ReadFile(srcPath)
	if err != nil {
		return fmt.Errorf("reading %s: %w", srcPath, err)
	}

	engineName := engineNameFromPath(srcPath)

	// 1. Compile all 4 modes.
	fmt.Printf("compiling %s...\n", engineName)
	c, err := compileAllModes(string(src), false) // stats only for build command
	if err != nil {
		return err
	}

	// 2. Build ROM.
	rom, err := build.DefaultROMBytes()
	if err != nil {
		return fmt.Errorf("building ROM: %w", err)
	}
	c.ROM = rom

	// 3. Emit pack-go source tree.
	sourceDir := filepath.Join(os.TempDir(), "sonolus-pack-source", engineName)
	if err := os.RemoveAll(sourceDir); err != nil {
		return err
	}
	defer os.RemoveAll(sourceDir)

	meta := pack.EngineItemMeta{
		Title:      engineName,
		Subtitle:   "",
		Author:     author,
		Skin:       "default",
		Background: "default",
		Effect:     "default",
		Particle:   "default",
	}
	if err := pack.EmitPackSource(sourceDir, engineName, c, meta); err != nil {
		return fmt.Errorf("emitting source tree: %w", err)
	}
	if err := pack.EmitDefaultItems(sourceDir, engineName); err != nil {
		return fmt.Errorf("emitting default items: %w", err)
	}

	// 4. Run pack-go to produce db.json + repository/<hash>.
	packDir := filepath.Join("dist", engineName+"-pack")
	fmt.Printf("packing to %s...\n", packDir)
	if err := packer.Pack(context.Background(), packer.Options{
		Input:  sourceDir,
		Output: packDir,
	}); err != nil {
		return fmt.Errorf("pack: %w", err)
	}

	fmt.Printf("packed %s to %s/\n", engineName, packDir)
	return nil
}
