package scaffold

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"golang.org/x/mod/module"
	"golang.org/x/mod/semver"
)

const SonolusModulePath = "github.com/WindowsSov8forUs/sonolus-go/v2"

type Options struct {
	Directory         string
	ModulePath        string
	DependencyVersion string
}

type Result struct {
	Directory     string
	ModulePath    string
	CreatedModule bool
	Files         []string
}

func Init(options Options) (*Result, error) {
	directory := options.Directory
	if directory == "" {
		directory = "."
	}
	directory, err := filepath.Abs(directory)
	if err != nil {
		return nil, fmt.Errorf("scaffold: resolve target directory: %w", err)
	}

	info, err := os.Stat(directory)
	exists := err == nil
	if err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("scaffold: inspect target directory: %w", err)
	}
	if exists && !info.IsDir() {
		return nil, fmt.Errorf("scaffold: target %q is not a directory", directory)
	}

	moduleRoot, err := findModuleRoot(directory, exists)
	if err != nil {
		return nil, err
	}
	createModule := options.ModulePath != "" || moduleRoot == ""
	modulePath := options.ModulePath
	if createModule && modulePath == "" {
		modulePath = filepath.Base(directory)
	}
	if createModule {
		if err := module.CheckImportPath(modulePath); err != nil {
			return nil, fmt.Errorf("scaffold: invalid module path %q: %w", modulePath, err)
		}
	}
	if options.DependencyVersion != "" {
		if !semver.IsValid(options.DependencyVersion) || semver.Major(options.DependencyVersion) != "v2" {
			return nil, fmt.Errorf("scaffold: invalid sonolus-go version %q; expected v2 semantic version", options.DependencyVersion)
		}
	}

	files := templateFiles()
	if createModule {
		files["go.mod"] = []byte(goMod(modulePath, options.DependencyVersion))
	}
	if exists {
		if err := validateExistingDirectory(directory, createModule); err != nil {
			return nil, err
		}
		if err := writeFiles(directory, files); err != nil {
			return nil, err
		}
	} else {
		if err := writeNewDirectory(directory, files); err != nil {
			return nil, err
		}
	}

	paths := make([]string, 0, len(files))
	for path := range files {
		paths = append(paths, filepath.ToSlash(path))
	}
	sort.Strings(paths)
	return &Result{Directory: directory, ModulePath: modulePath, CreatedModule: createModule, Files: paths}, nil
}

func findModuleRoot(directory string, exists bool) (string, error) {
	current := directory
	if !exists {
		current = filepath.Dir(directory)
	}
	for {
		path := filepath.Join(current, "go.mod")
		if info, err := os.Stat(path); err == nil {
			if info.IsDir() {
				return "", fmt.Errorf("scaffold: %q is a directory", path)
			}
			return current, nil
		} else if !os.IsNotExist(err) {
			return "", fmt.Errorf("scaffold: inspect module file %q: %w", path, err)
		}
		parent := filepath.Dir(current)
		if parent == current {
			return "", nil
		}
		current = parent
	}
}

func validateExistingDirectory(directory string, createModule bool) error {
	entries, err := os.ReadDir(directory)
	if err != nil {
		return fmt.Errorf("scaffold: read target directory: %w", err)
	}
	allowed := map[string]bool{".git": true, "go.mod": true, "go.sum": true}
	for _, entry := range entries {
		if !allowed[entry.Name()] {
			return fmt.Errorf("scaffold: target directory %q is not empty; only .git, go.mod, and go.sum may already exist", directory)
		}
		if createModule && entry.Name() == "go.mod" {
			return fmt.Errorf("scaffold: target directory %q already contains go.mod", directory)
		}
	}
	return nil
}

func writeNewDirectory(directory string, files map[string][]byte) error {
	parent, base := filepath.Dir(directory), filepath.Base(directory)
	if err := os.MkdirAll(parent, 0o755); err != nil {
		return fmt.Errorf("scaffold: create parent directory: %w", err)
	}
	stage, err := os.MkdirTemp(parent, "."+base+"-init-")
	if err != nil {
		return fmt.Errorf("scaffold: create staging directory: %w", err)
	}
	defer os.RemoveAll(stage)
	if err := writeFiles(stage, files); err != nil {
		return err
	}
	if err := os.Rename(stage, directory); err != nil {
		return fmt.Errorf("scaffold: install target directory: %w", err)
	}
	return nil
}

func writeFiles(directory string, files map[string][]byte) error {
	paths := make([]string, 0, len(files))
	for path := range files {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	var created []string
	rollback := func() {
		for i := len(created) - 1; i >= 0; i-- {
			_ = os.Remove(created[i])
		}
		_ = os.Remove(filepath.Join(directory, ".vscode"))
	}
	for _, relative := range paths {
		path := filepath.Join(directory, filepath.FromSlash(relative))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			rollback()
			return fmt.Errorf("scaffold: create directory for %q: %w", relative, err)
		}
		file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
		if err != nil {
			rollback()
			return fmt.Errorf("scaffold: create %q: %w", relative, err)
		}
		created = append(created, path)
		if _, err := file.Write(files[relative]); err != nil {
			_ = file.Close()
			rollback()
			return fmt.Errorf("scaffold: write %q: %w", relative, err)
		}
		if err := file.Close(); err != nil {
			rollback()
			return fmt.Errorf("scaffold: close %q: %w", relative, err)
		}
	}
	return nil
}

func goMod(modulePath, dependencyVersion string) string {
	var builder strings.Builder
	fmt.Fprintf(&builder, "module %s\n\ngo 1.25.12\n", modulePath)
	if dependencyVersion != "" {
		fmt.Fprintf(&builder, "\nrequire %s %s\n", SonolusModulePath, dependencyVersion)
	}
	return builder.String()
}

func templateFiles() map[string][]byte {
	return map[string][]byte{
		".gitignore": []byte("/dist/\n"),
		".vscode/settings.json": []byte(`{
  "gopls": {
    "buildFlags": ["-tags=play"],
    "standaloneTags": ["ignore"],
    "staticcheck": true,
    "analyses": {
      "SA4017": false
    },
    "gofumpt": false
  },
  "[go]": {
    "editor.defaultFormatter": "golang.go",
    "editor.formatOnSave": true,
    "editor.codeActionsOnSave": {
      "source.organizeImports": "explicit"
    }
  }
}
`),
		"main.go": []byte(`package main

import "github.com/WindowsSov8forUs/sonolus-go/v2/sonolus"

type EngineConfiguration struct {
	sonolus.Configuration
}

var Configuration = EngineConfiguration{}

func main() {}
`),
		"play.go": []byte(`//go:build play

package main

import "github.com/WindowsSov8forUs/sonolus-go/v2/sonolus/play"

type PlayStage struct {
	play.Archetype ` + "`archetype:\"name=Stage\"`" + `
}
`),
		"watch.go": []byte(`//go:build watch

package main

import "github.com/WindowsSov8forUs/sonolus-go/v2/sonolus/watch"

type WatchStage struct {
	watch.Archetype ` + "`archetype:\"name=Stage\"`" + `
}
`),
		"preview.go": []byte(`//go:build preview

package main

import "github.com/WindowsSov8forUs/sonolus-go/v2/sonolus/preview"

type PreviewStage struct {
	preview.Archetype ` + "`archetype:\"name=Stage\"`" + `
}
`),
		"tutorial.go": []byte(`//go:build tutorial

package main

import "github.com/WindowsSov8forUs/sonolus-go/v2/sonolus/tutorial"

type TutorialCallbacks struct {
	tutorial.GlobalCallbacks
}

var Tutorial TutorialCallbacks
`),
	}
}
