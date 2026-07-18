package main

import (
	"fmt"
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

func cmdInit(directory, modulePath, dependencyVersion string) error {
	result, err := scaffold.Init(scaffold.Options{
		Directory:         directory,
		ModulePath:        modulePath,
		DependencyVersion: dependencyVersion,
	})
	if err != nil {
		return err
	}
	fmt.Printf("initialized Sonolus engine in %s\n", result.Directory)
	if result.CreatedModule && dependencyVersion == "" {
		fmt.Println("sonolus-go dependency was not pinned because this is a development build")
	}
	fmt.Println("next:")
	fmt.Printf("  cd %s\n", result.Directory)
	fmt.Println("  go mod tidy")
	fmt.Println("  sonolus-go vet .")
	return nil
}
