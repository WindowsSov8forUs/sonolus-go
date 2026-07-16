package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/WindowsSov8forUs/sonolus-go/v2/godori/internal/leveldata"
)

func main() {
	output := flag.String("o", "dev-level.json", "output LevelData path")
	flag.Parse()
	if flag.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "usage: go run ./levelgen [-o dev-level.json]")
		os.Exit(2)
	}
	data, err := leveldata.Marshal()
	if err != nil {
		fmt.Fprintf(os.Stderr, "build Godori development level: %v\n", err)
		os.Exit(1)
	}
	if err := os.WriteFile(*output, data, 0o644); err != nil {
		fmt.Fprintf(os.Stderr, "write Godori development level: %v\n", err)
		os.Exit(1)
	}
}
