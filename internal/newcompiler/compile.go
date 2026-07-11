// Package newcompiler contains the declaration frontend for the next compiler.
package newcompiler

import (
	"fmt"

	"github.com/WindowsSov8forUs/sonolus-go/internal/newcompiler/frontend"
	"github.com/WindowsSov8forUs/sonolus-go/internal/newcompiler/mode"
	"github.com/WindowsSov8forUs/sonolus-go/internal/newcompiler/source"
)

// ParseDeclarations loads one main package and normalizes its declarations for
// the requested Sonolus mode. Callback bodies are deliberately not lowered.
func ParseDeclarations(m mode.Mode, patterns ...string) (*frontend.EngineDeclarations, error) {
	pkgs, err := source.Load(patterns...)
	if err != nil {
		return nil, err
	}
	if len(pkgs) != 1 {
		return nil, fmt.Errorf("expected exactly one main package, got %d", len(pkgs))
	}
	return frontend.ParsePackageToFrontend(pkgs[0], m)
}
