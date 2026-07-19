package scaffold

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"golang.org/x/mod/modfile"
	"golang.org/x/mod/module"
	"golang.org/x/mod/semver"
)

const SonolusModulePath = "github.com/WindowsSov8forUs/sonolus-go/v2"

type ModuleOptions struct {
	Directory         string
	ModulePath        string
	DependencyVersion string
}

type ModuleResult struct {
	Directory  string
	ModulePath string
	Files      []string
}

type WorkspaceOptions struct {
	Directory         string
	ModuleDirectories []string
}

type WorkspaceResult struct {
	Directory string
	Modules   []string
	Files     []string
}

type Options struct {
	Directory string
}

type Result struct {
	Directory  string
	ModuleRoot string
	Files      []string
}

func InitModule(options ModuleOptions) (*ModuleResult, error) {
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

	modulePath := options.ModulePath
	if modulePath == "" {
		return nil, fmt.Errorf("scaffold: module path is required")
	}
	if err := module.CheckImportPath(modulePath); err != nil {
		return nil, fmt.Errorf("scaffold: invalid module path %q: %w", modulePath, err)
	}
	if options.DependencyVersion != "" {
		if !semver.IsValid(options.DependencyVersion) || semver.Major(options.DependencyVersion) != "v2" {
			return nil, fmt.Errorf("scaffold: invalid sonolus-go version %q; expected v2 semantic version", options.DependencyVersion)
		}
	}

	files := moduleTemplateFiles(modulePath, options.DependencyVersion)
	if exists {
		if err := validateModuleDirectory(directory); err != nil {
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
	return &ModuleResult{Directory: directory, ModulePath: modulePath, Files: paths}, nil
}

func InitWorkspace(options WorkspaceOptions) (*WorkspaceResult, error) {
	directory := options.Directory
	if directory == "" {
		directory = "."
	}
	directory, err := filepath.Abs(directory)
	if err != nil {
		return nil, fmt.Errorf("scaffold: resolve workspace directory: %w", err)
	}
	info, err := os.Stat(directory)
	if err != nil {
		return nil, fmt.Errorf("scaffold: inspect workspace directory: %w", err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("scaffold: workspace target %q is not a directory", directory)
	}

	work, err := modfile.ParseWork("go.work", nil, nil)
	if err != nil {
		return nil, fmt.Errorf("scaffold: initialize workspace file: %w", err)
	}
	if err := work.AddGoStmt("1.25.12"); err != nil {
		return nil, fmt.Errorf("scaffold: initialize workspace: %w", err)
	}
	modules := make([]string, 0, len(options.ModuleDirectories))
	for _, moduleDirectory := range options.ModuleDirectories {
		if moduleDirectory == "" {
			return nil, fmt.Errorf("scaffold: workspace module directory is empty")
		}
		modulePath, err := workspaceModulePath(directory, moduleDirectory)
		if err != nil {
			return nil, err
		}
		usePath := filepath.Clean(moduleDirectory)
		if !filepath.IsAbs(usePath) && usePath != "." && usePath != ".." && !strings.HasPrefix(usePath, "."+string(filepath.Separator)) {
			usePath = "." + string(filepath.Separator) + usePath
		}
		usePath = filepath.ToSlash(usePath)
		if err := work.AddUse(usePath, modulePath); err != nil {
			return nil, fmt.Errorf("scaffold: add workspace module %q: %w", moduleDirectory, err)
		}
		modules = append(modules, usePath)
	}
	work.SortBlocks()
	if err := writeFiles(directory, map[string][]byte{"go.work": modfile.Format(work.Syntax)}); err != nil {
		return nil, err
	}
	return &WorkspaceResult{Directory: directory, Modules: modules, Files: []string{"go.work"}}, nil
}

func workspaceModulePath(workspaceDirectory, moduleDirectory string) (string, error) {
	directory := moduleDirectory
	if !filepath.IsAbs(directory) {
		directory = filepath.Join(workspaceDirectory, directory)
	}
	info, err := os.Stat(directory)
	if err != nil {
		return "", fmt.Errorf("scaffold: inspect workspace module %q: %w", moduleDirectory, err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("scaffold: workspace module %q is not a directory", moduleDirectory)
	}
	goModPath := filepath.Join(directory, "go.mod")
	data, err := os.ReadFile(goModPath)
	if err != nil {
		return "", fmt.Errorf("scaffold: read workspace module file %q: %w", goModPath, err)
	}
	parsed, err := modfile.Parse(goModPath, data, nil)
	if err != nil || parsed.Module == nil || parsed.Module.Mod.Path == "" {
		if err == nil {
			err = fmt.Errorf("module directive is required")
		}
		return "", fmt.Errorf("scaffold: invalid workspace module file %q: %w", goModPath, err)
	}
	return parsed.Module.Mod.Path, nil
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
	if moduleRoot == "" {
		return nil, fmt.Errorf("scaffold: no Sonolus engine module found for %q; run sonolus-go mod init <project-directory> first", directory)
	}
	atModuleRoot := filepath.Clean(moduleRoot) == filepath.Clean(directory)

	files := engineTemplateFiles()
	if exists {
		var err error
		if atModuleRoot {
			err = validateRootEngineDirectory(directory)
		} else {
			err = validateEngineDirectory(directory)
		}
		if err != nil {
			return nil, err
		}
		if err := writeFiles(directory, files); err != nil {
			return nil, err
		}
	} else if err := writeNewDirectory(directory, files); err != nil {
		return nil, err
	}

	paths := make([]string, 0, len(files))
	for path := range files {
		paths = append(paths, filepath.ToSlash(path))
	}
	sort.Strings(paths)
	return &Result{Directory: directory, ModuleRoot: moduleRoot, Files: paths}, nil
}

func findModuleRoot(directory string, exists bool) (string, error) {
	current := directory
	if !exists {
		current = filepath.Dir(directory)
	}
	for {
		goModPath := filepath.Join(current, "go.mod")
		goSumPath := filepath.Join(current, "go.sum")
		goModInfo, goModErr := os.Stat(goModPath)
		goSumInfo, goSumErr := os.Stat(goSumPath)
		goModExists := goModErr == nil
		goSumExists := goSumErr == nil
		if goModErr != nil && !os.IsNotExist(goModErr) {
			return "", fmt.Errorf("scaffold: inspect module file %q: %w", goModPath, goModErr)
		}
		if goSumErr != nil && !os.IsNotExist(goSumErr) {
			return "", fmt.Errorf("scaffold: inspect checksum file %q: %w", goSumPath, goSumErr)
		}
		if goModExists || goSumExists {
			if !goModExists || !goSumExists {
				missing := goModPath
				if goModExists {
					missing = goSumPath
				}
				return "", fmt.Errorf("scaffold: invalid Sonolus engine module %q: missing %s", current, filepath.Base(missing))
			}
			if !goModInfo.Mode().IsRegular() || !goSumInfo.Mode().IsRegular() {
				return "", fmt.Errorf("scaffold: go.mod and go.sum in %q must be regular files", current)
			}
			data, err := os.ReadFile(goModPath)
			if err != nil {
				return "", fmt.Errorf("scaffold: read module file %q: %w", goModPath, err)
			}
			parsed, err := modfile.Parse(goModPath, data, nil)
			if err != nil || parsed.Module == nil || parsed.Module.Mod.Path == "" {
				if err == nil {
					err = fmt.Errorf("module directive is required")
				}
				return "", fmt.Errorf("scaffold: invalid module file %q: %w", goModPath, err)
			}
			return current, nil
		}
		parent := filepath.Dir(current)
		if parent == current {
			return "", nil
		}
		current = parent
	}
}

func validateModuleDirectory(directory string) error {
	entries, err := os.ReadDir(directory)
	if err != nil {
		return fmt.Errorf("scaffold: read target directory: %w", err)
	}
	allowed := map[string]bool{".git": true}
	for _, entry := range entries {
		if !allowed[entry.Name()] {
			return fmt.Errorf("scaffold: module target directory %q is not empty; only .git may already exist", directory)
		}
	}
	return nil
}

func validateEngineDirectory(directory string) error {
	entries, err := os.ReadDir(directory)
	if err != nil {
		return fmt.Errorf("scaffold: read target directory: %w", err)
	}
	if len(entries) != 0 {
		return fmt.Errorf("scaffold: engine target directory %q is not empty", directory)
	}
	return nil
}

func validateRootEngineDirectory(directory string) error {
	entries, err := os.ReadDir(directory)
	if err != nil {
		return fmt.Errorf("scaffold: read module root: %w", err)
	}
	allowed := map[string]bool{".git": true, ".gitignore": true, ".vscode": true, "go.mod": true, "go.sum": true}
	for _, entry := range entries {
		if !allowed[entry.Name()] {
			return fmt.Errorf("scaffold: module root %q already contains %q; initialize engine packages in subdirectories", directory, entry.Name())
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
		if _, createsSettings := files[".vscode/settings.json"]; createsSettings {
			_ = os.Remove(filepath.Join(directory, ".vscode"))
		}
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

func moduleTemplateFiles(modulePath, dependencyVersion string) map[string][]byte {
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
		"go.mod": []byte(goMod(modulePath, dependencyVersion)),
		"go.sum": {},
	}
}

func engineTemplateFiles() map[string][]byte {
	return map[string][]byte{
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
