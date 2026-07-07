// sonolus-go is the Go implementation of the sonolus engine compiler.
// It compiles engine source files into Sonolus engine data (Play, Watch,
// Preview, Tutorial modes), packages them for distribution, and serves them.
package main

import (
	"flag"
	"fmt"
	"os"
)

// Build metadata — populated by ldflags at build time.
var (
	version = "dev"
	commit  = "unknown"
	date    = "unknown"
)

func main() {
	flag.Usage = usage
	versionFlag := flag.Bool("version", false, "print version and exit")
	statsFlag := flag.Bool("stats", false, "print per-mode compilation timing")
	outDir := flag.String("o", "dist", "output directory")
	modeFlag := flag.String("m", "all", "engine mode: play, watch, preview, tutorial, all")
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

	cmd := flag.Arg(0)
	srcPath := flag.Arg(1)

	switch cmd {
	case "serve":
		if err := cmdServe(srcPath, *addrFlag, *romFlag); err != nil {
			fatalf("%v", err)
		}
	case "level":
		if err := cmdLevel(srcPath, *outDir); err != nil {
			fatalf("%v", err)
		}
	case "pack":
		if err := cmdPack(srcPath, *authorFlag); err != nil {
			fatalf("%v", err)
		}
	case "host":
		if err := cmdHost(srcPath, *addrFlag, *authorFlag); err != nil {
			fatalf("%v", err)
		}
	case "build":
		if err := cmdBuild(srcPath, *outDir, *modeFlag, *optFlag, *romFlag, *statsFlag); err != nil {
			fatalf("%v", err)
		}
	default:
		flag.Usage()
		os.Exit(1)
	}
}

func usage() {
	fmt.Fprintf(os.Stderr, "usage: sonolus-go build <engine.go> [-o <out-dir>] [-m <mode>] [-O <level>]\n")
	fmt.Fprintf(os.Stderr, "       sonolus-go serve <engine.go> [-rom <file>]\n")
	fmt.Fprintf(os.Stderr, "       sonolus-go level <chart.json>\n")
	fmt.Fprintf(os.Stderr, "       sonolus-go pack  <engine.go> [-author <name>] [-rom <file>]\n")
	fmt.Fprintf(os.Stderr, "       sonolus-go host  <engine.go> [-addr <:8080>] [-author <name>] [-rom <file>]\n")
	fmt.Fprintf(os.Stderr, "  build modes: play (default), watch, preview, tutorial, all\n")
	fmt.Fprintf(os.Stderr, "  opt levels:  0=minimal, 1=fast, 2=standard (default)\n")
	flag.PrintDefaults()
}

func fatalf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}
