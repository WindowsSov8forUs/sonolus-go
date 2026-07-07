package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/WindowsSov8forUs/sonolus-go/internal/compiler/level"
)

func cmdLevel(levelPath, outDir string) error {
	blob, err := level.PackageLevel(levelPath)
	if err != nil {
		return fmt.Errorf("packaging level: %w", err)
	}
	dir := filepath.Join(outDir, engineNameFromPath(levelPath))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("creating %s: %w", dir, err)
	}
	if err := os.WriteFile(filepath.Join(dir, level.FileName), blob, 0o644); err != nil {
		return fmt.Errorf("writing level: %w", err)
	}
	fmt.Printf("wrote level to %s/\n", dir)
	return nil
}
