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

var engineOutputRoot = "dist"

func cmdBuild(patterns []string, outputName, modeFlag string, optFlag int, romFlag string, statsFlag bool) error {
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
	targets, err := compiler.DiscoverTargets(selected.CompilerMode(), patterns...)
	if err != nil {
		return err
	}
	named, err := nameTargets(targets, outputName)
	if err != nil {
		return err
	}
	type result struct {
		name     string
		packaged *build.PackagedEngine
	}
	results := make([]result, 0, len(named))
	for _, target := range named {
		engineCompiler := compiler.NewCompiler(compiler.Options{Optimization: optimization, FallbackROM: fallback}, target.target.PackagePath)
		var artifacts *compiler.Artifacts
		if selected == ModeAll {
			artifacts, err = engineCompiler.CompileAll()
		} else {
			artifacts, err = engineCompiler.Compile(selected.CompilerMode())
		}
		if err != nil {
			return fmt.Errorf("compiling engine %q: %w", target.target.PackagePath, err)
		}
		if statsFlag {
			fmt.Printf("engine %s:\n", target.name)
			printCompileStats(engineCompiler.Stats())
		}
		packaged, err := build.PackageArtifacts(artifacts)
		if err != nil {
			return err
		}
		results = append(results, result{name: target.name, packaged: packaged})
	}
	for _, result := range results {
		dir := filepath.Join(engineOutputRoot, result.name)
		if err := result.packaged.WriteAtomic(dir); err != nil {
			return fmt.Errorf("writing engine package: %w", err)
		}
		if selected == ModeAll {
			fmt.Printf("wrote all (%s) engine to %s/\n", strings.Join(allModeNames(), ", "), dir)
		} else {
			fmt.Printf("wrote %s engine to %s/\n", selected, dir)
		}
	}
	return nil
}

func cmdServe(patterns []string, outputName, addr string, optimization int, romPath string, stats bool) error {
	return runDevServer(patterns, outputName, addr, optimization, romPath, stats)
}

func cmdPack(patterns []string, outputName, author string, optimization int, romPath string, stats bool) error {
	return runPack(patterns, outputName, author, optimization, romPath, stats)
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
