package engine

import (
	"fmt"
	"go/ast"
	"reflect"
	"strings"

	"github.com/WindowsSov8forUs/sonolus-core-go/core/resource"
)

// ── Default values ────────────────────────────────────────────────────────────

func defaultVisibility() resource.EngineConfigurationVisibility {
	return resource.EngineConfigurationVisibility{Scale: 1, Alpha: 1}
}

func defaultTween() resource.EngineConfigurationAnimationTween {
	return resource.EngineConfigurationAnimationTween{
		From: 1, To: 1, Duration: 0,
		Ease: resource.EngineConfigurationAnimationTweenEaseLinear,
	}
}

// isInstructionIcon reports whether t is an InstructionIconName type (or
// compatible) rather than a Text type. Used by buildResources to route
// Instruction struct fields into Texts vs Icons.
func isInstructionIcon(t ast.Expr) bool {
	switch e := t.(type) {
	case *ast.SelectorExpr:
		return e.Sel.Name == "InstructionIconName"
	case *ast.Ident:
		return e.Name == "InstructionIconName"
	}
	return false
}

func defaultUI() resource.EngineConfigurationUI {
	vis := defaultVisibility()
	tween := defaultTween()
	return resource.EngineConfigurationUI{
		PrimaryMetric:                 resource.EngineConfigurationMetricArcade,
		SecondaryMetric:               resource.EngineConfigurationMetricAccuracy,
		MenuVisibility:                vis,
		JudgmentVisibility:            vis,
		ComboVisibility:               vis,
		PrimaryMetricVisibility:       vis,
		SecondaryMetricVisibility:     vis,
		ProgressVisibility:            vis,
		TutorialNavigationVisibility:  vis,
		TutorialInstructionVisibility: vis,
		JudgmentAnimation:             resource.EngineConfigurationAnimation{Scale: tween, Alpha: tween},
		ComboAnimation:                resource.EngineConfigurationAnimation{Scale: tween, Alpha: tween},
		JudgmentErrorStyle:            resource.EngineConfigurationJudgmentErrorStyleLate,
		JudgmentErrorPlacement:        resource.EngineConfigurationJudgmentErrorPlacementLeftRight,
		JudgmentErrorMin:              0,
	}
}

// ── Lookup maps ───────────────────────────────────────────────────────────────

// metricName maps valid EngineConfigurationMetric string values to their
// constants. Used by buildUI for validation.
var metricName = map[string]resource.EngineConfigurationMetric{
	"arcade":                  resource.EngineConfigurationMetricArcade,
	"arcadePercentage":        resource.EngineConfigurationMetricArcadePercentage,
	"accuracy":                resource.EngineConfigurationMetricAccuracy,
	"accuracyPercentage":      resource.EngineConfigurationMetricAccuracyPercentage,
	"life":                    resource.EngineConfigurationMetricLife,
	"perfect":                 resource.EngineConfigurationMetricPerfect,
	"perfectPercentage":       resource.EngineConfigurationMetricPerfectPercentage,
	"greatGoodMiss":           resource.EngineConfigurationMetricGreatGoodMiss,
	"greatGoodMissPercentage": resource.EngineConfigurationMetricGreatGoodMissPercentage,
	"miss":                    resource.EngineConfigurationMetricMiss,
	"missPercentage":          resource.EngineConfigurationMetricMissPercentage,
	"errorHeatmap":            resource.EngineConfigurationMetricErrorHeatmap,
}

