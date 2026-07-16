package tracer

import (
	"fmt"
	"go/constant"
	"go/token"
	"go/types"
	"math"
	"math/big"
	"unicode/utf8"

	"golang.org/x/tools/go/packages"
)

const (
	maxStaticElements      = 1 << 16
	maxStaticGraphElements = 1 << 18
	maxStaticStringBytes   = 1 << 20
)

func staticConstant(typ types.Type, value constant.Value) StaticValue {
	return StaticValue{Type: typ, Kind: StaticConstant, Exact: value}
}

func staticNil(typ types.Type) StaticValue {
	return StaticValue{Type: typ, Kind: StaticNil}
}

func underlyingType(typ types.Type) types.Type {
	if typ == nil {
		return nil
	}
	return types.Unalias(typ).Underlying()
}

func isNilable(typ types.Type) bool {
	switch underlyingType(typ).(type) {
	case *types.Pointer, *types.Slice, *types.Map, *types.Interface, *types.Signature, *types.Chan:
		return true
	default:
		return false
	}
}

func (e *staticEvaluator) zeroValue(pkg *packages.Package, typ types.Type) (StaticValue, error) {
	cost, ok := staticValueCost(typ, maxStaticElements)
	if !ok || cost > maxStaticGraphElements-e.allocatedElements {
		return StaticValue{}, e.errorAt(pkg, nil, nil, fmt.Errorf("value is too large: %w", ErrNotStatic))
	}
	e.allocatedElements += cost
	return e.zeroValueUnchecked(pkg, typ)
}

func staticValueCost(typ types.Type, limit int64) (int64, bool) {
	if limit < 1 {
		return 0, false
	}
	switch valueType := underlyingType(typ).(type) {
	case *types.Array:
		childCost, ok := staticValueCost(valueType.Elem(), limit-1)
		if !ok || valueType.Len() > (limit-1)/childCost {
			return 0, false
		}
		return 1 + valueType.Len()*childCost, true
	case *types.Struct:
		cost := int64(1)
		for index := 0; index < valueType.NumFields(); index++ {
			childCost, ok := staticValueCost(valueType.Field(index).Type(), limit-cost)
			if !ok || childCost > limit-cost {
				return 0, false
			}
			cost += childCost
		}
		return cost, true
	default:
		return 1, true
	}
}

func (e *staticEvaluator) zeroValueUnchecked(pkg *packages.Package, typ types.Type) (StaticValue, error) {
	switch valueType := underlyingType(typ).(type) {
	case *types.Basic:
		switch {
		case valueType.Info()&types.IsBoolean != 0:
			return staticConstant(typ, constant.MakeBool(false)), nil
		case valueType.Info()&types.IsString != 0:
			return staticConstant(typ, constant.MakeString("")), nil
		case valueType.Info()&types.IsNumeric != 0:
			return e.convertConstant(pkg, constant.MakeInt64(0), typ)
		case valueType.Kind() == types.UnsafePointer:
			return staticNil(typ), nil
		default:
			return StaticValue{}, e.errorAt(pkg, nil, nil, fmt.Errorf("unsupported zero value type %s: %w", typ, ErrNotStatic))
		}
	case *types.Array:
		if valueType.Len() > maxStaticElements {
			return StaticValue{}, e.errorAt(pkg, nil, nil, fmt.Errorf("array is too large: %w", ErrNotStatic))
		}
		elements := make([]StaticValue, int(valueType.Len()))
		for index := range elements {
			value, err := e.zeroValueUnchecked(pkg, valueType.Elem())
			if err != nil {
				return StaticValue{}, err
			}
			elements[index] = value
		}
		return StaticValue{Type: typ, Kind: StaticArray, Elements: elements}, nil
	case *types.Struct:
		fields := make([]StaticField, valueType.NumFields())
		for index := range fields {
			field := valueType.Field(index)
			value, err := e.zeroValueUnchecked(pkg, field.Type())
			if err != nil {
				return StaticValue{}, err
			}
			fields[index] = StaticField{Field: field, Value: value}
		}
		return StaticValue{Type: typ, Kind: StaticStruct, Fields: fields}, nil
	case *types.Pointer, *types.Slice, *types.Map, *types.Interface, *types.Signature, *types.Chan:
		return staticNil(typ), nil
	default:
		return StaticValue{}, e.errorAt(pkg, nil, nil, fmt.Errorf("unsupported zero value type %s: %w", typ, ErrNotStatic))
	}
}

