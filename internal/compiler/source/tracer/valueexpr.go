package tracer

import (
	"fmt"
	"go/ast"
	"go/constant"
	"go/token"
	"go/types"

	"golang.org/x/tools/go/packages"
)

func (e *staticEvaluator) evalExprMulti(pkg *packages.Package, expr ast.Expr, arity int) ([]StaticValue, error) {
	if arity == 2 {
		switch expr := expr.(type) {
		case *ast.IndexExpr:
			value, ok, err := e.evalMapIndex(pkg, expr)
			if err == nil {
				return []StaticValue{
					value,
					staticConstant(types.Typ[types.Bool], constant.MakeBool(ok)),
				}, nil
			}
			return nil, err
		case *ast.TypeAssertExpr:
			value, ok, err := e.evalTypeAssertion(pkg, expr)
			if err != nil {
				return nil, err
			}
			return []StaticValue{
				value,
				staticConstant(types.Typ[types.Bool], constant.MakeBool(ok)),
			}, nil
		}
	}
	value, err := e.evalExpr(pkg, expr)
	if err != nil {
		return nil, err
	}
	return []StaticValue{value}, nil
}

func (e *staticEvaluator) evalExpr(pkg *packages.Package, expr ast.Expr) (StaticValue, error) {
	if expr == nil {
		return StaticValue{}, e.errorAt(pkg, nil, nil, ErrNotStatic)
	}
	info := pkg.TypesInfo
	if usesUnsafe(pkg, expr) {
		return StaticValue{}, e.errorAt(pkg, expr, nil, ErrNotStatic)
	}
	if typeAndValue, ok := info.Types[expr]; ok && typeAndValue.Value != nil {
		return staticConstant(typeAndValue.Type, typeAndValue.Value), nil
	}

	var value StaticValue
	var err error
	switch expr := expr.(type) {
	case *ast.Ident:
		value, err = e.evalIdent(pkg, expr)
	case *ast.SelectorExpr:
		value, err = e.evalSelector(pkg, expr)
	case *ast.ParenExpr:
		value, err = e.evalExpr(pkg, expr.X)
	case *ast.UnaryExpr:
		value, err = e.evalUnary(pkg, expr)
	case *ast.StarExpr:
		value, err = e.evalDereference(pkg, expr)
	case *ast.BinaryExpr:
		value, err = e.evalBinary(pkg, expr)
	case *ast.CompositeLit:
		value, err = e.evalComposite(pkg, expr)
	case *ast.IndexExpr:
		value, err = e.evalIndex(pkg, expr)
	case *ast.SliceExpr:
		value, err = e.evalSlice(pkg, expr)
	case *ast.TypeAssertExpr:
		var ok bool
		value, ok, err = e.evalTypeAssertion(pkg, expr)
		if err == nil && !ok {
			err = ErrStaticPanic
		}
	case *ast.CallExpr:
		value, err = e.evalCall(pkg, expr)
	case *ast.BasicLit, *ast.FuncLit, *ast.IndexListExpr:
		err = ErrNotStatic
	default:
		err = ErrNotStatic
	}
	if err != nil {
		return StaticValue{}, e.annotateError(pkg, expr, err)
	}
	return value, nil
}

func usesUnsafe(pkg *packages.Package, expr ast.Expr) bool {
	unsafeOperation := false
	ast.Inspect(expr, func(node ast.Node) bool {
		if unsafeOperation || node == nil {
			return false
		}
		ident, ok := node.(*ast.Ident)
		if !ok {
			return true
		}
		object := pkg.TypesInfo.ObjectOf(ident)
		if object != nil && object.Pkg() != nil && object.Pkg().Path() == "unsafe" {
			unsafeOperation = true
			return false
		}
		return true
	})
	return unsafeOperation
}

func (e *staticEvaluator) evalIdent(pkg *packages.Package, expr *ast.Ident) (StaticValue, error) {
	object := pkg.TypesInfo.ObjectOf(expr)
	switch object := object.(type) {
	case *types.Const:
		return staticConstant(object.Type(), object.Val()), nil
	case *types.Var:
		return e.readVariable(pkg, object)
	case *types.Nil:
		return staticNil(pkg.TypesInfo.TypeOf(expr)), nil
	case *types.Func, *types.Builtin:
		return StaticValue{}, ErrNotStatic
	default:
		return StaticValue{}, ErrNotStatic
	}
}

