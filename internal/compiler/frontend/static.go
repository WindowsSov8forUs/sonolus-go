package frontend

import (
	"fmt"
	"go/constant"
	"go/types"

	"github.com/WindowsSov8forUs/sonolus-go/internal/compiler/source"
)

func dereferenceStatic(value source.StaticValue) source.StaticValue {
	if value.Kind == source.StaticPointer && value.Pointer != nil && value.Pointer.Object != nil {
		return value.Pointer.Object.Value
	}
	return value
}

func staticField(value source.StaticValue, field *types.Var) (source.StaticValue, bool) {
	value = dereferenceStatic(value)
	if value.Kind != source.StaticStruct {
		return source.StaticValue{}, false
	}
	for _, item := range value.Fields {
		if item.Field == field {
			return item.Value, true
		}
	}
	return source.StaticValue{}, false
}

func staticString(value source.StaticValue) (string, bool) {
	value = dereferenceStatic(value)
	if value.Kind != source.StaticConstant || value.Exact == nil || value.Exact.Kind() != constant.String {
		return "", false
	}
	return constant.StringVal(value.Exact), true
}

func staticNumber(value source.StaticValue) (float64, bool) {
	value = dereferenceStatic(value)
	if value.Kind != source.StaticConstant || value.Exact == nil {
		return 0, false
	}
	number, exact := constant.Float64Val(constant.ToFloat(value.Exact))
	return number, exact
}

func staticElements(value source.StaticValue) ([]source.StaticValue, bool) {
	value = dereferenceStatic(value)
	if value.Kind == source.StaticArray {
		return value.Elements, true
	}
	if value.Kind == source.StaticSliceValue && value.Slice != nil && value.Slice.Backing != nil && len(value.Slice.Path) == 0 {
		backing := dereferenceStatic(value.Slice.Backing.Value)
		if backing.Kind != source.StaticArray || value.Slice.Offset < 0 || value.Slice.Offset+value.Slice.Len > int64(len(backing.Elements)) {
			return nil, false
		}
		return backing.Elements[value.Slice.Offset : value.Slice.Offset+value.Slice.Len], true
	}
	return nil, false
}

func firstStaticCall(value source.StaticValue, seen map[*source.StaticObject]bool) *source.StaticCall {
	if value.Kind == source.StaticFunctionCall {
		return value.Call
	}
	switch value.Kind {
	case source.StaticArray:
		for _, element := range value.Elements {
			if call := firstStaticCall(element, seen); call != nil {
				return call
			}
		}
	case source.StaticStruct:
		for _, field := range value.Fields {
			if call := firstStaticCall(field.Value, seen); call != nil {
				return call
			}
		}
	case source.StaticPointer:
		if value.Pointer != nil && value.Pointer.Object != nil && !seen[value.Pointer.Object] {
			seen[value.Pointer.Object] = true
			return firstStaticCall(value.Pointer.Object.Value, seen)
		}
	case source.StaticInterface:
		if value.Dynamic != nil {
			return firstStaticCall(*value.Dynamic, seen)
		}
	case source.StaticSliceValue:
		if value.Slice != nil && value.Slice.Backing != nil && !seen[value.Slice.Backing] {
			seen[value.Slice.Backing] = true
			return firstStaticCall(value.Slice.Backing.Value, seen)
		}
	}
	return nil
}

func pureStaticError(value source.StaticValue, context string) error {
	call := firstStaticCall(value, map[*source.StaticObject]bool{})
	if call == nil {
		return nil
	}
	name := "unknown function"
	if call.Object != nil {
		name = call.Object.FullName()
	}
	return fmt.Errorf("%s: %s must be a pure static value; call to %s is not allowed", call.Pos, context, name)
}
