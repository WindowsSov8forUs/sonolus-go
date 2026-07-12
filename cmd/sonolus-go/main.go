// sonolus-go is the Go implementation of the Sonolus engine compiler.
package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
)

var (
	version = "dev"
	commit  = "unknown"
	date    = "unknown"
)

func main() {
	if err := runCLI(os.Args[1:]); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return
		}
		fatalf("%v", err)
	}
}

func runCLI(args []string) error {
	if len(args) == 0 {
		usage()
		return fmt.Errorf("command is required")
	}
	if args[0] == "-version" || args[0] == "--version" || args[0] == "version" {
		fmt.Printf("sonolus-go %s (commit %s, built %s)\n", version, commit, date)
		return nil
	}

	command, args := args[0], args[1:]
	switch command {
	case "build":
		flags := commandFlags(command)
		out := flags.String("o", "", "output engine name (requires exactly one engine)")
		mode := flags.String("m", "all", "engine mode: play, watch, preview, tutorial, all")
		optimization := flags.Int("O", 2, "optimization level: 0=minimal, 1=fast, 2=standard")
		rom := flags.String("rom", "", "path to raw float32 ROM file (optional)")
		stats := flags.Bool("stats", false, "print compilation timing")
		if err := flags.Parse(args); err != nil {
			return err
		}
		return cmdBuild(flags.Args(), *out, *mode, *optimization, *rom, *stats)
	case "serve":
		flags := commandFlags(command)
		out := flags.String("o", "", "served engine name")
		addr := flags.String("addr", ":8080", "server listen address")
		optimization := flags.Int("O", 2, "optimization level: 0=minimal, 1=fast, 2=standard")
		rom := flags.String("rom", "", "path to raw float32 ROM file (optional)")
		stats := flags.Bool("stats", false, "print compilation timing")
		if err := flags.Parse(args); err != nil {
			return err
		}
		return cmdServe(flags.Args(), *out, *addr, *optimization, *rom, *stats)
	case "pack":
		flags := commandFlags(command)
		author := flags.String("author", "sonolus-go", "engine author")
		out := flags.String("o", "", "output engine name (requires exactly one engine)")
		optimization := flags.Int("O", 2, "optimization level: 0=minimal, 1=fast, 2=standard")
		rom := flags.String("rom", "", "path to raw float32 ROM file (optional)")
		stats := flags.Bool("stats", false, "print compilation timing")
		if err := flags.Parse(args); err != nil {
			return err
		}
		return cmdPack(flags.Args(), *out, *author, *optimization, *rom, *stats)
	case "level":
		flags := commandFlags(command)
		out := flags.String("o", "dist", "output directory")
		if err := flags.Parse(args); err != nil {
			return err
		}
		if flags.NArg() != 1 {
			return fmt.Errorf("level requires exactly one chart path")
		}
		return cmdLevel(flags.Arg(0), *out)
	default:
		usage()
		return fmt.Errorf("unknown command %q", command)
	}
}

func commandFlags(command string) *flag.FlagSet {
	flags := flag.NewFlagSet("sonolus-go "+command, flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	return flags
}

func usage() {
	fmt.Fprintln(os.Stderr, "usage: sonolus-go build [-o <name>] [-m <mode>] [-O 0|1|2] <package-pattern>...")
	fmt.Fprintln(os.Stderr, "       sonolus-go serve [-o <name>] [-addr <:8080>] [-O 0|1|2] [-rom <file>] <package-pattern>...")
	fmt.Fprintln(os.Stderr, "       sonolus-go level [-o <out-dir>] <chart.json>")
	fmt.Fprintln(os.Stderr, "       sonolus-go pack  [-o <name>] [-author <name>] [-O 0|1|2] [-rom <file>] <package-pattern>...")
	fmt.Fprintln(os.Stderr, "  build modes: play, watch, preview, tutorial, all (default)")
	fmt.Fprintln(os.Stderr, "  opt levels:  0=minimal, 1=fast, 2=standard (default)")
}

func fatalf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}