func (e *staticEvaluator) readVariable(pkg *packages.Package, variable *types.Var) (StaticValue, error) {
	owner, err := e.packageForObject(variable)
	if err != nil {
		return StaticValue{}, err
	}
	state, err := e.packageState(owner)
	if err != nil {
		return StaticValue{}, err
	}
	variableState, ok := state.vars[variable]
	if !ok {
		return StaticValue{}, e.errorAt(pkg, nil, variable, ErrNotStatic)
	}
	if err := e.ensureVar(variableState); err != nil {
		return StaticValue{}, err
	}
	return cloneStaticValue(variableState.storage.Value), nil
}

func (e *staticEvaluator) variableAddress(pkg *packages.Package, variable *types.Var) (*StaticAddress, error) {
	owner, err := e.packageForObject(variable)
	if err != nil {
		return nil, err
	}
	state, err := e.packageState(owner)
	if err != nil {
		return nil, err
	}
	variableState, ok := state.vars[variable]
	if !ok {
		return nil, e.errorAt(pkg, nil, variable, ErrNotStatic)
	}
	return &StaticAddress{Object: variableState.storage}, nil
}

func (e *staticEvaluator) evalSelector(pkg *packages.Package, expr *ast.SelectorExpr) (StaticValue, error) {
	selection := pkg.TypesInfo.Selections[expr]
	if selection == nil {
		object := pkg.TypesInfo.ObjectOf(expr.Sel)
		switch object := object.(type) {
		case *types.Const:
			return staticConstant(object.Type(), object.Val()), nil
		case *types.Var:
			return e.readVariable(pkg, object)
		default:
			return StaticValue{}, ErrNotStatic
		}
	}
	if selection.Kind() != types.FieldVal {
		return StaticValue{}, ErrNotStatic
	}
	value, err := e.evalExpr(pkg, expr.X)
	if err != nil {
		return StaticValue{}, err
	}
	for _, fieldIndex := range selection.Index() {
		if value.Kind == StaticNil {
			return StaticValue{}, ErrStaticPanic
		}
		if value.Kind == StaticPointer {
			value, err = e.loadAddress(pkg, value.Pointer)
			if err != nil {
				return StaticValue{}, err
			}
		}
		if value.Kind != StaticStruct || fieldIndex < 0 || fieldIndex >= len(value.Fields) {
			return StaticValue{}, ErrNotStatic
		}
		value = value.Fields[fieldIndex].Value
	}
	return cloneStaticValue(value), nil
}

func safeUnary(op token.Token, value constant.Value, precision uint) (result constant.Value, err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			err = fmt.Errorf("%v: %w", recovered, ErrStaticPanic)
		}
	}()
	return constant.UnaryOp(op, value, precision), nil
}

func safeBinary(left constant.Value, op token.Token, right constant.Value) (result constant.Value, err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			err = fmt.Errorf("%v: %w", recovered, ErrStaticPanic)
		}
	}()
	return constant.BinaryOp(left, op, right), nil
}

func safeShift(value constant.Value, op token.Token, shift uint) (result constant.Value, err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			err = fmt.Errorf("%v: %w", recovered, ErrStaticPanic)
		}
	}()
	return constant.Shift(value, op, shift), nil
}