func constantRat(value constant.Value) (*big.Rat, bool) {
	switch raw := constant.Val(value).(type) {
	case int64:
		return new(big.Rat).SetInt64(raw), true
	case *big.Int:
		return new(big.Rat).SetInt(raw), true
	case *big.Rat:
		return new(big.Rat).Set(raw), true
	case *big.Float:
		rat, _ := raw.Rat(nil)
		return rat, rat != nil
	default:
		return nil, false
	}
}

func constantInteger(value constant.Value) (*big.Int, bool) {
	rat, ok := constantRat(value)
	if !ok {
		return nil, false
	}
	return new(big.Int).Quo(rat.Num(), rat.Denom()), true
}

func integerBits(pkg *packages.Package, typ types.Type, kind types.BasicKind) (uint, bool) {
	switch kind {
	case types.Int8, types.Uint8:
		return 8, true
	case types.Int16, types.Uint16:
		return 16, true
	case types.Int32, types.Uint32, types.UntypedRune:
		return 32, true
	case types.Int64, types.Uint64:
		return 64, true
	case types.Int, types.Uint, types.Uintptr:
		if pkg == nil || pkg.TypesSizes == nil {
			return 0, false
		}
		size := pkg.TypesSizes.Sizeof(typ)
		if size <= 0 {
			return 0, false
		}
		return uint(size * 8), true
	default:
		return 0, false
	}
}

func wrapInteger(value *big.Int, bits uint, signed bool) *big.Int {
	modulus := new(big.Int).Lsh(big.NewInt(1), bits)
	result := new(big.Int).Mod(new(big.Int).Set(value), modulus)
	if signed && result.Bit(int(bits-1)) != 0 {
		result.Sub(result, modulus)
	}
	return result
}

func integerFits(pkg *packages.Package, value *big.Int, target types.Type, basic *types.Basic) (bool, error) {
	bits, ok := integerBits(pkg, target, basic.Kind())
	if !ok {
		return false, ErrMissingTypeInfo
	}
	if basic.Info()&types.IsUnsigned != 0 {
		if value.Sign() < 0 {
			return false, nil
		}
		limit := new(big.Int).Lsh(big.NewInt(1), bits)
		return value.Cmp(limit) < 0, nil
	}
	limit := new(big.Int).Lsh(big.NewInt(1), bits-1)
	minimum := new(big.Int).Neg(new(big.Int).Set(limit))
	maximum := new(big.Int).Sub(limit, big.NewInt(1))
	return value.Cmp(minimum) >= 0 && value.Cmp(maximum) <= 0, nil
}

