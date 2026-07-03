package engine

import (
	"fmt"
	"go/ast"
	"reflect"

	"github.com/WindowsSov8forUs/sonolus-core-go/core"
	"github.com/WindowsSov8forUs/sonolus-core-go/core/resource"
)

// tagDecoder maps tag key names to value setters. It provides a data-driven
// alternative to the repetitive switch-case pattern common across the engine
// resource builders (buildBuckets, buildConfig, buildUI, buildSkin).
type tagDecoder map[string]func(v string) error

func (d tagDecoder) decode(tag string) error {
	for _, p := range splitTag(tag) {
		k, v := keyVal(p)
		if setter, ok := d[k]; ok {
			if err := setter(v); err != nil {
				return err
			}
		}
	}
	return nil
}

// parseBucketSpriteTag parses a sprite field's sonolus tag into an
// EngineDataBucketSprite. It handles sprite=, x=, y=, w=, h=, rotation=,
// and fallback= key-value pairs.
func parseBucketSpriteTag(stag string, skinST *ast.StructType) (resource.EngineDataBucketSprite, error) {
	spr := resource.EngineDataBucketSprite{}
	dec := tagDecoder{
		"sprite": func(v string) error {
			id, err := spriteID(skinST, v)
			if err != nil {
				return fmt.Errorf("bucket sprite: %w", err)
			}
			spr.ID = id
			return nil
		},
		"fallback": func(v string) error {
			id, err := spriteID(skinST, v)
			if err != nil {
				return fmt.Errorf("bucket fallback sprite: %w", err)
			}
			spr.FallbackID = id
			return nil
		},
		"x":        func(v string) error { x, err := parseFloat(v); spr.X = x; return err },
		"y":        func(v string) error { y, err := parseFloat(v); spr.Y = y; return err },
		"w":        func(v string) error { w, err := parseFloat(v); spr.W = w; return err },
		"h":        func(v string) error { h, err := parseFloat(v); spr.H = h; return err },
		"rotation": func(v string) error { r, err := parseFloat(v); spr.Rotation = r; return err },
	}
	if err := dec.decode(stag); err != nil {
		return spr, err
	}
	return spr, nil
}

// buildBuckets parses a Buckets struct from the engine source and returns a
// list of EngineDataBucket. Each bucket field with sonolus:"bucket" tag creates
// one bucket; its nested struct fields (tagged with sprite attributes) become
// bucket sprites.
func buildBuckets(st *ast.StructType, skinST *ast.StructType) ([]resource.EngineDataBucket, error) {
	out := make([]resource.EngineDataBucket, 0, len(st.Fields.List))
	for _, f := range st.Fields.List {
		if f.Tag == nil || len(f.Names) == 0 {
			continue
		}
		tag := reflect.StructTag(stringLit(f.Tag.Value)).Get("sonolus")
		tagParts := splitTag(tag)
		if len(tagParts) == 0 || tagParts[0] != "bucket" {
			continue
		}
		ft, ok := f.Type.(*ast.StructType)
		if !ok {
			return nil, fmt.Errorf("buildBuckets: field %s tagged sonolus:\"bucket\" must be a struct type, got %T", f.Names[0].Name, f.Type)
		}
		sprites := make([]resource.EngineDataBucketSprite, 0, len(ft.Fields.List))
		for _, sf := range ft.Fields.List {
			if sf.Tag == nil {
				continue
			}
			for range sf.Names {
				spr := resource.EngineDataBucketSprite{}
				if stag, ok := reflect.StructTag(stringLit(sf.Tag.Value)).Lookup("sonolus"); ok {
					var err error
					spr, err = parseBucketSpriteTag(stag, skinST)
					if err != nil {
						return nil, err
					}
				}
				sprites = append(sprites, spr)
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
