package compiler

import (
	"fmt"
	"sort"
	"strings"

	"github.com/WindowsSov8forUs/sonolus-go/internal/compiler/mode"
	"golang.org/x/tools/go/packages"
)

// Target identifies one engine main package matched by package patterns.
type Target struct {
	PackagePath string
	ModulePath  string
}

// DiscoverTargets expands package patterns into engine main packages for a mode.
func DiscoverTargets(m mode.Mode, patterns ...string) ([]Target, error) {
	if !m.Valid() {
		return nil, fmt.Errorf("compiler: invalid discovery mode %q", m)
	}
	loaded, err := packages.Load(&packages.Config{
		Mode:       packages.NeedName | packages.NeedFiles | packages.NeedModule,
		BuildFlags: []string{"-tags=" + string(m)},
	}, patterns...)
	if err != nil {
		return nil, fmt.Errorf("compiler: discover %s targets: %w", m, err)
	}
	var messages []string
	for _, pkg := range loaded {
		for _, pkgErr := range pkg.Errors {
			messages = append(messages, pkgErr.Error())
		}
	}
	if len(messages) != 0 {
		sort.Strings(messages)
		return nil, fmt.Errorf("compiler: discover %s targets: %s", m, strings.Join(messages, "\n"))
	}
	targets := make([]Target, 0, len(loaded))
	seen := make(map[string]bool, len(loaded))
	for _, pkg := range loaded {
		if pkg.Name != "main" {
			continue
		}
		if pkg.Module == nil || !pkg.Module.Main || pkg.Module.Path == "" {
			return nil, fmt.Errorf("compiler: engine package %q does not belong to a main module", pkg.PkgPath)
		}
		if !seen[pkg.PkgPath] {
			targets = append(targets, Target{PackagePath: pkg.PkgPath, ModulePath: pkg.Module.Path})
			seen[pkg.PkgPath] = true
		}
	}
	if len(targets) == 0 {
		return nil, fmt.Errorf("compiler: package patterns matched no engine main packages")
	}
	sort.Slice(targets, func(i, j int) bool {
		return targets[i].PackagePath < targets[j].PackagePath
	})
	return targets, nil
}
