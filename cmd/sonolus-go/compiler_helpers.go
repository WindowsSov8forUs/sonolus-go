package main

import (
	"fmt"
	"os"
	"time"

	"github.com/WindowsSov8forUs/sonolus-go/internal/compiler"
)

func readFallbackROM(path string) ([]byte, error) {
	if path == "" {
		return nil, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("loading fallback ROM %q: %w", path, err)
	}
	if len(data)%4 != 0 {
		return nil, fmt.Errorf("loading fallback ROM %q: length %d is not divisible by 4", path, len(data))
	}
	return data, nil
}

func printCompileStats(stats compiler.CompileStats) {
	fmt.Fprintln(os.Stderr, "Compilation stats:")
	for _, m := range []compiler.Mode{compiler.ModePlay, compiler.ModeWatch, compiler.ModePreview, compiler.ModeTutorial} {
		value, ok := stats.Modes[m]
		if !ok {
			continue
		}
		fmt.Fprintf(os.Stderr, "  %-8s load=%s frontend=%s\n", m, value.Load.Round(time.Millisecond), value.Frontend.Round(time.Millisecond))
	}
	fmt.Fprintf(os.Stderr, "  shared   load=%s frontend=%s optimize=%s backend=%s total=%s cached=%t\n",
		stats.Load.Round(time.Millisecond), stats.Frontend.Round(time.Millisecond), stats.Optimize.Round(time.Millisecond),
		stats.Backend.Round(time.Millisecond), stats.Total.Round(time.Millisecond), stats.Cached)
}
