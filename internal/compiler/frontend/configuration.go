package frontend

import (
	"fmt"
	"go/types"
	"strconv"
	"strings"

	"github.com/WindowsSov8forUs/sonolus-core-go/core"
	"github.com/WindowsSov8forUs/sonolus-core-go/core/resource"
	"github.com/WindowsSov8forUs/sonolus-go/internal/compiler/source"
)

func parseOptionBase(field *types.Var, tag tagValue) resource.EngineConfigurationOptionBase {
	return resource.EngineConfigurationOptionBase{Name: core.Text(staticName(tag, field.Name())), Description: tag.Items["description"], Scope: tag.Items["scope"], Standard: tag.Flags["standard"], Advanced: tag.Flags["advanced"]}
}

func configurationUI(value source.StaticValue, typ *types.Struct) (resource.EngineConfigurationUI, error) {
	result := resource.EngineConfigurationUI{}
	allowed := func(value string, values ...string) bool {
		if value == "" {
			return true
		}
		for _, candidate := range values {
			if value == candidate {
				return true
			}
		}
		return false
	}
	stringValue := func(name string) (string, error) {
		for i := 0; i < typ.NumFields(); i++ {
			if typ.Field(i).Name() == name {
				field, ok := staticField(value, typ.Field(i))
				if !ok {
					return "", fmt.Errorf("missing UI field %s", name)
				}
				text, ok := staticString(field)
				if !ok {
					return "", fmt.Errorf("UI field %s must be a static string enum", name)
				}
				return text, nil
			}
		}
		return "", fmt.Errorf("unknown UI field %s", name)
	}
	numberValue := func(name string) (float64, error) {
		for i := 0; i < typ.NumFields(); i++ {
			if typ.Field(i).Name() == name {
				field, ok := staticField(value, typ.Field(i))
				if !ok {
					return 0, fmt.Errorf("missing UI field %s", name)
				}
				number, ok := staticNumber(field)
				if !ok {
					return 0, fmt.Errorf("UI field %s must be a static number", name)
				}
				return number, nil
			}
		}
		return 0, fmt.Errorf("unknown UI field %s", name)
	}
	visibility := func(name string) (resource.EngineConfigurationVisibility, error) {
		for i := 0; i < typ.NumFields(); i++ {
			if typ.Field(i).Name() != name {
				continue
			}
			fieldValue, ok := staticField(value, typ.Field(i))
			fieldType, typeOK := types.Unalias(typ.Field(i).Type()).Underlying().(*types.Struct)
			if !ok || !typeOK {
				return resource.EngineConfigurationVisibility{}, fmt.Errorf("UI field %s must be static UIVisibility", name)
			}
			var out resource.EngineConfigurationVisibility
			for j := 0; j < fieldType.NumFields(); j++ {
				n, valid := staticNumberField(fieldValue, fieldType.Field(j))
				if !valid {
					return out, fmt.Errorf("UI field %s.%s must be static", name, fieldType.Field(j).Name())
				}
				if fieldType.Field(j).Name() == "Scale" {
					out.Scale = n
				} else if fieldType.Field(j).Name() == "Alpha" {
					out.Alpha = n
				}
			}
			return out, nil
		}
		return resource.EngineConfigurationVisibility{}, fmt.Errorf("unknown UI field %s", name)
	}
	animation := func(name string) (resource.EngineConfigurationAnimation, error) {
		var out resource.EngineConfigurationAnimation
		for i := 0; i < typ.NumFields(); i++ {
			if typ.Field(i).Name() != name {
				continue
			}
			animationValue, ok := staticField(value, typ.Field(i))
			animationType, typeOK := types.Unalias(typ.Field(i).Type()).Underlying().(*types.Struct)
			if !ok || !typeOK {
				return out, fmt.Errorf("UI field %s must be static UIAnimation", name)
			}
			for j := 0; j < animationType.NumFields(); j++ {
				tweenField := animationType.Field(j)
				tweenValue, found := staticField(animationValue, tweenField)
				tweenType, tweenOK := types.Unalias(tweenField.Type()).Underlying().(*types.Struct)
				if !found || !tweenOK {
					return out, fmt.Errorf("UI field %s.%s must be static", name, tweenField.Name())
				}
				var tween resource.EngineConfigurationAnimationTween
				for k := 0; k < tweenType.NumFields(); k++ {
					f := tweenType.Field(k)
					v, exists := staticField(tweenValue, f)
					if !exists {
						return out, fmt.Errorf("UI field %s.%s.%s must be static", name, tweenField.Name(), f.Name())
					}
					if f.Name() == "Ease" {
						text, valid := staticString(v)
						if !valid {
							return out, fmt.Errorf("UI ease must be static")
						}
						if !allowed(text, "none", "inSine", "outSine", "inOutSine") {
							return out, fmt.Errorf("invalid UI ease %q", text)
						}
						tween.Ease = resource.EngineConfigurationAnimationTweenEase(text)
					} else {
						n, valid := staticNumber(v)
						if !valid {
							return out, fmt.Errorf("UI tween number must be static")
						}
						switch f.Name() {
						case "From":
							tween.From = n
						case "To":
							tween.To = n
						case "Duration":
							tween.Duration = n
						}
					}
				}
				if tweenField.Name() == "Scale" {
					out.Scale = tween
				} else if tweenField.Name() == "Alpha" {
					out.Alpha = tween
				}
			}
			return out, nil
		}
		return out, fmt.Errorf("unknown UI field %s", name)
	}
	var err error
	if result.Scope, err = stringValue("Scope"); err != nil {
		return result, err
	}
	primary, err := stringValue("PrimaryMetric")
	if err != nil {
		return result, err
	}
	result.PrimaryMetric = resource.EngineConfigurationMetric(primary)
	if !allowed(primary, "arcade", "accuracy", "life") {
		return result, fmt.Errorf("invalid primary metric %q", primary)
	}
	secondary, err := stringValue("SecondaryMetric")
	if err != nil {
		return result, err
	}
	result.SecondaryMetric = resource.EngineConfigurationMetric(secondary)
	if !allowed(secondary, "arcade", "accuracy", "life") {
		return result, fmt.Errorf("invalid secondary metric %q", secondary)
	}
	for _, item := range []struct {
		name   string
		target *resource.EngineConfigurationVisibility
	}{{"MenuVisibility", &result.MenuVisibility}, {"JudgmentVisibility", &result.JudgmentVisibility}, {"ComboVisibility", &result.ComboVisibility}, {"PrimaryMetricVisibility", &result.PrimaryMetricVisibility}, {"SecondaryMetricVisibility", &result.SecondaryMetricVisibility}, {"ProgressVisibility", &result.ProgressVisibility}, {"TutorialNavigationVisibility", &result.TutorialNavigationVisibility}, {"TutorialInstructionVisibility", &result.TutorialInstructionVisibility}} {
		*item.target, err = visibility(item.name)
		if err != nil {
			return result, err
		}
	}
	result.JudgmentAnimation, err = animation("JudgmentAnimation")
	if err != nil {
		return result, err
	}
	result.ComboAnimation, err = animation("ComboAnimation")
	if err != nil {
		return result, err
	}
	style, err := stringValue("JudgmentErrorStyle")
	if err != nil {
		return result, err
	}
	result.JudgmentErrorStyle = resource.EngineConfigurationJudgmentErrorStyle(style)
	if !allowed(style, "none", "plus", "minus", "arrow") {
		return result, fmt.Errorf("invalid judgment error style %q", style)
	}
	placement, err := stringValue("JudgmentErrorPlacement")
	if err != nil {
		return result, err
	}
	result.JudgmentErrorPlacement = resource.EngineConfigurationJudgmentErrorPlacement(placement)
	if !allowed(placement, "top", "bottom") {
		return result, fmt.Errorf("invalid judgment error placement %q", placement)
	}
	result.JudgmentErrorMin, err = numberValue("JudgmentErrorMin")
	return result, err
}

