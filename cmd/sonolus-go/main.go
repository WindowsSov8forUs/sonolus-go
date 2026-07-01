// sonolus-go is the Go implementation of the sonolus engine compiler.
// It compiles engine source files into Sonolus engine data (Play, Watch,
// Preview, Tutorial modes), packages them for distribution, and serves them.
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/WindowsSov8forUs/sonolus-core-go/core/resource"

	"github.com/WindowsSov8forUs/sonolus-go/compiler/build"
	"github.com/WindowsSov8forUs/sonolus-go/compiler/engine"
	"github.com/WindowsSov8forUs/sonolus-go/compiler/level"
)

// Build metadata — populated by ldflags at build time.
var (
	version = "dev"
	commit  = "unknown"
	date    = "unknown"
)

func main() {
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "usage: sonolus-go build <engine.go> [-o <out-dir>] [-m <mode>] [-O <level>]\n")
		fmt.Fprintf(os.Stderr, "       sonolus-go serve <engine.go>\n")
		fmt.Fprintf(os.Stderr, "       sonolus-go level <chart.json>\n")
		fmt.Fprintf(os.Stderr, "       sonolus-go pack  <engine.go> [-author <name>]\n")
		fmt.Fprintf(os.Stderr, "       sonolus-go host  <engine.go> [-addr <:8080>]\n")
		fmt.Fprintf(os.Stderr, "  build modes: play (default), watch, preview, tutorial, all\n")
		fmt.Fprintf(os.Stderr, "  opt levels:  0=minimal, 1=fast, 2=standard (default)\n")
		flag.PrintDefaults()
	}
	versionFlag := flag.Bool("version", false, "print version and exit")
	statsFlag := flag.Bool("stats", false, "print per-mode compilation timing")
	outDir := flag.String("o", "dist", "output directory")
	modeFlag := flag.String("m", "play", "engine mode: play, watch, preview, tutorial, all")
	optFlag := flag.Int("O", 2, "optimization level: 0=minimal, 1=fast, 2=standard")
	romFlag := flag.String("rom", "", "path to raw float32 ROM file (optional)")
	authorFlag := flag.String("author", "sonolus-go", "engine author (for pack)")
	addrFlag := flag.String("addr", ":8080", "server listen address (for host)")
	flag.Parse()

	if *versionFlag {
		fmt.Printf("sonolus-go %s (commit %s, built %s)\n", version, commit, date)
		return
	}

	if flag.NArg() < 1 {
		flag.Usage()
		os.Exit(1)
	}

	// Dev server (in-process, with auto-recompile).
	if flag.Arg(0) == "serve" {
		srcPath := flag.Arg(1)
		if err := runDevServer(srcPath, *addrFlag); err != nil {
			fatalf("%v", err)
		}
		return
	}

	// Level packing.
	if flag.Arg(0) == "level" {
		levelPath := flag.Arg(1)
		blob, err := level.PackageLevel(levelPath)
		if err != nil {
			fatalf("packaging level: %v", err)
		}
		dir := filepath.Join(*outDir, engineNameFromPath(levelPath))
		if err := os.MkdirAll(dir, 0o755); err != nil {
			fatalf("creating %s: %v", dir, err)
		}
		if err := os.WriteFile(filepath.Join(dir, level.FileName), blob, 0o644); err != nil {
			fatalf("writing level: %v", err)
		}
		fmt.Printf("wrote level to %s/\n", dir)
		return
	}

	// Pack: compile → emit pack source → run packer.Pack.
	if flag.Arg(0) == "pack" {
		srcPath := flag.Arg(1)
		if err := runPack(srcPath, *authorFlag); err != nil {
			fatalf("%v", err)
		}
		return
	}

	// Host: pack + production server via sonolus-server-go.
	if flag.Arg(0) == "host" {
		srcPath := flag.Arg(1)
		if err := runPackServe(srcPath, *addrFlag); err != nil {
			fatalf("%v", err)
		}
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

	name := engineNameFromPath(srcPath)
	dir := filepath.Join(*outDir, name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		fatalf("creating %s: %v", dir, err)
	}

	// Parse mode early so we can skip redundant Play compilation for -m all.
	mode, err := ParseMode(*modeFlag)
	if err != nil {
		fatalf("%v", err)
	}

	optLevel, err := parseOptLevel(*optFlag)
	if err != nil {
		fatalf("%v", err)
	}

	// Config and ROM are engine-global, extracted from Play compile.
	// For -m all, skip standalone Play compilation — compileAllModes provides it.
	cfg := &resource.EngineConfiguration{}
	var playData *resource.EnginePlayData
	var playErr error
	if mode != ModeAll {
		playData, cfg, playErr = engine.CompilePlayFileWithStats(string(src), buildOpts(false, nil, optLevel))
	}
	// If play compilation failed, cfg stays as default empty config.
	// Failure is deferred until we know whether play mode is in the target set.
	// Load ROM: from --rom flag if specified, otherwise use defaults.
	var rom []byte
	if *romFlag != "" {
		var err error
		rom, err = build.BuildROMFromFile(*romFlag)
		if err != nil {
			fatalf("loading ROM: %v", err)
		}
	} else {
		var err error
		rom, err = build.DefaultROMBytes()
		if err != nil {
			fatalf("building ROM: %v", err)
		}
	}

	modes := mode.Expand()

	// If play compilation failed but play mode is not in the target set,
	// log a warning so the author knows the config may be degraded.
	if playErr != nil {
		needsPlay := false
		for _, m := range modes {
			if m == ModePlay {
				needsPlay = true
				break
			}
		}
		if !needsPlay {
			fmt.Fprintf(os.Stderr, "warning: play compilation failed (config will be empty): %v\n", playErr)
		}
	}

	// Compile and package each requested mode.
	if mode == ModeAll {
		c, err := compileAllModes(string(src), *statsFlag, optLevel)
		if err != nil {
			fatalf("compiling: %v", err)
		}
		cfg = &c.Configuration
		packageAndWritePlay(dir, cfg, &c.PlayData, rom)
		packageAndWriteNonPlay(dir, cfg, rom, c.WatchData, build.FileWatchData, "watch")
		packageAndWriteNonPlay(dir, cfg, rom, c.PreviewData, build.FilePreviewData, "preview")
		packageAndWriteNonPlay(dir, cfg, rom, c.TutorialData, build.FileTutorialData, "tutorial")
	} else {
		for _, m := range modes {
			switch m {
			case ModePlay:
				if playData == nil {
					fatalf("compiling play: %v", playErr)
				}
				packageAndWritePlay(dir, cfg, playData, rom)
			case ModeWatch:
				data, err := engine.CompileWatchFileWithStats(string(src), buildOpts(false, nil, optLevel))
				if err != nil {
					fatalf("compiling watch: %v", err)
				}
				packageAndWriteNonPlay(dir, cfg, rom, *data, build.FileWatchData, "watch")
			case ModePreview:
				data, err := engine.CompilePreviewFileWithStats(string(src), buildOpts(false, nil, optLevel))
				if err != nil {
					fatalf("compiling preview: %v", err)
				}
				packageAndWriteNonPlay(dir, cfg, rom, *data, build.FilePreviewData, "preview")
			case ModeTutorial:
				data, err := engine.CompileTutorialFileWithStats(string(src), buildOpts(false, nil, optLevel))
				if err != nil {
					fatalf("compiling tutorial: %v", err)
				}
				packageAndWriteNonPlay(dir, cfg, rom, *data, build.FileTutorialData, "tutorial")
			}
		}
	}

	if mode == ModeAll {
		fmt.Printf("wrote all (%s) engine to %s/\n", strings.Join(allModeNames(), ", "), dir)
	} else {
		fmt.Printf("wrote %s engine to %s/\n", mode, dir)
	}
}

func packageAndWritePlay(dir string, cfg *resource.EngineConfiguration,
	data *resource.EnginePlayData, rom []byte) {
	pkg, err := build.PackagePlay(cfg, data, rom)
	if err != nil {
		fatalf("packaging play: %v", err)
	}
	if err := pkg.Write(dir); err != nil {
		fatalf("writing play: %v", err)
	}
}

func packageAndWriteNonPlay[D any](dir string, cfg *resource.EngineConfiguration,
	rom []byte, data D, fileKind string, name string) {
	pkg, err := build.PackageNonPlay(cfg, rom, &data, fileKind)
	if err != nil {
		fatalf("packaging %s: %v", name, err)
	}
	if err := pkg.Write(dir); err != nil {
		fatalf("writing %s: %v", name, err)
	}
}

func fatalf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}
