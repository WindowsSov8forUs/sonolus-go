package main

import (
	"fmt"
	"path/filepath"
	"strings"

	"golang.org/x/mod/semver"

	"github.com/WindowsSov8forUs/sonolus-go/v2/internal/scaffold"
)

func defaultSonolusVersion() string {
	value := currentBuildMetadata().version
	if !strings.HasPrefix(value, "v") {
		value = "v" + value
	}
	if !semver.IsValid(value) || semver.Major(value) != "v2" {
		return ""
	}
	return value
}

func cmdModInit(directory, modulePath, dependencyVersion string) error {
	result, err := scaffold.InitModule(scaffold.ModuleOptions{
		Directory:         directory,
		ModulePath:        modulePath,
		DependencyVersion: dependencyVersion,
	})
	if err != nil {
		return err
	}
	fmt.Printf("initialized Sonolus engine module in %s\n", result.Directory)
	if dependencyVersion == "" {
		fmt.Println("sonolus-go dependency was not pinned because this is a development build")
	}
	fmt.Println("next:")
	fmt.Printf("  cd %s\n", result.Directory)
	fmt.Println("  sonolus-go init ./engine")
	return nil
}

func cmdWorkInit(moduleDirectories []string) error {
	result, err := scaffold.InitWorkspace(scaffold.WorkspaceOptions{
		ModuleDirectories: moduleDirectories,
	})
	if err != nil {
		return err
	}
	fmt.Printf("initialized Sonolus workspace in %s\n", result.Directory)
	if len(result.Modules) == 0 {
		fmt.Println("next: go work use <module-directory>")
	}
	return nil
}

func cmdInit(directory string) error {
	result, err := scaffold.Init(scaffold.Options{Directory: directory})
	if err != nil {
		return err
	}
	relative, err := filepath.Rel(result.ModuleRoot, result.Directory)
	if err != nil {
		return fmt.Errorf("resolve engine package relative path: %w", err)
	}
	fmt.Printf("initialized Sonolus engine package in %s\n", result.Directory)
	fmt.Println("next:")
	fmt.Printf("  cd %s\n", result.ModuleRoot)
	fmt.Println("  go mod tidy")
	pattern := "."
	if relative != "." {
		pattern = "./" + filepath.ToSlash(relative)
	}
	fmt.Printf("  sonolus-go vet %s\n", pattern)
	return nil
}
