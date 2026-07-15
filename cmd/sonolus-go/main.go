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
		fmt.Println(currentBuildMetadata())
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
		checks := flags.String("runtime-checks", "none", "runtime checks: none, terminate, notify")
		stats := flags.Bool("stats", false, "print compilation timing")
		if err := flags.Parse(args); err != nil {
			return err
		}
		return cmdBuild(flags.Args(), *out, *mode, *optimization, *rom, *checks, *stats)
	case "vet":
		flags := commandFlags(command)
		mode := flags.String("m", "all", "engine mode: play, watch, preview, tutorial, all")
		optimization := flags.Int("O", 2, "optimization level: 0=minimal, 1=fast, 2=standard")
		rom := flags.String("rom", "", "path to raw float32 ROM file (optional)")
		checks := flags.String("runtime-checks", "none", "runtime checks: none, terminate, notify")
		stats := flags.Bool("stats", false, "print compilation timing")
		if err := flags.Parse(args); err != nil {
			return err
		}
		return cmdVet(flags.Args(), *mode, *optimization, *rom, *checks, *stats)
	case "list":
		flags := commandFlags(command)
		if err := flags.Parse(args); err != nil {
			return err
		}
		return cmdList(flags.Args(), os.Stdout)
	case "dev":
		flags := commandFlags(command)
		out := flags.String("o", "", "development engine name")
		addr := flags.String("addr", ":8080", "development server listen address")
		optimization := flags.Int("O", 2, "optimization level: 0=minimal, 1=fast, 2=standard")
		rom := flags.String("rom", "", "path to raw float32 ROM file (optional)")
		checks := flags.String("runtime-checks", "notify", "runtime checks: none, terminate, notify")
		stats := flags.Bool("stats", false, "print compilation timing")
		if err := flags.Parse(args); err != nil {
			return err
		}
		return cmdDev(flags.Args(), *out, *addr, *optimization, *rom, *checks, *stats)
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
	fmt.Fprintln(os.Stderr, "usage: sonolus-go build [-o <name>] [-m <mode>] [-O 0|1|2] [-runtime-checks <level>] <package-pattern>...")
	fmt.Fprintln(os.Stderr, "       sonolus-go vet [-m <mode>] [-O 0|1|2] [-rom <file>] [-runtime-checks <level>] [-stats] <package-pattern>...")
	fmt.Fprintln(os.Stderr, "       sonolus-go list <package-pattern>...")
	fmt.Fprintln(os.Stderr, "       sonolus-go dev [-o <name>] [-addr <:8080>] [-O 0|1|2] [-rom <file>] [-runtime-checks <level>] <package-pattern>...")
	fmt.Fprintln(os.Stderr, "  build modes: play, watch, preview, tutorial, all (default)")
	fmt.Fprintln(os.Stderr, "  opt levels:  0=minimal, 1=fast, 2=standard (default)")
}

func fatalf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}