func (e *staticEvaluator) convertConstant(pkg *packages.Package, value constant.Value, target types.Type) (StaticValue, error) {
	basic, ok := underlyingType(target).(*types.Basic)
	if !ok {
		return StaticValue{}, e.errorAt(pkg, nil, nil, fmt.Errorf("cannot convert constant to %s: %w", target, ErrNotStatic))
	}

	switch {
	case basic.Info()&types.IsBoolean != 0:
		if value.Kind() != constant.Bool {
			return StaticValue{}, e.errorAt(pkg, nil, nil, fmt.Errorf("cannot convert %s to %s: %w", value, target, ErrNotStatic))
		}
		return staticConstant(target, constant.MakeBool(constant.BoolVal(value))), nil
	case basic.Info()&types.IsString != 0:
		if value.Kind() == constant.String {
			return staticConstant(target, constant.MakeString(constant.StringVal(value))), nil
		}
		integer, ok := constantInteger(constant.Real(value))
		if !ok {
			return StaticValue{}, e.errorAt(pkg, nil, nil, fmt.Errorf("cannot convert %s to string: %w", value, ErrNotStatic))
		}
		runeValue := utf8.RuneError
		if integer.IsInt64() {
			candidate := rune(integer.Int64())
			if utf8.ValidRune(candidate) {
				runeValue = candidate
			}
		}
		return staticConstant(target, constant.MakeString(string(runeValue))), nil
	case basic.Info()&types.IsInteger != 0:
		if constant.Sign(constant.Imag(value)) != 0 {
			return StaticValue{}, e.errorAt(pkg, nil, nil, fmt.Errorf("cannot convert complex value to %s: %w", target, ErrNotStatic))
		}
		integer, ok := constantInteger(constant.Real(value))
		if !ok {
			return StaticValue{}, e.errorAt(pkg, nil, nil, fmt.Errorf("cannot convert %s to %s: %w", value, target, ErrNotStatic))
		}
		bits, ok := integerBits(pkg, target, basic.Kind())
		if !ok {
			if basic.Info()&types.IsUntyped != 0 {
				return staticConstant(target, constant.Make(integer)), nil
			}
			return StaticValue{}, e.errorAt(pkg, nil, nil, ErrMissingTypeInfo)
		}
		signed := basic.Info()&types.IsUnsigned == 0
		return staticConstant(target, constant.Make(wrapInteger(integer, bits, signed))), nil
	case basic.Info()&types.IsFloat != 0:
		if constant.Sign(constant.Imag(value)) != 0 {
			return StaticValue{}, e.errorAt(pkg, nil, nil, fmt.Errorf("cannot convert complex value to %s: %w", target, ErrNotStatic))
		}
		realValue := constant.Real(value)
		if basic.Kind() == types.Float32 {
			converted, _ := constant.Float32Val(realValue)
			if math.IsInf(float64(converted), 0) {
				return StaticValue{}, e.errorAt(pkg, nil, nil, fmt.Errorf("float32 result is not finite: %w", ErrNotStatic))
			}
			return staticConstant(target, constant.MakeFloat64(float64(converted))), nil
		}
		converted, _ := constant.Float64Val(realValue)
		if math.IsInf(converted, 0) {
			return StaticValue{}, e.errorAt(pkg, nil, nil, fmt.Errorf("float64 result is not finite: %w", ErrNotStatic))
		}
		return staticConstant(target, constant.MakeFloat64(converted)), nil
	case basic.Info()&types.IsComplex != 0:
		componentKind := types.Float64
		if basic.Kind() == types.Complex64 {
			componentKind = types.Float32
		}
		componentType := types.Typ[componentKind]
		realPart, err := e.convertConstant(pkg, constant.Real(value), componentType)
		if err != nil {
			return StaticValue{}, err
		}
		imagPart, err := e.convertConstant(pkg, constant.Imag(value), componentType)
		if err != nil {
			return StaticValue{}, err
		}
		imaginary := constant.MakeImag(imagPart.Exact)
		return staticConstant(target, constant.BinaryOp(realPart.Exact, token.ADD, imaginary)), nil
	default:
		return StaticValue{}, e.errorAt(pkg, nil, nil, fmt.Errorf("unsupported constant target %s: %w", target, ErrNotStatic))
	}
}

