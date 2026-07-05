package engine

import (
	"fmt"
	"go/ast"
	"strconv"
	"strings"

	"github.com/WindowsSov8forUs/sonolus-go/internal/compiler/frontend"
)

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
	"EntityInfo":        {"index", "archetype", "state"},
	"EntityRef":         {"index"},
	"JudgmentWindow":    {"perfectMin", "perfectMax", "greatMin", "greatMax", "goodMin", "goodMax"},
}

// ContainerFieldMeta stores compile-time metadata for a container-typed struct
// field (VarArray, ArrayMap, ArraySet, FrozenNumSet). It is threaded from the
// engine parse layer to the frontend tracer so method dispatch can emit correct
// IR for memory-backed container operations.
type ContainerFieldMeta struct {
	Name     string // Go field name, e.g. "Candidates"
	TypeName string // record type: "varArray", "arrayMap", "arraySet", "frozenNumSet"
	Capacity int    // max element count (from tag)
	ElemSize int    // slots per element: 1 for VarArray/ArraySet, 2 for ArrayMap
}

// containerTypeNames maps Go type names to their record type names and element sizes.
var containerTypeNames = map[string]struct {
	recordName string
	elemSize   int
}{
	"VarArray":     {"varArray", 1},
	"ArrayMap":     {"arrayMap", 2},
	"ArraySet":     {"arraySet", 1},
	"FrozenNumSet": {"frozenNumSet", 1},
}

// parseContainerTag extracts capacity from a sonolus struct tag.
// Accepted format: "memory,cap=64" — key=value after comma.
// Returns 0 if capacity is not specified or unparseable.
func parseContainerTag(tag string) (capacity int, ok bool) {
	parts := splitTag(tag)
	for _, p := range parts {
		if v, found := strings.CutPrefix(p, "cap="); found {
			n, err := strconv.Atoi(v)
			if err == nil && n > 0 {
				return n, true
			}
		}
	}
	return 0, false
}

// appendContainerFieldNames expands a container struct field into its slot names.
// For a VarArray with capacity N: ["Field.__size__", "Field.__data_0__", ..., "Field.__data_{N-1}__"]
// For an ArrayMap with capacity N: ["Field.__size__", "Field.__key_0__", "Field.__val_0__", ...]
// For ArraySet/FrozenNumSet: same as VarArray.
func appendContainerFieldNames(dst []string, fieldName string, typeName string, capacity int) []string {
	dst = append(dst, fieldName+".__size__")
	meta, ok := containerTypeNames[typeName]
	if !ok {
		return dst
	}
	if meta.elemSize == 2 {
		for i := 0; i < capacity; i++ {
			dst = append(dst, fmt.Sprintf("%s.__key_%d__", fieldName, i))
			dst = append(dst, fmt.Sprintf("%s.__val_%d__", fieldName, i))
		}
	} else {
		for i := 0; i < capacity; i++ {
			dst = append(dst, fmt.Sprintf("%s.__data_%d__", fieldName, i))
		}
	}
	return dst
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

func frontendContainerFieldMeta(in []ContainerFieldMeta) []frontend.ContainerFieldMeta {
	out := make([]frontend.ContainerFieldMeta, len(in))
	for i, c := range in {
		out[i] = frontend.ContainerFieldMeta{Name: c.Name, TypeName: c.TypeName, Capacity: c.Capacity, ElemSize: c.ElemSize}
	}
	return out
}
