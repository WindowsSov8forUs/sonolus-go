package engine

import (
	"fmt"
	"go/ast"
	"reflect"
	"strconv"

	"github.com/WindowsSov8forUs/sonolus-core-go/core"
	"github.com/WindowsSov8forUs/sonolus-core-go/core/resource"
)

func resourceRole(name string) string {
	switch name {
	case "Skin":
		return "skin"
	case "Effect":
		return "effect"
	case "Particle":
		return "particle"
	case "Buckets":
		return "buckets"
	case "Config":
		return "config"
	case "Instruction":
		return "instruction"
	case "UI":
		return "ui"
	}
	return ""
}

type parsedResources struct {
	skin        resource.EngineSkinData
	effect      resource.EngineEffectData
	particle    resource.EngineParticleData
	buckets     []resource.EngineDataBucket
	config      resource.EngineConfiguration
	instruction resource.EngineInstructionData
	ui          *resource.EngineConfigurationUI // non-nil if a UI struct was parsed
}

func buildResources(typeSpecs map[string]*ast.StructType) (parsedResources, error) {
	var r parsedResources

	for name, st := range typeSpecs {
		switch resourceRole(name) {
		case "skin":
			r.skin.RenderMode = skinRenderMode(st)
			for _, f := range st.Fields.List {
				for _, n := range f.Names {
					r.skin.Sprites = append(r.skin.Sprites,
						resource.EngineSkinDataSprite{Name: resource.SkinSpriteName(n.Name), ID: len(r.skin.Sprites)})
				}
			}
		case "effect":
			for _, f := range st.Fields.List {
				for _, n := range f.Names {
					r.effect.Clips = append(r.effect.Clips,
						resource.EngineEffectDataClip{Name: resource.EffectClipName(n.Name), ID: len(r.effect.Clips)})
				}
			}
		case "particle":
			for _, f := range st.Fields.List {
				for _, n := range f.Names {
					r.particle.Effects = append(r.particle.Effects,
						resource.EngineParticleDataEffect{Name: resource.ParticleEffectName(n.Name), ID: len(r.particle.Effects)})
				}
			}
		case "buckets":
			var err error
			r.buckets, err = buildBuckets(st, typeSpecs["Skin"])
			if err != nil {
				return parsedResources{}, err
			}
		case "instruction":
			for _, f := range st.Fields.List {
				for _, n := range f.Names {
					if isInstructionIcon(f.Type) {
						r.instruction.Icons = append(r.instruction.Icons,
							resource.EngineInstructionDataIcon{Name: resource.InstructionIconName(n.Name), ID: len(r.instruction.Icons)})
					} else {
						r.instruction.Texts = append(r.instruction.Texts,
							resource.EngineInstructionDataText{Name: core.Text(n.Name), ID: len(r.instruction.Texts)})
					}
				}
			}
		case "config":
			var err error
			r.config, err = buildConfig(st)
			if err != nil {
				return parsedResources{}, err
			}
		case "ui":
			ui, err := buildUI(st)
			if err != nil {
				return parsedResources{}, err
			}
			r.ui = &ui
		}
	}
	// If the author provided a UI struct, its values override the defaults;
	// otherwise fall back to defaultUI() so zero-valued visibility/animation
	// don't break client rendering.
	if r.ui != nil {
		r.config.UI = *r.ui
	} else if r.config.UI.PrimaryMetric == "" {
		r.config.UI = defaultUI()
	}
	return r, nil
}

