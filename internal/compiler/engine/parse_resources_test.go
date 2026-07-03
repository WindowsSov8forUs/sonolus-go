package engine

import (
	"go/ast"
	"go/parser"
	"go/token"
	"testing"

	"github.com/WindowsSov8forUs/sonolus-core-go/core/resource"
)

// parseStructType parses a Go source string containing a single struct
// definition and returns the *ast.StructType. The source must contain
// exactly one type declaration.
func parseStructType(t *testing.T, src string) *ast.StructType {
	t.Helper()
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "test.go", "package p\n"+src, 0)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	for _, d := range f.Decls {
		if gd, ok := d.(*ast.GenDecl); ok && gd.Tok == token.TYPE {
			if ts, ok := gd.Specs[0].(*ast.TypeSpec); ok {
				return ts.Type.(*ast.StructType)
			}
		}
	}
	t.Fatal("no struct type found in source")
	return nil
}

// --- buildConfig tests ---

func TestBuildConfig_Empty(t *testing.T) {
	st := parseStructType(t, "type C struct{}")
	cfg, err := buildConfig(st)
	if err != nil {
		t.Fatalf("buildConfig: %v", err)
	}
	if len(cfg.Options) != 0 {
		t.Errorf("expected 0 options, got %d", len(cfg.Options))
	}
}

func TestBuildConfig_SliderOption(t *testing.T) {
	st := parseStructType(t, `type C struct {
		Speed float64 `+"`sonolus:\"slider,min=0,max=2,step=0.1,def=1\"`"+`
	}`)
	cfg, err := buildConfig(st)
	if err != nil {
		t.Fatalf("buildConfig: %v", err)
	}
	if len(cfg.Options) != 1 {
		t.Fatalf("expected 1 option, got %d", len(cfg.Options))
	}
	opt, ok := cfg.Options[0].(resource.EngineConfigurationSliderOption)
	if !ok {
		t.Fatalf("expected SliderOption, got %T", cfg.Options[0])
	}
	if opt.Name != "Speed" {
		t.Errorf("name = %q, want Speed", opt.Name)
	}
	if opt.Min != 0 || opt.Max != 2 || opt.Step != 0.1 || opt.Def != 1 {
		t.Errorf("slider params = min=%.1f max=%.1f step=%.1f def=%.1f, want 0/2/0.1/1",
			opt.Min, opt.Max, opt.Step, opt.Def)
	}
	if !opt.Standard {
		t.Error("expected Standard to be true")
	}
}

func TestBuildConfig_ToggleOption(t *testing.T) {
	st := parseStructType(t, `type C struct {
		Enable float64 `+"`sonolus:\"toggle,def=1\"`"+`
	}`)
	cfg, err := buildConfig(st)
	if err != nil {
		t.Fatalf("buildConfig: %v", err)
	}
	if len(cfg.Options) != 1 {
		t.Fatalf("expected 1 option, got %d", len(cfg.Options))
	}
	opt, ok := cfg.Options[0].(resource.EngineConfigurationToggleOption)
	if !ok {
		t.Fatalf("expected ToggleOption, got %T", cfg.Options[0])
	}
	if opt.Name != "Enable" {
		t.Errorf("name = %q, want Enable", opt.Name)
	}
	if opt.Def != 1 {
		t.Errorf("def = %d, want 1", opt.Def)
	}
}

func TestBuildConfig_SelectOption(t *testing.T) {
	st := parseStructType(t, `type C struct {
		Style float64 `+"`sonolus:\"select,values=a,def=1\"`"+`
	}`)
	cfg, err := buildConfig(st)
	if err != nil {
		t.Fatalf("buildConfig: %v", err)
	}
	if len(cfg.Options) != 1 {
		t.Fatalf("expected 1 option, got %d", len(cfg.Options))
	}
	opt, ok := cfg.Options[0].(resource.EngineConfigurationSelectOption)
	if !ok {
		t.Fatalf("expected SelectOption, got %T", cfg.Options[0])
	}
	if opt.Name != "Style" {
		t.Errorf("name = %q, want Style", opt.Name)
	}
	if len(opt.Values) != 1 {
		t.Errorf("expected 1 value, got %d: %v", len(opt.Values), opt.Values)
	}
	if opt.Def != 1 {
		t.Errorf("def = %d, want 1", opt.Def)
	}
}

func TestBuildConfig_AdvancedOption(t *testing.T) {
	st := parseStructType(t, `type C struct {
		Extra float64 `+"`sonolus:\"toggle,def=0,advanced,scope=extra\"`"+`
	}`)
	cfg, err := buildConfig(st)
	if err != nil {
		t.Fatalf("buildConfig: %v", err)
	}
	opt, ok := cfg.Options[0].(resource.EngineConfigurationToggleOption)
	if !ok {
		t.Fatalf("expected ToggleOption, got %T", cfg.Options[0])
	}
	if !opt.Advanced {
		t.Error("expected Advanced to be true")
	}
	if opt.Scope != "extra" {
		t.Errorf("scope = %q, want extra", opt.Scope)
	}
}

