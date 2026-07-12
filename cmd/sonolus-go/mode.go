package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/WindowsSov8forUs/sonolus-go/internal/newcompiler/mode"
	"github.com/WindowsSov8forUs/sonolus-go/internal/newcompiler/optimize"
)

// errUnknownMode is returned when the user supplies an unrecognized mode string.
var errUnknownMode = errors.New("unknown mode")

// Mode is a Sonolus engine compilation mode.
type Mode string

const (
	ModePlay     Mode = "play"
	ModeWatch    Mode = "watch"
	ModePreview  Mode = "preview"
	ModeTutorial Mode = "tutorial"
	ModeAll      Mode = "all"
)

// allModes lists every individual (non-all) mode in canonical order.
var allModes = []Mode{ModePlay, ModeWatch, ModePreview, ModeTutorial}

// ParseMode converts a user-supplied mode string to a Mode.
func ParseMode(s string) (Mode, error) {
	switch s {
	case "play", "watch", "preview", "tutorial", "all":
		return Mode(s), nil
	}
	return "", fmt.Errorf("%w: %s (valid: play, watch, preview, tutorial, all)", errUnknownMode, s)
}

// Expand returns a slice of individual modes. ModeAll expands to all four; every
// other mode returns a single-element slice. The returned slice is always a fresh
// copy — mutating it does not affect the package-level allModes.
func (m Mode) Expand() []Mode {
	if m == ModeAll {
		out := make([]Mode, len(allModes))
		copy(out, allModes)
		return out
	}
	return []Mode{m}
}

// allModeNames returns the string names of all individual modes, for display.
func allModeNames() []string {
	names := make([]string, len(allModes))
	for i, m := range allModes {
		names[i] = m.String()
	}
	return names
}

// String implements fmt.Stringer.
func (m Mode) String() string { return string(m) }

// IRMode converts a main.Mode to the compiler's ir.Mode representation.
// ModeAll and unknown modes default to ir.ModePlay.
func (m Mode) CompilerMode() mode.Mode {
	switch m {
	case ModePlay:
		return mode.ModePlay
	case ModeWatch:
		return mode.ModeWatch
	case ModePreview:
		return mode.ModePreview
	case ModeTutorial:
		return mode.ModeTutorial
	default:
		return mode.ModePlay
	}
}

// engineNameFromPath extracts the engine name from an engine source file path
// or directory path. For files, it strips the extension; for directories, it
// uses the directory base name.
// e.g. "engines/my-engine.go" → "my-engine", "engines/my-engine/" → "my-engine"
func engineNameFromPath(srcPath string) string {
	if absolute, err := filepath.Abs(srcPath); err == nil {
		srcPath = absolute
	}
	base := filepath.Base(srcPath)
	ext := filepath.Ext(base)
	if ext == ".go" {
		return base[:len(base)-len(ext)]
	}
	// Directory or other: use the base name directly.
	return base
}

func resolveEngineName(patterns []string, explicit string) (string, error) {
	if explicit != "" {
		return explicit, nil
	}
	if len(patterns) != 1 {
		return "", fmt.Errorf("-name is required when compiling multiple package patterns")
	}
	pattern := patterns[0]
	if strings.ContainsAny(pattern, "*?[") || strings.HasSuffix(pattern, "...") {
		return "", fmt.Errorf("-name is required for wildcard package pattern %q", pattern)
	}
	info, err := os.Stat(pattern)
	if err != nil || !info.IsDir() {
		return "", fmt.Errorf("-name is required when %q is not an unambiguous directory pattern", pattern)
	}
	return engineNameFromPath(pattern), nil
}

// parseOptLevel converts a CLI -O flag value to an optimizer level.
// Valid values: 0=minimal, 1=fast, 2=standard (default).
func parseOptLevel(n int) (optimize.Level, error) {
	switch n {
	case 0:
		return optimize.LevelMinimal, nil
	case 1, 2:
		return 0, fmt.Errorf("optimization level %d is not implemented by newcompiler; only 0=minimal is currently available", n)
	default:
		return 0, fmt.Errorf("invalid optimization level %d (valid: 0=minimal)", n)
	}
}