func (e *staticEvaluator) assignValue(pkg *packages.Package, value StaticValue, target types.Type, explicit bool) (StaticValue, error) {
	if target == nil {
		return StaticValue{}, e.errorAt(pkg, nil, nil, ErrNotStatic)
	}

	if _, ok := underlyingType(target).(*types.Interface); ok {
		if value.Kind == StaticInterface {
			result := cloneStaticValue(value)
			result.Type = target
			return result, nil
		}
		if value.Kind == StaticNil {
			if basic, ok := underlyingType(value.Type).(*types.Basic); ok && basic.Kind() == types.UntypedNil {
				return staticNil(target), nil
			}
			if _, ok := underlyingType(value.Type).(*types.Interface); ok {
				return staticNil(target), nil
			}
			dynamic := cloneStaticValue(value)
			return StaticValue{Type: target, Kind: StaticInterface, Dynamic: &dynamic}, nil
		}
		if value.Kind == StaticConstant {
			dynamicType := value.Type
			if basic, ok := underlyingType(dynamicType).(*types.Basic); ok && basic.Info()&types.IsUntyped != 0 {
				dynamicType = types.Default(dynamicType)
			}
			if dynamicType == nil {
				return StaticValue{}, e.errorAt(pkg, nil, nil, ErrNotStatic)
			}
			materialized, err := e.convertConstant(pkg, value.Exact, dynamicType)
			if err != nil {
				return StaticValue{}, err
			}
			value = materialized
		}
		dynamic := cloneStaticValue(value)
		return StaticValue{Type: target, Kind: StaticInterface, Dynamic: &dynamic}, nil
	}

	if value.Kind == StaticNil {
		if !isNilable(target) {
			return StaticValue{}, e.errorAt(pkg, nil, nil, fmt.Errorf("nil is not assignable to %s: %w", target, ErrNotStatic))
		}
		return staticNil(target), nil
	}

	if value.Kind == StaticInterface {
		return StaticValue{}, e.errorAt(pkg, nil, nil, fmt.Errorf("interface conversion requires a type assertion: %w", ErrNotStatic))
	}

	if value.Kind == StaticConstant {
		if targetBasic, ok := underlyingType(target).(*types.Basic); ok {
			if sourceBasic, ok := underlyingType(value.Type).(*types.Basic); explicit && ok && sourceBasic.Info()&types.IsFloat != 0 && targetBasic.Info()&types.IsInteger != 0 {
				integer, ok := constantInteger(value.Exact)
				if !ok {
					return StaticValue{}, e.errorAt(pkg, nil, nil, ErrNotStatic)
				}
				fits, err := integerFits(pkg, integer, target, targetBasic)
				if err != nil {
					return StaticValue{}, e.errorAt(pkg, nil, nil, err)
				}
				if !fits {
					return StaticValue{}, e.errorAt(pkg, nil, nil, fmt.Errorf("float to %s conversion is implementation-dependent: %w", target, ErrNotStatic))
				}
			}
			return e.convertConstant(pkg, value.Exact, target)
		}
	}

	if explicit {
		if !types.ConvertibleTo(value.Type, target) {
			return StaticValue{}, e.errorAt(pkg, nil, nil, fmt.Errorf("%s is not convertible to %s: %w", value.Type, target, ErrNotStatic))
		}
	} else if !types.AssignableTo(value.Type, target) {
		return StaticValue{}, e.errorAt(pkg, nil, nil, fmt.Errorf("%s is not assignable to %s: %w", value.Type, target, ErrNotStatic))
	}

	targetUnderlying := underlyingType(target)
	switch value.Kind {
	case StaticFunctionCall:
		result := cloneStaticValue(value)
		result.Type = target
		return result, nil
	case StaticArray:
		arrayType, ok := targetUnderlying.(*types.Array)
		if !ok || int64(len(value.Elements)) != arrayType.Len() {
			return StaticValue{}, e.errorAt(pkg, nil, nil, ErrNotStatic)
		}
		elements := make([]StaticValue, len(value.Elements))
		for index, element := range value.Elements {
			converted, err := e.assignValue(pkg, element, arrayType.Elem(), false)
			if err != nil {
				return StaticValue{}, err
			}
			elements[index] = converted
		}
		return StaticValue{Type: target, Kind: StaticArray, Elements: elements}, nil
	case StaticStruct:
		structType, ok := targetUnderlying.(*types.Struct)
		if !ok || len(value.Fields) != structType.NumFields() {
			return StaticValue{}, e.errorAt(pkg, nil, nil, ErrNotStatic)
		}
		fields := make([]StaticField, len(value.Fields))
		for index, field := range value.Fields {
			targetField := structType.Field(index)
			converted, err := e.assignValue(pkg, field.Value, targetField.Type(), false)
			if err != nil {
				return StaticValue{}, err
			}
			fields[index] = StaticField{Field: targetField, Value: converted, Explicit: field.Explicit}
		}
		return StaticValue{Type: target, Kind: StaticStruct, Fields: fields}, nil
	case StaticSliceValue, StaticMapValue, StaticPointer:
		result := cloneStaticValue(value)
		result.Type = target
		return result, nil
	default:
		return StaticValue{}, e.errorAt(pkg, nil, nil, ErrNotStatic)
	}
}

func (e *staticEvaluator) ensureVar(variable *staticVarState) error {
	if variable == nil {
		return nil
	}
	if variable.init == nil {
		return variable.zeroErr
	}
	return e.ensureInitializer(variable.init)
}

