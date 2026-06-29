package main

import (
	"errors"
	"fmt"
	"path/filepath"
)

// ErrUnknownMode is returned when the user supplies an unrecognized mode string.
var ErrUnknownMode = errors.New("unknown mode")

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
	return "", fmt.Errorf("%w: %s (valid: play, watch, preview, tutorial, all)", ErrUnknownMode, s)
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

// engineNameFromPath extracts the engine name from an engine source file path.
// e.g. "engines/my-engine.go" → "my-engine".
func engineNameFromPath(srcPath string) string {
	return filepath.Base(srcPath[:len(srcPath)-len(filepath.Ext(srcPath))])
}
