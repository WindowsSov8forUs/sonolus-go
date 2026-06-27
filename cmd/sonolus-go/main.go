package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/WindowsSov8forUs/sonolus-go/compiler/build"
	"github.com/WindowsSov8forUs/sonolus-go/compiler/engine"
)

func main() {
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "usage: sonolus-go build <engine.go> [-o <out-dir>] [-m <mode>]\n")
		fmt.Fprintf(os.Stderr, "  modes: play (default), watch, preview, tutorial\n")
		flag.PrintDefaults()
	}
	outDir := flag.String("o", "dist", "output directory")
	mode := flag.String("m", "play", "engine mode: play, watch, preview, tutorial")
	flag.Parse()

	if flag.NArg() < 1 || flag.Arg(0) != "build" {
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

	switch *mode {
	case "play":
		data, cfg, err := engine.CompilePlayFile(string(src))
		if err != nil {
			fatalf("compiling: %v", err)
		}
		rom, err := build.BuildRom(build.DefaultRom())
		if err != nil {
			fatalf("building ROM: %v", err)
		}
		pkg, err := build.PackagePlay(cfg, data, rom)
		if err != nil {
			fatalf("packaging: %v", err)
		}
		if err := pkg.Write(dir); err != nil {
			fatalf("writing: %v", err)
		}
	case "watch":
		data, err := engine.CompileWatchFile(string(src))
		if err != nil {
			fatalf("compiling: %v", err)
		}
		writeGzip(dir, build.FileWatchData, data)
	case "preview":
		data, err := engine.CompilePreviewFile(string(src))
		if err != nil {
			fatalf("compiling: %v", err)
		}
		writeGzip(dir, build.FilePreviewData, data)
	case "tutorial":
		data, err := engine.CompileTutorialFile(string(src))
		if err != nil {
			fatalf("compiling: %v", err)
		}
		writeGzip(dir, build.FileTutorialData, data)
	default:
		fatalf("unknown mode: %s", *mode)
	}

	fmt.Printf("wrote %s engine to %s/\n", *mode, dir)
}

func writeGzip(dir, name string, data any) error {
	blob, err := build.PackageAny(data)
	if err != nil {
		return fmt.Errorf("packaging: %w", err)
	}
	return os.WriteFile(filepath.Join(dir, name), blob, 0o644)
}

func fatalf(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}
