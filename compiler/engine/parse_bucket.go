package engine

import (
	"fmt"
	"go/ast"
	"reflect"

	"github.com/WindowsSov8forUs/sonolus-core-go/core"
	"github.com/WindowsSov8forUs/sonolus-core-go/core/resource"
)

// parseBucketSpriteTag parses a sprite field's sonolus tag into an
// EngineDataBucketSprite. It handles sprite=, x=, y=, w=, h=, rotation=,
// and fallback= key-value pairs.
func parseBucketSpriteTag(stag string, skinST *ast.StructType) (resource.EngineDataBucketSprite, error) {
	spr := resource.EngineDataBucketSprite{}
	for _, p := range splitTag(stag) {
		k, v := keyVal(p)
		switch k {
		case "sprite":
			id, err := spriteID(skinST, v)
			if err != nil {
				return spr, fmt.Errorf("bucket sprite: %w", err)
			}
			spr.ID = id
		case "x":
			x, err := parseFloat(v)
			if err != nil {
				return spr, err
			}
			spr.X = x
		case "y":
			y, err := parseFloat(v)
			if err != nil {
				return spr, err
			}
			spr.Y = y
		case "w":
			w, err := parseFloat(v)
			if err != nil {
				return spr, err
			}
			spr.W = w
		case "h":
			h, err := parseFloat(v)
			if err != nil {
				return spr, err
			}
			spr.H = h
		case "rotation":
			r, err := parseFloat(v)
			if err != nil {
				return spr, err
			}
			spr.Rotation = r
		case "fallback":
			id, err := spriteID(skinST, v)
			if err != nil {
				return spr, fmt.Errorf("bucket fallback sprite: %w", err)
			}
			spr.FallbackID = id
		}
	}
	return spr, nil
}

// buildBuckets parses a Buckets struct from the engine source and returns a
// list of EngineDataBucket. Each bucket field with sonolus:"bucket" tag creates
// one bucket; its nested struct fields (tagged with sprite attributes) become
// bucket sprites.
func buildBuckets(st *ast.StructType, skinST *ast.StructType) ([]resource.EngineDataBucket, error) {
	var out []resource.EngineDataBucket
	for _, f := range st.Fields.List {
		tag := reflect.StructTag(stringLit(f.Tag.Value)).Get("sonolus")
		tagParts := splitTag(tag)
		if len(tagParts) == 0 || tagParts[0] != "bucket" || len(f.Names) == 0 {
			continue
		}
		ft, ok := f.Type.(*ast.StructType)
		if !ok {
			out = append(out, resource.EngineDataBucket{})
			continue
		}
		var sprites []resource.EngineDataBucketSprite
		for _, sf := range ft.Fields.List {
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
