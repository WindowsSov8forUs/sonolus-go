package engine

import (
	"fmt"
	"go/ast"
	"go/token"
	"reflect"
	"strconv"
	"strings"
)

// uiEvalContext holds the registered UI constant names for enum resolution.
type uiEvalContext struct {
	constants map[string]string // ident name → constant string value
}

// newUIEvalContext builds a context with known sonolus UI constants pre-registered.
func newUIEvalContext() *uiEvalContext {
	return &uiEvalContext{
		constants: map[string]string{
			// Metrics
			"MetricArcade": "arcade", "MetricArcadePercentage": "arcadePercentage",
			"MetricAccuracy": "accuracy", "MetricAccuracyPercentage": "accuracyPercentage",
			"MetricLife": "life", "MetricPerfect": "perfect", "MetricPerfectPercentage": "perfectPercentage",
			"MetricGreatGoodMiss": "greatGoodMiss", "MetricGreatGoodMissPct": "greatGoodMissPercentage",
			"MetricMiss": "miss", "MetricMissPercentage": "missPercentage", "MetricErrorHeatmap": "errorHeatmap",
			// Judgment error styles
			"JudgmentErrorNone": "none", "JudgmentErrorLate": "late", "JudgmentErrorEarly": "early",
			"JudgmentErrorPlus": "plus", "JudgmentErrorMinus": "minus",
			"JudgmentErrorArrowUp": "arrowUp", "JudgmentErrorArrowDown": "arrowDown",
			"JudgmentErrorArrowLeft": "arrowLeft", "JudgmentErrorArrowRight": "arrowRight",
			"JudgmentErrorTriangleUp": "triangleUp", "JudgmentErrorTriangleDown": "triangleDown",
			"JudgmentErrorTriangleLeft": "triangleLeft", "JudgmentErrorTriangleRight": "triangleRight",
			// Judgment error placements
			"JudgmentErrorPlacementLeft": "left", "JudgmentErrorPlacementRight": "right",
			"JudgmentErrorPlacementLeftRight": "leftRight", "JudgmentErrorPlacementTop": "top",
			"JudgmentErrorPlacementBottom": "bottom", "JudgmentErrorPlacementTopBottom": "topBottom",
			"JudgmentErrorPlacementCenter": "center",
			// Easing
			"EasingLinear": "linear", "EasingNone": "none",
			"EasingInSine": "inSine", "EasingInQuad": "inQuad", "EasingInCubic": "inCubic",
			"EasingInQuart": "inQuart", "EasingInQuint": "inQuint",
			"EasingInExpo": "inExpo", "EasingInCirc": "inCirc",
			"EasingInBack": "inBack", "EasingInElastic": "inElastic",
			"EasingOutSine": "outSine", "EasingOutQuad": "outQuad", "EasingOutCubic": "outCubic",
			"EasingOutQuart": "outQuart", "EasingOutQuint": "outQuint",
			"EasingOutExpo": "outExpo", "EasingOutCirc": "outCirc",
			"EasingOutBack": "outBack", "EasingOutElastic": "outElastic",
			"EasingInOutSine": "inOutSine", "EasingInOutQuad": "inOutQuad",
			"EasingInOutCubic": "inOutCubic", "EasingInOutQuart": "inOutQuart",
			"EasingInOutQuint": "inOutQuint", "EasingInOutExpo": "inOutExpo",
			"EasingInOutCirc": "inOutCirc", "EasingInOutBack": "inOutBack",
			"EasingInOutElastic": "inOutElastic", "EasingOutInSine": "outInSine",
			"EasingOutInQuad": "outInQuad", "EasingOutInCubic": "outInCubic",
			"EasingOutInQuart": "outInQuart", "EasingOutInQuint": "outInQuint",
			"EasingOutInExpo": "outInExpo", "EasingOutInCirc": "outInCirc",
			"EasingOutInBack": "outInBack", "EasingOutInElastic": "outInElastic",
		},
	}
}

