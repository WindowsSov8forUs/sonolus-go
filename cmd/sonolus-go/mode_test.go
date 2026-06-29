package main

import (
	"testing"
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
