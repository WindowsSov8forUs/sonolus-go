package tracer

import (
	"go/ast"
	"go/constant"
	"go/token"
	"go/types"
	"math"
	"unicode/utf8"

	"golang.org/x/tools/go/packages"
)

func (e *staticEvaluator) evalCall(pkg *packages.Package, expr *ast.CallExpr) (StaticValue, error) {
	if typeAndValue, ok := pkg.TypesInfo.Types[expr.Fun]; ok && typeAndValue.IsType() {
		if len(expr.Args) != 1 || expr.Ellipsis.IsValid() {
			return StaticValue{}, ErrNotStatic
		}
		value, err := e.evalExpr(pkg, expr.Args[0])
		if err != nil {
			return StaticValue{}, err
		}
		return e.convertValue(pkg, value, typeAndValue.Type)
	}

	builtin := builtinObject(pkg, expr.Fun)
	if builtin == nil {
		return StaticValue{}, ErrNotStatic
	}
	switch builtin.Name() {
	case "len":
		return e.evalLenCap(pkg, expr, false)
	case "cap":
		return e.evalLenCap(pkg, expr, true)
	case "min":
		return e.evalMinMax(pkg, expr, false)
	case "max":
		return e.evalMinMax(pkg, expr, true)
	case "complex":
		return e.evalComplex(pkg, expr)
	case "real":
		return e.evalRealImag(pkg, expr, false)
	case "imag":
		return e.evalRealImag(pkg, expr, true)
	case "new":
		return e.evalNew(pkg, expr)
	case "make":
		return e.evalMake(pkg, expr)
	default:
		return StaticValue{}, ErrNotStatic
	}
}

func builtinObject(pkg *packages.Package, expr ast.Expr) *types.Builtin {
	for {
		paren, ok := expr.(*ast.ParenExpr)
		if !ok {
			break
		}
		expr = paren.X
	}
	ident, ok := expr.(*ast.Ident)
	if !ok {
		return nil
	}
	builtin, _ := pkg.TypesInfo.ObjectOf(ident).(*types.Builtin)
	return builtin
}

func (e *staticEvaluator) convertValue(pkg *packages.Package, value StaticValue, target types.Type) (StaticValue, error) {
	if value.Type != nil {
		if basic, ok := underlyingType(value.Type).(*types.Basic); ok && basic.Kind() == types.UnsafePointer {
			return StaticValue{}, ErrNotStatic
		}
	}
	if basic, ok := underlyingType(target).(*types.Basic); ok && basic.Kind() == types.UnsafePointer {
		return StaticValue{}, ErrNotStatic
	}

	if targetSlice, ok := underlyingType(target).(*types.Slice); ok && value.Kind == StaticConstant && value.Exact.Kind() == constant.String {
		return e.convertStringToSlice(pkg, constant.StringVal(value.Exact), target, targetSlice)
	}
	if targetBasic, ok := underlyingType(target).(*types.Basic); ok && targetBasic.Info()&types.IsString != 0 {
		if sourceSlice, ok := underlyingType(value.Type).(*types.Slice); ok {
			return e.convertSliceToString(pkg, value, target, sourceSlice)
		}
	}
	if targetArray, ok := underlyingType(target).(*types.Array); ok {
		if _, ok := underlyingType(value.Type).(*types.Slice); ok {
			return e.convertSliceToArray(pkg, value, target, targetArray)
		}
	}
	if targetPointer, ok := underlyingType(target).(*types.Pointer); ok {
		if targetArray, ok := underlyingType(targetPointer.Elem()).(*types.Array); ok {
			if _, ok := underlyingType(value.Type).(*types.Slice); ok {
				return e.convertSliceToArrayPointer(pkg, value, target, targetArray)
			}
		}
	}
	return e.assignValue(pkg, value, target, true)
}

func stringSliceElementKind(typ types.Type) (types.BasicKind, bool) {
	basic, ok := underlyingType(typ).(*types.Basic)
	if !ok {
		return types.Invalid, false
	}
	switch basic.Kind() {
	case types.Uint8:
		return types.Uint8, true
	case types.Int32:
		return types.Int32, true
	default:
		return types.Invalid, false
	}
}

