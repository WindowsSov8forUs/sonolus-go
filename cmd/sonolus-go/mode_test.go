package main

import (
	"path/filepath"
	"testing"

	"github.com/WindowsSov8forUs/sonolus-go/internal/compiler/engine"
	"github.com/WindowsSov8forUs/sonolus-go/internal/compiler/ir"
	"github.com/WindowsSov8forUs/sonolus-go/internal/compiler/ir/optimize"
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
		{1, optimize.LevelFast, false},
		{2, optimize.LevelStandard, false},
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

func TestBuildOpts(t *testing.T) {
	// Without stats: Stats should be nil, Opt set.
	opts := buildOpts(nil, optimize.LevelStandard)
	if opts.Stats != nil {
		t.Error("buildOpts(nil, ...).Stats should be nil")
	}
	if opts.Opt != optimize.LevelStandard {
		t.Errorf("buildOpts Opt = %v, want LevelStandard", opts.Opt)
	}

	// With existing CompileStats: should be carried through.
	s := &engine.CompileStats{}
	opts2 := buildOpts(s, optimize.LevelFast)
	if opts2.Stats != s {
		t.Error("buildOpts with non-nil stats should carry them through")
	}
	if opts2.Opt != optimize.LevelFast {
		t.Errorf("buildOpts Opt = %v, want LevelFast", opts2.Opt)
	}
}

func TestIRMode(t *testing.T) {
	tests := []struct {
		mode Mode
		want ir.Mode
	}{
		{ModePlay, ir.ModePlay},
		{ModeWatch, ir.ModeWatch},
		{ModePreview, ir.ModePreview},
		{ModeTutorial, ir.ModeTutorial},
		{Mode("unknown"), ir.ModePlay}, // default fallback
	}
	for _, tt := range tests {
		if got := tt.mode.IRMode(); got != tt.want {
			t.Errorf("IRMode() = %v, want %v", got, tt.want)
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
