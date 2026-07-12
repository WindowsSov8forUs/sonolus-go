package tracer

import (
	"go/ast"
	"go/constant"
	"go/token"
	"go/types"
)

// StaticKind identifies the representation stored in a StaticValue.
type StaticKind uint8

const (
	StaticInvalid StaticKind = iota
	StaticConstant
	StaticNil
	StaticArray
	StaticStruct
	StaticSliceValue
	StaticMapValue
	StaticPointer
	StaticInterface
	StaticFunctionCall
)

type StaticPathKind uint8

const (
	StaticPathField StaticPathKind = iota
	StaticPathElement
)

type StaticPathStep struct {
	Kind  StaticPathKind
	Index int64
	Field *types.Var
}

// StaticAddress identifies storage by object identity and a field or element path.
type StaticAddress struct {
	Object *StaticObject
	Path   []StaticPathStep
	// ArrayOffset and ArrayView describe the array view created by a safe
	// slice-to-array-pointer conversion. They do not change pointer identity.
	ArrayOffset int64
	ArrayView   types.Type
}

// StaticSlice describes a view into an array storage object.
type StaticSlice struct {
	Backing *StaticObject
	Path    []StaticPathStep
	Offset  int64
	Len     int64
	Cap     int64
}

type StaticMapEntry struct {
	Key   StaticValue
	Value StaticValue
}

// StaticMap preserves map identity and source insertion order.
type StaticMap struct {
	ID      uint64
	Type    *types.Map
	Entries []StaticMapEntry
}

type StaticField struct {
	Field *types.Var
	Value StaticValue
}

// StaticCall preserves a statically resolvable function call without executing
// it or assigning domain-specific meaning to it.
type StaticCall struct {
	Object    *types.Func
	Receiver  *StaticValue
	Args      []StaticValue
	Expr      *ast.CallExpr
	Pos       token.Position
	Signature *types.Signature
}

// StaticValue is a typed value in the supported compile-time subset.
type StaticValue struct {
	Type     types.Type
	Kind     StaticKind
	Exact    constant.Value
	Elements []StaticValue
	Fields   []StaticField
	Slice    *StaticSlice
	Map      *StaticMap
	Pointer  *StaticAddress
	Dynamic  *StaticValue
	Call     *StaticCall
}

// StaticObject is an identity-bearing storage allocation within one tracer.
type StaticObject struct {
	ID    uint64
	Type  types.Type
	Value StaticValue

	owner *staticVarState
}

// StaticBinding associates a declaration with its immutable evaluated graph.
// Storage is nil for constants.
type StaticBinding struct {
	Name    string
	Object  types.Object
	Storage *StaticObject
	Value   StaticValue
}

func cloneStaticPath(path []StaticPathStep) []StaticPathStep {
	return append([]StaticPathStep(nil), path...)
}

func cloneStaticAddress(address *StaticAddress) *StaticAddress {
	if address == nil {
		return nil
	}
	return &StaticAddress{
		Object:      address.Object,
		Path:        cloneStaticPath(address.Path),
		ArrayOffset: address.ArrayOffset,
		ArrayView:   address.ArrayView,
	}
}

func cloneStaticValue(value StaticValue) StaticValue {
	result := value
	switch value.Kind {
	case StaticArray:
		result.Elements = make([]StaticValue, len(value.Elements))
		for index, element := range value.Elements {
			result.Elements[index] = cloneStaticValue(element)
		}
	case StaticStruct:
		result.Fields = make([]StaticField, len(value.Fields))
		for index, field := range value.Fields {
			result.Fields[index] = StaticField{
				Field: field.Field,
				Value: cloneStaticValue(field.Value),
			}
		}
	case StaticPointer:
		result.Pointer = cloneStaticAddress(value.Pointer)
	case StaticSliceValue:
		if value.Slice != nil {
			result.Slice = &StaticSlice{
				Backing: value.Slice.Backing,
				Path:    cloneStaticPath(value.Slice.Path),
				Offset:  value.Slice.Offset,
				Len:     value.Slice.Len,
				Cap:     value.Slice.Cap,
			}
		}
	case StaticInterface:
		if value.Dynamic != nil {
			dynamic := cloneStaticValue(*value.Dynamic)
			result.Dynamic = &dynamic
		}
	case StaticFunctionCall:
		if value.Call != nil {
			result.Call = &StaticCall{Object: value.Call.Object, Expr: value.Call.Expr, Pos: value.Call.Pos, Signature: value.Call.Signature, Args: make([]StaticValue, len(value.Call.Args))}
			if value.Call.Receiver != nil {
				receiver := cloneStaticValue(*value.Call.Receiver)
				result.Call.Receiver = &receiver
			}
			for i, arg := range value.Call.Args {
				result.Call.Args[i] = cloneStaticValue(arg)
			}
		}
	}
	return result
}

func staticAddressEqual(left, right *StaticAddress) bool {
	if left == nil || right == nil {
		return left == right
	}
	if left.Object != right.Object || left.ArrayOffset != right.ArrayOffset || len(left.Path) != len(right.Path) {
		return false
	}
	for index := range left.Path {
		leftStep := left.Path[index]
		rightStep := right.Path[index]
		if leftStep.Kind != rightStep.Kind || leftStep.Index != rightStep.Index || leftStep.Field != rightStep.Field {
			return false
		}
	}
	return true
}