func (e *staticEvaluator) ensureInitializer(initializer *staticInitializer) error {
	switch initializer.state {
	case staticDone:
		return nil
	case staticFailed:
		return initializer.err
	case staticEvaluating:
		err := e.errorAt(
			initializer.pkg.pkg,
			initializer.init.Rhs,
			nil,
			ErrStaticCycle,
		)
		return err
	}

	initializer.state = staticEvaluating
	values, err := e.evalExprMulti(initializer.pkg.pkg, initializer.init.Rhs, len(initializer.init.Lhs))
	if err == nil && len(values) != len(initializer.init.Lhs) {
		err = e.errorAt(
			initializer.pkg.pkg,
			initializer.init.Rhs,
			nil,
			fmt.Errorf("initializer returned %d values for %d variables: %w", len(values), len(initializer.init.Lhs), ErrNotStatic),
		)
	}
	converted := make([]StaticValue, len(values))
	if err == nil {
		for index, value := range values {
			converted[index], err = e.assignValue(initializer.pkg.pkg, value, initializer.init.Lhs[index].Type(), false)
			if err != nil {
				break
			}
		}
	}
	if err != nil {
		initializer.state = staticFailed
		initializer.err = err
		return err
	}

	for index, variable := range initializer.init.Lhs {
		if variableState, ok := initializer.pkg.vars[variable]; ok {
			variableState.storage.Value = converted[index]
			variableState.zeroErr = nil
		}
	}
	initializer.values = converted
	initializer.state = staticDone
	return nil
}

func (e *staticEvaluator) loadAddress(pkg *packages.Package, address *StaticAddress) (StaticValue, error) {
	if address == nil || address.Object == nil {
		return StaticValue{}, e.errorAt(pkg, nil, nil, ErrStaticPanic)
	}
	if address.Object.owner != nil {
		if err := e.ensureVar(address.Object.owner); err != nil {
			return StaticValue{}, err
		}
	}
	value := address.Object.Value
	for _, step := range address.Path {
		switch step.Kind {
		case StaticPathField:
			if value.Kind != StaticStruct || step.Index < 0 || step.Index >= int64(len(value.Fields)) {
				return StaticValue{}, e.errorAt(pkg, nil, nil, ErrStaticPanic)
			}
			value = value.Fields[step.Index].Value
		case StaticPathElement:
			if value.Kind != StaticArray || step.Index < 0 || step.Index >= int64(len(value.Elements)) {
				return StaticValue{}, e.errorAt(pkg, nil, nil, ErrStaticPanic)
			}
			value = value.Elements[step.Index]
		default:
			return StaticValue{}, e.errorAt(pkg, nil, nil, ErrStaticPanic)
		}
	}
	if address.ArrayView != nil {
		arrayType, ok := underlyingType(address.ArrayView).(*types.Array)
		if !ok || value.Kind != StaticArray || address.ArrayOffset < 0 || address.ArrayOffset+arrayType.Len() > int64(len(value.Elements)) {
			return StaticValue{}, e.errorAt(pkg, nil, nil, ErrStaticPanic)
		}
		elements := make([]StaticValue, arrayType.Len())
		for index := range elements {
			elements[index] = cloneStaticValue(value.Elements[address.ArrayOffset+int64(index)])
		}
		return StaticValue{Type: address.ArrayView, Kind: StaticArray, Elements: elements}, nil
	}
	return cloneStaticValue(value), nil
}

func (e *staticEvaluator) sliceElementAddress(slice *StaticSlice, index int64) *StaticAddress {
	path := cloneStaticPath(slice.Path)
	path = append(path, StaticPathStep{Kind: StaticPathElement, Index: slice.Offset + index})
	return &StaticAddress{Object: slice.Backing, Path: path}
}

func staticBool(value StaticValue) (bool, bool) {
	if value.Kind != StaticConstant || value.Exact == nil || value.Exact.Kind() != constant.Bool {
		return false, false
	}
	return constant.BoolVal(value.Exact), true
}

func staticIndex(value StaticValue) (int64, bool) {
	if value.Kind != StaticConstant || value.Exact == nil || value.Exact.Kind() != constant.Int {
		return 0, false
	}
	index, ok := constant.Int64Val(value.Exact)
	return index, ok
}

