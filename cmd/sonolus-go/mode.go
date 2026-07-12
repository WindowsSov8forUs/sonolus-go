package main

import (
	"errors"
	"fmt"
	"path"
	"strings"

	"github.com/WindowsSov8forUs/sonolus-go/internal/compiler"
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
func (m Mode) CompilerMode() compiler.Mode {
	switch m {
	case ModePlay:
		return compiler.ModePlay
	case ModeWatch:
		return compiler.ModeWatch
	case ModePreview:
		return compiler.ModePreview
	case ModeTutorial:
		return compiler.ModeTutorial
	default:
		return compiler.ModePlay
	}
}

// engineNameFromModule derives the engine name from the module path.
func engineNameFromModule(modulePath string) (string, error) {
	name := path.Base(strings.TrimSuffix(modulePath, "/"))
	if name == "" || name == "." || name == "/" {
		return "", fmt.Errorf("cannot derive engine name from module path %q", modulePath)
	}
	return name, nil
}

type namedTarget struct {
	target compiler.Target
	name   string
}

func nameTargets(targets []compiler.Target, outputName string) ([]namedTarget, error) {
	if outputName != "" && len(targets) != 1 {
		return nil, fmt.Errorf("-o requires exactly one engine, but package patterns matched %d", len(targets))
	}
	result := make([]namedTarget, 0, len(targets))
	seen := make(map[string]string, len(targets))
	for _, target := range targets {
		name := outputName
		if name == "" {
			var err error
			name, err = engineNameFromModule(target.ModulePath)
			if err != nil {
				return nil, err
			}
		}
		if strings.ContainsAny(name, `/\\`) || name == "." || name == ".." {
			return nil, fmt.Errorf("invalid engine output name %q", name)
		}
		if previous, ok := seen[name]; ok {
			return nil, fmt.Errorf("engine name %q is ambiguous for packages %q and %q; use separate invocations with -o", name, previous, target.PackagePath)
		}
		seen[name] = target.PackagePath
		result = append(result, namedTarget{target: target, name: name})
	}
	return result, nil
}

// parseOptLevel converts a CLI -O flag value to an optimizer level.
// Valid values: 0=minimal, 1=fast, 2=standard (default).
func parseOptLevel(n int) (compiler.OptimizationLevel, error) {
	switch n {
	case 0:
		return compiler.OptimizationMinimal, nil
	case 1:
		return compiler.OptimizationFast, nil
	case 2:
		return compiler.OptimizationStandard, nil
	default:
		return 0, fmt.Errorf("invalid optimization level %d (valid: 0=minimal, 1=fast, 2=standard)", n)
	}
}