func (e *staticEvaluator) evalUnary(pkg *packages.Package, expr *ast.UnaryExpr) (StaticValue, error) {
	if expr.Op == token.AND {
		address, err := e.evalAddress(pkg, expr.X)
		if err != nil {
			return StaticValue{}, err
		}
		return StaticValue{Type: pkg.TypesInfo.TypeOf(expr), Kind: StaticPointer, Pointer: address}, nil
	}
	if expr.Op == token.ARROW {
		return StaticValue{}, ErrNotStatic
	}
	value, err := e.evalExpr(pkg, expr.X)
	if err != nil {
		return StaticValue{}, err
	}
	if value.Kind != StaticConstant {
		return StaticValue{}, ErrNotStatic
	}
	precision := uint(0)
	if expr.Op == token.XOR {
		if basic, ok := underlyingType(value.Type).(*types.Basic); ok {
			precision, _ = integerBits(pkg, value.Type, basic.Kind())
		}
	}
	result, err := safeUnary(expr.Op, value.Exact, precision)
	if err != nil {
		return StaticValue{}, err
	}
	resultType := pkg.TypesInfo.TypeOf(expr)
	return e.assignValue(pkg, staticConstant(resultType, result), resultType, false)
}

func (e *staticEvaluator) evalDereference(pkg *packages.Package, expr *ast.StarExpr) (StaticValue, error) {
	pointer, err := e.evalExpr(pkg, expr.X)
	if err != nil {
		return StaticValue{}, err
	}
	if pointer.Kind == StaticNil {
		return StaticValue{}, ErrStaticPanic
	}
	if pointer.Kind != StaticPointer || pointer.Pointer == nil {
		return StaticValue{}, ErrNotStatic
	}
	return e.loadAddress(pkg, pointer.Pointer)
}

func (e *staticEvaluator) evalBinary(pkg *packages.Package, expr *ast.BinaryExpr) (StaticValue, error) {
	left, err := e.evalExpr(pkg, expr.X)
	if err != nil {
		return StaticValue{}, err
	}
	if expr.Op == token.LAND || expr.Op == token.LOR {
		leftBool, ok := staticBool(left)
		if !ok {
			return StaticValue{}, ErrNotStatic
		}
		if expr.Op == token.LAND && !leftBool || expr.Op == token.LOR && leftBool {
			return staticConstant(pkg.TypesInfo.TypeOf(expr), constant.MakeBool(leftBool)), nil
		}
		right, err := e.evalExpr(pkg, expr.Y)
		if err != nil {
			return StaticValue{}, err
		}
		rightBool, ok := staticBool(right)
		if !ok {
			return StaticValue{}, ErrNotStatic
		}
		return staticConstant(pkg.TypesInfo.TypeOf(expr), constant.MakeBool(rightBool)), nil
	}

	right, err := e.evalExpr(pkg, expr.Y)
	if err != nil {
		return StaticValue{}, err
	}
	if expr.Op == token.EQL || expr.Op == token.NEQ {
		equal, err := e.equalValues(pkg, left, right)
		if err != nil {
			return StaticValue{}, err
		}
		if expr.Op == token.NEQ {
			equal = !equal
		}
		return staticConstant(pkg.TypesInfo.TypeOf(expr), constant.MakeBool(equal)), nil
	}
	if expr.Op == token.LSS || expr.Op == token.LEQ || expr.Op == token.GTR || expr.Op == token.GEQ {
		if left.Kind != StaticConstant || right.Kind != StaticConstant {
			return StaticValue{}, ErrNotStatic
		}
		return staticConstant(
			pkg.TypesInfo.TypeOf(expr),
			constant.MakeBool(constant.Compare(left.Exact, expr.Op, right.Exact)),
		), nil
	}
	if left.Kind != StaticConstant || right.Kind != StaticConstant {
		return StaticValue{}, ErrNotStatic
	}

	var result constant.Value
	if expr.Op == token.SHL || expr.Op == token.SHR {
		if right.Exact.Kind() == constant.Int && constant.Sign(right.Exact) < 0 {
			return StaticValue{}, ErrStaticPanic
		}
		shift, ok := constant.Uint64Val(right.Exact)
		if !ok {
			return StaticValue{}, ErrNotStatic
		}
		basic, typedInteger := underlyingType(left.Type).(*types.Basic)
		bits, bounded := uint(0), false
		if typedInteger && basic.Info()&types.IsInteger != 0 && basic.Info()&types.IsUntyped == 0 {
			bits, bounded = integerBits(pkg, left.Type, basic.Kind())
		}
		if bounded && shift >= uint64(bits) {
			if expr.Op == token.SHR && basic.Info()&types.IsUnsigned == 0 && constant.Sign(left.Exact) < 0 {
				result = constant.MakeInt64(-1)
			} else {
				result = constant.MakeInt64(0)
			}
		} else {
			if shift > uint64(^uint(0)) {
				return StaticValue{}, ErrNotStatic
			}
			result, err = safeShift(left.Exact, expr.Op, uint(shift))
		}
	} else {
		op := expr.Op
		if op == token.ADD && left.Exact.Kind() == constant.String && right.Exact.Kind() == constant.String {
			leftText := constant.StringVal(left.Exact)
			rightText := constant.StringVal(right.Exact)
			if len(leftText) > maxStaticStringBytes-len(rightText) {
				return StaticValue{}, ErrNotStatic
			}
		}
		if op == token.QUO {
			if basic, ok := underlyingType(left.Type).(*types.Basic); ok {
				if constant.Sign(right.Exact) == 0 && basic.Info()&(types.IsFloat|types.IsComplex) != 0 {
					return StaticValue{}, ErrNotStatic
				}
				if basic.Info()&types.IsInteger != 0 {
					op = token.QUO_ASSIGN
				}
			}
		}
		result, err = safeBinary(left.Exact, op, right.Exact)
	}
	if err != nil {
		return StaticValue{}, err
	}
	resultType := pkg.TypesInfo.TypeOf(expr)
	return e.assignValue(pkg, staticConstant(resultType, result), resultType, false)
}