// judgmentErrorStyleName maps valid EngineConfigurationJudgmentErrorStyle
// string values to their constants.
var judgmentErrorStyleName = map[string]resource.EngineConfigurationJudgmentErrorStyle{
	"none":          resource.EngineConfigurationJudgmentErrorStyleNone,
	"late":          resource.EngineConfigurationJudgmentErrorStyleLate,
	"early":         resource.EngineConfigurationJudgmentErrorStyleEarly,
	"plus":          resource.EngineConfigurationJudgmentErrorStylePlus,
	"minus":         resource.EngineConfigurationJudgmentErrorStyleMinus,
	"arrowUp":       resource.EngineConfigurationJudgmentErrorStyleArrowUp,
	"arrowDown":     resource.EngineConfigurationJudgmentErrorStyleArrowDown,
	"arrowLeft":     resource.EngineConfigurationJudgmentErrorStyleArrowLeft,
	"arrowRight":    resource.EngineConfigurationJudgmentErrorStyleArrowRight,
	"triangleUp":    resource.EngineConfigurationJudgmentErrorStyleTriangleUp,
	"triangleDown":  resource.EngineConfigurationJudgmentErrorStyleTriangleDown,
	"triangleLeft":  resource.EngineConfigurationJudgmentErrorStyleTriangleLeft,
	"triangleRight": resource.EngineConfigurationJudgmentErrorStyleTriangleRight,
}

// judgmentErrorPlacementName maps valid EngineConfigurationJudgmentErrorPlacement
// string values to their constants.
var judgmentErrorPlacementName = map[string]resource.EngineConfigurationJudgmentErrorPlacement{
	"left":      resource.EngineConfigurationJudgmentErrorPlacementLeft,
	"right":     resource.EngineConfigurationJudgmentErrorPlacementRight,
	"leftRight": resource.EngineConfigurationJudgmentErrorPlacementLeftRight,
	"top":       resource.EngineConfigurationJudgmentErrorPlacementTop,
	"bottom":    resource.EngineConfigurationJudgmentErrorPlacementBottom,
	"topBottom": resource.EngineConfigurationJudgmentErrorPlacementTopBottom,
	"center":    resource.EngineConfigurationJudgmentErrorPlacementCenter,
}

// easeName maps valid EngineConfigurationAnimationTweenEase string values to
// their constants.
var easeName = map[string]resource.EngineConfigurationAnimationTweenEase{
	"linear":       resource.EngineConfigurationAnimationTweenEaseLinear,
	"inSine":       resource.EngineConfigurationAnimationTweenEaseInSine,
	"outSine":      resource.EngineConfigurationAnimationTweenEaseOutSine,
	"inOutSine":    resource.EngineConfigurationAnimationTweenEaseInOutSine,
	"outInSine":    resource.EngineConfigurationAnimationTweenEaseOutInSine,
	"inQuad":       resource.EngineConfigurationAnimationTweenEaseInQuad,
	"outQuad":      resource.EngineConfigurationAnimationTweenEaseOutQuad,
	"inOutQuad":    resource.EngineConfigurationAnimationTweenEaseInOutQuad,
	"outInQuad":    resource.EngineConfigurationAnimationTweenEaseOutInQuad,
	"inCubic":      resource.EngineConfigurationAnimationTweenEaseInCubic,
	"outCubic":     resource.EngineConfigurationAnimationTweenEaseOutCubic,
	"inOutCubic":   resource.EngineConfigurationAnimationTweenEaseInOutCubic,
	"outInCubic":   resource.EngineConfigurationAnimationTweenEaseOutInCubic,
	"inQuart":      resource.EngineConfigurationAnimationTweenEaseInQuart,
	"outQuart":     resource.EngineConfigurationAnimationTweenEaseOutQuart,
	"inOutQuart":   resource.EngineConfigurationAnimationTweenEaseInOutQuart,
	"outInQuart":   resource.EngineConfigurationAnimationTweenEaseOutInQuart,
	"inQuint":      resource.EngineConfigurationAnimationTweenEaseInQuint,
	"outQuint":     resource.EngineConfigurationAnimationTweenEaseOutQuint,
	"inOutQuint":   resource.EngineConfigurationAnimationTweenEaseInOutQuint,
	"outInQuint":   resource.EngineConfigurationAnimationTweenEaseOutInQuint,
	"inExpo":       resource.EngineConfigurationAnimationTweenEaseInExpo,
	"outExpo":      resource.EngineConfigurationAnimationTweenEaseOutExpo,
	"inOutExpo":    resource.EngineConfigurationAnimationTweenEaseInOutExpo,
	"outInExpo":    resource.EngineConfigurationAnimationTweenEaseOutInExpo,
	"inCirc":       resource.EngineConfigurationAnimationTweenEaseInCirc,
	"outCirc":      resource.EngineConfigurationAnimationTweenEaseOutCirc,
	"inOutCirc":    resource.EngineConfigurationAnimationTweenEaseInOutCirc,
	"outInCirc":    resource.EngineConfigurationAnimationTweenEaseOutInCirc,
	"inBack":       resource.EngineConfigurationAnimationTweenEaseInBack,
	"outBack":      resource.EngineConfigurationAnimationTweenEaseOutBack,
	"inOutBack":    resource.EngineConfigurationAnimationTweenEaseInOutBack,
	"outInBack":    resource.EngineConfigurationAnimationTweenEaseOutInBack,
	"inElastic":    resource.EngineConfigurationAnimationTweenEaseInElastic,
	"outElastic":   resource.EngineConfigurationAnimationTweenEaseOutElastic,
	"inOutElastic": resource.EngineConfigurationAnimationTweenEaseInOutElastic,
	"outInElastic": resource.EngineConfigurationAnimationTweenEaseOutInElastic,
	"none":         resource.EngineConfigurationAnimationTweenEaseNone,
}

