package compiler

import "testing"

func TestDiscoverTargets(t *testing.T) {
	targets, err := DiscoverTargets(ModePlay, "../../examples/...", "./mode")
	if err != nil {
		t.Fatal(err)
	}
	if len(targets) != 2 {
		t.Fatalf("targets = %#v", targets)
	}
	if targets[0].PackagePath >= targets[1].PackagePath {
		t.Fatalf("targets are not sorted: %#v", targets)
	}
	for _, target := range targets {
		if target.ModulePath != "github.com/WindowsSov8forUs/sonolus-go/v2" {
			t.Fatalf("module path = %q", target.ModulePath)
		}
	}
}

func TestDiscoverTargetsRejectsNoEngine(t *testing.T) {
	if _, err := DiscoverTargets(ModePlay, "./mode"); err == nil {
		t.Fatal("non-main package was accepted as an engine")
	}
}
