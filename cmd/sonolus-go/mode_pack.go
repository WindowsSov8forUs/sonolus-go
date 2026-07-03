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
	"github.com/WindowsSov8forUs/sonolus-go/compiler/ir/optimize"
	"github.com/WindowsSov8forUs/sonolus-go/compiler/pack"
	"github.com/WindowsSov8forUs/sonolus-pack-go/packer"
)

// compileWithStats compiles one mode with optional stats capture. The two
// compile functions handle the with-stats and without-stats paths,
// eliminating the copy-paste stats/timing boilerplate from the four
// parallel goroutines in compileAllModes.
func compileWithStats(
	wg *sync.WaitGroup, modeName string, withStats bool,
	compile func() error,
	compileStats func(*engine.CompileStats) error,
	dur *time.Duration,
) {
	defer wg.Done()
	t0 := time.Now()
	if withStats {
		cs := &engine.CompileStats{}
		if err := compileStats(cs); err != nil {
			*dur = time.Since(t0)
			return
		}
		*dur = time.Since(t0)
		cs.WriteSummary(os.Stderr, modeName)
	} else {
		// compile() error is captured by closure (stored in outer err variable via
		// the calling compileWithStats closure). We intentionally discard it here
		// because the caller checks the closure-captured error after the loop.
		_ = compile()
		*dur = time.Since(t0)
	}
}

// compileAllModes compiles the engine source for all four modes in parallel,
// returning the CompiledEngine bundle with the shared configuration.
// Go has no GIL, so this scales to all available cores — a structural advantage
// over the Python reference which requires no-GIL builds for parallelism.
// If stats is true, per-mode compilation times are printed to stdout.
func compileAllModes(ess *engine.EngineSources, stats bool, optLevel optimize.Level) (pack.CompiledEngine, error) {
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

	go compileWithStats(&wg, "play", stats,
		func() error {
			opts := &engine.CompileOptions{Opt: optLevel}
			playData, playCfg, playErr = engine.CompilePlaySources(ess, opts)
			return playErr
		},
		func(cs *engine.CompileStats) error {
			opts := &engine.CompileOptions{Opt: optLevel, Stats: cs}
			playData, playCfg, playErr = engine.CompilePlaySources(ess, opts)
			return playErr
		},
		&playDur,
	)
	go compileWithStats(&wg, "watch", stats,
		func() error {
			opts := &engine.CompileOptions{Opt: optLevel}
			watchData, watchErr = engine.CompileWatchSources(ess, opts)
			return watchErr
		},
		func(cs *engine.CompileStats) error {
			opts := &engine.CompileOptions{Opt: optLevel, Stats: cs}
			watchData, watchErr = engine.CompileWatchSources(ess, opts)
			return watchErr
		},
		&watchDur,
	)
	go compileWithStats(&wg, "preview", stats,
		func() error {
			opts := &engine.CompileOptions{Opt: optLevel}
			previewData, previewErr = engine.CompilePreviewSources(ess, opts)
			return previewErr
		},
		func(cs *engine.CompileStats) error {
			opts := &engine.CompileOptions{Opt: optLevel, Stats: cs}
			previewData, previewErr = engine.CompilePreviewSources(ess, opts)
			return previewErr
		},
		&previewDur,
	)
	go compileWithStats(&wg, "tutorial", stats,
		func() error {
			opts := &engine.CompileOptions{Opt: optLevel}
			tutorialData, tutorialErr = engine.CompileTutorialSources(ess, opts)
			return tutorialErr
		},
		func(cs *engine.CompileStats) error {
			opts := &engine.CompileOptions{Opt: optLevel, Stats: cs}
			tutorialData, tutorialErr = engine.CompileTutorialSources(ess, opts)
			return tutorialErr
		},
		&tutorialDur,
	)

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
	ess, engineName, err := resolveSourceArg(srcPath)
	if err != nil {
		return err
	}

	// 1. Compile all 4 modes.
	fmt.Printf("compiling %s...\n", engineName)
	c, err := compileAllModes(ess, false, optimize.LevelStandard)
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
		return fmt.Errorf("cleaning pack source dir %s: %w", sourceDir, err)
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
	if err := pack.EmitDefaultItems(sourceDir, engineName, meta); err != nil {
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