func buildConfig(st *ast.StructType) (resource.EngineConfiguration, error) {
	var opts []resource.EngineConfigurationOption
	for _, f := range st.Fields.List {
		if f.Tag == nil || len(f.Names) == 0 {
			continue
		}
		tag := reflect.StructTag(stringLit(f.Tag.Value)).Get("sonolus")
		if tag == "" {
			continue
		}
		tagVals := splitTag(tag)
		base := resource.EngineConfigurationOptionBase{
			Name:     core.Text(f.Names[0].Name),
			Standard: true,
			Advanced: hasTag(tagVals, "advanced"),
			Scope:    tagVal(tagVals, "scope"),
		}

		switch kind := tagVals[0]; kind {
		case "slider":
			min, err := parseFloatParam(tagVals, "min", 0)
			if err != nil {
				return resource.EngineConfiguration{}, fmt.Errorf("field %q min: %w", f.Names[0].Name, err)
			}
			max, err := parseFloatParam(tagVals, "max", 1)
			if err != nil {
				return resource.EngineConfiguration{}, fmt.Errorf("field %q max: %w", f.Names[0].Name, err)
			}
			step, err := parseFloatParam(tagVals, "step", 0.01)
			if err != nil {
				return resource.EngineConfiguration{}, fmt.Errorf("field %q step: %w", f.Names[0].Name, err)
			}
			def, err := parseFloatParam(tagVals, "def", 0)
			if err != nil {
				return resource.EngineConfiguration{}, fmt.Errorf("field %q def: %w", f.Names[0].Name, err)
			}
			opts = append(opts, resource.EngineConfigurationSliderOption{
				EngineConfigurationOptionBase: base,
				Type:                          resource.EngineConfigurationOptionTypeSlider,
				Min:                           min,
				Max:                           max,
				Step:                          step,
				Def:                           def,
			})
		case "toggle":
			def, err := parseFloatParam(tagVals, "def", 1)
			if err != nil {
				return resource.EngineConfiguration{}, fmt.Errorf("field %q def: %w", f.Names[0].Name, err)
			}
			opts = append(opts, resource.EngineConfigurationToggleOption{
				EngineConfigurationOptionBase: base,
				Type:                          resource.EngineConfigurationOptionTypeToggle,
				Def:                           int(def),
			})
		case "select":
			var vals []core.Text
			for _, v := range splitTag(tagVal(tagVals, "values")) {
				vals = append(vals, core.Text(v))
			}
			if len(vals) == 0 {
				vals = append(vals, core.Text("value1"))
			}
			def, err := parseFloatParam(tagVals, "def", 0)
			if err != nil {
				return resource.EngineConfiguration{}, fmt.Errorf("field %q def: %w", f.Names[0].Name, err)
			}
			opts = append(opts, resource.EngineConfigurationSelectOption{
				EngineConfigurationOptionBase: base,
				Type:                          resource.EngineConfigurationOptionTypeSelect,
				Values:                        vals,
				Def:                           int(def),
			})
		}
	}
	return resource.EngineConfiguration{Options: opts, UI: defaultUI()}, nil
}

func hasTag(tags []string, key string) bool {
	prefix := key + "="
	for _, t := range tags {
		if t == key {
			return true
		}
		if len(t) > len(prefix) && t[:len(prefix)] == prefix {
			return true
		}
	}
	return false
}

func tagVal(tags []string, key string) string {
	prefix := key + "="
	for _, t := range tags {
		if len(t) > len(prefix) && t[:len(prefix)] == prefix {
			return t[len(prefix):]
		}
	}
	return ""
}

func parseFloatParam(tags []string, key string, def float64) (float64, error) {
	v := tagVal(tags, key)
	if v == "" {
		return def, nil
	}
	return parseFloat(v)
}

func buildBuckets(st *ast.StructType, skinST *ast.StructType) ([]resource.EngineDataBucket, error) {
	var out []resource.EngineDataBucket
	for _, f := range st.Fields.List {
		tag := reflect.StructTag(stringLit(f.Tag.Value)).Get("sonolus")
		tagParts := splitTag(tag)
		if len(tagParts) == 0 || tagParts[0] != "bucket" || len(f.Names) == 0 {
			continue
		}
		var sprites []resource.EngineDataBucketSprite
		if ft, ok := f.Type.(*ast.StructType); ok {
			for _, sf := range ft.Fields.List {
				for range sf.Names {
					spr := resource.EngineDataBucketSprite{}
					if stag, ok := reflect.StructTag(stringLit(sf.Tag.Value)).Lookup("sonolus"); ok {
						for _, p := range splitTag(stag) {
							k, v := keyVal(p)
							switch k {
							case "sprite":
								id, err := spriteID(skinST, v)
								if err != nil {
									return nil, fmt.Errorf("bucket sprite: %w", err)
								}
								spr.ID = id
							case "x":
								var err error
								spr.X, err = parseFloat(v)
								if err != nil {
									return nil, err
								}
							case "y":
								var err error
								spr.Y, err = parseFloat(v)
								if err != nil {
									return nil, err
								}
							case "w":
								var err error
								spr.W, err = parseFloat(v)
								if err != nil {
									return nil, err
								}
							case "h":
								var err error
								spr.H, err = parseFloat(v)
								if err != nil {
									return nil, err
								}
							case "rotation":
								var err error
								spr.Rotation, err = parseFloat(v)
								if err != nil {
									return nil, err
								}
							case "fallback":
								id, err := spriteID(skinST, v)
								if err != nil {
									return nil, fmt.Errorf("bucket fallback sprite: %w", err)
								}
								spr.FallbackID = id
							}
						}
					}
					sprites = append(sprites, spr)
				}
			}
		}
		bucket := resource.EngineDataBucket{Sprites: sprites}
		if unit := tagVal(tagParts, "unit"); unit != "" {
			bucket.Unit = core.Text(unit)
		}
		out = append(out, bucket)
	}
	return out, nil
}