func (e *staticEvaluator) evalIndex(pkg *packages.Package, expr *ast.IndexExpr) (StaticValue, error) {
	if _, ok := underlyingType(pkg.TypesInfo.TypeOf(expr.X)).(*types.Map); ok {
		value, _, err := e.evalMapIndex(pkg, expr)
		return value, err
	}
	container, err := e.evalExpr(pkg, expr.X)
	if err != nil {
		return StaticValue{}, err
	}
	indexValue, err := e.evalExpr(pkg, expr.Index)
	if err != nil {
		return StaticValue{}, err
	}
	index, ok := staticIndex(indexValue)
	if !ok || index < 0 {
		return StaticValue{}, ErrStaticPanic
	}
	if container.Kind == StaticPointer {
		container, err = e.loadAddress(pkg, container.Pointer)
		if err != nil {
			return StaticValue{}, err
		}
	}
	switch container.Kind {
	case StaticArray:
		if index >= int64(len(container.Elements)) {
			return StaticValue{}, ErrStaticPanic
		}
		return cloneStaticValue(container.Elements[index]), nil
	case StaticSliceValue:
		if container.Slice == nil || index >= container.Slice.Len {
			return StaticValue{}, ErrStaticPanic
		}
		return e.loadAddress(pkg, e.sliceElementAddress(container.Slice, index))
	case StaticConstant:
		if container.Exact.Kind() != constant.String {
			return StaticValue{}, ErrNotStatic
		}
		text := constant.StringVal(container.Exact)
		if index >= int64(len(text)) {
			return StaticValue{}, ErrStaticPanic
		}
		return staticConstant(pkg.TypesInfo.TypeOf(expr), constant.MakeInt64(int64(text[index]))), nil
	case StaticNil:
		return StaticValue{}, ErrStaticPanic
	default:
		return StaticValue{}, ErrNotStatic
	}
}

func (e *staticEvaluator) evalMapIndex(pkg *packages.Package, expr *ast.IndexExpr) (StaticValue, bool, error) {
	mapType, ok := underlyingType(pkg.TypesInfo.TypeOf(expr.X)).(*types.Map)
	if !ok {
		return StaticValue{}, false, ErrNotStatic
	}
	container, err := e.evalExpr(pkg, expr.X)
	if err != nil {
		return StaticValue{}, false, err
	}
	key, err := e.evalExpr(pkg, expr.Index)
	if err != nil {
		return StaticValue{}, false, err
	}
	key, err = e.assignValue(pkg, key, mapType.Key(), false)
	if err != nil {
		return StaticValue{}, false, err
	}
	if err := e.ensureRuntimeComparable(pkg, key); err != nil {
		return StaticValue{}, false, err
	}
	if container.Kind == StaticNil {
		zero, err := e.zeroValue(pkg, mapType.Elem())
		return zero, false, err
	}
	if container.Kind != StaticMapValue || container.Map == nil {
		return StaticValue{}, false, ErrNotStatic
	}
	return e.mapLookup(pkg, container.Map, key)
}

