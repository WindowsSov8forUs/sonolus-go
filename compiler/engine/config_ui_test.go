package engine

import (
	"testing"

	"github.com/WindowsSov8forUs/sonolus-core-go/core/resource"
)

// compileConfig compiles a minimal play engine source and returns the resulting
// EngineConfiguration (which includes the parsed UI).
func compileConfig(src string) (*resource.EngineConfiguration, error) {
	_, cfg, err := CompilePlayFile(src)
	if err != nil {
		return nil, err
	}
	return cfg, nil
}

// TestBuildUI_DefaultOnly verifies that a source without a UI struct still gets
// defaultUI() values.
func TestBuildUI_DefaultOnly(t *testing.T) {
	src := `package test

type Skin struct {
	Note int ` + "`sonolus:\"renderMode=standard\"`" + `
}

type Config struct {
	Speed float64 ` + "`sonolus:\"slider,min=1,max=10,step=0.1,def=3\"`" + `
}

type Archetype struct{}

func (a Archetype) UpdateParallel(dt float64) {}
`
	cfg, err := compileConfig(src)
	if err != nil {
		t.Fatalf("compileConfig: %v", err)
	}
	if cfg.UI.PrimaryMetric != resource.EngineConfigurationMetricArcade {
		t.Errorf("PrimaryMetric = %q, want arcade", cfg.UI.PrimaryMetric)
	}
	if cfg.UI.SecondaryMetric != resource.EngineConfigurationMetricAccuracy {
		t.Errorf("SecondaryMetric = %q, want accuracy", cfg.UI.SecondaryMetric)
	}
	if cfg.UI.JudgmentErrorStyle != resource.EngineConfigurationJudgmentErrorStyleLate {
		t.Errorf("JudgmentErrorStyle = %q, want late", cfg.UI.JudgmentErrorStyle)
	}
	if cfg.UI.MenuVisibility.Scale != 1 || cfg.UI.MenuVisibility.Alpha != 1 {
		t.Errorf("MenuVisibility = {%f, %f}, want {1, 1}", cfg.UI.MenuVisibility.Scale, cfg.UI.MenuVisibility.Alpha)
	}
}

// TestBuildUI_Overrides verifies that a UI struct overrides specific fields.
func TestBuildUI_Overrides(t *testing.T) {
	src := `package test

type UI struct {
	PrimaryMetric             string  ` + "`sonolus:\"primaryMetric=arcadePercentage\"`" + `
	JudgmentErrorStyle        string  ` + "`sonolus:\"judgmentErrorStyle=early\"`" + `
	JudgmentErrorPlacement    string  ` + "`sonolus:\"judgmentErrorPlacement=top\"`" + `
	JudgmentErrorMin          float64 ` + "`sonolus:\"judgmentErrorMin=20\"`" + `
	MenuVisibilityScale       float64 ` + "`sonolus:\"menuVisibilityScale=0.5\"`" + `
}

type Skin struct {
	Note int
}

type Archetype struct{}

func (a Archetype) UpdateParallel(dt float64) {}
`
	cfg, err := compileConfig(src)
	if err != nil {
		t.Fatalf("compileConfig: %v", err)
	}

	if cfg.UI.PrimaryMetric != resource.EngineConfigurationMetricArcadePercentage {
		t.Errorf("PrimaryMetric = %q, want arcadePercentage", cfg.UI.PrimaryMetric)
	}
	if cfg.UI.JudgmentErrorStyle != resource.EngineConfigurationJudgmentErrorStyleEarly {
		t.Errorf("JudgmentErrorStyle = %q, want early", cfg.UI.JudgmentErrorStyle)
	}
	if cfg.UI.JudgmentErrorPlacement != resource.EngineConfigurationJudgmentErrorPlacementTop {
		t.Errorf("JudgmentErrorPlacement = %q, want top", cfg.UI.JudgmentErrorPlacement)
	}
	if cfg.UI.JudgmentErrorMin != 20 {
		t.Errorf("JudgmentErrorMin = %f, want 20", cfg.UI.JudgmentErrorMin)
	}
	if cfg.UI.MenuVisibility.Scale != 0.5 {
		t.Errorf("MenuVisibility.Scale = %f, want 0.5", cfg.UI.MenuVisibility.Scale)
	}
	if cfg.UI.MenuVisibility.Alpha != 1 {
		t.Errorf("MenuVisibility.Alpha = %f, want 1 (default)", cfg.UI.MenuVisibility.Alpha)
	}
	// Unset fields stay at defaults
	if cfg.UI.JudgmentVisibility.Scale != 1 {
		t.Errorf("JudgmentVisibility.Scale = %f, want 1 (default)", cfg.UI.JudgmentVisibility.Scale)
	}
}

