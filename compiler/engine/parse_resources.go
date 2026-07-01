package engine

import (
	"fmt"
	"go/ast"
	"strconv"

	"github.com/WindowsSov8forUs/sonolus-core-go/core"
	"github.com/WindowsSov8forUs/sonolus-core-go/core/resource"
)

// ── Resource role dispatch ────────────────────────────────────────────────────

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
	seen := map[string]bool{}

	for name, st := range typeSpecs {
		role := resourceRole(name)
		if role == "" {
			continue
		}
		if seen[role] {
			return parsedResources{}, fmt.Errorf("duplicate resource type %q: only one %s struct is allowed", name, name)
		}
		seen[role] = true
		switch role {
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

// ── Tag helpers ───────────────────────────────────────────────────────────────

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

func parseFloat(s string) (float64, error) {
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid number %q: %w", s, err)
	}
	return f, nil
}