// evaluateUIConfig walks the UI struct type to find tagged fields, then reads
// their values from the composite literal. Returns a flat map of sonolus tag
// key → string value. Nested structs (Visibility, Tween, Animation) are
// flattened into dot-separated keys matching the existing uiSetters.
func evaluateUIConfig(uiType *ast.StructType, uiLit *ast.CompositeLit, ctx *uiEvalContext) (map[string]string, error) {
	// Phase 1: build tagKey → fieldName index from the type definition.
	type uiField struct {
		name   string
		tagKey string // e.g. "primaryMetric", "judgmentAnimation"
	}
	var fields []uiField
	for _, f := range uiType.Fields.List {
		if f.Tag == nil || len(f.Names) == 0 {
			continue
		}
		tag := reflect.StructTag(stringLit(f.Tag.Value)).Get("sonolus")
		if tag == "" {
			continue
		}
		key, _ := keyVal(tag)
		if key == "" {
			continue
		}
		// RuntimeUiConfig: "ui" tag expands to field.scale + field.alpha
		if key == "ui" && resolveFieldTypeName(f.Type) == "RuntimeUiConfig" {
			for _, n := range f.Names {
				name := n.Name
				prefix := strings.ToLower(name[:1]) + name[1:]
				fields = append(fields, uiField{name: name + ".Scale", tagKey: prefix + ".scale"})
				fields = append(fields, uiField{name: name + ".Alpha", tagKey: prefix + ".alpha"})
			}
			continue
		}
		for _, n := range f.Names {
			fields = append(fields, uiField{name: n.Name, tagKey: key})
		}
	}

	// Phase 2: extract values and flatten.
	result := make(map[string]string)
	for _, elt := range uiLit.Elts {
		kv, ok := elt.(*ast.KeyValueExpr)
		if !ok {
			continue
		}
		keyIdent, ok := kv.Key.(*ast.Ident)
		if !ok {
			continue
		}
		for _, f := range fields {
			if f.name != keyIdent.Name {
				continue
			}
			val, err := evalUIValue(kv.Value, ctx)
			if err != nil {
				return nil, fmt.Errorf("field %q: %w", f.name, err)
			}
			flattenUIVal(f.tagKey, "", val, result)
			break
		}
	}
	return result, nil
}

// flattenUIVal flattens a typed UI value into the flat key=value map expected
// by uiSetters. Scalar values and strings are stored directly; nested struct
// maps are flattened with key prefixes (e.g., "menuVisibility" + "Scale" → "menuVisibilityScale").
func flattenUIVal(tagKey, suffix string, val any, dst map[string]string) {
	switch v := val.(type) {
	case string:
		dst[tagKey+suffix] = v
	case float64:
		dst[tagKey+suffix] = strconv.FormatFloat(v, 'f', -1, 64)
	case map[string]any:
		for subKey, subVal := range v {
			// Map known nested keys to the flat uiSetters convention:
			// Animation{Scale: Tween{From: 0}} → judgmentAnimationScaleFrom
			flattenUIVal(tagKey, suffix+subKey, subVal, dst)
		}
	}
}

