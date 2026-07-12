package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/WindowsSov8forUs/sonolus-pack-go/packer"

	"github.com/WindowsSov8forUs/sonolus-go/internal/compiler"
	"github.com/WindowsSov8forUs/sonolus-go/internal/pack"
)

func compileAllModes(patterns []string, fallback []byte, level compiler.OptimizationLevel, stats bool) (*compiler.Artifacts, *compiler.Compiler, error) {
	engineCompiler := compiler.NewCompiler(compiler.Options{Optimization: level, FallbackROM: fallback}, patterns...)
	artifacts, err := engineCompiler.CompileAll()
	if stats {
		printCompileStats(engineCompiler.Stats())
	}
	return artifacts, engineCompiler, err
}

func runPack(patterns []string, outputName, author string, optimization int, romPath string, stats bool) error {
	targets, err := compiler.DiscoverTargets(compiler.ModePlay, patterns...)
	if err != nil {
		return err
	}
	named, err := nameTargets(targets, outputName)
	if err != nil {
		return err
	}
	fallback, err := readFallbackROM(romPath)
	if err != nil {
		return err
	}
	level, err := parseOptLevel(optimization)
	if err != nil {
		return err
	}
	for _, target := range named {
		fmt.Printf("compiling %s...\n", target.name)
		artifacts, _, err := compileAllModes([]string{target.target.PackagePath}, fallback, level, stats)
		if err != nil {
			return fmt.Errorf("compiling engine %q: %w", target.target.PackagePath, err)
		}
		if err := packTarget(target.name, author, artifacts); err != nil {
			return err
		}
	}
	return nil
}

func packTarget(engineName, author string, artifacts *compiler.Artifacts) error {
	tempRoot := filepath.Join(os.TempDir(), "sonolus-pack-source")
	if err := os.MkdirAll(tempRoot, 0o755); err != nil {
		return fmt.Errorf("creating pack temporary root: %w", err)
	}
	sourceDir, err := os.MkdirTemp(tempRoot, engineName+"-")
	if err != nil {
		return fmt.Errorf("creating pack source dir: %w", err)
	}
	defer os.RemoveAll(sourceDir)
	meta := pack.EngineItemMeta{
		Title: engineName, Author: author,
		Skin: "default", Background: "default", Effect: "default", Particle: "default",
	}
	if err := pack.EmitArtifactsSource(sourceDir, engineName, artifacts, meta); err != nil {
		return fmt.Errorf("emitting source tree: %w", err)
	}
	if err := pack.EmitDefaultItems(sourceDir, engineName, meta); err != nil {
		return fmt.Errorf("emitting default items: %w", err)
	}
	packDir := filepath.Join(engineOutputRoot, engineName)
	fmt.Printf("packing to %s...\n", packDir)
	if err := packer.Pack(context.Background(), packer.Options{Input: sourceDir, Output: packDir}); err != nil {
		return fmt.Errorf("pack: %w", err)
	}
	fmt.Printf("packed %s to %s/\n", engineName, packDir)
	return nil
}