func (e *staticEvaluator) convertStringToSlice(
	pkg *packages.Package,
	text string,
	target types.Type,
	sliceType *types.Slice,
) (StaticValue, error) {
	if !types.ConvertibleTo(types.Typ[types.String], target) {
		return StaticValue{}, ErrNotStatic
	}
	kind, ok := stringSliceElementKind(sliceType.Elem())
	if !ok {
		return StaticValue{}, ErrNotStatic
	}

	var raw []int64
	if kind == types.Uint8 {
		if len(text) > maxStaticElements {
			return StaticValue{}, ErrNotStatic
		}
		raw = make([]int64, len(text))
		for index := 0; index < len(text); index++ {
			raw[index] = int64(text[index])
		}
	} else {
		if utf8.RuneCountInString(text) > maxStaticElements {
			return StaticValue{}, ErrNotStatic
		}
		runes := []rune(text)
		raw = make([]int64, len(runes))
		for index, value := range runes {
			raw[index] = int64(value)
		}
	}

	arrayType := types.NewArray(sliceType.Elem(), int64(len(raw)))
	arrayValue, err := e.zeroValue(pkg, arrayType)
	if err != nil {
		return StaticValue{}, err
	}
	for index, value := range raw {
		converted, err := e.convertConstant(pkg, constant.MakeInt64(value), sliceType.Elem())
		if err != nil {
			return StaticValue{}, err
		}
		arrayValue.Elements[index] = converted
	}
	backing := e.newObject(arrayType, arrayValue)
	return StaticValue{
		Type: target,
		Kind: StaticSliceValue,
		Slice: &StaticSlice{
			Backing: backing,
			Len:     int64(len(raw)),
			Cap:     int64(len(raw)),
		},
	}, nil
}

func (e *staticEvaluator) convertSliceToString(
	pkg *packages.Package,
	value StaticValue,
	target types.Type,
	sliceType *types.Slice,
) (StaticValue, error) {
	if !types.ConvertibleTo(value.Type, target) {
		return StaticValue{}, ErrNotStatic
	}
	kind, ok := stringSliceElementKind(sliceType.Elem())
	if !ok {
		return StaticValue{}, ErrNotStatic
	}
	if value.Kind == StaticNil {
		return staticConstant(target, constant.MakeString("")), nil
	}
	if value.Kind != StaticSliceValue || value.Slice == nil {
		return StaticValue{}, ErrNotStatic
	}

	if kind == types.Uint8 {
		bytes := make([]byte, value.Slice.Len)
		for index := int64(0); index < value.Slice.Len; index++ {
			element, err := e.loadAddress(pkg, e.sliceElementAddress(value.Slice, index))
			if err != nil {
				return StaticValue{}, err
			}
			integer, ok := constant.Uint64Val(element.Exact)
			if element.Kind != StaticConstant || !ok || integer > math.MaxUint8 {
				return StaticValue{}, ErrNotStatic
			}
			bytes[index] = byte(integer)
		}
		return staticConstant(target, constant.MakeString(string(bytes))), nil
	}

	runes := make([]rune, value.Slice.Len)
	for index := int64(0); index < value.Slice.Len; index++ {
		element, err := e.loadAddress(pkg, e.sliceElementAddress(value.Slice, index))
		if err != nil {
			return StaticValue{}, err
		}
		integer, ok := constant.Int64Val(element.Exact)
		if element.Kind != StaticConstant || !ok {
			return StaticValue{}, ErrNotStatic
		}
		candidate := rune(integer)
		if !utf8.ValidRune(candidate) {
			candidate = utf8.RuneError
		}
		runes[index] = candidate
	}
	return staticConstant(target, constant.MakeString(string(runes))), nil
}

func (e *staticEvaluator) convertSliceToArray(
	pkg *packages.Package,
	value StaticValue,
	target types.Type,
	arrayType *types.Array,
) (StaticValue, error) {
	if !types.ConvertibleTo(value.Type, target) {
		return StaticValue{}, ErrNotStatic
	}
	if value.Kind == StaticNil && arrayType.Len() == 0 {
		return e.zeroValue(pkg, target)
	}
	if value.Kind == StaticNil || value.Kind != StaticSliceValue || value.Slice == nil || value.Slice.Len < arrayType.Len() {
		return StaticValue{}, ErrStaticPanic
	}
	result, err := e.zeroValue(pkg, target)
	if err != nil {
		return StaticValue{}, err
	}
	for index := int64(0); index < arrayType.Len(); index++ {
		element, err := e.loadAddress(pkg, e.sliceElementAddress(value.Slice, index))
		if err != nil {
			return StaticValue{}, err
		}
		result.Elements[index] = cloneStaticValue(element)
	}
	return result, nil
}

func (e *staticEvaluator) convertSliceToArrayPointer(
	pkg *packages.Package,
	value StaticValue,
	target types.Type,
	arrayType *types.Array,
) (StaticValue, error) {
	if !types.ConvertibleTo(value.Type, target) {
		return StaticValue{}, ErrNotStatic
	}
	if value.Kind == StaticNil && arrayType.Len() == 0 {
		return staticNil(target), nil
	}
	if value.Kind == StaticNil || value.Kind != StaticSliceValue || value.Slice == nil || value.Slice.Len < arrayType.Len() {
		return StaticValue{}, ErrStaticPanic
	}
	return StaticValue{
		Type: target,
		Kind: StaticPointer,
		Pointer: &StaticAddress{
			Object:      value.Slice.Backing,
			Path:        cloneStaticPath(value.Slice.Path),
			ArrayOffset: value.Slice.Offset,
			ArrayView:   underlyingType(target).(*types.Pointer).Elem(),
		},
	}, nil
}

