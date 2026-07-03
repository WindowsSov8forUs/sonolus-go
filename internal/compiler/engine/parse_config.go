package engine

import (
	"fmt"
	"go/ast"
	"reflect"

	"github.com/WindowsSov8forUs/sonolus-core-go/core"
	"github.com/WindowsSov8forUs/sonolus-core-go/core/resource"
)

// buildConfig parses a Config struct from the engine source and returns an
// EngineConfiguration. It reads sonolus struct tags on each field to determine
// option type (slider/toggle/select) and parameters.
func buildConfig(st *ast.StructType) (resource.EngineConfiguration, error) {
	var opts []resource.EngineConfigurationOption
	var replayFallbackNames []core.Text
	for _, f := range st.Fields.List {
		if f.Tag == nil || len(f.Names) == 0 {
			continue
		}
		tag := reflect.StructTag(stringLit(f.Tag.Value)).Get("sonolus")
		if tag == "" {
			continue
		}
		tagVals := splitTag(tag)
		name := core.Text(f.Names[0].Name)
		base := resource.EngineConfigurationOptionBase{
			Name:     name,
			Standard: true,
			Advanced: hasTag(tagVals, "advanced"),
			Scope:    tagVal(tagVals, "scope"),
		}

		switch kind := tagVals[0]; kind {
		case "replayFallback":
			replayFallbackNames = append(replayFallbackNames, name)
			continue
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
	return resource.EngineConfiguration{Options: opts, UI: defaultUI(), ReplayFallbackOptionNames: replayFallbackNames}, nil
}
