package engine

import "go/ast"

// recordFieldLayout maps known record type names to their sub-field layouts
// (in memory order). Used to expand record-typed struct fields into multiple
// float64 slots.
var recordFieldLayouts = map[string][]string{
	"Vec2":     {"x", "y"},
	"Quad":     {"blx", "bly", "tlx", "tly", "trx", "try", "brx", "bry"},
	"Mat":      {"m11", "m12", "m13", "m21", "m22", "m23"},
	"Rect":     {"t", "r", "b", "l"},
	"Trans":    {"m11", "m12", "m13", "m21", "m22", "m23", "m31", "m32", "m33"},
	"Pair":     {"first", "second"},
	"EntityRef": {"index"},
}

// resolveFieldTypeName extracts the Go type name from a field's type AST node.
// It handles both bare identifiers (Vec2) and package-qualified selectors
// (sonolus.Vec2). Returns the type name, or "" if the type is not a named type
// (e.g. float64, int, bool, []float64, etc.).
func resolveFieldTypeName(expr ast.Expr) string {
	switch t := expr.(type) {
	case *ast.Ident:
		return t.Name
	case *ast.SelectorExpr:
		if pkg, ok := t.X.(*ast.Ident); ok && pkg.Name == "sonolus" {
			return t.Sel.Name
		}
	}
	return ""
}

// resolveFieldLayout returns the sub-field layout if the given type expression
// names a known record type (Vec2, Quad, Mat, Rect, Trans, Pair). Returns nil
// for scalar types and unknown types.
func resolveFieldLayout(expr ast.Expr) ([]string, bool) {
	name := resolveFieldTypeName(expr)
	fields, ok := recordFieldLayouts[name]
	return fields, ok
}

// appendFieldNames appends field names to dst. If the field type is a known
// record type (Vec2, Quad, etc.), the field name is expanded into sub-field
// names like "pos.x", "pos.y". For scalar types, the original name is used.
func appendFieldNames(dst []string, fieldName string, fieldType ast.Expr) []string {
	if fields, ok := resolveFieldLayout(fieldType); ok {
		for _, sf := range fields {
			dst = append(dst, fieldName+"."+sf)
		}
	} else {
		dst = append(dst, fieldName)
	}
	return dst
}