// ── UI setter infrastructure ──────────────────────────────────────────────────

// uiField returns an accessor that extracts a pointer to a specific field from
// a given *resource.EngineConfigurationUI. Each setter calls the accessor on
// the actual UI instance before writing, so values go to the right struct.
type uiField[T any] func(ui *resource.EngineConfigurationUI) *T

// uiSetFloat creates a setter for a float64-valued UI field.
func uiSetFloat(field uiField[float64]) func(*resource.EngineConfigurationUI, string) error {
	return func(ui *resource.EngineConfigurationUI, v string) error {
		f, err := parseFloat(v)
		if err != nil {
			return err
		}
		*field(ui) = f
		return nil
	}
}

// uiSetEnum creates a setter for an enum-valued UI field that is looked up
// from a name→value map. name is used in error messages (e.g. "ease").
func uiSetEnum[T comparable](field uiField[T], lookup map[string]T, name string) func(*resource.EngineConfigurationUI, string) error {
	return func(ui *resource.EngineConfigurationUI, v string) error {
		val, ok := lookup[v]
		if !ok {
			return fmt.Errorf("engine: unknown %s %q", name, v)
		}
		*field(ui) = val
		return nil
	}
}

// uiSetters maps each sonolus UI tag key to its setter function.
var uiSetters = buildUISetters()