func TestBuildConfig_SkipsUntagged(t *testing.T) {
	st := parseStructType(t, `type C struct {
		A float64 `+"`sonolus:\"slider,def=0.5\"`"+`
		B float64
	}`)
	cfg, err := buildConfig(st)
	if err != nil {
		t.Fatalf("buildConfig: %v", err)
	}
	if len(cfg.Options) != 1 {
		t.Errorf("expected 1 option (untagged field skipped), got %d", len(cfg.Options))
	}
}

func TestBuildConfig_MultipleOptions(t *testing.T) {
	st := parseStructType(t, `type C struct {
		Vol  float64 `+"`sonolus:\"slider,min=0,max=1,def=0.8\"`"+`
		Mute float64 `+"`sonolus:\"toggle,def=0\"`"+`
	}`)
	cfg, err := buildConfig(st)
	if err != nil {
		t.Fatalf("buildConfig: %v", err)
	}
	if len(cfg.Options) != 2 {
		t.Errorf("expected 2 options, got %d", len(cfg.Options))
	}
}

func TestBuildConfig_ReplayFallbackOptionNames(t *testing.T) {
	st := parseStructType(t, `type C struct {
		Speed       float64 `+"`sonolus:\"slider,min=0,max=2,def=1\"`"+`
		ReplaySpeed string  `+"`sonolus:\"replayFallback\"`"+`
	}`)
	cfg, err := buildConfig(st)
	if err != nil {
		t.Fatalf("buildConfig: %v", err)
	}
	if len(cfg.Options) != 1 {
		t.Errorf("expected 1 option, got %d", len(cfg.Options))
	}
	if len(cfg.ReplayFallbackOptionNames) != 1 {
		t.Fatalf("expected 1 replay fallback name, got %d", len(cfg.ReplayFallbackOptionNames))
	}
	if cfg.ReplayFallbackOptionNames[0] != "ReplaySpeed" {
		t.Errorf("replay fallback name = %q, want ReplaySpeed", cfg.ReplayFallbackOptionNames[0])
	}
}

// --- isInstructionIcon tests ---

func TestIsInstructionIcon_SelectorExpr(t *testing.T) {
	// Parse a selector expression: sonolus.InstructionIconName
	expr, err := parser.ParseExpr("sonolus.InstructionIconName")
	if err != nil {
		t.Fatal(err)
	}
	if !isInstructionIcon(expr) {
		t.Error("expected SelectorExpr with InstructionIconName to be recognized")
	}
}

func TestIsInstructionIcon_Ident(t *testing.T) {
	expr, err := parser.ParseExpr("InstructionIconName")
	if err != nil {
		t.Fatal(err)
	}
	if !isInstructionIcon(expr) {
		t.Error("expected Ident with InstructionIconName to be recognized")
	}
}

func TestIsInstructionIcon_NotIcon(t *testing.T) {
	expr, err := parser.ParseExpr("RegularFloat")
	if err != nil {
		t.Fatal(err)
	}
	if isInstructionIcon(expr) {
		t.Error("expected RegularFloat to NOT be recognized as instruction icon")
	}
}

func TestIsInstructionIcon_WrongSelector(t *testing.T) {
	expr, err := parser.ParseExpr("sonolus.NotAnIcon")
	if err != nil {
		t.Fatal(err)
	}
	if isInstructionIcon(expr) {
		t.Error("expected wrong selector to NOT be recognized as instruction icon")
	}
}

// --- buildUI tests ---

func TestBuildUI_Empty(t *testing.T) {
	st := parseStructType(t, "type U struct{}")
	ui, err := buildUI(st, nil)
	if err != nil {
		t.Fatalf("buildUI: %v", err)
	}
	// An empty struct with no tagged fields produces a zero-value UI.
	if ui.Scope != "" {
		t.Errorf("expected empty scope, got %q", ui.Scope)
	}
}

func TestBuildUI_Metrics(t *testing.T) {
	st := parseStructType(t, `type U struct {
		PrimaryMetric  float64 `+"`sonolus:\"primaryMetric=arcadePercentage\"`"+`
		SecondaryMetric float64 `+"`sonolus:\"secondaryMetric=accuracyPercentage\"`"+`
	}`)
	ui, err := buildUI(st, nil)
	if err != nil {
		t.Fatalf("buildUI: %v", err)
	}
	if ui.PrimaryMetric != resource.EngineConfigurationMetricArcadePercentage {
		t.Errorf("primaryMetric = %v, want arcadePercentage", ui.PrimaryMetric)
	}
}