func splitTag(s string) []string {
	var parts []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == ',' {
			parts = append(parts, s[start:i])
			start = i + 1
		}
	}
	parts = append(parts, s[start:])
	return parts
}

func keyVal(s string) (string, string) {
	for i := 0; i < len(s); i++ {
		if s[i] == '=' {
			return s[:i], s[i+1:]
		}
	}
	return s, ""
}

// skinRenderMode reads the renderMode tag on the Skin struct, defaulting to
// lightweight. Accepted values: default, standard, lightweight.
func skinRenderMode(st *ast.StructType) resource.EngineRenderMode {
	for _, f := range st.Fields.List {
		if f.Tag == nil || len(f.Names) == 0 {
			continue
		}
		tag := reflect.StructTag(stringLit(f.Tag.Value)).Get("sonolus")
		if tag == "" {
			continue
		}
		tagParts := splitTag(tag)
		if hasTag(tagParts, "renderMode") {
			switch tagVal(tagParts, "renderMode") {
			case "default":
				return resource.EngineRenderModeDefault
			case "standard":
				return resource.EngineRenderModeStandard
			}
			return resource.EngineRenderModeLightweight
		}
	}
	return resource.EngineRenderModeLightweight
}

func parseFloat(s string) (float64, error) {
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid number %q: %w", s, err)
	}
	return f, nil
}

func spriteID(skinST *ast.StructType, name string) (int, error) {
	if skinST == nil {
		return 0, fmt.Errorf("unknown sprite name %q (no skin struct declared)", name)
	}
	id := 0
	for _, f := range skinST.Fields.List {
		for _, n := range f.Names {
			if n.Name == name {
				return id, nil
			}
			id++
		}
	}
	return 0, fmt.Errorf("unknown sprite name %q", name)
}

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

// uiSetEase creates a setter for an ease-valued UI field.
func uiSetEase(field uiField[resource.EngineConfigurationAnimationTweenEase]) func(*resource.EngineConfigurationUI, string) error {
	return func(ui *resource.EngineConfigurationUI, v string) error {
		e, ok := easeName[v]
		if !ok {
			return fmt.Errorf("unknown ease %q", v)
		}
		*field(ui) = e
		return nil
	}
}

// uiSetMetric creates a setter for a metric-valued UI field.
func uiSetMetric(field uiField[resource.EngineConfigurationMetric]) func(*resource.EngineConfigurationUI, string) error {
	return func(ui *resource.EngineConfigurationUI, v string) error {
		m, ok := metricName[v]
		if !ok {
			return fmt.Errorf("unknown metric %q", v)
		}
		*field(ui) = m
		return nil
	}
}

// uiSetJudgmentStyle creates a setter for judgmentErrorStyle.
func uiSetJudgmentStyle(field uiField[resource.EngineConfigurationJudgmentErrorStyle]) func(*resource.EngineConfigurationUI, string) error {
	return func(ui *resource.EngineConfigurationUI, v string) error {
		s, ok := judgmentErrorStyleName[v]
		if !ok {
			return fmt.Errorf("unknown judgmentErrorStyle %q", v)
		}
		*field(ui) = s
		return nil
	}
}

// uiSetJudgmentPlacement creates a setter for judgmentErrorPlacement.
func uiSetJudgmentPlacement(field uiField[resource.EngineConfigurationJudgmentErrorPlacement]) func(*resource.EngineConfigurationUI, string) error {
	return func(ui *resource.EngineConfigurationUI, v string) error {
		p, ok := judgmentErrorPlacementName[v]
		if !ok {
			return fmt.Errorf("unknown judgmentErrorPlacement %q", v)
		}
		*field(ui) = p
		return nil
	}
}

// uiSetters maps each sonolus UI tag key to its setter function.
var uiSetters = buildUISetters()