func (e *staticEvaluator) evalBound(pkg *packages.Package, expr ast.Expr, fallback int64) (int64, error) {
	if expr == nil {
		return fallback, nil
	}
	value, err := e.evalExpr(pkg, expr)
	if err != nil {
		return 0, err
	}
	if value.Kind != StaticConstant || value.Exact == nil || value.Exact.Kind() != constant.Int {
		return 0, ErrNotStatic
	}
	bound, ok := constant.Int64Val(value.Exact)
	if !ok {
		return 0, ErrStaticPanic
	}
	return bound, nil
}

func (e *staticEvaluator) evalSlice(pkg *packages.Package, expr *ast.SliceExpr) (StaticValue, error) {
	targetType := pkg.TypesInfo.TypeOf(expr)
	containerType := underlyingType(pkg.TypesInfo.TypeOf(expr.X))
	if _, ok := containerType.(*types.Basic); ok {
		container, err := e.evalExpr(pkg, expr.X)
		if err != nil {
			return StaticValue{}, err
		}
		if container.Kind != StaticConstant || container.Exact.Kind() != constant.String {
			return StaticValue{}, ErrNotStatic
		}
		text := constant.StringVal(container.Exact)
		low, err := e.evalBound(pkg, expr.Low, 0)
		if err != nil {
			return StaticValue{}, err
		}
		high, err := e.evalBound(pkg, expr.High, int64(len(text)))
		if err != nil {
			return StaticValue{}, err
		}
		if low < 0 || low > high || high > int64(len(text)) {
			return StaticValue{}, ErrStaticPanic
		}
		return staticConstant(targetType, constant.MakeString(text[low:high])), nil
	}

	var descriptor StaticSlice
	switch containerType := containerType.(type) {
	case *types.Array:
		address, err := e.evalAddress(pkg, expr.X)
		if err != nil {
			return StaticValue{}, err
		}
		descriptor = StaticSlice{
			Backing: address.Object,
			Path:    cloneStaticPath(address.Path),
			Len:     containerType.Len(),
			Cap:     containerType.Len(),
		}
	case *types.Pointer:
		pointer, err := e.evalExpr(pkg, expr.X)
		if err != nil {
			return StaticValue{}, err
		}
		if pointer.Kind == StaticNil {
			return StaticValue{}, ErrStaticPanic
		}
		if pointer.Kind != StaticPointer || pointer.Pointer == nil {
			return StaticValue{}, ErrNotStatic
		}
		arrayType, ok := underlyingType(containerType.Elem()).(*types.Array)
		if !ok {
			return StaticValue{}, ErrNotStatic
		}
		descriptor = StaticSlice{
			Backing: pointer.Pointer.Object,
			Path:    cloneStaticPath(pointer.Pointer.Path),
			Offset:  pointer.Pointer.ArrayOffset,
			Len:     arrayType.Len(),
			Cap:     arrayType.Len(),
		}
	case *types.Slice:
		container, err := e.evalExpr(pkg, expr.X)
		if err != nil {
			return StaticValue{}, err
		}
		if container.Kind == StaticNil {
			descriptor = StaticSlice{}
		} else if container.Kind == StaticSliceValue && container.Slice != nil {
			descriptor = *container.Slice
			descriptor.Path = cloneStaticPath(descriptor.Path)
		} else {
			return StaticValue{}, ErrNotStatic
		}
	default:
		return StaticValue{}, ErrNotStatic
	}

	low, err := e.evalBound(pkg, expr.Low, 0)
	if err != nil {
		return StaticValue{}, err
	}
	high, err := e.evalBound(pkg, expr.High, descriptor.Len)
	if err != nil {
		return StaticValue{}, err
	}
	max := descriptor.Cap
	if expr.Slice3 {
		max, err = e.evalBound(pkg, expr.Max, descriptor.Cap)
		if err != nil {
			return StaticValue{}, err
		}
	}
	if low < 0 || low > high || high > max || max > descriptor.Cap {
		return StaticValue{}, ErrStaticPanic
	}
	result := &StaticSlice{
		Backing: descriptor.Backing,
		Path:    cloneStaticPath(descriptor.Path),
		Offset:  descriptor.Offset + low,
		Len:     high - low,
		Cap:     max - low,
	}
	if result.Backing == nil && result.Len == 0 && result.Cap == 0 {
		return staticNil(targetType), nil
	}
	return StaticValue{Type: targetType, Kind: StaticSliceValue, Slice: result}, nil
}

