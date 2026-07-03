package engine

import (
	"fmt"
	"go/ast"
	"reflect"

	"github.com/WindowsSov8forUs/sonolus-core-go/core/resource"
)

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

// buildSpriteIndex builds a sprite-name → ID map from parsed skin sprites.
// Returns nil if no skin is available (e.g., before resources are built).
func buildSpriteIndex(skin resource.EngineSkinData) map[string]float64 {
	m := make(map[string]float64, len(skin.Sprites))
	for _, s := range skin.Sprites {
		m[string(s.Name)] = float64(s.ID)
	}
	return m
}

// spriteID looks up a sprite name in the Skin struct and returns its positional
// index (ID), or an error if the name is unknown or no Skin struct was declared.
func spriteID(skinST *ast.StructType, name string) (int, error) {
	if skinST == nil {
		return 0, fmt.Errorf("engine: unknown sprite name %q (no skin struct declared)", name)
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
	return 0, fmt.Errorf("engine: unknown sprite name %q", name)
}