func buildUISetters() map[string]func(*resource.EngineConfigurationUI, string) error {
	m := map[string]func(*resource.EngineConfigurationUI, string) error{}

	// Metrics
	m["primaryMetric"] = uiSetMetric(func(ui *resource.EngineConfigurationUI) *resource.EngineConfigurationMetric { return &ui.PrimaryMetric })
	m["secondaryMetric"] = uiSetMetric(func(ui *resource.EngineConfigurationUI) *resource.EngineConfigurationMetric { return &ui.SecondaryMetric })

	// Judgment error
	m["judgmentErrorStyle"] = uiSetJudgmentStyle(func(ui *resource.EngineConfigurationUI) *resource.EngineConfigurationJudgmentErrorStyle { return &ui.JudgmentErrorStyle })
	m["judgmentErrorPlacement"] = uiSetJudgmentPlacement(func(ui *resource.EngineConfigurationUI) *resource.EngineConfigurationJudgmentErrorPlacement { return &ui.JudgmentErrorPlacement })
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
	m["judgmentAnimationScaleEase"] = uiSetEase(func(ui *resource.EngineConfigurationUI) *resource.EngineConfigurationAnimationTweenEase { return &ui.JudgmentAnimation.Scale.Ease })
	m["judgmentAnimationAlphaFrom"] = uiSetFloat(func(ui *resource.EngineConfigurationUI) *float64 { return &ui.JudgmentAnimation.Alpha.From })
	m["judgmentAnimationAlphaTo"] = uiSetFloat(func(ui *resource.EngineConfigurationUI) *float64 { return &ui.JudgmentAnimation.Alpha.To })
	m["judgmentAnimationAlphaDuration"] = uiSetFloat(func(ui *resource.EngineConfigurationUI) *float64 { return &ui.JudgmentAnimation.Alpha.Duration })
	m["judgmentAnimationAlphaEase"] = uiSetEase(func(ui *resource.EngineConfigurationUI) *resource.EngineConfigurationAnimationTweenEase { return &ui.JudgmentAnimation.Alpha.Ease })

	// Combo animation
	m["comboAnimationScaleFrom"] = uiSetFloat(func(ui *resource.EngineConfigurationUI) *float64 { return &ui.ComboAnimation.Scale.From })
	m["comboAnimationScaleTo"] = uiSetFloat(func(ui *resource.EngineConfigurationUI) *float64 { return &ui.ComboAnimation.Scale.To })
	m["comboAnimationScaleDuration"] = uiSetFloat(func(ui *resource.EngineConfigurationUI) *float64 { return &ui.ComboAnimation.Scale.Duration })
	m["comboAnimationScaleEase"] = uiSetEase(func(ui *resource.EngineConfigurationUI) *resource.EngineConfigurationAnimationTweenEase { return &ui.ComboAnimation.Scale.Ease })
	m["comboAnimationAlphaFrom"] = uiSetFloat(func(ui *resource.EngineConfigurationUI) *float64 { return &ui.ComboAnimation.Alpha.From })
	m["comboAnimationAlphaTo"] = uiSetFloat(func(ui *resource.EngineConfigurationUI) *float64 { return &ui.ComboAnimation.Alpha.To })
	m["comboAnimationAlphaDuration"] = uiSetFloat(func(ui *resource.EngineConfigurationUI) *float64 { return &ui.ComboAnimation.Alpha.Duration })
	m["comboAnimationAlphaEase"] = uiSetEase(func(ui *resource.EngineConfigurationUI) *resource.EngineConfigurationAnimationTweenEase { return &ui.ComboAnimation.Alpha.Ease })

	return m
}

// buildUI parses a UI struct from the engine source and returns an
// EngineConfigurationUI. It starts from defaultUI() and overrides fields
// based on sonolus tag key=value pairs on each struct field.
func buildUI(st *ast.StructType) (resource.EngineConfigurationUI, error) {
	ui := defaultUI()
	for _, f := range st.Fields.List {
		if f.Tag == nil || len(f.Names) == 0 {
			continue
		}
		tag := reflect.StructTag(stringLit(f.Tag.Value)).Get("sonolus")
		if tag == "" {
			continue
		}
		k, v := keyVal(tag)
		if k == "" {
			continue
		}
		set, ok := uiSetters[k]
		if !ok {
			return resource.EngineConfigurationUI{}, fmt.Errorf("unknown UI field %q", k)
		}
		if err := set(&ui, v); err != nil {
			return resource.EngineConfigurationUI{}, fmt.Errorf("UI field %q: %w", k, err)
		}
	}
	return ui, nil
}
