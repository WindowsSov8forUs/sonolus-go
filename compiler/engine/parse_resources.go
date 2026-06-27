package engine

import (
	"fmt"
	"go/ast"
	"reflect"

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
	}
	return ""
}

type parsedResources struct {
	skin     resource.EngineSkinData
	effect   resource.EngineEffectData
	particle resource.EngineParticleData
	buckets  []resource.EngineDataBucket
	config   resource.EngineConfiguration
}

func buildResources(typeSpecs map[string]*ast.StructType) parsedResources {
	var r parsedResources
	roleOf := func(name string) string {
		for n, role := range map[string]string{
			"Skin": "skin", "Effect": "effect", "Particle": "particle",
			"Buckets": "buckets", "Config": "config",
		} {
			if n == name {
				return role
			}
		}
		return ""
	}

	for name, st := range typeSpecs {
		switch roleOf(name) {
		case "skin":
			r.skin.RenderMode = resource.EngineRenderModeLightweight
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
			r.buckets = buildBuckets(st, typeSpecs["Skin"])
		case "config":
			r.config = buildConfig(st)
		}
	}
	return r
}

func buildConfig(st *ast.StructType) resource.EngineConfiguration {
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
			opts = append(opts, resource.EngineConfigurationSliderOption{
				EngineConfigurationOptionBase: base,
				Type:                          resource.EngineConfigurationOptionTypeSlider,
				Min:                           parseFloatParam(tagVals, "min", 0),
				Max:                           parseFloatParam(tagVals, "max", 1),
				Step:                          parseFloatParam(tagVals, "step", 0.01),
				Def:                           parseFloatParam(tagVals, "def", 0),
			})
		case "toggle":
			opts = append(opts, resource.EngineConfigurationToggleOption{
				EngineConfigurationOptionBase: base,
				Type:                          resource.EngineConfigurationOptionTypeToggle,
				Def:                           int(parseFloatParam(tagVals, "def", 1)),
			})
		case "select":
			var vals []core.Text
			for _, v := range splitTag(tagVal(tagVals, "values")) {
				vals = append(vals, core.Text(v))
			}
			if len(vals) == 0 {
				vals = append(vals, core.Text("value1"))
			}
			opts = append(opts, resource.EngineConfigurationSelectOption{
				EngineConfigurationOptionBase: base,
				Type:                          resource.EngineConfigurationOptionTypeSelect,
				Values:                        vals,
				Def:                           int(parseFloatParam(tagVals, "def", 0)),
			})
		}
	}
	return resource.EngineConfiguration{Options: opts}
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

func parseFloatParam(tags []string, key string, def float64) float64 {
	v := tagVal(tags, key)
	if v == "" {
		return def
	}
	return parseFloat(v)
}

func buildBuckets(st *ast.StructType, skinST *ast.StructType) []resource.EngineDataBucket {
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
								spr.ID = spriteID(skinST, v)
							case "x":
								spr.X = parseFloat(v)
							case "y":
								spr.Y = parseFloat(v)
							case "w":
								spr.W = parseFloat(v)
							case "h":
								spr.H = parseFloat(v)
							case "rotation":
								spr.Rotation = parseFloat(v)
							case "fallback":
								spr.FallbackID = spriteID(skinST, v)
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
	return out
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

func parseFloat(s string) float64 {
	var f float64
	_, _ = fmt.Sscanf(s, "%f", &f)
	return f
}

func spriteID(skinST *ast.StructType, name string) int {
	if skinST == nil {
		return 0
	}
	id := 0
	for _, f := range skinST.Fields.List {
		for _, n := range f.Names {
			if n.Name == name {
				return id
			}
			id++
		}
	}
	return 0
}