// TestBuildUI_AnimationOverrides verifies that animation tween fields can be
// overridden.
func TestBuildUI_AnimationOverrides(t *testing.T) {
	src := `package test

type UI struct {
	JudgmentAnimationScaleFrom     float64 ` + "`sonolus:\"judgmentAnimationScaleFrom=0.5\"`" + `
	JudgmentAnimationScaleTo       float64 ` + "`sonolus:\"judgmentAnimationScaleTo=1.2\"`" + `
	JudgmentAnimationScaleDuration float64 ` + "`sonolus:\"judgmentAnimationScaleDuration=0.3\"`" + `
	JudgmentAnimationScaleEase     string  ` + "`sonolus:\"judgmentAnimationScaleEase=inQuart\"`" + `
	JudgmentAnimationAlphaFrom     float64 ` + "`sonolus:\"judgmentAnimationAlphaFrom=1\"`" + `
	JudgmentAnimationAlphaTo       float64 ` + "`sonolus:\"judgmentAnimationAlphaTo=0\"`" + `
	JudgmentAnimationAlphaDuration float64 ` + "`sonolus:\"judgmentAnimationAlphaDuration=0.2\"`" + `
	JudgmentAnimationAlphaEase     string  ` + "`sonolus:\"judgmentAnimationAlphaEase=inBack\"`" + `
}

type Skin struct {
	Note int
}

type Archetype struct{}

func (a Archetype) UpdateParallel(dt float64) {}
`
	cfg, err := compileConfig(src)
	if err != nil {
		t.Fatalf("compileConfig: %v", err)
	}

	sa := cfg.UI.JudgmentAnimation.Scale
	if sa.From != 0.5 {
		t.Errorf("judgmentAnimation.scale.from = %f, want 0.5", sa.From)
	}
	if sa.To != 1.2 {
		t.Errorf("judgmentAnimation.scale.to = %f, want 1.2", sa.To)
	}
	if sa.Duration != 0.3 {
		t.Errorf("judgmentAnimation.scale.duration = %f, want 0.3", sa.Duration)
	}
	if sa.Ease != resource.EngineConfigurationAnimationTweenEaseInQuart {
		t.Errorf("judgmentAnimation.scale.ease = %q, want inQuart", sa.Ease)
	}

	aa := cfg.UI.JudgmentAnimation.Alpha
	if aa.From != 1 {
		t.Errorf("judgmentAnimation.alpha.from = %f, want 1", aa.From)
	}
	if aa.To != 0 {
		t.Errorf("judgmentAnimation.alpha.to = %f, want 0", aa.To)
	}
	if aa.Duration != 0.2 {
		t.Errorf("judgmentAnimation.alpha.duration = %f, want 0.2", aa.Duration)
	}
	if aa.Ease != resource.EngineConfigurationAnimationTweenEaseInBack {
		t.Errorf("judgmentAnimation.alpha.ease = %q, want inBack", aa.Ease)
	}

	ca := cfg.UI.ComboAnimation.Scale
	if ca.From != 1 || ca.To != 1 || ca.Duration != 0 {
		t.Errorf("ComboAnimation.Scale should stay at default, got from=%f to=%f duration=%f", ca.From, ca.To, ca.Duration)
	}
}

// TestBuildUI_InvalidMetric verifies error on unknown metric value.
func TestBuildUI_InvalidMetric(t *testing.T) {
	src := `package test

type UI struct {
	PrimaryMetric string ` + "`sonolus:\"primaryMetric=nope\"`" + `
}

type Skin struct {
	Note int
}

type Archetype struct{}

func (a Archetype) UpdateParallel(dt float64) {}
`
	_, err := compileConfig(src)
	if err == nil {
		t.Fatal("expected error for unknown primaryMetric, got nil")
	}
}

// TestBuildUI_InvalidEase verifies error on unknown ease value.
func TestBuildUI_InvalidEase(t *testing.T) {
	src := `package test

type UI struct {
	JudgmentAnimationScaleEase string ` + "`sonolus:\"judgmentAnimationScaleEase=notAnEase\"`" + `
}

type Skin struct {
	Note int
}

type Archetype struct{}

func (a Archetype) UpdateParallel(dt float64) {}
`
	_, err := compileConfig(src)
	if err == nil {
		t.Fatal("expected error for unknown ease, got nil")
	}
}

// TestBuildUI_UnknownField verifies error on unrecognized tag key.
func TestBuildUI_UnknownField(t *testing.T) {
	src := `package test

type UI struct {
	Foo string ` + "`sonolus:\"nonsense=42\"`" + `
}

type Skin struct {
	Note int
}

type Archetype struct{}

func (a Archetype) UpdateParallel(dt float64) {}
`
	_, err := compileConfig(src)
	if err == nil {
		t.Fatal("expected error for unknown UI field, got nil")
	}
}
