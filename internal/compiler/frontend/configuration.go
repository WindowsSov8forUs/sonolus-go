package frontend

import (
	"fmt"
	"go/constant"
	"go/types"
	"math"

	"github.com/WindowsSov8forUs/sonolus-core-go/core"
	"github.com/WindowsSov8forUs/sonolus-core-go/core/resource"
	"github.com/WindowsSov8forUs/sonolus-go/v2/internal/compiler/catalog"
	"github.com/WindowsSov8forUs/sonolus-go/v2/internal/compiler/source"
)

func configurationUI(value source.StaticValue, typ *types.Struct) (resource.EngineConfigurationUI, error) {
	result := defaultConfigurationUI()
	allowed := func(value string, values ...string) bool {
		for _, candidate := range values {
			if value == candidate {
				return true
			}
		}
		return false
	}
	fieldValue := func(name string) (source.StaticField, bool, error) {
		for i := 0; i < typ.NumFields(); i++ {
			if typ.Field(i).Name() == name {
				for _, field := range value.Fields {
					if field.Field == typ.Field(i) || field.Field.Name() == name {
						return field, true, nil
					}
				}
				return source.StaticField{}, false, fmt.Errorf("missing UI field %s", name)
			}
		}
		return source.StaticField{}, false, fmt.Errorf("unknown UI field %s", name)
	}
	stringValue := func(name, fallback string) (string, error) {
		field, _, err := fieldValue(name)
		if err != nil {
			return "", err
		}
		if !field.Explicit {
			return fallback, nil
		}
		text, ok := staticString(field.Value)
		if !ok {
			return "", fmt.Errorf("UI field %s must be a static string enum", name)
		}
		return text, nil
	}
	numberValue := func(name string, fallback float64) (float64, error) {
		field, _, err := fieldValue(name)
		if err != nil {
			return 0, err
		}
		if !field.Explicit {
			return fallback, nil
		}
		number, ok := staticNumber(field.Value)
		if !ok {
			return 0, fmt.Errorf("UI field %s must be a static number", name)
		}
		return number, nil
	}
	visibility := func(name string, fallback resource.EngineConfigurationVisibility) (resource.EngineConfigurationVisibility, error) {
		for i := 0; i < typ.NumFields(); i++ {
			if typ.Field(i).Name() != name {
				continue
			}
			field, ok, err := fieldValue(name)
			if err != nil {
				return fallback, err
			}
			if !field.Explicit {
				return fallback, nil
			}
			fieldValue := field.Value
			fieldType, typeOK := types.Unalias(typ.Field(i).Type()).Underlying().(*types.Struct)
			if !ok || !typeOK {
				return resource.EngineConfigurationVisibility{}, fmt.Errorf("UI field %s must be static UIVisibility", name)
			}
			out := fallback
			for j := 0; j < fieldType.NumFields(); j++ {
				field, _, fieldErr := func() (source.StaticField, bool, error) {
					for _, nested := range fieldValue.Fields {
						if nested.Field == fieldType.Field(j) || nested.Field.Name() == fieldType.Field(j).Name() {
							return nested, true, nil
						}
					}
					return source.StaticField{}, false, fmt.Errorf("missing UI field %s.%s", name, fieldType.Field(j).Name())
				}()
				if fieldErr != nil {
					return out, fieldErr
				}
				if !field.Explicit {
					continue
				}
				n, valid := staticNumber(field.Value)
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
	animation := func(name string, fallback resource.EngineConfigurationAnimation) (resource.EngineConfigurationAnimation, error) {
		out := fallback
		for i := 0; i < typ.NumFields(); i++ {
			if typ.Field(i).Name() != name {
				continue
			}
			field, ok, err := fieldValue(name)
			if err != nil {
				return out, err
			}
			if !field.Explicit {
				return out, nil
			}
			animationValue := field.Value
			animationType, typeOK := types.Unalias(typ.Field(i).Type()).Underlying().(*types.Struct)
			if !ok || !typeOK {
				return out, fmt.Errorf("UI field %s must be static UIAnimation", name)
			}
			for j := 0; j < animationType.NumFields(); j++ {
				tweenField := animationType.Field(j)
				tweenStatic, found := func() (source.StaticField, bool) {
					for _, nested := range animationValue.Fields {
						if nested.Field == tweenField || nested.Field.Name() == tweenField.Name() {
							return nested, true
						}
					}
					return source.StaticField{}, false
				}()
				if !found {
					return out, fmt.Errorf("UI field %s.%s must be static", name, tweenField.Name())
				}
				if !tweenStatic.Explicit {
					continue
				}
				tweenValue := tweenStatic.Value
				tweenType, tweenOK := types.Unalias(tweenField.Type()).Underlying().(*types.Struct)
				if !tweenOK {
					return out, fmt.Errorf("UI field %s.%s must be static", name, tweenField.Name())
				}
				tween := out.Scale
				if tweenField.Name() == "Alpha" {
					tween = out.Alpha
				}
				for k := 0; k < tweenType.NumFields(); k++ {
					f := tweenType.Field(k)
					staticField, exists := func() (source.StaticField, bool) {
						for _, nested := range tweenValue.Fields {
							if nested.Field == f || nested.Field.Name() == f.Name() {
								return nested, true
							}
						}
						return source.StaticField{}, false
					}()
					if !exists {
						return out, fmt.Errorf("UI field %s.%s.%s must be static", name, tweenField.Name(), f.Name())
					}
					if !staticField.Explicit {
						continue
					}
					v := staticField.Value
					if f.Name() == "Ease" {
						text, valid := staticString(v)
						if !valid {
							return out, fmt.Errorf("UI ease must be static")
						}
						if !allowed(text,
							"linear", "none",
							"inSine", "outSine", "inOutSine", "outInSine",
							"inQuad", "outQuad", "inOutQuad", "outInQuad",
							"inCubic", "outCubic", "inOutCubic", "outInCubic",
							"inQuart", "outQuart", "inOutQuart", "outInQuart",
							"inQuint", "outQuint", "inOutQuint", "outInQuint",
							"inExpo", "outExpo", "inOutExpo", "outInExpo",
							"inCirc", "outCirc", "inOutCirc", "outInCirc",
							"inBack", "outBack", "inOutBack", "outInBack",
							"inElastic", "outElastic", "inOutElastic", "outInElastic",
						) {
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
	if result.Scope, err = stringValue("Scope", result.Scope); err != nil {
		return result, err
	}
	primary, err := stringValue("PrimaryMetric", string(result.PrimaryMetric))
	if err != nil {
		return result, err
	}
	result.PrimaryMetric = resource.EngineConfigurationMetric(primary)
	if !allowed(primary, "arcade", "arcadePercentage", "accuracy", "accuracyPercentage", "life", "perfect", "perfectPercentage", "greatGoodMiss", "greatGoodMissPercentage", "miss", "missPercentage", "errorHeatmap") {
		return result, fmt.Errorf("invalid primary metric %q", primary)
	}
	secondary, err := stringValue("SecondaryMetric", string(result.SecondaryMetric))
	if err != nil {
		return result, err
	}
	result.SecondaryMetric = resource.EngineConfigurationMetric(secondary)
	if !allowed(secondary, "arcade", "arcadePercentage", "accuracy", "accuracyPercentage", "life", "perfect", "perfectPercentage", "greatGoodMiss", "greatGoodMissPercentage", "miss", "missPercentage", "errorHeatmap") {
		return result, fmt.Errorf("invalid secondary metric %q", secondary)
	}
	for _, item := range []struct {
		name   string
		target *resource.EngineConfigurationVisibility
	}{{"MenuVisibility", &result.MenuVisibility}, {"JudgmentVisibility", &result.JudgmentVisibility}, {"ComboVisibility", &result.ComboVisibility}, {"PrimaryMetricVisibility", &result.PrimaryMetricVisibility}, {"SecondaryMetricVisibility", &result.SecondaryMetricVisibility}, {"ProgressVisibility", &result.ProgressVisibility}, {"TutorialNavigationVisibility", &result.TutorialNavigationVisibility}, {"TutorialInstructionVisibility", &result.TutorialInstructionVisibility}} {
		*item.target, err = visibility(item.name, *item.target)
		if err != nil {
			return result, err
		}
	}
	result.JudgmentAnimation, err = animation("JudgmentAnimation", result.JudgmentAnimation)
	if err != nil {
		return result, err
	}
	result.ComboAnimation, err = animation("ComboAnimation", result.ComboAnimation)
	if err != nil {
		return result, err
	}
	style, err := stringValue("JudgmentErrorStyle", string(result.JudgmentErrorStyle))
	if err != nil {
		return result, err
	}
	result.JudgmentErrorStyle = resource.EngineConfigurationJudgmentErrorStyle(style)
	if !allowed(style, "none", "late", "early", "plus", "minus", "arrowUp", "arrowDown", "arrowLeft", "arrowRight", "triangleUp", "triangleDown", "triangleLeft", "triangleRight") {
		return result, fmt.Errorf("invalid judgment error style %q", style)
	}
	placement, err := stringValue("JudgmentErrorPlacement", string(result.JudgmentErrorPlacement))
	if err != nil {
		return result, err
	}
	result.JudgmentErrorPlacement = resource.EngineConfigurationJudgmentErrorPlacement(placement)
	if !allowed(placement, "left", "right", "leftRight", "top", "bottom", "topBottom", "center") {
		return result, fmt.Errorf("invalid judgment error placement %q", placement)
	}
	result.JudgmentErrorMin, err = numberValue("JudgmentErrorMin", result.JudgmentErrorMin)
	return result, err
}

func defaultConfigurationUI() resource.EngineConfigurationUI {
	visible := resource.EngineConfigurationVisibility{Scale: 1, Alpha: 1}
	return resource.EngineConfigurationUI{
		PrimaryMetric:                 resource.EngineConfigurationMetricArcade,
		SecondaryMetric:               resource.EngineConfigurationMetricLife,
		MenuVisibility:                visible,
		JudgmentVisibility:            visible,
		ComboVisibility:               visible,
		PrimaryMetricVisibility:       visible,
		SecondaryMetricVisibility:     visible,
		ProgressVisibility:            visible,
		TutorialNavigationVisibility:  visible,
		TutorialInstructionVisibility: visible,
		JudgmentAnimation: resource.EngineConfigurationAnimation{
			Scale: resource.EngineConfigurationAnimationTween{From: 0, To: 1, Duration: 0.1, Ease: resource.EngineConfigurationAnimationTweenEaseOutCubic},
			Alpha: resource.EngineConfigurationAnimationTween{From: 1, To: 0, Duration: 0.3, Ease: resource.EngineConfigurationAnimationTweenEaseNone},
		},
		ComboAnimation: resource.EngineConfigurationAnimation{
			Scale: resource.EngineConfigurationAnimationTween{From: 1.2, To: 1, Duration: 0.2, Ease: resource.EngineConfigurationAnimationTweenEaseInCubic},
			Alpha: resource.EngineConfigurationAnimationTween{From: 1, To: 1, Ease: resource.EngineConfigurationAnimationTweenEaseNone},
		},
		JudgmentErrorStyle:     resource.EngineConfigurationJudgmentErrorStyleLate,
		JudgmentErrorPlacement: resource.EngineConfigurationJudgmentErrorPlacementTop,
	}
}

func staticNumberField(value source.StaticValue, field *types.Var) (float64, bool) {
	item, ok := staticField(value, field)
	if !ok {
		return 0, false
	}
	return staticNumber(item)
}

func staticBool(value source.StaticValue) (bool, bool) {
	value = dereferenceStatic(value)
	if value.Kind != source.StaticConstant || value.Exact == nil || value.Exact.Kind() != constant.Bool {
		return false, false
	}
	return constant.BoolVal(value.Exact), true
}

func staticInt(value source.StaticValue) (int, bool) {
	value = dereferenceStatic(value)
	if value.Kind != source.StaticConstant || value.Exact == nil || value.Exact.Kind() != constant.Int {
		return 0, false
	}
	n, exact := constant.Int64Val(value.Exact)
	return int(n), exact && int64(int(n)) == n
}

func staticStructFieldByName(value source.StaticValue, name string) (source.StaticValue, bool) {
	value = dereferenceStatic(value)
	if value.Kind != source.StaticStruct {
		return source.StaticValue{}, false
	}
	for _, item := range value.Fields {
		if item.Field.Name() == name {
			return item.Value, true
		}
	}
	return source.StaticValue{}, false
}

func staticStringConfig(value source.StaticValue, name string) (string, bool) {
	field, ok := staticStructFieldByName(value, name)
	if !ok {
		return "", false
	}
	return staticString(field)
}

func staticBoolConfig(value source.StaticValue, name string) (bool, bool) {
	field, ok := staticStructFieldByName(value, name)
	if !ok {
		return false, false
	}
	return staticBool(field)
}

func staticNumberConfig(value source.StaticValue, name string) (float64, bool) {
	field, ok := staticStructFieldByName(value, name)
	if !ok {
		return 0, false
	}
	return staticNumber(field)
}

func staticIntConfig(value source.StaticValue, name string) (int, bool) {
	field, ok := staticStructFieldByName(value, name)
	if !ok {
		return 0, false
	}
	return staticInt(field)
}

func optionCall(value source.StaticValue, field *types.Var) (*source.StaticCall, source.StaticValue, error) {
	value = dereferenceStatic(value)
	if value.Kind != source.StaticFunctionCall || value.Call == nil || value.Call.Object == nil || value.Call.Receiver != nil || len(value.Call.Args) != 1 {
		return nil, source.StaticValue{}, fmt.Errorf("configuration.%s: initialize the field with the matching sonolus option constructor", field.Name())
	}
	call := value.Call
	symbol, ok := catalog.LookupObject(call.Object)
	want := map[string]string{"float64": "SliderOption", "bool": "ToggleOption", "int": "SelectOption"}[field.Type().String()]
	if !ok || symbol.Package != "sonolus" || symbol.Kind != catalog.KindFunction || symbol.Name != want {
		return nil, source.StaticValue{}, fmt.Errorf("%s: configuration.%s must use sonolus.%s", call.Pos, field.Name(), want)
	}
	if call.Signature == nil || call.Signature.Results().Len() != 1 || !types.Identical(call.Signature.Results().At(0).Type(), field.Type()) {
		return nil, source.StaticValue{}, fmt.Errorf("%s: configuration.%s constructor has an invalid result type", call.Pos, field.Name())
	}
	config := call.Args[0]
	if typeID(config.Type) != rootID(want+"Config") {
		return nil, source.StaticValue{}, fmt.Errorf("%s: configuration.%s constructor requires sonolus.%sConfig", call.Pos, field.Name(), want)
	}
	if err := pureStaticError(config, "configuration option constructor argument"); err != nil {
		return nil, source.StaticValue{}, err
	}
	return call, config, nil
}

func optionBase(field *types.Var, value source.StaticValue) (resource.EngineConfigurationOptionBase, error) {
	name, nameOK := staticStringConfig(value, "Name")
	title, titleOK := staticStringConfig(value, "Title")
	description, descriptionOK := staticStringConfig(value, "Description")
	standard, standardOK := staticBoolConfig(value, "Standard")
	advanced, advancedOK := staticBoolConfig(value, "Advanced")
	scope, scopeOK := staticStringConfig(value, "Scope")
	if !nameOK || !titleOK || !descriptionOK || !standardOK || !advancedOK || !scopeOK {
		return resource.EngineConfigurationOptionBase{}, fmt.Errorf("configuration.%s: option metadata must be static", field.Name())
	}
	if name == "" {
		name = field.Name()
	}
	return resource.EngineConfigurationOptionBase{
		Name: core.Text(name), Title: core.Text(title), Description: description,
		Standard: standard, Advanced: advanced, Scope: scope,
	}, nil
}

func parseConfiguration(named *types.Named, singleton *types.Var, tracer *source.ASTTracer) (*resource.EngineConfiguration, map[*types.Var]int, map[*types.Var]float64, []error) {
	cfg := &resource.EngineConfiguration{Options: []resource.EngineConfigurationOption{}, UI: defaultConfigurationUI()}
	optionIDs := map[*types.Var]int{}
	defaults := map[*types.Var]float64{}
	var errs []error
	st := named.Underlying().(*types.Struct)
	binding, evalErr := tracer.EvalObject(singleton)
	if evalErr != nil {
		return cfg, optionIDs, defaults, []error{fmt.Errorf("%s: configuration singleton must be statically evaluable: %w", singleton.Name(), evalErr)}
	}
	configurationValue := binding.Value
	externalNames := map[string]bool{}
	uiFields, fallbackFields := 0, 0
	for i := 0; i < st.NumFields(); i++ {
		field := st.Field(i)
		if field.Embedded() {
			if typeID(field.Type()) != rootID("Configuration") {
				errs = append(errs, fmt.Errorf("configuration.%s: only sonolus.Configuration may be embedded", field.Name()))
			}
			continue
		}
		if _, legacy := sonolusTag(st.Tag(i)); legacy {
			errs = append(errs, fmt.Errorf("configuration.%s: struct tags are no longer supported; use a constructor such as sonolus.SliderOption(sonolus.SliderOptionConfig{...})", field.Name()))
			continue
		}
		if _, legacy := configurationTag(st.Tag(i)); legacy {
			errs = append(errs, fmt.Errorf("configuration.%s: configuration tags are no longer supported; use a constructor such as sonolus.SliderOption(sonolus.SliderOptionConfig{...})", field.Name()))
			continue
		}
		if typeID(field.Type()) == rootID("UIConfig") {
			uiFields++
			if uiFields > 1 {
				errs = append(errs, fmt.Errorf("configuration.%s: only one sonolus.UIConfig field is allowed", field.Name()))
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
		if slice, ok := types.Unalias(field.Type()).(*types.Slice); ok && types.Identical(slice.Elem(), types.Typ[types.String]) {
			fallbackFields++
			if fallbackFields > 1 {
				errs = append(errs, fmt.Errorf("configuration.%s: only one []string replay fallback field is allowed", field.Name()))
				continue
			}
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
		if !types.Identical(field.Type(), types.Typ[types.Float64]) && !types.Identical(field.Type(), types.Typ[types.Bool]) && !types.Identical(field.Type(), types.Typ[types.Int]) {
			errs = append(errs, fmt.Errorf("configuration.%s: unsupported field type %s", field.Name(), field.Type()))
			continue
		}
		value, found := staticField(configurationValue, field)
		if !found {
			errs = append(errs, fmt.Errorf("configuration.%s: option initializer is not static", field.Name()))
			continue
		}
		call, optionConfig, err := optionCall(value, field)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		base, err := optionBase(field, optionConfig)
		if err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", call.Pos, err))
			continue
		}
		if externalNames[string(base.Name)] {
			errs = append(errs, fmt.Errorf("configuration.%s: duplicate option name %q", field.Name(), base.Name))
		} else {
			externalNames[string(base.Name)] = true
		}
		switch field.Type() {
		case types.Typ[types.Float64]:
			def, defOK := staticNumberConfig(optionConfig, "Default")
			min, minOK := staticNumberConfig(optionConfig, "Min")
			max, maxOK := staticNumberConfig(optionConfig, "Max")
			step, stepOK := staticNumberConfig(optionConfig, "Step")
			unit, unitOK := staticStringConfig(optionConfig, "Unit")
			if !defOK || !minOK || !maxOK || !stepOK || !unitOK || math.IsNaN(def) || math.IsInf(def, 0) || math.IsNaN(min) || math.IsInf(min, 0) || math.IsNaN(max) || math.IsInf(max, 0) || math.IsNaN(step) || math.IsInf(step, 0) {
				errs = append(errs, fmt.Errorf("%s: configuration.%s slider values must be finite static numbers", call.Pos, field.Name()))
				continue
			}
			if min > max || def < min || def > max || step <= 0 {
				errs = append(errs, fmt.Errorf("%s: configuration.%s requires min <= default <= max and step > 0", call.Pos, field.Name()))
				continue
			}
			optionIDs[field] = len(cfg.Options)
			defaults[field] = def
			cfg.Options = append(cfg.Options, resource.EngineConfigurationSliderOption{EngineConfigurationOptionBase: base, Type: resource.EngineConfigurationOptionTypeSlider, Def: def, Min: min, Max: max, Step: step, Unit: core.Text(unit)})
		case types.Typ[types.Bool]:
			def, ok := staticBoolConfig(optionConfig, "Default")
			if !ok {
				errs = append(errs, fmt.Errorf("%s: configuration.%s default must be a static bool", call.Pos, field.Name()))
				continue
			}
			n := 0
			if def {
				n = 1
			}
			optionIDs[field] = len(cfg.Options)
			defaults[field] = float64(n)
			cfg.Options = append(cfg.Options, resource.EngineConfigurationToggleOption{EngineConfigurationOptionBase: base, Type: resource.EngineConfigurationOptionTypeToggle, Def: n})
		case types.Typ[types.Int]:
			def, defOK := staticIntConfig(optionConfig, "Default")
			raw, valuesOK := staticStructFieldByName(optionConfig, "Values")
			elements, valuesOK := staticElements(raw)
			var values []core.Text
			if valuesOK {
				for _, element := range elements {
					text, ok := staticString(element)
					if !ok || text == "" {
						errs = append(errs, fmt.Errorf("%s: configuration.%s select values must be non-empty static strings", call.Pos, field.Name()))
						valuesOK = false
						continue
					}
					values = append(values, core.Text(text))
				}
			}
			if !defOK || !valuesOK || len(values) == 0 || def < 0 || def >= len(values) {
				errs = append(errs, fmt.Errorf("%s: configuration.%s default must index a non-empty static values list", call.Pos, field.Name()))
				continue
			}
			optionIDs[field] = len(cfg.Options)
			defaults[field] = float64(def)
			cfg.Options = append(cfg.Options, resource.EngineConfigurationSelectOption{EngineConfigurationOptionBase: base, Type: resource.EngineConfigurationOptionTypeSelect, Def: def, Values: values})
		}
	}
	for _, fallback := range cfg.ReplayFallbackOptionNames {
		if !externalNames[string(fallback)] {
			errs = append(errs, fmt.Errorf("configuration: replay fallback %q does not name an option", fallback))
		}
	}
	return cfg, optionIDs, defaults, errs
}
