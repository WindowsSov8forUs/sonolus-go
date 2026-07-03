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
	skinST      *ast.StructType // Skin AST (for tag-based sprite name resolution)
	effect      resource.EngineEffectData
	particle    resource.EngineParticleData
	buckets     []resource.EngineDataBucket
	config      resource.EngineConfiguration
	instruction resource.EngineInstructionData
	ui          *resource.EngineConfigurationUI // non-nil if a UI struct was parsed
}

func buildResources(typeSpecs map[string]*ast.StructType, uiVarLit *ast.CompositeLit) (parsedResources, error) {
	var r parsedResources
	seen := map[string]bool{}

	for name, st := range typeSpecs {
		role := resourceRole(name)
		if role == "" {
			continue
		}
		if seen[role] {
			return parsedResources{}, fmt.Errorf("duplicate resource type %q (role %s): only one resource of this role is allowed", name, role)
		}
		seen[role] = true
		switch role {
		case "skin":
			r.skinST = st
			r.skin.RenderMode = skinRenderMode(st)
			r.skin.Sprites = collectFieldItems(st, func(name string, id int) resource.EngineSkinDataSprite {
				return resource.EngineSkinDataSprite{Name: resource.SkinSpriteName(name), ID: id}
			})
		case "effect":
			r.effect.Clips = collectFieldItems(st, func(name string, id int) resource.EngineEffectDataClip {
				return resource.EngineEffectDataClip{Name: resource.EffectClipName(name), ID: id}
			})
		case "particle":
			r.particle.Effects = collectFieldItems(st, func(name string, id int) resource.EngineParticleDataEffect {
				return resource.EngineParticleDataEffect{Name: resource.ParticleEffectName(name), ID: id}
			})
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
			ui, err := buildUI(st, uiVarLit)
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

// collectFieldItems iterates over the fields of a struct type and builds a slice
// of T by calling makeItem for each named field with auto-incrementing IDs.
func collectFieldItems[T any](st *ast.StructType, makeItem func(name string, id int) T) []T {
	var items []T
	for _, f := range st.Fields.List {
		for _, n := range f.Names {
			items = append(items, makeItem(n.Name, len(items)))
		}
	}
	return items
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