func (e *staticEvaluator) evalTypeAssertion(pkg *packages.Package, expr *ast.TypeAssertExpr) (StaticValue, bool, error) {
	value, err := e.evalExpr(pkg, expr.X)
	if err != nil {
		return StaticValue{}, false, err
	}
	var target types.Type
	if expr.Type != nil {
		target = pkg.TypesInfo.TypeOf(expr.Type)
	} else {
		target = pkg.TypesInfo.TypeOf(expr)
	}
	if value.Kind == StaticNil || value.Kind != StaticInterface || value.Dynamic == nil {
		zero, zeroErr := e.zeroValue(pkg, target)
		return zero, false, zeroErr
	}
	dynamic := *value.Dynamic
	if _, ok := underlyingType(target).(*types.Interface); ok {
		if types.AssignableTo(dynamic.Type, target) {
			converted, err := e.assignValue(pkg, dynamic, target, false)
			return converted, err == nil, err
		}
	} else if types.Identical(dynamic.Type, target) {
		converted, err := e.assignValue(pkg, dynamic, target, false)
		return converted, err == nil, err
	}
	zero, zeroErr := e.zeroValue(pkg, target)
	return zero, false, zeroErr
}

func (e *staticEvaluator) evalAddress(pkg *packages.Package, expr ast.Expr) (*StaticAddress, error) {
	switch expr := expr.(type) {
	case *ast.ParenExpr:
		return e.evalAddress(pkg, expr.X)
	case *ast.Ident:
		variable, ok := pkg.TypesInfo.ObjectOf(expr).(*types.Var)
		if !ok {
			return nil, ErrNotStatic
		}
		return e.variableAddress(pkg, variable)
	case *ast.StarExpr:
		pointer, err := e.evalExpr(pkg, expr.X)
		if err != nil {
			return nil, err
		}
		if pointer.Kind == StaticNil {
			return nil, ErrStaticPanic
		}
		if pointer.Kind != StaticPointer || pointer.Pointer == nil {
			return nil, ErrNotStatic
		}
		return cloneStaticAddress(pointer.Pointer), nil
	case *ast.CompositeLit:
		value, err := e.evalComposite(pkg, expr)
		if err != nil {
			return nil, err
		}
		object := e.newObject(value.Type, value)
		return &StaticAddress{Object: object}, nil
	case *ast.SelectorExpr:
		return e.evalSelectorAddress(pkg, expr)
	case *ast.IndexExpr:
		return e.evalIndexAddress(pkg, expr)
	default:
		return nil, ErrNotStatic
	}
}