func buildUISetters() map[string]func(*resource.EngineConfigurationUI, string) error {
	m := map[string]func(*resource.EngineConfigurationUI, string) error{}

	// Metrics
	m["primaryMetric"] = uiSetEnum(func(ui *resource.EngineConfigurationUI) *resource.EngineConfigurationMetric { return &ui.PrimaryMetric }, metricName, "metric")
	m["secondaryMetric"] = uiSetEnum(func(ui *resource.EngineConfigurationUI) *resource.EngineConfigurationMetric {
		return &ui.SecondaryMetric
	}, metricName, "metric")

	// Judgment error
	m["judgmentErrorStyle"] = uiSetEnum(func(ui *resource.EngineConfigurationUI) *resource.EngineConfigurationJudgmentErrorStyle {
		return &ui.JudgmentErrorStyle
	}, judgmentErrorStyleName, "judgmentErrorStyle")
	m["judgmentErrorPlacement"] = uiSetEnum(func(ui *resource.EngineConfigurationUI) *resource.EngineConfigurationJudgmentErrorPlacement {
		return &ui.JudgmentErrorPlacement
	}, judgmentErrorPlacementName, "judgmentErrorPlacement")
	m["judgmentErrorMin"] = uiSetFloat(func(ui *resource.EngineConfigurationUI) *float64 { return &ui.JudgmentErrorMin })

	// Visibility
	m["menuVisibilityScale"] = uiSetFloat(func(ui *resource.EngineConfigurationUI) *float64 { return &ui.MenuVisibility.Scale })
	m["menuVisibilityAlpha"] = uiSetFloat(func(ui *resource.EngineConfigurationUI) *float64 { return &ui.MenuVisibility.Alpha })
	m["judgmentVisibilityScale"] = uiSetFloat(func(ui *resource.EngineConfigurationUI) *float64 { return &ui.JudgmentVisibility.Scale })
	m["judgmentVisibilityAlpha"] = uiSetFloat(func(ui *resource.EngineConfigurationUI) *float64 { return &ui.JudgmentVisibility.Alpha })
	m["comboVisibilityScale"] = uiSetFloat(func(ui *resource.EngineConfigurationUI) *float64 { return &ui.ComboVisibility.Scale })
	m["comboVisibilityAlpha"] = uiSetFloat(func(ui *resource.EngineConfigurationUI) *float64 { return &ui.ComboVisibility.Alpha })
	m["primaryMetricVisibilityScale"] = uiSetFloat(func(ui *resource.EngineConfigurationUI) *float64 { return &ui.PrimaryMetricVisibility.Scale })
	m["primaryMetricVisibilityAlpha"] = uiSetFloat(func(ui *resource.EngineConfigurationUI) *float64 { return &ui.PrimaryMetricVisibility.Alpha })
	m["secondaryMetricVisibilityScale"] = uiSetFloat(func(ui *resource.EngineConfigurationUI) *float64 { return &ui.SecondaryMetricVisibility.Scale })
	m["secondaryMetricVisibilityAlpha"] = uiSetFloat(func(ui *resource.EngineConfigurationUI) *float64 { return &ui.SecondaryMetricVisibility.Alpha })
	m["progressVisibilityScale"] = uiSetFloat(func(ui *resource.EngineConfigurationUI) *float64 { return &ui.ProgressVisibility.Scale })
	m["progressVisibilityAlpha"] = uiSetFloat(func(ui *resource.EngineConfigurationUI) *float64 { return &ui.ProgressVisibility.Alpha })
	m["tutorialNavigationVisibilityScale"] = uiSetFloat(func(ui *resource.EngineConfigurationUI) *float64 { return &ui.TutorialNavigationVisibility.Scale })
	m["tutorialNavigationVisibilityAlpha"] = uiSetFloat(func(ui *resource.EngineConfigurationUI) *float64 { return &ui.TutorialNavigationVisibility.Alpha })
	m["tutorialInstructionVisibilityScale"] = uiSetFloat(func(ui *resource.EngineConfigurationUI) *float64 { return &ui.TutorialInstructionVisibility.Scale })
	m["tutorialInstructionVisibilityAlpha"] = uiSetFloat(func(ui *resource.EngineConfigurationUI) *float64 { return &ui.TutorialInstructionVisibility.Alpha })

	// Judgment animation
	m["judgmentAnimationScaleFrom"] = uiSetFloat(func(ui *resource.EngineConfigurationUI) *float64 { return &ui.JudgmentAnimation.Scale.From })
	m["judgmentAnimationScaleTo"] = uiSetFloat(func(ui *resource.EngineConfigurationUI) *float64 { return &ui.JudgmentAnimation.Scale.To })
	m["judgmentAnimationScaleDuration"] = uiSetFloat(func(ui *resource.EngineConfigurationUI) *float64 { return &ui.JudgmentAnimation.Scale.Duration })
	m["judgmentAnimationScaleEase"] = uiSetEnum(func(ui *resource.EngineConfigurationUI) *resource.EngineConfigurationAnimationTweenEase {
		return &ui.JudgmentAnimation.Scale.Ease
	}, easeName, "ease")
	m["judgmentAnimationAlphaFrom"] = uiSetFloat(func(ui *resource.EngineConfigurationUI) *float64 { return &ui.JudgmentAnimation.Alpha.From })
	m["judgmentAnimationAlphaTo"] = uiSetFloat(func(ui *resource.EngineConfigurationUI) *float64 { return &ui.JudgmentAnimation.Alpha.To })
	m["judgmentAnimationAlphaDuration"] = uiSetFloat(func(ui *resource.EngineConfigurationUI) *float64 { return &ui.JudgmentAnimation.Alpha.Duration })
	m["judgmentAnimationAlphaEase"] = uiSetEnum(func(ui *resource.EngineConfigurationUI) *resource.EngineConfigurationAnimationTweenEase {
		return &ui.JudgmentAnimation.Alpha.Ease
	}, easeName, "ease")

	// Combo animation
	m["comboAnimationScaleFrom"] = uiSetFloat(func(ui *resource.EngineConfigurationUI) *float64 { return &ui.ComboAnimation.Scale.From })
	m["comboAnimationScaleTo"] = uiSetFloat(func(ui *resource.EngineConfigurationUI) *float64 { return &ui.ComboAnimation.Scale.To })
	m["comboAnimationScaleDuration"] = uiSetFloat(func(ui *resource.EngineConfigurationUI) *float64 { return &ui.ComboAnimation.Scale.Duration })
	m["comboAnimationScaleEase"] = uiSetEnum(func(ui *resource.EngineConfigurationUI) *resource.EngineConfigurationAnimationTweenEase {
		return &ui.ComboAnimation.Scale.Ease
	}, easeName, "ease")
	m["comboAnimationAlphaFrom"] = uiSetFloat(func(ui *resource.EngineConfigurationUI) *float64 { return &ui.ComboAnimation.Alpha.From })
	m["comboAnimationAlphaTo"] = uiSetFloat(func(ui *resource.EngineConfigurationUI) *float64 { return &ui.ComboAnimation.Alpha.To })
	m["comboAnimationAlphaDuration"] = uiSetFloat(func(ui *resource.EngineConfigurationUI) *float64 { return &ui.ComboAnimation.Alpha.Duration })
	m["comboAnimationAlphaEase"] = uiSetEnum(func(ui *resource.EngineConfigurationUI) *resource.EngineConfigurationAnimationTweenEase {
		return &ui.ComboAnimation.Alpha.Ease
	}, easeName, "ease")

	return m
}

