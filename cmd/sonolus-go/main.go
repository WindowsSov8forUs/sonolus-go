package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/WindowsSov8forUs/sonolus-core-go/core/resource"

	"github.com/WindowsSov8forUs/sonolus-go/compiler/build"
	"github.com/WindowsSov8forUs/sonolus-go/compiler/engine"
	"github.com/WindowsSov8forUs/sonolus-go/compiler/level"
)

func main() {
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "usage: sonolus-go build <engine.go> [-o <out-dir>] [-m <mode>]\n")
		fmt.Fprintf(os.Stderr, "       sonolus-go serve <engine.go>\n")
		fmt.Fprintf(os.Stderr, "       sonolus-go level <chart.json>\n")
		fmt.Fprintf(os.Stderr, "  modes: play (default), watch, preview, tutorial, all\n")
		flag.PrintDefaults()
	}
	outDir := flag.String("o", "dist", "output directory")
	mode := flag.String("m", "play", "engine mode: play, watch, preview, tutorial, all")
	flag.Parse()

	if flag.NArg() < 1 {
		flag.Usage()
		os.Exit(1)
	}

	if flag.Arg(0) == "serve" {
		srcPath := flag.Arg(1)
		if err := runDevServer(srcPath, 8080); err != nil {
			fatalf("%v", err)
		}
		return
	}

	if flag.Arg(0) == "level" {
		levelPath := flag.Arg(1)
		blob, err := level.PackageLevel(levelPath)
		if err != nil {
			fatalf("packaging level: %v", err)
		}
		dir := filepath.Join(*outDir, filepath.Base(levelPath[:len(levelPath)-len(filepath.Ext(levelPath))]))
		os.MkdirAll(dir, 0o755)
		os.WriteFile(filepath.Join(dir, level.FileName), blob, 0o644)
		fmt.Printf("wrote level to %s/\n", dir)
		return
	}

	if flag.Arg(0) != "build" {
		flag.Usage()
		os.Exit(1)
	}

	srcPath := flag.Arg(1)
	src, err := os.ReadFile(srcPath)
	if err != nil {
		fatalf("reading %s: %v", srcPath, err)
	}

	name := filepath.Base(srcPath[:len(srcPath)-len(filepath.Ext(srcPath))])
	dir := filepath.Join(*outDir, name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		fatalf("creating %s: %v", dir, err)
	}

	// Config and ROM are engine-global, extracted from Play compile.
	var cfg *resource.EngineConfiguration
	playData, playCfg, err := engine.CompilePlayFile(string(src))
	if err == nil {
		cfg = playCfg
	} else {
		cfg = &resource.EngineConfiguration{}
	}
	rom, err := build.BuildRom(build.DefaultRom())
	if err != nil {
		fatalf("building ROM: %v", err)
	}

	modes := []string{*mode}
	if *mode == "all" {
		modes = []string{"play", "watch", "preview", "tutorial"}
	}

	for _, m := range modes {
		switch m {
		case "play":
			if playData == nil {
				fatalf("play compilation failed")
			}
			pkg, err := build.PackagePlay(cfg, playData, rom)
			if err != nil {
				fatalf("packaging play: %v", err)
			}
			if err := pkg.Write(dir); err != nil {
				fatalf("writing: %v", err)
			}
		case "watch":
			wd, err := engine.CompileWatchFile(string(src))
			if err != nil {
				fatalf("compiling watch: %v", err)
			}
			writeAll(dir, cfg, rom, map[string]any{build.FileWatchData: wd})
		case "preview":
			pd, err := engine.CompilePreviewFile(string(src))
			if err != nil {
				fatalf("compiling preview: %v", err)
			}
			writeAll(dir, cfg, rom, map[string]any{build.FilePreviewData: pd})
		case "tutorial":
			td, err := engine.CompileTutorialFile(string(src))
			if err != nil {
				fatalf("compiling tutorial: %v", err)
			}
			writeAll(dir, cfg, rom, map[string]any{build.FileTutorialData: td})
		}
	}

	fmt.Printf("wrote %s engine to %s/\n", *mode, dir)
}

func writeAll(dir string, cfg *resource.EngineConfiguration, rom []byte, dataFiles map[string]any) error {
	configBlob, err := build.PackageConfiguration(cfg)
	if err != nil {
		return fmt.Errorf("packaging config: %w", err)
	}
	if err := os.WriteFile(filepath.Join(dir, build.FileConfiguration), configBlob, 0o644); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(dir, build.FileRom), rom, 0o644); err != nil {
		return err
	}
	for name, data := range dataFiles {
		blob, err := build.PackageAny(data)
		if err != nil {
			return fmt.Errorf("packaging %s: %w", name, err)
		}
		if err := os.WriteFile(filepath.Join(dir, name), blob, 0o644); err != nil {
			return err
		}
	}
	return nil
}

func fatalf(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}
