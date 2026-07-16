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
	cfg := &resource.EngineConfiguration{Options: []resource.EngineConfigurationOption{}}
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