// evalUIValue evaluates an AST expression to a concrete Go value (float64,
// string, or map[string]any for nested structs). Only constant expressions
// are supported.
func evalUIValue(expr ast.Expr, ctx *uiEvalContext) (any, error) {
	switch e := expr.(type) {
	case *ast.BasicLit:
		switch e.Kind {
		case token.INT:
			v, err := strconv.ParseInt(e.Value, 0, 64)
			if err != nil {
				return nil, fmt.Errorf("invalid int %q: %w", e.Value, err)
			}
			return float64(v), nil
		case token.FLOAT:
			v, err := strconv.ParseFloat(e.Value, 64)
			if err != nil {
				return nil, fmt.Errorf("invalid float %q: %w", e.Value, err)
			}
			return v, nil
		case token.STRING:
			s, err := strconv.Unquote(e.Value)
			if err != nil {
				return nil, fmt.Errorf("invalid string %q: %w", e.Value, err)
			}
			return s, nil
		default:
			return nil, fmt.Errorf("unsupported literal kind %v", e.Kind)
		}
	case *ast.Ident:
		// Bare constant name: MetricArcade → "arcade"
		if v, ok := ctx.constants[e.Name]; ok {
			return v, nil
		}
		return nil, fmt.Errorf("unknown UI constant %q", e.Name)
	case *ast.SelectorExpr:
		// Package-qualified: sonolus.MetricArcade → strip pkg, resolve constant
		if pkg, ok := e.X.(*ast.Ident); ok && pkg.Name == "sonolus" {
			if v, ok2 := ctx.constants[e.Sel.Name]; ok2 {
				return v, nil
			}
			return nil, fmt.Errorf("unknown sonolus UI constant %q", e.Sel.Name)
		}
		return nil, fmt.Errorf("unsupported selector %v", e)
	case *ast.CompositeLit:
		// Nested struct: sonolus.Visibility{Scale: 0.8, Alpha: 1}
		return evalUIComposite(e, ctx)
	case *ast.UnaryExpr:
		if e.Op == token.SUB {
			v, err := evalUIValue(e.X, ctx)
			if err != nil {
				return nil, err
			}
			if f, ok := v.(float64); ok {
				return -f, nil
			}
			return nil, fmt.Errorf("unary minus requires numeric value")
		}
		return nil, fmt.Errorf("unsupported unary operator %v", e.Op)
	default:
		return nil, fmt.Errorf("unsupported UI value expression %T", expr)
	}
}

// evalUIComposite evaluates a composite literal for a nested UI struct (e.g.
// sonolus.Visibility{Scale: 0.8, Alpha: 1}). Returns map[string]any.
func evalUIComposite(lit *ast.CompositeLit, ctx *uiEvalContext) (map[string]any, error) {
	result := make(map[string]any)
	for _, elt := range lit.Elts {
		kv, ok := elt.(*ast.KeyValueExpr)
		if !ok {
			return nil, fmt.Errorf("nested composite literal must use key:value syntax")
		}
		keyIdent, ok := kv.Key.(*ast.Ident)
		if !ok {
			return nil, fmt.Errorf("nested composite key must be an identifier")
		}
		val, err := evalUIValue(kv.Value, ctx)
		if err != nil {
			return nil, fmt.Errorf("field %q: %w", keyIdent.Name, err)
		}
		result[keyIdent.Name] = val
	}
	return result, nil
}

// findUIVar scans a list of AST declarations for a variable of type UI
// initialized with a composite literal. Returns nil if not found.
func findUIVar(decls []ast.Decl) *ast.CompositeLit {
	for _, d := range decls {
		gd, ok := d.(*ast.GenDecl)
		if !ok || gd.Tok != token.VAR {
			continue
		}
		for _, spec := range gd.Specs {
			vs, ok := spec.(*ast.ValueSpec)
			if !ok || len(vs.Names) == 0 || len(vs.Values) == 0 {
				continue
			}
			name := vs.Names[0].Name
			if name != "ui" && name != "uiConfig" {
				continue
			}
			// Check the type is right: UI or UIConfig
			if lit, ok2 := vs.Values[0].(*ast.CompositeLit); ok2 {
				if typeIdent, ok3 := lit.Type.(*ast.Ident); ok3 && typeIdent.Name == "UI" {
					return lit
				}
			}
			if typeName, ok := vs.Type.(*ast.Ident); ok && typeName.Name == "UI" {
				if lit, ok2 := vs.Values[0].(*ast.CompositeLit); ok2 {
					return lit
				}
			}
		}
	}
	return nil
}

// reflectTagValue extracts a struct tag value for the given key, handling
// both raw string tags (\`sonolus:"...\"\`) and backtick-delimited tags.
func reflectTagValue(raw, key string) string {
	raw = strings.TrimSpace(raw)
	if len(raw) < len(key)+4 {
		return ""
	}
	// Look for key:"value" pattern.
	idx := strings.Index(raw, key+`:"`)
	if idx < 0 {
		return ""
	}
	start := idx + len(key) + 2
	end := strings.IndexByte(raw[start:], '"')
	if end < 0 {
		return ""
	}
	return raw[start : start+end]
}
