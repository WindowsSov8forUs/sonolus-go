package main

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/WindowsSov8forUs/sonolus-go/internal/build"
	"github.com/WindowsSov8forUs/sonolus-go/internal/newcompiler"
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
	level, err := parseOptLevel(optFlag)
	if err != nil {
		return err
	}
	fallback, err := readFallbackROM(romFlag)
	if err != nil {
		return err
	}
	compiler := newcompiler.NewCompiler(newcompiler.Options{Optimization: level, FallbackROM: fallback}, patterns...)
	var artifacts *newcompiler.Artifacts
	if selected == ModeAll {
		artifacts, err = compiler.CompileAll()
	} else {
		artifacts, err = compiler.Compile(selected.CompilerMode())
	}
	if err != nil {
		return fmt.Errorf("compiling: %w", err)
	}
	if statsFlag {
		printCompileStats(compiler.Stats())
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
