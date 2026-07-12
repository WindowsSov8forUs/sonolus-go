package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/WindowsSov8forUs/sonolus-pack-go/packer"

	"github.com/WindowsSov8forUs/sonolus-go/internal/newcompiler"
	"github.com/WindowsSov8forUs/sonolus-go/internal/newcompiler/optimize"
	"github.com/WindowsSov8forUs/sonolus-go/internal/pack"
)

func compileAllModes(patterns []string, fallback []byte, stats bool) (*newcompiler.Artifacts, *newcompiler.Compiler, error) {
	compiler := newcompiler.NewCompiler(newcompiler.Options{Optimization: optimize.LevelMinimal, FallbackROM: fallback}, patterns...)
	artifacts, err := compiler.CompileAll()
	if stats {
		printCompileStats(compiler.Stats())
	}
	return artifacts, compiler, err
}

func runPack(patterns []string, explicitName, author, romPath string, stats bool) error {
	engineName, err := resolveEngineName(patterns, explicitName)
	if err != nil {
		return err
	}
	fallback, err := readFallbackROM(romPath)
	if err != nil {
		return err
	}
	fmt.Printf("compiling %s...\n", engineName)
	artifacts, _, err := compileAllModes(patterns, fallback, stats)
	if err != nil {
		return err
	}

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

	packDir := filepath.Join("dist", engineName+"-pack")
	fmt.Printf("packing to %s...\n", packDir)
	if err := packer.Pack(context.Background(), packer.Options{Input: sourceDir, Output: packDir}); err != nil {
		return fmt.Errorf("pack: %w", err)
	}
	fmt.Printf("packed %s to %s/\n", engineName, packDir)
	return nil
}