func (e *staticEvaluator) evalLenCap(pkg *packages.Package, expr *ast.CallExpr, wantCap bool) (StaticValue, error) {
	if len(expr.Args) != 1 || expr.Ellipsis.IsValid() {
		return StaticValue{}, ErrNotStatic
	}
	typ := underlyingType(pkg.TypesInfo.TypeOf(expr.Args[0]))
	var size int64
	switch typ := typ.(type) {
	case *types.Array:
		size = typ.Len()
	case *types.Pointer:
		arrayType, ok := underlyingType(typ.Elem()).(*types.Array)
		if !ok {
			return StaticValue{}, ErrNotStatic
		}
		size = arrayType.Len()
	case *types.Basic:
		if wantCap || typ.Info()&types.IsString == 0 {
			return StaticValue{}, ErrNotStatic
		}
		value, err := e.evalExpr(pkg, expr.Args[0])
		if err != nil {
			return StaticValue{}, err
		}
		if value.Kind != StaticConstant || value.Exact.Kind() != constant.String {
			return StaticValue{}, ErrNotStatic
		}
		size = int64(len(constant.StringVal(value.Exact)))
	case *types.Slice:
		value, err := e.evalExpr(pkg, expr.Args[0])
		if err != nil {
			return StaticValue{}, err
		}
		if value.Kind == StaticNil {
			size = 0
		} else if value.Kind != StaticSliceValue || value.Slice == nil {
			return StaticValue{}, ErrNotStatic
		} else if wantCap {
			size = value.Slice.Cap
		} else {
			size = value.Slice.Len
		}
	case *types.Map:
		if wantCap {
			return StaticValue{}, ErrNotStatic
		}
		value, err := e.evalExpr(pkg, expr.Args[0])
		if err != nil {
			return StaticValue{}, err
		}
		if value.Kind == StaticNil {
			size = 0
		} else if value.Kind != StaticMapValue || value.Map == nil {
			return StaticValue{}, ErrNotStatic
		} else {
			size = int64(len(value.Map.Entries))
		}
	default:
		return StaticValue{}, ErrNotStatic
	}
	return e.convertConstant(pkg, constant.MakeInt64(size), pkg.TypesInfo.TypeOf(expr))
}

func (e *staticEvaluator) evalMinMax(pkg *packages.Package, expr *ast.CallExpr, wantMax bool) (StaticValue, error) {
	if len(expr.Args) == 0 || expr.Ellipsis.IsValid() {
		return StaticValue{}, ErrNotStatic
	}
	result, err := e.evalExpr(pkg, expr.Args[0])
	if err != nil {
		return StaticValue{}, err
	}
	if result.Kind != StaticConstant {
		return StaticValue{}, ErrNotStatic
	}
	for _, argument := range expr.Args[1:] {
		candidate, err := e.evalExpr(pkg, argument)
		if err != nil {
			return StaticValue{}, err
		}
		if candidate.Kind != StaticConstant {
			return StaticValue{}, ErrNotStatic
		}
		op := token.LSS
		if wantMax {
			op = token.GTR
		}
		if constant.Compare(candidate.Exact, op, result.Exact) {
			result = candidate
		}
	}
	return e.assignValue(pkg, result, pkg.TypesInfo.TypeOf(expr), false)
}

func (e *staticEvaluator) evalComplex(pkg *packages.Package, expr *ast.CallExpr) (StaticValue, error) {
	if len(expr.Args) != 2 || expr.Ellipsis.IsValid() {
		return StaticValue{}, ErrNotStatic
	}
	realValue, err := e.evalExpr(pkg, expr.Args[0])
	if err != nil {
		return StaticValue{}, err
	}
	imagValue, err := e.evalExpr(pkg, expr.Args[1])
	if err != nil {
		return StaticValue{}, err
	}
	if realValue.Kind != StaticConstant || imagValue.Kind != StaticConstant {
		return StaticValue{}, ErrNotStatic
	}
	value := constant.BinaryOp(realValue.Exact, token.ADD, constant.MakeImag(imagValue.Exact))
	return e.convertConstant(pkg, value, pkg.TypesInfo.TypeOf(expr))
}