// buildUI parses a UI struct from the engine source and returns an
// EngineConfigurationUI. If uiLit is non-nil, field values are read from
// the composite literal (typed mode); otherwise from sonolus:"key=value" tags.
func buildUI(st *ast.StructType, uiLit *ast.CompositeLit) (resource.EngineConfigurationUI, error) {
	ui := defaultUI()

	// Typed mode: evaluate the composite literal.
	if uiLit != nil {
		ctx := newUIEvalContext()
		vals, err := evaluateUIConfig(st, uiLit, ctx)
		if err != nil {
			return resource.EngineConfigurationUI{}, fmt.Errorf("UI: %w", err)
		}
		for k, v := range vals {
			set, ok := uiSetters[k]
			if !ok {
				return resource.EngineConfigurationUI{}, fmt.Errorf("engine: unknown UI field %q", k)
			}
			if err := set(&ui, v); err != nil {
				return resource.EngineConfigurationUI{}, fmt.Errorf("UI field %q: %w", k, err)
			}
		}
		return ui, nil
	}

	// String mode: parse sonolus:"key=value" tags.
	for _, f := range st.Fields.List {
		if f.Tag == nil || len(f.Names) == 0 {
			continue
		}
		tag := reflect.StructTag(stringLit(f.Tag.Value)).Get("sonolus")
		if tag == "" {
			continue
		// RuntimeUiConfig: "ui" tag expands to field.scale + field.alpha
		if tag == "ui" && resolveFieldTypeName(f.Type) == "RuntimeUiConfig" {
			name := f.Names[0].Name
			key := strings.ToLower(name[:1]) + name[1:]
			for _, sf := range []string{"scale", "alpha"} {
				if set, ok := uiSetters[key+"."+sf]; ok {
					set(&ui, "1.0")
				}
			}
			continue
		}
		}
		k, v := keyVal(tag)
		if k == "" {
			continue
		}
		set, ok := uiSetters[k]
		if !ok {
			return resource.EngineConfigurationUI{}, fmt.Errorf("engine: unknown UI field %q", k)
		}
		if err := set(&ui, v); err != nil {
			return resource.EngineConfigurationUI{}, fmt.Errorf("UI field %q: %w", k, err)
		}
	}
	return ui, nil
}
