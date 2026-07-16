package sonolus

import "testing"

func TestOptionConstructorsReturnDefaults(t *testing.T) {
	if got := SliderOption(SliderOptionConfig{Default: 1.25}); got != 1.25 {
		t.Fatalf("SliderOption() = %v, want 1.25", got)
	}
	if got := ToggleOption(ToggleOptionConfig{Default: true}); !got {
		t.Fatal("ToggleOption() = false, want true")
	}
	if got := SelectOption(SelectOptionConfig{Default: 2}); got != 2 {
		t.Fatalf("SelectOption() = %v, want 2", got)
	}
}