func (e *staticEvaluator) equalValues(pkg *packages.Package, left, right StaticValue) (bool, error) {
	if left.Kind == StaticNil || right.Kind == StaticNil {
		return left.Kind == StaticNil && right.Kind == StaticNil, nil
	}
	if left.Kind != right.Kind {
		return false, nil
	}
	switch left.Kind {
	case StaticConstant:
		return constant.Compare(left.Exact, token.EQL, right.Exact), nil
	case StaticPointer:
		return staticAddressEqual(left.Pointer, right.Pointer), nil
	case StaticArray:
		if !types.Comparable(left.Type) || !types.Comparable(right.Type) {
			return false, e.errorAt(pkg, nil, nil, ErrStaticPanic)
		}
		if len(left.Elements) != len(right.Elements) {
			return false, nil
		}
		for index := range left.Elements {
			equal, err := e.equalValues(pkg, left.Elements[index], right.Elements[index])
			if err != nil || !equal {
				return equal, err
			}
		}
		return true, nil
	case StaticStruct:
		if !types.Comparable(left.Type) || !types.Comparable(right.Type) {
			return false, e.errorAt(pkg, nil, nil, ErrStaticPanic)
		}
		if len(left.Fields) != len(right.Fields) {
			return false, nil
		}
		for index := range left.Fields {
			equal, err := e.equalValues(pkg, left.Fields[index].Value, right.Fields[index].Value)
			if err != nil || !equal {
				return equal, err
			}
		}
		return true, nil
	case StaticInterface:
		if left.Dynamic == nil || right.Dynamic == nil {
			return left.Dynamic == nil && right.Dynamic == nil, nil
		}
		if !types.Identical(left.Dynamic.Type, right.Dynamic.Type) {
			return false, nil
		}
		if !types.Comparable(left.Dynamic.Type) {
			return false, e.errorAt(pkg, nil, nil, ErrStaticPanic)
		}
		return e.equalValues(pkg, *left.Dynamic, *right.Dynamic)
	case StaticSliceValue, StaticMapValue:
		return false, e.errorAt(pkg, nil, nil, ErrStaticPanic)
	default:
		return false, e.errorAt(pkg, nil, nil, ErrNotStatic)
	}
}

func (e *staticEvaluator) ensureRuntimeComparable(pkg *packages.Package, value StaticValue) error {
	switch value.Kind {
	case StaticConstant, StaticNil, StaticPointer:
		return nil
	case StaticArray:
		if !types.Comparable(value.Type) {
			return e.errorAt(pkg, nil, nil, ErrStaticPanic)
		}
		for _, element := range value.Elements {
			if err := e.ensureRuntimeComparable(pkg, element); err != nil {
				return err
			}
		}
		return nil
	case StaticStruct:
		if !types.Comparable(value.Type) {
			return e.errorAt(pkg, nil, nil, ErrStaticPanic)
		}
		for _, field := range value.Fields {
			if err := e.ensureRuntimeComparable(pkg, field.Value); err != nil {
				return err
			}
		}
		return nil
	case StaticInterface:
		if value.Dynamic == nil {
			return nil
		}
		if !types.Comparable(value.Dynamic.Type) {
			return e.errorAt(pkg, nil, nil, ErrStaticPanic)
		}
		return e.ensureRuntimeComparable(pkg, *value.Dynamic)
	case StaticSliceValue, StaticMapValue:
		return e.errorAt(pkg, nil, nil, ErrStaticPanic)
	default:
		return e.errorAt(pkg, nil, nil, ErrNotStatic)
	}
}

func (e *staticEvaluator) mapLookup(pkg *packages.Package, value *StaticMap, key StaticValue) (StaticValue, bool, error) {
	if err := e.ensureRuntimeComparable(pkg, key); err != nil {
		return StaticValue{}, false, err
	}
	for _, entry := range value.Entries {
		equal, err := e.equalValues(pkg, entry.Key, key)
		if err != nil {
			return StaticValue{}, false, err
		}
		if equal {
			return cloneStaticValue(entry.Value), true, nil
		}
	}
	zero, err := e.zeroValue(pkg, value.Type.Elem())
	return zero, false, err
}

func (e *staticEvaluator) mapSet(pkg *packages.Package, value *StaticMap, key, element StaticValue) error {
	if err := e.ensureRuntimeComparable(pkg, key); err != nil {
		return err
	}
	for index := range value.Entries {
		equal, err := e.equalValues(pkg, value.Entries[index].Key, key)
		if err != nil {
			return err
		}
		if equal {
			value.Entries[index].Value = cloneStaticValue(element)
			return nil
		}
	}
	value.Entries = append(value.Entries, StaticMapEntry{
		Key:   cloneStaticValue(key),
		Value: cloneStaticValue(element),
	})
	return nil
}