func staticNumberField(value source.StaticValue, field *types.Var) (float64, bool) {
	item, ok := staticField(value, field)
	if !ok {
		return 0, false
	}
	return staticNumber(item)
}

func parseConfiguration(named *types.Named, singleton *types.Var, tracer *source.ASTTracer) (*resource.EngineConfiguration, []error) {
	cfg := &resource.EngineConfiguration{Options: []resource.EngineConfigurationOption{}}
	var errs []error
	st := named.Underlying().(*types.Struct)
	binding, evalErr := tracer.EvalObject(singleton)
	if evalErr != nil {
		errs = append(errs, fmt.Errorf("%s: configuration singleton must be statically evaluable: %w", singleton.Name(), evalErr))
	}
	configurationValue := binding.Value
	externalNames := map[string]bool{}
	for i := 0; i < st.NumFields(); i++ {
		field := st.Field(i)
		if field.Embedded() {
			continue
		}
		if _, legacy := sonolusTag(st.Tag(i)); legacy {
			errs = append(errs, fmt.Errorf("configuration.%s: use the configuration struct tag instead of sonolus", field.Name()))
			continue
		}
		tag, ok := configurationTag(st.Tag(i))
		if !ok {
			continue
		}
		errs = append(errs, validateTag("configuration."+field.Name(), tag,
			[]string{"slider", "toggle", "select", "standard", "advanced", "ui", "replayFallback"},
			[]string{"name", "description", "scope", "def", "min", "max", "step", "unit", "values"})...)
		kinds := 0
		for _, kind := range []string{"slider", "toggle", "select", "ui", "replayFallback"} {
			if tag.Flags[kind] {
				kinds++
			}
		}
		if kinds != 1 {
			errs = append(errs, fmt.Errorf("configuration.%s: exactly one field kind is required", field.Name()))
			continue
		}
		if tag.Flags["ui"] {
			if typeID(field.Type()) != rootID("UIConfig") {
				errs = append(errs, fmt.Errorf("configuration.%s: ui field must be sonolus.UIConfig", field.Name()))
				continue
			}
			value, found := staticField(configurationValue, field)
			if !found {
				errs = append(errs, fmt.Errorf("configuration.%s: ui value is not static", field.Name()))
				continue
			}
			if err := pureStaticError(value, "configuration UI"); err != nil {
				errs = append(errs, err)
				continue
			}
			uiType := types.Unalias(field.Type()).Underlying().(*types.Struct)
			ui, err := configurationUI(value, uiType)
			if err != nil {
				errs = append(errs, fmt.Errorf("configuration.%s: %w", field.Name(), err))
			} else {
				cfg.UI = ui
			}
			continue
		}
		if tag.Flags["replayFallback"] {
			value, found := staticField(configurationValue, field)
			if !found {
				errs = append(errs, fmt.Errorf("configuration.%s: replay fallback value is not static", field.Name()))
				continue
			}
			if err := pureStaticError(value, "configuration replay fallback"); err != nil {
				errs = append(errs, err)
				continue
			}
			elements, valid := staticElements(value)
			if !valid {
				errs = append(errs, fmt.Errorf("configuration.%s: replay fallback must be a static string slice", field.Name()))
				continue
			}
			seen := map[string]bool{}
			for _, element := range elements {
				text, valid := staticString(element)
				if !valid || text == "" {
					errs = append(errs, fmt.Errorf("configuration.%s: replay fallback names must be non-empty static strings", field.Name()))
					continue
				}
				if seen[text] {
					errs = append(errs, fmt.Errorf("configuration.%s: duplicate replay fallback %q", field.Name(), text))
					continue
				}
				seen[text] = true
				cfg.ReplayFallbackOptionNames = append(cfg.ReplayFallbackOptionNames, core.Text(text))
			}
			continue
		}
		base := parseOptionBase(field, tag)
		if externalNames[string(base.Name)] {
			errs = append(errs, fmt.Errorf("configuration.%s: duplicate option name %q", field.Name(), base.Name))
		} else {
			externalNames[string(base.Name)] = true
		}
		parseFloat := func(key string) float64 {
			value, err := strconv.ParseFloat(tag.Items[key], 64)
			if err != nil {
				errs = append(errs, fmt.Errorf("configuration.%s: invalid %s", field.Name(), key))
				return 0
			}
			return value
		}
		switch {
		case tag.Flags["slider"]:
			if !types.Identical(field.Type(), types.Typ[types.Float64]) {
				errs = append(errs, fmt.Errorf("configuration.%s: slider field must be float64", field.Name()))
			}
			def, min, max, step := parseFloat("def"), parseFloat("min"), parseFloat("max"), parseFloat("step")
			if min > max || def < min || def > max || step <= 0 {
				errs = append(errs, fmt.Errorf("configuration.%s: require min <= def <= max and step > 0", field.Name()))
			}
			cfg.Options = append(cfg.Options, resource.EngineConfigurationSliderOption{EngineConfigurationOptionBase: base, Type: resource.EngineConfigurationOptionTypeSlider, Def: def, Min: min, Max: max, Step: step, Unit: core.Text(tag.Items["unit"])})
		case tag.Flags["toggle"]:
			if !types.Identical(field.Type(), types.Typ[types.Bool]) {
				errs = append(errs, fmt.Errorf("configuration.%s: toggle field must be bool", field.Name()))
			}
			def, parseErr := strconv.ParseBool(tag.Items["def"])
			if parseErr != nil {
				errs = append(errs, fmt.Errorf("configuration.%s: invalid def", field.Name()))
			}
			n := 0
			if def {
				n = 1
			}
			cfg.Options = append(cfg.Options, resource.EngineConfigurationToggleOption{EngineConfigurationOptionBase: base, Type: resource.EngineConfigurationOptionTypeToggle, Def: n})
		case tag.Flags["select"]:
			if !types.Identical(field.Type(), types.Typ[types.Int]) {
				errs = append(errs, fmt.Errorf("configuration.%s: select field must be int", field.Name()))
			}
			def, err := strconv.Atoi(tag.Items["def"])
			if err != nil {
				errs = append(errs, fmt.Errorf("configuration.%s: invalid def", field.Name()))
			}
			raw := tag.Items["values"]
			var values []core.Text
			if raw != "" {
				for _, v := range strings.Split(raw, "|") {
					if v == "" {
						errs = append(errs, fmt.Errorf("configuration.%s: select values must be non-empty", field.Name()))
					}
					values = append(values, core.Text(v))
				}
			}
			if len(values) == 0 || def < 0 || def >= len(values) {
				errs = append(errs, fmt.Errorf("configuration.%s: select def must index a non-empty values list", field.Name()))
			}
			cfg.Options = append(cfg.Options, resource.EngineConfigurationSelectOption{EngineConfigurationOptionBase: base, Type: resource.EngineConfigurationOptionTypeSelect, Def: def, Values: values})
		default:
			errs = append(errs, fmt.Errorf("configuration.%s: missing option kind", field.Name()))
		}
	}
	for _, fallback := range cfg.ReplayFallbackOptionNames {
		if !externalNames[string(fallback)] {
			errs = append(errs, fmt.Errorf("configuration: replay fallback %q does not name an option", fallback))
		}
	}
	return cfg, errs
}
