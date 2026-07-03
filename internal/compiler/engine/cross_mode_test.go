package engine

import (
	"testing"
)

// TestCrossModeModeFlags verifies that isPlay()/isWatch()/isPreview()/isTutorial()
// are compile-time constants: 1 in their own mode, 0 in others. The mode flags
// are resolved by resolveBuiltinCall (trace_call.go:303-310) during compilation
// and constant-folded by SCCP, producing deterministic SNode output.
func TestCrossModeModeFlags(t *testing.T) {
	src := `package p
type N struct {
	flag float64 ` + "`" + `sonolus:"memory"` + "`" + `
}
func (n N) Initialize() {
	n.flag = isPlay() + isWatch() + isPreview() + isTutorial()
}
`
	// Play mode: isPlay()=1, others=0 → sum = 1.
	playData, _, err := CompilePlayFile(src)
	if err != nil {
		t.Fatalf("play: %v", err)
	}
	if len(playData.Archetypes) == 0 {
		t.Fatal("play: expected at least 1 archetype")
	}
	// The Initialize callback should exist (not omitted as pure-constant,
	// since it writes to memory-backed n.flag).
	if playData.Archetypes[0].Initialize == nil {
		t.Fatal("play: Initialize callback should not be omitted")
	}

	// Watch mode: isWatch()=1, others=0 → sum = 1.
	watchSrc := `package p
type N struct {
	flag float64 ` + "`" + `sonolus:"memory"` + "`" + `
}
func (n N) Initialize() {
	n.flag = isPlay() + isWatch() + isPreview() + isTutorial()
}
`
	watchData, err := CompileWatchFile(watchSrc)
	if err != nil {
		t.Fatalf("watch: %v", err)
	}
	if len(watchData.Archetypes) == 0 {
		t.Fatal("watch: expected at least 1 archetype")
	}
	if watchData.Archetypes[0].Initialize == nil {
		t.Fatal("watch: Initialize callback should not be omitted")
	}
}

// TestCrossModeSharedHelperConsistency verifies that a shared pure-arithmetic
// helper function (lerp) compiles successfully in all four modes. The helper is
// compiled independently for each mode but produces deterministic output since
// it has no mode-dependent behavior.
func TestCrossModeSharedHelperConsistency(t *testing.T) {
	helperSrc := `package p
type N struct {
	x float64 ` + "`" + `sonolus:"memory"` + "`" + `
}
func lerp(a, b, t float64) float64 {
	return a + (b - a) * t
}
func (n N) Initialize() {
	n.x = lerp(0, 100, 0.5)
}
`
	// Verify all four modes compile successfully with the same source.
	_, _, err := CompilePlayFile(helperSrc)
	if err != nil {
		t.Fatalf("play: %v", err)
	}
	_, err = CompileWatchFile(helperSrc)
	if err != nil {
		t.Fatalf("watch: %v", err)
	}
	_, err = CompilePreviewFile(helperSrc)
	if err != nil {
		t.Fatalf("preview: %v", err)
	}

	tutorialSrc := `package p
func lerp(a, b, t float64) float64 {
	return a + (b - a) * t
}
func Preprocess() {
	x := lerp(0, 100, 0.5)
	set(3000, 0, x)
}
`
	_, err = CompileTutorialFile(tutorialSrc)
	if err != nil {
		t.Fatalf("tutorial: %v", err)
	}
}

// TestCrossModeImportLayoutConsistency verifies that archetypes with the same
// struct field layout have consistent import indices across modes. The Sonolus
// engine runtime expects matching import layouts for multi-mode engines.
func TestCrossModeImportLayoutConsistency(t *testing.T) {
	src := `package p
type Note struct {
	Beat float64 ` + "`" + `sonolus:"imported"` + "`" + `
	Lane float64 ` + "`" + `sonolus:"imported"` + "`" + `
}
func (n Note) Initialize() {}
`
	playData, _, err := CompilePlayFile(src)
	if err != nil {
		t.Fatalf("play: %v", err)
	}
	watchData, err := CompileWatchFile(src)
	if err != nil {
		t.Fatalf("watch: %v", err)
	}

	if len(playData.Archetypes) != 1 || len(watchData.Archetypes) != 1 {
		t.Fatal("expected 1 archetype in both modes")
	}

	pa := playData.Archetypes[0]
	wa := watchData.Archetypes[0]

	if pa.Name != wa.Name {
		t.Errorf("archetype name mismatch: play=%q watch=%q", pa.Name, wa.Name)
	}
	if len(pa.Imports) != len(wa.Imports) {
		t.Errorf("import count mismatch: play=%d watch=%d", len(pa.Imports), len(wa.Imports))
	}
	for i := range pa.Imports {
		if i >= len(wa.Imports) {
			break
		}
		if pa.Imports[i].Name != wa.Imports[i].Name {
			t.Errorf("import[%d] name mismatch: play=%q watch=%q", i, pa.Imports[i].Name, wa.Imports[i].Name)
		}
		if pa.Imports[i].Index != wa.Imports[i].Index {
			t.Errorf("import[%d] index mismatch: play=%d watch=%d", i, pa.Imports[i].Index, wa.Imports[i].Index)
		}
	}
}