func (e *staticEvaluator) evalRealImag(pkg *packages.Package, expr *ast.CallExpr, wantImag bool) (StaticValue, error) {
	if len(expr.Args) != 1 || expr.Ellipsis.IsValid() {
		return StaticValue{}, ErrNotStatic
	}
	value, err := e.evalExpr(pkg, expr.Args[0])
	if err != nil {
		return StaticValue{}, err
	}
	if value.Kind != StaticConstant {
		return StaticValue{}, ErrNotStatic
	}
	component := constant.Real(value.Exact)
	if wantImag {
		component = constant.Imag(value.Exact)
	}
	return e.convertConstant(pkg, component, pkg.TypesInfo.TypeOf(expr))
}

func (e *staticEvaluator) evalNew(pkg *packages.Package, expr *ast.CallExpr) (StaticValue, error) {
	if len(expr.Args) != 1 || expr.Ellipsis.IsValid() {
		return StaticValue{}, ErrNotStatic
	}
	typeAndValue, ok := pkg.TypesInfo.Types[expr.Args[0]]
	if !ok || !typeAndValue.IsType() {
		return StaticValue{}, ErrMissingTypeInfo
	}
	zero, err := e.zeroValue(pkg, typeAndValue.Type)
	if err != nil {
		return StaticValue{}, err
	}
	object := e.newObject(typeAndValue.Type, zero)
	return StaticValue{
		Type:    pkg.TypesInfo.TypeOf(expr),
		Kind:    StaticPointer,
		Pointer: &StaticAddress{Object: object},
	}, nil
}

func (e *staticEvaluator) evalMake(pkg *packages.Package, expr *ast.CallExpr) (StaticValue, error) {
	if len(expr.Args) == 0 || expr.Ellipsis.IsValid() {
		return StaticValue{}, ErrNotStatic
	}
	typeAndValue, ok := pkg.TypesInfo.Types[expr.Args[0]]
	if !ok || !typeAndValue.IsType() {
		return StaticValue{}, ErrMissingTypeInfo
	}
	switch valueType := underlyingType(typeAndValue.Type).(type) {
	case *types.Slice:
		return e.evalMakeSlice(pkg, expr, typeAndValue.Type, valueType)
	case *types.Map:
		return e.evalMakeMap(pkg, expr, typeAndValue.Type, valueType)
	default:
		return StaticValue{}, ErrNotStatic
	}
}

func (e *staticEvaluator) evalRuntimeSize(pkg *packages.Package, expr ast.Expr) (int64, error) {
	value, err := e.evalExpr(pkg, expr)
	if err != nil {
		return 0, err
	}
	if value.Kind != StaticConstant || value.Exact.Kind() != constant.Int {
		return 0, ErrNotStatic
	}
	size, ok := constant.Int64Val(value.Exact)
	if !ok || size < 0 {
		return 0, ErrStaticPanic
	}
	intType := types.Typ[types.Int]
	bits, ok := integerBits(pkg, intType, types.Int)
	if !ok {
		return 0, ErrMissingTypeInfo
	}
	if bits < 64 && uint64(size) >= uint64(1)<<bits-1 {
		return 0, ErrStaticPanic
	}
	return size, nil
}

func (e *staticEvaluator) evalMakeSlice(
	pkg *packages.Package,
	expr *ast.CallExpr,
	typ types.Type,
	sliceType *types.Slice,
) (StaticValue, error) {
	if len(expr.Args) < 2 || len(expr.Args) > 3 {
		return StaticValue{}, ErrNotStatic
	}
	length, err := e.evalRuntimeSize(pkg, expr.Args[1])
	if err != nil {
		return StaticValue{}, err
	}
	capacity := length
	if len(expr.Args) == 3 {
		capacity, err = e.evalRuntimeSize(pkg, expr.Args[2])
		if err != nil {
			return StaticValue{}, err
		}
	}
	if length > capacity || capacity > maxStaticElements {
		return StaticValue{}, ErrStaticPanic
	}
	arrayType := types.NewArray(sliceType.Elem(), capacity)
	arrayValue, err := e.zeroValue(pkg, arrayType)
	if err != nil {
		return StaticValue{}, err
	}
	backing := e.newObject(arrayType, arrayValue)
	return StaticValue{
		Type: typ,
		Kind: StaticSliceValue,
		Slice: &StaticSlice{
			Backing: backing,
			Len:     length,
			Cap:     capacity,
		},
	}, nil
}

func (e *staticEvaluator) evalMakeMap(
	pkg *packages.Package,
	expr *ast.CallExpr,
	typ types.Type,
	mapType *types.Map,
) (StaticValue, error) {
	if len(expr.Args) > 2 {
		return StaticValue{}, ErrNotStatic
	}
	if len(expr.Args) == 2 {
		if _, err := e.evalRuntimeSize(pkg, expr.Args[1]); err != nil {
			return StaticValue{}, err
		}
	}
	return StaticValue{Type: typ, Kind: StaticMapValue, Map: e.newMap(mapType)}, nil
}
