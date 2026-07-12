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
		name := flags.String("name", "", "engine name (required for ambiguous package patterns)")
		out := flags.String("o", "dist", "output directory")
		mode := flags.String("m", "all", "engine mode: play, watch, preview, tutorial, all")
		optimization := flags.Int("O", 0, "optimization level: 0=minimal")
		rom := flags.String("rom", "", "path to raw float32 ROM file (optional)")
		stats := flags.Bool("stats", false, "print compilation timing")
		if err := flags.Parse(args); err != nil {
			return err
		}
		return cmdBuild(flags.Args(), *name, *out, *mode, *optimization, *rom, *stats)
	case "serve":
		flags := commandFlags(command)
		name := flags.String("name", "", "engine name (required for ambiguous package patterns)")
		addr := flags.String("addr", ":8080", "server listen address")
		rom := flags.String("rom", "", "path to raw float32 ROM file (optional)")
		stats := flags.Bool("stats", false, "print compilation timing")
		if err := flags.Parse(args); err != nil {
			return err
		}
		return cmdServe(flags.Args(), *name, *addr, *rom, *stats)
	case "pack":
		flags := commandFlags(command)
		name := flags.String("name", "", "engine name (required for ambiguous package patterns)")
		author := flags.String("author", "sonolus-go", "engine author")
		rom := flags.String("rom", "", "path to raw float32 ROM file (optional)")
		stats := flags.Bool("stats", false, "print compilation timing")
		if err := flags.Parse(args); err != nil {
			return err
		}
		return cmdPack(flags.Args(), *name, *author, *rom, *stats)
	case "host":
		flags := commandFlags(command)
		name := flags.String("name", "", "engine name (required for ambiguous package patterns)")
		addr := flags.String("addr", ":8080", "server listen address")
		author := flags.String("author", "sonolus-go", "engine author")
		rom := flags.String("rom", "", "path to raw float32 ROM file (optional)")
		stats := flags.Bool("stats", false, "print compilation timing")
		if err := flags.Parse(args); err != nil {
			return err
		}
		return cmdHost(flags.Args(), *name, *addr, *author, *rom, *stats)
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
	fmt.Fprintln(os.Stderr, "usage: sonolus-go build [-name <name>] [-o <out-dir>] [-m <mode>] [-O 0] <package-pattern>...")
	fmt.Fprintln(os.Stderr, "       sonolus-go serve [-name <name>] [-addr <:8080>] [-rom <file>] <package-pattern>...")
	fmt.Fprintln(os.Stderr, "       sonolus-go level [-o <out-dir>] <chart.json>")
	fmt.Fprintln(os.Stderr, "       sonolus-go pack  [-name <name>] [-author <name>] [-rom <file>] <package-pattern>...")
	fmt.Fprintln(os.Stderr, "       sonolus-go host  [-name <name>] [-addr <:8080>] [-author <name>] [-rom <file>] <package-pattern>...")
	fmt.Fprintln(os.Stderr, "  build modes: play, watch, preview, tutorial, all (default)")
	fmt.Fprintln(os.Stderr, "  opt levels:  0=minimal (Fast and Standard are not implemented)")
}

func fatalf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}
