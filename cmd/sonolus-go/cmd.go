package main

import (
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"github.com/WindowsSov8forUs/sonolus-go/v2/internal/build"
	"github.com/WindowsSov8forUs/sonolus-go/v2/internal/compiler"
)

var engineOutputRoot = "dist"

type compilationRequest struct {
	mode         Mode
	optimization compiler.OptimizationLevel
	fallbackROM  []byte
	targets      []compiler.Target
}

func prepareCompilation(patterns []string, modeFlag string, optFlag int, romFlag string) (*compilationRequest, error) {
	selected, err := ParseMode(modeFlag)
	if err != nil {
		return nil, err
	}
	optimization, err := parseOptLevel(optFlag)
	if err != nil {
		return nil, err
	}
	fallback, err := readFallbackROM(romFlag)
	if err != nil {
		return nil, err
	}
	targets, err := compiler.DiscoverTargets(selected.CompilerMode(), patterns...)
	if err != nil {
		return nil, err
	}
	return &compilationRequest{mode: selected, optimization: optimization, fallbackROM: fallback, targets: targets}, nil
}

func compileTarget(request *compilationRequest, target compiler.Target) (*compiler.Compiler, *compiler.Artifacts, error) {
	engineCompiler := compiler.NewCompiler(compiler.Options{Optimization: request.optimization, FallbackROM: request.fallbackROM}, target.PackagePath)
	var (
		artifacts *compiler.Artifacts
		err       error
	)
	if request.mode == ModeAll {
		artifacts, err = engineCompiler.CompileAll()
	} else {
		artifacts, err = engineCompiler.Compile(request.mode.CompilerMode())
	}
	if err != nil {
		return nil, nil, fmt.Errorf("compiling engine %q: %w", target.PackagePath, err)
	}
	return engineCompiler, artifacts, nil
}

func cmdBuild(patterns []string, outputName, modeFlag string, optFlag int, romFlag string, statsFlag bool) error {
	request, err := prepareCompilation(patterns, modeFlag, optFlag, romFlag)
	if err != nil {
		return err
	}
	named, err := nameTargets(request.targets, outputName)
	if err != nil {
		return err
	}
	type result struct {
		name     string
		packaged *build.PackagedEngine
	}
	results := make([]result, 0, len(named))
	for _, target := range named {
		engineCompiler, artifacts, err := compileTarget(request, target.target)
		if err != nil {
			return err
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
		if request.mode == ModeAll {
			fmt.Printf("wrote all (%s) engine to %s/\n", strings.Join(allModeNames(), ", "), dir)
		} else {
			fmt.Printf("wrote %s engine to %s/\n", request.mode, dir)
		}
	}
	return nil
}

func cmdVet(patterns []string, modeFlag string, optFlag int, romFlag string, statsFlag bool) error {
	request, err := prepareCompilation(patterns, modeFlag, optFlag, romFlag)
	if err != nil {
		return err
	}
	for _, target := range request.targets {
		engineCompiler, _, err := compileTarget(request, target)
		if err != nil {
			return err
		}
		if statsFlag {
			fmt.Printf("engine %s:\n", target.PackagePath)
			printCompileStats(engineCompiler.Stats())
		}
	}
	modeName := request.mode.String()
	if request.mode == ModeAll {
		modeName = fmt.Sprintf("all (%s)", strings.Join(allModeNames(), ", "))
	}
	noun := "engines"
	if len(request.targets) == 1 {
		noun = "engine"
	}
	fmt.Printf("vetted %d %s for %s\n", len(request.targets), noun, modeName)
	return nil
}

func cmdList(patterns []string, out io.Writer) error {
	targets, err := compiler.DiscoverTargets(compiler.ModePlay, patterns...)
	if err != nil {
		return err
	}
	if len(targets) != 1 {
		return fmt.Errorf("list requires exactly one engine, but package patterns matched %d", len(targets))
	}
	target := targets[0]
	engineCompiler := compiler.NewCompiler(compiler.Options{}, target.PackagePath)
	projectSchema, err := engineCompiler.Schema()
	if err != nil {
		return fmt.Errorf("listing schema for engine %q: %w", target.PackagePath, err)
	}
	encoder := json.NewEncoder(out)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(projectSchema); err != nil {
		return fmt.Errorf("encoding listed schema for engine %q: %w", target.PackagePath, err)
	}
	return nil
}

func cmdDev(patterns []string, outputName, addr string, optimization int, romPath string, stats bool) error {
	return runDevServer(patterns, outputName, addr, optimization, romPath, stats)
}
