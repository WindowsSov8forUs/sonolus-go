package main

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/WindowsSov8forUs/sonolus-go/internal/newcompiler/mode"
	"github.com/WindowsSov8forUs/sonolus-go/internal/newcompiler/optimize"
)

func TestParseMode(t *testing.T) {
	tests := []struct {
		input string
		want  Mode
		err   bool
	}{
		{"play", ModePlay, false},
		{"watch", ModeWatch, false},
		{"preview", ModePreview, false},
		{"tutorial", ModeTutorial, false},
		{"all", ModeAll, false},
		{"", "", true},
		{"unknown", "", true},
		{"PLAY", "", true}, // case-sensitive
	}
	for _, tt := range tests {
		got, err := ParseMode(tt.input)
		if tt.err && err == nil {
			t.Errorf("ParseMode(%q): expected error, got %v", tt.input, got)
		}
		if !tt.err && err != nil {
			t.Errorf("ParseMode(%q): unexpected error: %v", tt.input, err)
		}
		if !tt.err && got != tt.want {
			t.Errorf("ParseMode(%q) = %v, want %v", tt.input, got, tt.want)
		}
	}
}

func TestModeExpand(t *testing.T) {
	if len(ModePlay.Expand()) != 1 || ModePlay.Expand()[0] != ModePlay {
		t.Error("single mode should expand to itself")
	}
	got := ModeAll.Expand()
	if len(got) != 4 {
		t.Fatalf("ModeAll.Expand() len = %d, want 4", len(got))
	}
	for i, m := range []Mode{ModePlay, ModeWatch, ModePreview, ModeTutorial} {
		if got[i] != m {
			t.Errorf("ModeAll.Expand()[%d] = %v, want %v", i, got[i], m)
		}
	}
}

func TestAllModeNames(t *testing.T) {
	names := allModeNames()
	if len(names) != 4 {
		t.Fatalf("allModeNames() len = %d, want 4", len(names))
	}
	for i, want := range []string{"play", "watch", "preview", "tutorial"} {
		if names[i] != want {
			t.Errorf("allModeNames()[%d] = %q, want %q", i, names[i], want)
		}
	}
}

func TestMode_String(t *testing.T) {
	tests := []struct {
		mode Mode
		want string
	}{
		{ModePlay, "play"},
		{ModeWatch, "watch"},
		{ModePreview, "preview"},
		{ModeTutorial, "tutorial"},
		{ModeAll, "all"},
	}
	for _, tt := range tests {
		if got := tt.mode.String(); got != tt.want {
			t.Errorf("Mode(%q).String() = %q, want %q", tt.mode, got, tt.want)
		}
	}
}

func TestParseOptLevel(t *testing.T) {
	tests := []struct {
		input   int
		want    optimize.Level
		wantErr bool
	}{
		{0, optimize.LevelMinimal, false},
		{1, 0, true},
		{2, 0, true},
		{-1, 0, true},
		{3, 0, true},
	}
	for _, tt := range tests {
		got, err := parseOptLevel(tt.input)
		if (err != nil) != tt.wantErr {
			t.Errorf("parseOptLevel(%d) error = %v, wantErr %v", tt.input, err, tt.wantErr)
		}
		if !tt.wantErr && got != tt.want {
			t.Errorf("parseOptLevel(%d) = %v, want %v", tt.input, got, tt.want)
		}
	}
}

func TestRunCLIParsesSubcommandFlags(t *testing.T) {
	err := runCLI([]string{"build", "-name", "fixture", "-O", "2", "./testdata/multimode"})
	if err == nil || !strings.Contains(err.Error(), "optimization level 2 is not implemented") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunCLILevelRequiresChart(t *testing.T) {
	err := runCLI([]string{"level", "-o", t.TempDir()})
	if err == nil || !strings.Contains(err.Error(), "exactly one chart path") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCompilerMode(t *testing.T) {
	tests := []struct {
		mode Mode
		want mode.Mode
	}{
		{ModePlay, mode.ModePlay},
		{ModeWatch, mode.ModeWatch},
		{ModePreview, mode.ModePreview},
		{ModeTutorial, mode.ModeTutorial},
		{Mode("unknown"), mode.ModePlay},
	}
	for _, tt := range tests {
		if got := tt.mode.CompilerMode(); got != tt.want {
			t.Errorf("CompilerMode() = %v, want %v", got, tt.want)
		}
	}
}

func TestEngineNameFromPath(t *testing.T) {
	tests := []struct{ path, want string }{
		{"engines/my-engine.go", "my-engine"},
		{"engine.go", "engine"},
		{filepath.Join("a", "b", "c.go"), "c"},
		{"no-ext", "no-ext"},
		{"/absolute/path/to/test.go", "test"},
	}
	for _, tt := range tests {
		if got := engineNameFromPath(tt.path); got != tt.want {
			t.Errorf("engineNameFromPath(%q) = %q, want %q", tt.path, got, tt.want)
		}
	}
}