func TestBuildUI_Visibility(t *testing.T) {
	st := parseStructType(t, `type U struct {
		Visible float64 `+"`sonolus:\"menuVisibilityScale=1.0\"`"+`
	}`)
	ui, err := buildUI(st, nil)
	if err != nil {
		t.Fatalf("buildUI: %v", err)
	}
	if ui.MenuVisibility.Scale != 1.0 {
		t.Errorf("expected MenuVisibility.Scale=1.0, got %.1f", ui.MenuVisibility.Scale)
	}
}

// --- buildBuckets tests ---

func TestBuildBuckets_Empty(t *testing.T) {
	st := parseStructType(t, "type B struct{}")
	skinST := parseStructType(t, "type S struct{}")
	buckets, err := buildBuckets(st, skinST)
	if err != nil {
		t.Fatalf("buildBuckets: %v", err)
	}
	if len(buckets) != 0 {
		t.Errorf("expected 0 buckets, got %d", len(buckets))
	}
}

func TestBuildBuckets_SingleBucket(t *testing.T) {
	st := parseStructType(t, `type B struct {
		TapNote struct {
			Head float64 `+"`sonolus:\"sprite=TapNote\"`"+`
		} `+"`sonolus:\"bucket\"`"+`
	}`)
	// Buckets reference skin sprites by name.
	skinST := parseStructType(t, `type S struct {
		TapNote float64 `+"`sonolus:\"sprite\"`"+`
	}`)
	buckets, err := buildBuckets(st, skinST)
	if err != nil {
		t.Fatalf("buildBuckets: %v", err)
	}
	if len(buckets) != 1 {
		t.Fatalf("expected 1 bucket, got %d", len(buckets))
	}
	if buckets[0].Unit != "" {
		t.Logf("unit = %q", buckets[0].Unit)
	}
}

func TestBuildBuckets_SpriteCoordinates(t *testing.T) {
	// Verify sprite tag parsing: sprite name, coordinates, fallback, and bucket unit.
	st := parseStructType(t, `type B struct {
		Notes struct {
			Head float64 `+"`sonolus:\"sprite=NOTE_HEAD_NEUTRAL,x=100,y=200,w=64,h=64,rotation=45,fallback=NOTE_TICK_NEUTRAL\"`"+`
		} `+"`sonolus:\"bucket,unit=#BPM\"`"+`
	}`)
	skinST := parseStructType(t, `type S struct {
		NOTE_HEAD_NEUTRAL float64 `+"`sonolus:\"sprite\"`"+`
		NOTE_TICK_NEUTRAL float64 `+"`sonolus:\"sprite\"`"+`
	}`)
	buckets, err := buildBuckets(st, skinST)
	if err != nil {
		t.Fatalf("buildBuckets: %v", err)
	}
	if len(buckets) != 1 {
		t.Fatalf("expected 1 bucket, got %d", len(buckets))
	}
	b := buckets[0]
	if b.Unit != "#BPM" {
		t.Errorf("unit = %q, want #BPM", b.Unit)
	}
	if len(b.Sprites) != 1 {
		t.Fatalf("expected 1 sprite, got %d", len(b.Sprites))
	}
	s := b.Sprites[0]
	if s.ID != 0 {
		t.Errorf("sprite ID = %d, want 0 (NOTE_HEAD_NEUTRAL)", s.ID)
	}
	if s.FallbackID != 1 {
		t.Errorf("fallback sprite ID = %d, want 1 (NOTE_TICK_NEUTRAL)", s.FallbackID)
	}
	if s.X != 100 || s.Y != 200 || s.W != 64 || s.H != 64 {
		t.Errorf("rect = (%.0f,%.0f,%.0f,%.0f), want (100,200,64,64)", s.X, s.Y, s.W, s.H)
	}
	if s.Rotation != 45 {
		t.Errorf("rotation = %.0f, want 45", s.Rotation)
	}
}

func TestBuildBuckets_MultipleSprites(t *testing.T) {
	st := parseStructType(t, `type B struct {
		Notes struct {
			Head float64 `+"`sonolus:\"sprite=NOTE_HEAD_NEUTRAL,x=0,y=0\"`"+`
			Tail float64 `+"`sonolus:\"sprite=NOTE_TAIL_NEUTRAL,x=64,y=0\"`"+`
		} `+"`sonolus:\"bucket\"`"+`
	}`)
	skinST := parseStructType(t, `type S struct {
		NOTE_HEAD_NEUTRAL float64 `+"`sonolus:\"sprite\"`"+`
		NOTE_TAIL_NEUTRAL float64 `+"`sonolus:\"sprite\"`"+`
	}`)
	buckets, err := buildBuckets(st, skinST)
	if err != nil {
		t.Fatalf("buildBuckets: %v", err)
	}
	if len(buckets) != 1 {
		t.Fatalf("expected 1 bucket, got %d", len(buckets))
	}
	if len(buckets[0].Sprites) != 2 {
		t.Fatalf("expected 2 sprites, got %d", len(buckets[0].Sprites))
	}
}