func (e *staticEvaluator) evalSelectorAddress(pkg *packages.Package, expr *ast.SelectorExpr) (*StaticAddress, error) {
	selection := pkg.TypesInfo.Selections[expr]
	if selection == nil {
		variable, ok := pkg.TypesInfo.ObjectOf(expr.Sel).(*types.Var)
		if !ok {
			return nil, ErrNotStatic
		}
		return e.variableAddress(pkg, variable)
	}
	if selection.Kind() != types.FieldVal {
		return nil, ErrNotStatic
	}

	var address *StaticAddress
	var err error
	if _, ok := underlyingType(pkg.TypesInfo.TypeOf(expr.X)).(*types.Pointer); ok {
		pointer, evalErr := e.evalExpr(pkg, expr.X)
		if evalErr != nil {
			return nil, evalErr
		}
		if pointer.Kind == StaticNil {
			return nil, ErrStaticPanic
		}
		if pointer.Kind != StaticPointer {
			return nil, ErrNotStatic
		}
		address = cloneStaticAddress(pointer.Pointer)
	} else {
		address, err = e.evalAddress(pkg, expr.X)
		if err != nil {
			return nil, err
		}
	}

	currentType := pkg.TypesInfo.TypeOf(expr.X)
	if pointer, ok := underlyingType(currentType).(*types.Pointer); ok {
		currentType = pointer.Elem()
	}
	for position, fieldIndex := range selection.Index() {
		structType, ok := underlyingType(currentType).(*types.Struct)
		if !ok || fieldIndex < 0 || fieldIndex >= structType.NumFields() {
			return nil, ErrNotStatic
		}
		field := structType.Field(fieldIndex)
		address.Path = append(address.Path, StaticPathStep{
			Kind:  StaticPathField,
			Index: int64(fieldIndex),
			Field: field,
		})
		currentType = field.Type()
		if position < len(selection.Index())-1 {
			if pointerType, ok := underlyingType(currentType).(*types.Pointer); ok {
				pointerValue, loadErr := e.loadAddress(pkg, address)
				if loadErr != nil {
					return nil, loadErr
				}
				if pointerValue.Kind == StaticNil {
					return nil, ErrStaticPanic
				}
				if pointerValue.Kind != StaticPointer {
					return nil, ErrNotStatic
				}
				address = cloneStaticAddress(pointerValue.Pointer)
				currentType = pointerType.Elem()
			}
		}
	}
	return address, nil
}

func (e *staticEvaluator) evalIndexAddress(pkg *packages.Package, expr *ast.IndexExpr) (*StaticAddress, error) {
	containerType := underlyingType(pkg.TypesInfo.TypeOf(expr.X))
	switch containerType := containerType.(type) {
	case *types.Array:
		address, err := e.evalAddress(pkg, expr.X)
		if err != nil {
			return nil, err
		}
		indexValue, err := e.evalExpr(pkg, expr.Index)
		if err != nil {
			return nil, err
		}
		index, ok := staticIndex(indexValue)
		if !ok || index < 0 {
			return nil, ErrStaticPanic
		}
		if index >= containerType.Len() {
			return nil, ErrStaticPanic
		}
		address.Path = append(address.Path, StaticPathStep{Kind: StaticPathElement, Index: index})
		return address, nil
	case *types.Pointer:
		arrayType, ok := underlyingType(containerType.Elem()).(*types.Array)
		if !ok {
			return nil, ErrNotStatic
		}
		pointer, err := e.evalExpr(pkg, expr.X)
		if err != nil {
			return nil, err
		}
		if pointer.Kind != StaticPointer && pointer.Kind != StaticNil {
			return nil, ErrNotStatic
		}
		indexValue, err := e.evalExpr(pkg, expr.Index)
		if err != nil {
			return nil, err
		}
		index, ok := staticIndex(indexValue)
		if !ok || index < 0 || index >= arrayType.Len() || pointer.Kind == StaticNil || pointer.Pointer == nil {
			return nil, ErrStaticPanic
		}
		address := cloneStaticAddress(pointer.Pointer)
		address.Path = append(address.Path, StaticPathStep{
			Kind:  StaticPathElement,
			Index: address.ArrayOffset + index,
		})
		address.ArrayOffset = 0
		address.ArrayView = nil
		return address, nil
	case *types.Slice:
		container, err := e.evalExpr(pkg, expr.X)
		if err != nil {
			return nil, err
		}
		if container.Kind != StaticSliceValue && container.Kind != StaticNil {
			return nil, ErrNotStatic
		}
		indexValue, err := e.evalExpr(pkg, expr.Index)
		if err != nil {
			return nil, err
		}
		index, ok := staticIndex(indexValue)
		if !ok || index < 0 || container.Kind == StaticNil || container.Slice == nil || index >= container.Slice.Len {
			return nil, ErrStaticPanic
		}
		return e.sliceElementAddress(container.Slice, index), nil
	default:
		return nil, ErrNotStatic
	}
}
