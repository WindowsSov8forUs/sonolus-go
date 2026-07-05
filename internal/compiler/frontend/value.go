package frontend

import (
	"fmt"
	"sort"

	"github.com/WindowsSov8forUs/sonolus-go/internal/compiler/ir"
)

// NumKind distinguishes the three semantic categories of a traced value:
// scalar, record (composite with named fields), and array (indexed).
type numKind byte

const (
	kindScalar numKind = iota
	kindRecord
	kindArray
)

// Num is a traced value: scalar constant, IR expression, record (composite
// with named fields), or array of elements. When typeName is non-empty, the
// record carries a type tag (e.g. "quad", "vec2") enabling method dispatch on
// expression results (chain calls like f().Rotate(...)).
type Num struct {
	kind numKind

	isConst bool
	c       float64
	e       ir.Node

	// Record: each field is tracked as a separate Num so reads can be
	// constant-folded or SSA-folded by the optimizer without a memory read.
	fields   map[string]Num
	typeName string // record type name for method dispatch; empty = untyped

	// Array: per-element Nums. Fixed length, scalar or record elements.
	arr []Num
}

func constNum(v float64) Num { return Num{kind: kindScalar, isConst: true, c: v} }
func exprNum(n ir.Node) Num  { return Num{kind: kindScalar, e: n} }

func boolNum(b bool) Num {
	if b {
		return constNum(1)
	}
	return constNum(0)
}

// compNum creates an untyped record Num with individually tracked fields.
// For method return values that should support chaining, use compNumTyped.
func compNum(fields map[string]Num) Num { return Num{kind: kindRecord, fields: fields} }

// compNumTyped creates a typed record Num whose typeName enables method dispatch
// on the returned composite (e.g. chain calls like f().Rotate(...)). typeName
// must match a key in recordMethods (e.g. "quad", "vec2", "mat").
func compNumTyped(typeName string, fields map[string]Num) Num {
	return Num{kind: kindRecord, typeName: typeName, fields: fields}
}

// arrayNum creates an array Num with per-element values.
func arrayNum(elems []Num) Num { return Num{kind: kindArray, arr: elems} }

// IsScalar reports whether this value is a scalar (const or expression).
func (n Num) IsScalar() bool { return n.kind == kindScalar }

// IsComposite reports whether this value is a record with named fields.
func (n Num) IsComposite() bool { return n.kind == kindRecord }

// IsArray reports whether this value is an indexed array.
func (n Num) IsArray() bool { return n.kind == kindArray }

// Len returns the element count for arrays, 0 otherwise.
func (n Num) Len() int {
	if n.kind == kindArray {
		return len(n.arr)
	}
	return 0
}

// Index returns the Num at position i for arrays.
// Returns an error if the receiver is not an array or i is out of bounds.
func (n Num) Index(i int) (Num, error) {
	if n.kind != kindArray {
		return Num{}, fmt.Errorf("Num.Index: not an array")
	}
	if i < 0 || i >= len(n.arr) {
		return Num{}, fmt.Errorf("Num.Index: index %d out of bounds [0,%d)", i, len(n.arr))
	}
	return n.arr[i], nil
}

// CompositeSize returns the number of fields in a record.
func (n Num) CompositeSize() int {
	if n.kind != kindRecord {
		return 0
	}
	return len(n.fields)
}

// CompositeFieldOrder returns field names in deterministic sorted order.
// Returns an error if the receiver is not a record.
func (n Num) CompositeFieldOrder() ([]string, error) {
	if n.kind != kindRecord {
		return nil, fmt.Errorf("Num.CompositeFieldOrder: not a record")
	}
	out := make([]string, 0, len(n.fields))
	for k := range n.fields {
		out = append(out, k)
	}
	sort.Strings(out)
	return out, nil
}

// TryField returns the Num for a named field of a record, with ok=true.
// The returned Num carries the same typeName as the parent record, enabling
// chain calls through type-preserving methods.
// Returns ok=false if the receiver is not a record or the field does not exist.
func (n Num) TryField(name string) (Num, bool) {
	if n.kind != kindRecord {
		return Num{}, false
	}
	v, ok := n.fields[name]
	if ok && v.IsComposite() && v.typeName == "" {
		v.typeName = n.typeName
	}
	return v, ok
}

// MustField returns the Num for a named field of a record, or panics.
// For user-reachable code paths that may receive non-record values,
// use TryField which returns ok=false instead.
//
// MustField is safe to call in record-method implementations because the D2/D3
// type-driven dispatch system validates the receiver type before the method
// runs, guaranteeing both that the receiver is a record and that the named
// field exists.
//
// MustField follows the Go MustXxx naming convention for functions that panic
// on error (cf. regexp.MustCompile, template.Must).
func (n Num) MustField(name string) Num {
	v, ok := n.TryField(name)
	if !ok {
		panic("Num.MustField: unknown field " + name)
	}
	return v
}

// Field returns the named field from a record value, or an error if the
// receiver is not a record or the field does not exist. Prefer Field over
// MustField in user-reachable code paths (e.g. field access tracing) where
// the field name comes from user source and may be invalid.
func (n Num) Field(name string) (Num, error) {
	v, ok := n.TryField(name)
	if !ok {
		return Num{}, fmt.Errorf("Num.Field: unknown field %q", name)
	}
	return v, nil
}

// SetField updates a named field in a record.
// Returns an error if the receiver is not a record.
func (n *Num) SetField(name string, val Num) error {
	if n.kind != kindRecord {
		return fmt.Errorf("Num.SetField: not a record")
	}
	n.fields[name] = val
	return nil
}

// Node returns the IR node for this value: a Const for constants, or the
// tracked expression for memory expressions.
// Returns an error for records/arrays (which must be destructured into
// individual field/element nodes before lowering).
func (n Num) Node() (ir.Node, error) {
	if n.isConst {
		return ir.Const(n.c), nil
	}
	if n.kind != kindScalar {
		return nil, fmt.Errorf("Num.Node: non-scalar value has no single IR node; destructure fields/elements first")
	}
	return n.e, nil
}

// mustNode returns the IR node for a scalar value, or panics.
// It is intended for internal use in code paths where the Num has already been
// validated as scalar (e.g., after IsScalar() checks or in record-method
// implementations that destructure fields individually). Callers that may
// receive non-scalar values should use Node() and handle errors.
func (n Num) mustNode() ir.Node {
	nd, err := n.Node()
	if err != nil {
		panic(err)
	}
	return nd
}
