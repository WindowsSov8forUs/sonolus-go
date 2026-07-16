package main

import (
	"runtime/debug"
	"testing"
)

func TestResolveBuildMetadataPrefersInjectedValues(t *testing.T) {
	info := &debug.BuildInfo{
		Main: debug.Module{Version: "v2.0.0-rc2"},
		Settings: []debug.BuildSetting{
			{Key: "vcs.revision", Value: "fallback-commit"},
			{Key: "vcs.time", Value: "fallback-date"},
		},
	}

	got := resolveBuildMetadata("2.0.0-rc2", "release-commit", "release-date", info)
	want := buildMetadata{version: "2.0.0-rc2", commit: "release-commit", date: "release-date"}
	if got != want {
		t.Fatalf("resolveBuildMetadata() = %#v, want %#v", got, want)
	}
}

func TestResolveBuildMetadataUsesModuleVersion(t *testing.T) {
	info := &debug.BuildInfo{Main: debug.Module{Version: "v2.0.0-rc1"}}

	got := resolveBuildMetadata("dev", "unknown", "unknown", info)
	want := buildMetadata{version: "2.0.0-rc1", commit: "unknown", date: "unknown"}
	if got != want {
		t.Fatalf("resolveBuildMetadata() = %#v, want %#v", got, want)
	}
	if formatted := got.String(); formatted != "sonolus-go 2.0.0-rc1" {
		t.Fatalf("String() = %q", formatted)
	}
}

func TestResolveBuildMetadataUsesVCSSettings(t *testing.T) {
	info := &debug.BuildInfo{
		Main: debug.Module{Version: "(devel)"},
		Settings: []debug.BuildSetting{
			{Key: "vcs.revision", Value: "local-commit"},
			{Key: "vcs.time", Value: "2026-07-13T00:00:00Z"},
		},
	}

	got := resolveBuildMetadata("dev", "unknown", "unknown", info)
	if formatted := got.String(); formatted != "sonolus-go dev (commit local-commit, built 2026-07-13T00:00:00Z)" {
		t.Fatalf("String() = %q", formatted)
	}
}

func TestBuildMetadataStringOmitsUnavailableFields(t *testing.T) {
	metadata := buildMetadata{version: "dev", commit: "local-commit", date: "unknown"}
	if got := metadata.String(); got != "sonolus-go dev (commit local-commit)" {
		t.Fatalf("String() = %q", got)
	}
}
