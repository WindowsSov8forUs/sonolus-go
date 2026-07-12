package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/WindowsSov8forUs/sonolus-go/internal/build"
	"github.com/WindowsSov8forUs/sonolus-go/internal/compiler"
	"github.com/WindowsSov8forUs/sonolus-go/internal/level"
)

func cmdBuild(patterns []string, explicitName, outDir, modeFlag string, optFlag int, romFlag string, statsFlag bool) error {
	name, err := resolveEngineName(patterns, explicitName)
	if err != nil {
		return err
	}
	selected, err := ParseMode(modeFlag)
	if err != nil {
		return err
	}
	optimization, err := parseOptLevel(optFlag)
	if err != nil {
		return err
	}
	fallback, err := readFallbackROM(romFlag)
	if err != nil {
		return err
	}
	engineCompiler := compiler.NewCompiler(compiler.Options{Optimization: optimization, FallbackROM: fallback}, patterns...)
	var artifacts *compiler.Artifacts
	if selected == ModeAll {
		artifacts, err = engineCompiler.CompileAll()
	} else {
		artifacts, err = engineCompiler.Compile(selected.CompilerMode())
	}
	if err != nil {
		return fmt.Errorf("compiling: %w", err)
	}
	if statsFlag {
		printCompileStats(engineCompiler.Stats())
	}
	packaged, err := build.PackageArtifacts(artifacts)
	if err != nil {
		return err
	}
	dir := filepath.Join(outDir, name)
	if err := packaged.WriteAtomic(dir); err != nil {
		return fmt.Errorf("writing engine package: %w", err)
	}
	if selected == ModeAll {
		fmt.Printf("wrote all (%s) engine to %s/\n", strings.Join(allModeNames(), ", "), dir)
	} else {
		fmt.Printf("wrote %s engine to %s/\n", selected, dir)
	}
	return nil
}

func cmdServe(patterns []string, name, addr string, optimization int, romPath string, stats bool) error {
	return runDevServer(patterns, name, addr, optimization, romPath, stats)
}

func cmdPack(patterns []string, name, author string, optimization int, romPath string, stats bool) error {
	return runPack(patterns, name, author, optimization, romPath, stats)
}

func cmdLevel(levelPath, outDir string) error {
	blob, err := level.PackageLevel(levelPath)
	if err != nil {
		return fmt.Errorf("packaging level: %w", err)
	}
	dir := filepath.Join(outDir, engineNameFromPath(levelPath))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("creating %s: %w", dir, err)
	}
	if err := os.WriteFile(filepath.Join(dir, level.FileName), blob, 0o644); err != nil {
		return fmt.Errorf("writing level: %w", err)
	}
	fmt.Printf("wrote level to %s/\n", dir)
	return nil
}
