package tracer

import (
	"fmt"
	"go/ast"
	"go/types"

	"golang.org/x/tools/go/packages"
)

func (e *staticEvaluator) evalComposite(pkg *packages.Package, expr *ast.CompositeLit) (StaticValue, error) {
	typ := pkg.TypesInfo.TypeOf(expr)
	if typ == nil {
		return StaticValue{}, ErrMissingTypeInfo
	}
	switch valueType := underlyingType(typ).(type) {
	case *types.Struct:
		return e.evalStructComposite(pkg, expr, typ, valueType)
	case *types.Array:
		return e.evalArrayComposite(pkg, expr, typ, valueType)
	case *types.Slice:
		return e.evalSliceComposite(pkg, expr, typ, valueType)
	case *types.Map:
		return e.evalMapComposite(pkg, expr, typ, valueType)
	default:
		return StaticValue{}, ErrNotStatic
	}
}

func structFieldIndex(structType *types.Struct, field *ast.Ident, info *types.Info) int {
	if object, ok := info.ObjectOf(field).(*types.Var); ok {
		for index := 0; index < structType.NumFields(); index++ {
			if structType.Field(index) == object {
				return index
			}
		}
	}
	for index := 0; index < structType.NumFields(); index++ {
		if structType.Field(index).Name() == field.Name {
			return index
		}
	}
	return -1
}

func (e *staticEvaluator) evalStructComposite(
	pkg *packages.Package,
	expr *ast.CompositeLit,
	typ types.Type,
	structType *types.Struct,
) (StaticValue, error) {
	result, err := e.zeroValue(pkg, typ)
	if err != nil {
		return StaticValue{}, err
	}
	keyed := len(expr.Elts) > 0
	if keyed {
		_, keyed = expr.Elts[0].(*ast.KeyValueExpr)
	}
	for index, element := range expr.Elts {
		fieldIndex := index
		valueExpr := element
		if pair, ok := element.(*ast.KeyValueExpr); ok {
			field, ok := pair.Key.(*ast.Ident)
			if !ok {
				return StaticValue{}, ErrNotStatic
			}
			fieldIndex = structFieldIndex(structType, field, pkg.TypesInfo)
			valueExpr = pair.Value
		} else if keyed {
			return StaticValue{}, ErrNotStatic
		}
		if fieldIndex < 0 || fieldIndex >= structType.NumFields() {
			return StaticValue{}, ErrStaticPanic
		}
		value, err := e.evalExpr(pkg, valueExpr)
		if err != nil {
			return StaticValue{}, err
		}
		value, err = e.assignValue(pkg, value, structType.Field(fieldIndex).Type(), false)
		if err != nil {
			return StaticValue{}, err
		}
		result.Fields[fieldIndex].Value = value
	}
	return result, nil
}

func (e *staticEvaluator) compositeIndex(pkg *packages.Package, expr ast.Expr, fallback int64) (int64, error) {
	if expr == nil {
		return fallback, nil
	}
	value, err := e.evalExpr(pkg, expr)
	if err != nil {
		return 0, err
	}
	index, ok := staticIndex(value)
	if !ok || index < 0 {
		return 0, ErrNotStatic
	}
	return index, nil
}

func (e *staticEvaluator) evalArrayComposite(
	pkg *packages.Package,
	expr *ast.CompositeLit,
	typ types.Type,
	arrayType *types.Array,
) (StaticValue, error) {
	result, err := e.zeroValue(pkg, typ)
	if err != nil {
		return StaticValue{}, err
	}
	next := int64(0)
	for _, element := range expr.Elts {
		index := next
		valueExpr := element
		if pair, ok := element.(*ast.KeyValueExpr); ok {
			index, err = e.compositeIndex(pkg, pair.Key, next)
			if err != nil {
				return StaticValue{}, err
			}
			valueExpr = pair.Value
		}
		if index < 0 || index >= arrayType.Len() {
			return StaticValue{}, ErrStaticPanic
		}
		value, err := e.evalExpr(pkg, valueExpr)
		if err != nil {
			return StaticValue{}, err
		}
		value, err = e.assignValue(pkg, value, arrayType.Elem(), false)
		if err != nil {
			return StaticValue{}, err
		}
		result.Elements[index] = value
		next = index + 1
	}
	return result, nil
}

type indexedStaticValue struct {
	index int64
	value StaticValue
}

func (e *staticEvaluator) evalSliceComposite(
	pkg *packages.Package,
	expr *ast.CompositeLit,
	typ types.Type,
	sliceType *types.Slice,
) (StaticValue, error) {
	indexed := make([]indexedStaticValue, 0, len(expr.Elts))
	next := int64(0)
	length := int64(0)
	for _, element := range expr.Elts {
		index := next
		valueExpr := element
		if pair, ok := element.(*ast.KeyValueExpr); ok {
			var err error
			index, err = e.compositeIndex(pkg, pair.Key, next)
			if err != nil {
				return StaticValue{}, err
			}
			valueExpr = pair.Value
		}
		value, err := e.evalExpr(pkg, valueExpr)
		if err != nil {
			return StaticValue{}, err
		}
		value, err = e.assignValue(pkg, value, sliceType.Elem(), false)
		if err != nil {
			return StaticValue{}, err
		}
		indexed = append(indexed, indexedStaticValue{index: index, value: value})
		if index+1 > length {
			length = index + 1
		}
		next = index + 1
	}
	if length > maxStaticElements {
		return StaticValue{}, fmt.Errorf("slice literal is too large: %w", ErrNotStatic)
	}
	arrayType := types.NewArray(sliceType.Elem(), length)
	arrayValue, err := e.zeroValue(pkg, arrayType)
	if err != nil {
		return StaticValue{}, err
	}
	for _, element := range indexed {
		arrayValue.Elements[element.index] = element.value
	}
	backing := e.newObject(arrayType, arrayValue)
	return StaticValue{
		Type: typ,
		Kind: StaticSliceValue,
		Slice: &StaticSlice{
			Backing: backing,
			Len:     length,
			Cap:     length,
		},
	}, nil
}

func (e *staticEvaluator) evalMapComposite(
	pkg *packages.Package,
	expr *ast.CompositeLit,
	typ types.Type,
	mapType *types.Map,
) (StaticValue, error) {
	staticMap := e.newMap(mapType)
	for _, element := range expr.Elts {
		pair, ok := element.(*ast.KeyValueExpr)
		if !ok {
			return StaticValue{}, ErrNotStatic
		}
		key, err := e.evalExpr(pkg, pair.Key)
		if err != nil {
			return StaticValue{}, err
		}
		key, err = e.assignValue(pkg, key, mapType.Key(), false)
		if err != nil {
			return StaticValue{}, err
		}
		value, err := e.evalExpr(pkg, pair.Value)
		if err != nil {
			return StaticValue{}, err
		}
		value, err = e.assignValue(pkg, value, mapType.Elem(), false)
		if err != nil {
			return StaticValue{}, err
		}
		if err := e.mapSet(pkg, staticMap, key, value); err != nil {
			return StaticValue{}, err
		}
	}
	return StaticValue{Type: typ, Kind: StaticMapValue, Map: staticMap}, nil
}
