package frontend

import (
	"fmt"
	"go/ast"
	"go/constant"
	"go/token"
	"go/types"
	"math"
	"path/filepath"
	"reflect"
	"sort"
	"strings"

	"golang.org/x/tools/go/packages"

	"github.com/WindowsSov8forUs/sonolus-core-go/core/resource"
	"github.com/WindowsSov8forUs/sonolus-go/v2/internal/compiler/catalog"
	"github.com/WindowsSov8forUs/sonolus-go/v2/internal/compiler/intrinsic"
	"github.com/WindowsSov8forUs/sonolus-go/v2/internal/compiler/ir"
	"github.com/WindowsSov8forUs/sonolus-go/v2/internal/compiler/mode"
	"github.com/WindowsSov8forUs/sonolus-go/v2/internal/compiler/source"
)

type lowerValue struct {
	type_             types.Type
	slots             []ir.Expr
	places            []ir.Place
	multi             []lowerValue
	container         *containerValue
	variadic          *variadicValue
	entity            *entityReferenceValue
	entityField       bool
	callable          *staticCallable
	callableArray     *callableArrayValue
	stream            *streamValue
	pointer           *pointerValue
	persistentPointer *persistentPointerValue
	pointerLoad       *pointerLoad
	nilPointer        bool
	interface_        *interfaceValue
	containerVariant  *finiteVariant[lowerValue]
	aggregate         *aggregateValue
	aggregateIndex    *aggregateIndexValue
	aggregatePointer  *finiteVariant[lowerValue]
	aggregateLoad     *aggregatePointerLoadValue
	levelGlobal       *levelGlobalValue
	immutablePackage  bool
}

type aggregateValue struct {
	fields []lowerValue
}

type aggregateIndexValue struct {
	array lowerValue
	index lowerValue
	path  []int
}

type aggregatePointerLoadValue struct {
	pointer *finiteVariant[lowerValue]
	path    []aggregatePointerPathStep
}

type aggregatePointerPathStep struct {
	index     int
	dynamic   lowerValue
	isDynamic bool
}

type callableArrayValue struct {
	element   types.Type
	elements  []*staticCallable
	immutable bool
}

type finiteVariant[T any] struct {
	alternatives []T
	tag          lowerValue
}

func newFiniteVariant[T any](l *lowerer, name string, node ast.Node) finiteVariant[T] {
	tag := l.allocZeroed(name, types.Typ[types.Int], node)
	l.store(tag, scalarValue(ir.Const{Value: -1}, types.Typ[types.Int]), node)
	return finiteVariant[T]{tag: tag}
}

func (variant *finiteVariant[T]) add(value T, equal func(T, T) bool) (int, bool) {
	for index, candidate := range variant.alternatives {
		if equal(candidate, value) {
			return index, true
		}
	}
	if len(variant.alternatives) >= 256 {
		return -1, false
	}
	variant.alternatives = append(variant.alternatives, value)
	return len(variant.alternatives) - 1, true
}

func indexedCallableVariant(l *lowerer, callables []*staticCallable, tag lowerValue, node ast.Node) *staticCallable {
	if len(callables) > 256 {
		l.errorAt(node, "finite callable variant exceeds 256 alternatives")
		return &staticCallable{finiteVariant: finiteVariant[*staticCallable]{tag: tag}}
	}
	return &staticCallable{finiteVariant: finiteVariant[*staticCallable]{
		alternatives: append([]*staticCallable(nil), callables...),
		tag:          tag,
	}}
}

func (l *lowerer) newCallableCell(name string, callable *staticCallable, node ast.Node) *staticCallable {
	variant := newFiniteVariant[*staticCallable](l, name+".callable", node)
	cell := &staticCallable{finiteVariant: variant}
	if callable != nil {
		l.storeCallableCell(cell, callable, node)
	}
	return cell
}

func (l *lowerer) storeCallableCell(destination, source *staticCallable, node ast.Node) {
	if destination == nil || destination.tag.type_ == nil {
		l.errorAt(node, "callable assignment target is not a mutable local or parameter")
		return
	}
	if source == nil {
		l.errorAt(node, "callable assignment requires a statically finite source")
		return
	}
	if destination == source {
		return
	}
	if source.tag.type_ == nil {
		index, added := destination.add(source, sameCallable)
		if !added {
			l.errorAt(node, "finite callable variable exceeds 256 alternatives")
			return
		}
		if destination.resultType == nil {
			destination.resultType = source.resultType
		}
		l.store(destination.tag, scalarValue(ir.Const{Value: float64(index)}, types.Typ[types.Int]), node)
		return
	}
	if len(source.alternatives) == 0 {
		l.store(destination.tag, scalarValue(ir.Const{Value: -1}, types.Typ[types.Int]), node)
		return
	}
	tag := l.materialize("callable.assign.tag", source.tag, node)
	merge, invalid := l.newBlock(), l.newBlock()
	blocks := make([]*ir.Block, len(source.alternatives))
	cases := make([]ir.SwitchCase, len(blocks))
	for index := range blocks {
		blocks[index] = l.newBlock()
		cases[index] = ir.SwitchCase{Value: float64(index), Target: blocks[index].ID}
	}
	_ = l.builder.Switch(tag.slots[0], cases, invalid)
	for index, alternative := range source.alternatives {
		l.setCurrent(blocks[index])
		l.storeCallableCell(destination, alternative, node)
		l.jump(merge)
	}
	l.setCurrent(invalid)
	_ = l.builder.MarkUnreachable()
	l.setCurrent(merge)
}

func callableArrayType(t types.Type) (*types.Array, bool) {
	if t == nil {
		return nil, false
	}
	array, ok := types.Unalias(t).Underlying().(*types.Array)
	return array, ok && isFunctionType(array.Elem())
}

func (l *lowerer) newCallableArray(name string, t types.Type, node ast.Node) lowerValue {
	array, ok := callableArrayType(t)
	if !ok {
		return lowerValue{}
	}
	result := lowerValue{type_: t, callableArray: &callableArrayValue{element: array.Elem(), elements: make([]*staticCallable, int(array.Len()))}}
	for index := range result.callableArray.elements {
		result.callableArray.elements[index] = l.newCallableCell(fmt.Sprintf("%s.%d", name, index), nil, node)
	}
	return result
}

func (l *lowerer) storeCallableArray(destination, source lowerValue, node ast.Node) {
	if destination.callableArray == nil || source.callableArray == nil || len(destination.callableArray.elements) != len(source.callableArray.elements) {
		l.errorAt(node, "callable array assignment requires identical fixed array types")
		return
	}
	if destination.callableArray.immutable {
		l.errorAt(node, "package callable arrays are immutable")
		return
	}
	for index := range destination.callableArray.elements {
		l.storeCallableCell(destination.callableArray.elements[index], source.callableArray.elements[index], node)
	}
}

func (l *lowerer) copyCallableArray(name string, source lowerValue, node ast.Node) lowerValue {
	result := l.newCallableArray(name, source.type_, node)
	l.storeCallableArray(result, source, node)
	return result
}

func (l *lowerer) storeCallableArrayElement(array lowerValue, index lowerValue, callable *staticCallable, node ast.Node) {
	if array.callableArray == nil || len(index.slots) != 1 {
		l.errorAt(node, "callable array element assignment requires a scalar index")
		return
	}
	if array.callableArray.immutable {
		l.errorAt(node, "package callable arrays are immutable")
		return
	}
	if constantIndex, constantOK := index.slots[0].(ir.Const); constantOK {
		position := int(constantIndex.Value)
		if float64(position) != constantIndex.Value || position < 0 || position >= len(array.callableArray.elements) {
			l.errorAt(node, "callable array index is out of bounds")
			return
		}
		l.storeCallableCell(array.callableArray.elements[position], callable, node)
		return
	}
	index = l.materialize("callable.array.assign.index", index, node)
	inBounds := l.pure(resource.RuntimeFunctionAnd, node,
		l.pure(resource.RuntimeFunctionGreaterOr, node, index.slots[0], ir.Const{}),
		l.pure(resource.RuntimeFunctionLess, node, index.slots[0], ir.Const{Value: float64(len(array.callableArray.elements))}))
	l.guard(node, inBounds)
	merge, invalid := l.newBlock(), l.newBlock()
	blocks := make([]*ir.Block, len(array.callableArray.elements))
	cases := make([]ir.SwitchCase, len(blocks))
	for position := range blocks {
		blocks[position] = l.newBlock()
		cases[position] = ir.SwitchCase{Value: float64(position), Target: blocks[position].ID}
	}
	_ = l.builder.Switch(index.slots[0], cases, invalid)
	for position, block := range blocks {
		l.setCurrent(block)
		l.storeCallableCell(array.callableArray.elements[position], callable, node)
		l.jump(merge)
	}
	l.setCurrent(invalid)
	_ = l.builder.MarkUnreachable()
	l.setCurrent(merge)
}

func (l *lowerer) packageCallableArray(object types.Object, node ast.Node) (lowerValue, bool) {
	variable, ok := object.(*types.Var)
	if !ok || variable.Pkg() == nil || variable.IsField() {
		return lowerValue{}, false
	}
	if declaration := l.packagePointers[variable]; declaration != nil {
		storage, read, write := levelGlobalStorageAccess(declaration, l.mode, l.phase)
		return l.lowerPersistentPackagePointer(declaration, storage, read, write), true
	}
	if _, ok := callableArrayType(l.resolveType(variable.Type())); !ok {
		return lowerValue{}, false
	}
	owner := l.packages[variable.Pkg()]
	if owner == nil {
		return lowerValue{}, false
	}
	initializer, exists := variableInitializer(owner, variable)
	if !exists {
		l.errorAt(node, "package callable array %s requires a static initializer", variable.Name())
		return lowerValue{}, true
	}
	previous := l.pkg
	l.pkg = owner
	value := l.expr(initializer)
	l.pkg = previous
	if value.callableArray == nil {
		l.errorAt(node, "package callable array %s requires a statically finite initializer", variable.Name())
		return lowerValue{}, true
	}
	value.type_ = l.resolveType(variable.Type())
	value.callableArray.immutable = true
	return value, true
}

func (l *lowerer) lowerStaticRuntimeValue(value source.StaticValue, target types.Type, node ast.Node) (lowerValue, bool) {
	target = l.resolveType(target)
	switch value.Kind {
	case source.StaticConstant:
		expression, ok := constantExpr(value.Exact)
		if !ok {
			return lowerValue{}, false
		}
		return lowerValue{type_: target, slots: []ir.Expr{expression}}, true
	case source.StaticNil:
		return zeroValue(target), true
	case source.StaticArray:
		array, ok := types.Unalias(target).Underlying().(*types.Array)
		if !ok || len(value.Elements) != int(array.Len()) {
			return lowerValue{}, false
		}
		result := lowerValue{type_: target}
		for _, element := range value.Elements {
			lowered, ok := l.lowerStaticRuntimeValue(element, array.Elem(), node)
			if !ok {
				return lowerValue{}, false
			}
			result.slots = append(result.slots, lowered.slots...)
		}
		return result, true
	case source.StaticStruct:
		structure, ok := types.Unalias(target).Underlying().(*types.Struct)
		if !ok {
			return lowerValue{}, false
		}
		fields := make(map[*types.Var]source.StaticValue, len(value.Fields))
		for _, field := range value.Fields {
			fields[field.Field] = field.Value
		}
		result := lowerValue{type_: target}
		for index := 0; index < structure.NumFields(); index++ {
			field := structure.Field(index)
			static, exists := fields[field]
			if !exists {
				return lowerValue{}, false
			}
			lowered, ok := l.lowerStaticRuntimeValue(static, field.Type(), node)
			if !ok {
				return lowerValue{}, false
			}
			result.slots = append(result.slots, lowered.slots...)
		}
		return result, true
	default:
		return lowerValue{}, false
	}
}

func (l *lowerer) packageStaticValue(object types.Object, node ast.Node) (lowerValue, bool) {
	variable, ok := object.(*types.Var)
	if !ok || variable.Pkg() == nil || variable.IsField() {
		return lowerValue{}, false
	}
	target := l.resolveType(variable.Type())
	if l.containsAggregateDescriptor(target) {
		l.errorAt(node, "package value %s cannot contain callback-local descriptors", variable.Name())
		return lowerValue{}, true
	}
	switch underlying := types.Unalias(target).Underlying().(type) {
	case *types.Basic, *types.Struct:
	case *types.Array:
		if isFunctionType(underlying.Elem()) {
			return lowerValue{}, false
		}
	default:
		return lowerValue{}, false
	}
	owner := l.packages[variable.Pkg()]
	if owner == nil {
		return lowerValue{}, false
	}
	binding, err := source.NewASTTracer(owner).EvalObject(variable)
	if err != nil {
		l.errorAt(node, "package value %s requires a pure static initializer", variable.Name())
		return lowerValue{}, true
	}
	if binding.Value.Kind == source.StaticPointer && binding.Value.Pointer != nil && binding.Value.Pointer.Object != nil {
		if declaration := l.packageGlobals[binding.Value.Pointer.Object]; declaration != nil && len(binding.Value.Pointer.Path) == 0 {
			storage, read, write := levelGlobalStorageAccess(declaration, l.mode, l.phase)
			return l.lowerPersistentPackagePointer(declaration, storage, read, write), true
		}
	}
	value, ok := l.lowerStaticRuntimeValue(binding.Value, target, node)
	if !ok {
		l.errorAt(node, "package value %s contains data that is not representable in callbacks", variable.Name())
		return lowerValue{}, true
	}
	value.immutablePackage = true
	return value, true
}

func (l *lowerer) indexImmutablePackageArray(base lowerValue, array *types.Array, index lowerValue, node ast.Node) lowerValue {
	index = l.materialize("package.array.index", index, node)
	inBounds := l.pure(resource.RuntimeFunctionAnd, node,
		l.pure(resource.RuntimeFunctionGreaterOr, node, index.slots[0], ir.Const{}),
		l.pure(resource.RuntimeFunctionLess, node, index.slots[0], ir.Const{Value: float64(array.Len())}))
	l.guard(node, inBounds)
	elementType := array.Elem()
	elementSlots := l.runtimeTypeOf(types.NewArray(elementType, 1)).Slots
	result := l.allocZeroed("package.array.value", elementType, node)
	result.immutablePackage = true
	if elementSlots == 0 {
		return result
	}
	merge, invalid := l.newBlock(), l.newBlock()
	blocks := make([]*ir.Block, int(array.Len()))
	cases := make([]ir.SwitchCase, len(blocks))
	for position := range blocks {
		blocks[position] = l.newBlock()
		cases[position] = ir.SwitchCase{Value: float64(position), Target: blocks[position].ID}
	}
	_ = l.builder.Switch(index.slots[0], cases, invalid)
	for position, block := range blocks {
		l.setCurrent(block)
		start := position * elementSlots
		l.store(result, lowerValue{type_: elementType, slots: base.slots[start : start+elementSlots]}, node)
		l.jump(merge)
	}
	l.setCurrent(invalid)
	_ = l.builder.MarkUnreachable()
	l.setCurrent(merge)
	if binding, exists := l.entityBinding(elementType); exists {
		result.entity = &entityReferenceValue{binding: binding}
	}
	return result
}

type interfaceValue struct {
	finiteVariant[lowerValue]
	persistent bool
	rawTag     lowerValue
}

type pointerValue struct {
	finiteVariant[[]ir.Place]
}

type persistentPointerValue struct {
	handle  lowerValue
	storage string
	target  *LevelGlobalFieldDeclaration
	read    bool
	write   bool
}

func levelGlobalDeclarationHasDescriptors(declaration *LevelGlobalFieldDeclaration) bool {
	if declaration == nil {
		return false
	}
	if declaration.PersistentKind != "" || declaration.ContainerKind != "" {
		return true
	}
	for _, child := range declaration.Fields {
		if levelGlobalDeclarationHasDescriptors(child) {
			return true
		}
	}
	for _, child := range declaration.Elements {
		if levelGlobalDeclarationHasDescriptors(child) {
			return true
		}
	}
	return false
}

func (l *lowerer) finitePersistentPointer(value lowerValue, node ast.Node) lowerValue {
	persistent := value.persistentPointer
	if persistent == nil || persistent.target == nil || len(persistent.handle.slots) != 1 {
		return value
	}
	handle := l.materialize("persistent.pointer.snapshot", scalarValue(persistent.handle.slots[0], types.Typ[types.Int]), node)
	base := l.pure(resource.RuntimeFunctionSubtract, node, handle.slots[0], ir.Const{Value: 1})
	places := make([]ir.Place, persistent.target.Size)
	for offset := range places {
		places[offset] = l.memory(persistent.storage, base, 1, offset, persistent.read, persistent.write, node)
	}
	tag := scalarValue(l.pure(resource.RuntimeFunctionIf, node,
		l.pure(resource.RuntimeFunctionEqual, node, handle.slots[0], ir.Const{}), ir.Const{}, ir.Const{Value: 1}), types.Typ[types.Int])
	return lowerValue{type_: value.type_, pointer: &pointerValue{finiteVariant: finiteVariant[[]ir.Place]{
		alternatives: [][]ir.Place{nil, places}, tag: tag,
	}}}
}

type pointerLoad struct {
	pointer     *pointerValue
	offset      int
	size        int
	index       ir.Expr
	arrayLength int
	stride      int
}

func (l *lowerer) persistentPointerAddress(value lowerValue, storage string, node ast.Node) (ir.Expr, bool) {
	if value.nilPointer {
		return ir.Const{}, true
	}
	if value.persistentPointer != nil {
		if value.persistentPointer.storage != storage || len(value.persistentPointer.handle.slots) != 1 {
			l.errorAt(node, "persistent pointer assignment requires the same semantic memory storage")
			return nil, false
		}
		return value.persistentPointer.handle.slots[0], true
	}
	if value.levelGlobal != nil {
		if value.levelGlobal.storage != storage {
			l.errorAt(node, "persistent pointer assignment requires the same semantic memory storage")
			return nil, false
		}
		return l.pure(resource.RuntimeFunctionAdd, node, value.levelGlobal.base, ir.Const{Value: 1}), true
	}
	if value.pointer != nil && len(value.pointer.finiteVariant.tag.slots) == 1 {
		return l.selectPersistentPointerAddress(value.pointer.finiteVariant.tag.slots[0], value.pointer.finiteVariant.alternatives, storage, node)
	}
	if value.aggregatePointer != nil && len(value.aggregatePointer.tag.slots) == 1 {
		addresses := make([]ir.Expr, len(value.aggregatePointer.alternatives))
		for index, alternative := range value.aggregatePointer.alternatives {
			address, ok := l.persistentPointerAddress(alternative, storage, node)
			if !ok {
				return nil, false
			}
			addresses[index] = address
		}
		return l.selectPersistentPointerAddress(value.aggregatePointer.tag.slots[0], addresses, storage, node)
	}
	if len(value.places) == 0 {
		l.errorAt(node, "persistent pointer assignment requires a level global address or nil")
		return nil, false
	}
	first, ok := value.places[0].(ir.MemoryPlace)
	if !ok || first.Storage != storage || first.Stride != 1 {
		l.errorAt(node, "persistent pointer assignment requires an address in %s", storage)
		return nil, false
	}
	for _, raw := range value.places {
		place, placeOK := raw.(ir.MemoryPlace)
		if !placeOK || place.Storage != storage || place.Stride != 1 {
			l.errorAt(node, "persistent pointer target must use one contiguous %s layout", storage)
			return nil, false
		}
	}
	address := ir.Expr(first.Index)
	if first.Offset != 0 {
		address = l.pure(resource.RuntimeFunctionAdd, node, address, ir.Const{Value: float64(first.Offset)})
	}
	return l.pure(resource.RuntimeFunctionAdd, node, address, ir.Const{Value: 1}), true
}

func (l *lowerer) selectPersistentPointerAddress(tag ir.Expr, alternatives any, storage string, node ast.Node) (ir.Expr, bool) {
	var addresses []ir.Expr
	switch values := alternatives.(type) {
	case [][]ir.Place:
		addresses = make([]ir.Expr, len(values))
		for index, places := range values {
			if len(places) == 0 {
				addresses[index] = ir.Const{}
				continue
			}
			first, ok := places[0].(ir.MemoryPlace)
			if !ok || first.Storage != storage || first.Stride != 1 {
				l.errorAt(node, "persistent pointer target must use one contiguous %s layout", storage)
				return nil, false
			}
			for _, raw := range places {
				place, ok := raw.(ir.MemoryPlace)
				if !ok || place.Storage != storage || place.Stride != 1 {
					l.errorAt(node, "persistent pointer target must use one contiguous %s layout", storage)
					return nil, false
				}
			}
			address := ir.Expr(first.Index)
			if first.Offset != 0 {
				address = l.pure(resource.RuntimeFunctionAdd, node, address, ir.Const{Value: float64(first.Offset)})
			}
			addresses[index] = l.pure(resource.RuntimeFunctionAdd, node, address, ir.Const{Value: 1})
		}
	case []ir.Expr:
		addresses = values
	default:
		l.errorAt(node, "persistent pointer target has unsupported dynamic alternatives")
		return nil, false
	}
	result := ir.Expr(ir.Const{})
	for index := len(addresses) - 1; index >= 0; index-- {
		result = l.pure(resource.RuntimeFunctionIf, node,
			l.pure(resource.RuntimeFunctionEqual, node, tag, ir.Const{Value: float64(index)}), addresses[index], result)
	}
	return result, true
}

func (l *lowerer) storePersistentPointer(destination, source lowerValue, node ast.Node) {
	if destination.persistentPointer == nil || len(destination.persistentPointer.handle.places) != 1 {
		l.errorAt(node, "persistent pointer assignment target is not writable")
		return
	}
	encoded, ok := l.persistentPointerAddress(source, destination.persistentPointer.storage, node)
	if !ok {
		return
	}
	l.store(destination.persistentPointer.handle, scalarValue(encoded, types.Typ[types.Int]), node)
}

func (l *lowerer) copyPersistentPointer(name string, source lowerValue, node ast.Node) lowerValue {
	pointer := source.persistentPointer
	if pointer == nil || len(pointer.handle.slots) != 1 {
		return source
	}
	handle := l.alloc(name+".handle", types.Typ[types.Int])
	l.store(handle, scalarValue(pointer.handle.slots[0], types.Typ[types.Int]), node)
	return lowerValue{type_: source.type_, persistentPointer: &persistentPointerValue{
		handle: handle, storage: pointer.storage, target: pointer.target, read: pointer.read, write: pointer.write,
	}}
}

func (l *lowerer) loadPersistentPointer(value lowerValue, pointer *types.Pointer, node ast.Node) lowerValue {
	persistent := value.persistentPointer
	if persistent == nil || len(persistent.handle.slots) != 1 || persistent.target == nil {
		l.errorAt(node, "persistent pointer has no target layout")
		return zeroValue(pointer.Elem())
	}
	handle := l.materialize("persistent.pointer.handle", scalarValue(persistent.handle.slots[0], types.Typ[types.Int]), node)
	l.guardWith(node, l.pure(resource.RuntimeFunctionNotEqual, node, handle.slots[0], ir.Const{}), "nil pointer dereference", true)
	base := l.pure(resource.RuntimeFunctionSubtract, node, handle.slots[0], ir.Const{Value: 1})
	return l.lowerLevelGlobalValue(node, persistent.target, persistent.storage, base, persistent.read, persistent.write)
}

func (l *lowerer) mergePointerValue(destination lowerValue, source lowerValue, node ast.Node) lowerValue {
	if source.persistentPointer != nil {
		source = l.finitePersistentPointer(source, node)
	}
	if destination.pointer == nil {
		variant := newFiniteVariant[[]ir.Place](l, "pointer.variant", node)
		index, ok := variant.add(append([]ir.Place(nil), destination.places...), samePlaces)
		if !ok {
			l.errorAt(node, "finite pointer variant exceeds 256 alternatives")
			return destination
		}
		l.store(variant.tag, scalarValue(ir.Const{Value: float64(index)}, types.Typ[types.Int]), node)
		destination.pointer = &pointerValue{finiteVariant: variant}
		destination.places = nil
	}
	if source.pointer == destination.pointer {
		return destination
	}
	addAlternative := func(places []ir.Place) int {
		for index, candidate := range destination.pointer.alternatives {
			if samePlaces(candidate, places) {
				return index
			}
		}
		index, ok := destination.pointer.add(append([]ir.Place(nil), places...), samePlaces)
		if !ok {
			l.errorAt(node, "finite pointer variant exceeds 256 alternatives")
		}
		return index
	}
	if source.pointer == nil {
		index := addAlternative(source.places)
		l.store(destination.pointer.tag, scalarValue(ir.Const{Value: float64(index)}, types.Typ[types.Int]), node)
		return destination
	}
	merge, invalid := l.newBlock(), l.newBlock()
	blocks := make([]*ir.Block, len(source.pointer.alternatives))
	cases := make([]ir.SwitchCase, len(blocks))
	for index := range blocks {
		blocks[index] = l.newBlock()
		cases[index] = ir.SwitchCase{Value: float64(index), Target: blocks[index].ID}
	}
	_ = l.builder.Switch(source.pointer.tag.slots[0], cases, invalid)
	for index, places := range source.pointer.alternatives {
		l.setCurrent(blocks[index])
		mapped := addAlternative(places)
		l.store(destination.pointer.tag, scalarValue(ir.Const{Value: float64(mapped)}, types.Typ[types.Int]), node)
		l.jump(merge)
	}
	l.setCurrent(invalid)
	_ = l.builder.MarkUnreachable()
	l.setCurrent(merge)
	return destination
}

func (l *lowerer) newPointerCell(name string, pointerType types.Type, node ast.Node) lowerValue {
	variant := newFiniteVariant[[]ir.Place](l, name+".tag", node)
	return lowerValue{type_: pointerType, pointer: &pointerValue{finiteVariant: variant}}
}

func (l *lowerer) copyPointerValue(name string, source lowerValue, node ast.Node) lowerValue {
	if source.persistentPointer != nil {
		return l.copyPersistentPointer(name, source, node)
	}
	destination := l.newPointerCell(name, source.type_, node)
	if !isStaticPointer(source) {
		l.errorAt(node, "pointer local requires a finite static target set")
		return destination
	}
	return l.mergePointerValue(destination, source, node)
}

func (l *lowerer) newInterfaceValue(name string, interfaceType types.Type, node ast.Node) lowerValue {
	variant := newFiniteVariant[lowerValue](l, name+".tag", node)
	return lowerValue{type_: interfaceType, interface_: &interfaceValue{finiteVariant: variant}}
}

func (l *lowerer) persistentInterfaceValue(node ast.Node, declaration *LevelGlobalFieldDeclaration, storage string, base ir.Expr, read, write bool) lowerValue {
	placeAt := func(offset int) ir.Place {
		index := base
		if offset != 0 {
			index = l.pure(resource.RuntimeFunctionAdd, node, base, ir.Const{Value: float64(offset)})
		}
		return l.memory(storage, index, 1, 0, read, write, node)
	}
	tagPlace, handlePlace := placeAt(0), placeAt(1)
	rawTag := lowerValue{type_: types.Typ[types.Int], slots: []ir.Expr{ir.Load{Place: tagPlace}}, places: []ir.Place{tagPlace}}
	rawHandle := lowerValue{type_: types.Typ[types.Int], slots: []ir.Expr{ir.Load{Place: handlePlace}}, places: []ir.Place{handlePlace}}
	tag := lowerValue{type_: types.Typ[types.Int], slots: []ir.Expr{l.pure(resource.RuntimeFunctionSubtract, node, rawTag.slots[0], ir.Const{Value: 1})}}
	interfaceType, _ := types.Unalias(declaration.Type).Underlying().(*types.Interface)
	if interfaceType == nil {
		l.errorAt(node, "persistent interface field has invalid type %s", declaration.Type)
		return zeroValue(declaration.Type)
	}
	type candidate struct {
		name  string
		type_ types.Type
	}
	var candidates []candidate
	seen := map[string]bool{}
	for _, pkg := range l.packages {
		for _, named := range packageNamedTypes(pkg) {
			pointer := types.NewPointer(named)
			if !types.Implements(pointer, interfaceType) {
				continue
			}
			name := types.TypeString(pointer, func(owner *types.Package) string { return owner.Path() })
			if !seen[name] {
				seen[name] = true
				candidates = append(candidates, candidate{name: name, type_: pointer})
			}
		}
	}
	sort.Slice(candidates, func(i, j int) bool { return candidates[i].name < candidates[j].name })
	alternatives := make([]lowerValue, 0, len(candidates))
	for _, candidate := range candidates {
		pointer := candidate.type_.(*types.Pointer)
		target, err := persistentLevelGlobalTypeNode(pointer.Elem(), declaration.GoName+"."+candidate.name, declaration.Kind, storage)
		if err != nil {
			continue
		}
		if len(alternatives) >= 256 {
			l.errorAt(node, "persistent interface %s exceeds 256 concrete pointer alternatives", declaration.Type)
			break
		}
		alternatives = append(alternatives, lowerValue{type_: candidate.type_, persistentPointer: &persistentPointerValue{
			handle: rawHandle, storage: storage, target: target, read: read, write: write,
		}})
	}
	return lowerValue{type_: declaration.Type, interface_: &interfaceValue{
		finiteVariant: finiteVariant[lowerValue]{alternatives: alternatives, tag: tag}, persistent: true, rawTag: rawTag,
	}}
}

func (l *lowerer) storeInterfaceValue(destination, source lowerValue, node ast.Node) lowerValue {
	if destination.interface_ == nil {
		destination = l.newInterfaceValue("interface", destination.type_, node)
	}
	variant := destination.interface_
	if variant.persistent && source.nilPointer {
		l.store(variant.rawTag, scalarValue(ir.Const{}, types.Typ[types.Int]), node)
		for _, payload := range variant.alternatives {
			if payload.persistentPointer != nil {
				l.store(payload.persistentPointer.handle, scalarValue(ir.Const{}, types.Typ[types.Int]), node)
				break
			}
		}
		return destination
	}
	storeConcrete := func(value lowerValue) {
		if value.type_ == nil || isInterfaceType(value.type_) || value.callable != nil || value.entity != nil || isContainerValue(value) || l.containsEntityView(value.type_) {
			l.errorAt(node, "finite static interface requires a non-escaping concrete runtime value")
			return
		}
		index := -1
		for candidate, payload := range variant.alternatives {
			if payload.type_ != nil && types.Identical(payload.type_, value.type_) {
				index = candidate
				break
			}
		}
		if index < 0 {
			if variant.persistent {
				l.errorAt(node, "persistent interface %s does not include concrete pointer type %s", destination.type_, value.type_)
				return
			}
			payload := l.newDescriptorCell("interface.value", value.type_, node)
			var ok bool
			index, ok = variant.add(payload, func(left, right lowerValue) bool {
				return left.type_ != nil && right.type_ != nil && types.Identical(left.type_, right.type_)
			})
			if !ok {
				l.errorAt(node, "finite static interface exceeds 256 concrete alternatives")
				return
			}
		}
		payload := variant.alternatives[index]
		l.storeDescriptor(payload, value, node)
		if variant.persistent {
			l.store(variant.rawTag, scalarValue(ir.Const{Value: float64(index + 1)}, types.Typ[types.Int]), node)
		} else {
			l.store(variant.tag, scalarValue(ir.Const{Value: float64(index)}, types.Typ[types.Int]), node)
		}
	}
	if source.interface_ == nil {
		storeConcrete(source)
		return destination
	}
	sourceVariant := source.interface_
	merge, invalid := l.newBlock(), l.newBlock()
	blocks := make([]*ir.Block, len(sourceVariant.alternatives))
	cases := make([]ir.SwitchCase, len(blocks))
	for index := range blocks {
		blocks[index] = l.newBlock()
		cases[index] = ir.SwitchCase{Value: float64(index), Target: blocks[index].ID}
	}
	_ = l.builder.Switch(sourceVariant.tag.slots[0], cases, invalid)
	for index, alternative := range sourceVariant.alternatives {
		l.setCurrent(blocks[index])
		storeConcrete(alternative)
		l.jump(merge)
	}
	l.setCurrent(invalid)
	if sourceVariant.persistent && variant.persistent {
		l.store(variant.rawTag, scalarValue(ir.Const{}, types.Typ[types.Int]), node)
		if len(variant.alternatives) != 0 && variant.alternatives[0].persistentPointer != nil {
			l.store(variant.alternatives[0].persistentPointer.handle, scalarValue(ir.Const{}, types.Typ[types.Int]), node)
		}
		l.jump(merge)
	} else {
		_ = l.builder.MarkUnreachable()
	}
	l.setCurrent(merge)
	return destination
}

type streamValue struct {
	kind       string
	valueSlots int
	base       ir.Expr
	width      int
	length     int
}

type levelGlobalValue struct {
	declaration *LevelGlobalFieldDeclaration
	storage     string
	base        ir.Expr
	read        bool
	write       bool
}

func levelGlobalChild(declaration *LevelGlobalFieldDeclaration, object *types.Var) *LevelGlobalFieldDeclaration {
	for _, child := range declaration.Fields {
		if child.Object == object {
			return child
		}
	}
	return nil
}

func (l *lowerer) attachLevelGlobalAggregate(value lowerValue, node ast.Node) lowerValue {
	global := value.levelGlobal
	if global == nil {
		return value
	}
	children := global.declaration.Fields
	if len(children) == 0 {
		children = global.declaration.Elements
	}
	if len(children) == 0 {
		return value
	}
	fields := make([]lowerValue, len(children))
	for index, child := range children {
		base := global.base
		if child.RelativeOffset != 0 {
			base = l.pure(resource.RuntimeFunctionAdd, node, base, ir.Const{Value: float64(child.RelativeOffset)})
		}
		fields[index] = l.lowerLevelGlobalValue(node, child, global.storage, base, global.read, global.write)
		fields[index] = l.attachLevelGlobalAggregate(fields[index], node)
	}
	value.aggregate = &aggregateValue{fields: fields}
	return value
}

func (l *lowerer) lowerLevelGlobalValue(node ast.Node, declaration *LevelGlobalFieldDeclaration, storage string, base ir.Expr, read, write bool) lowerValue {
	placeAt := func(offset int) ir.Place {
		index := base
		if offset != 0 {
			index = l.pure(resource.RuntimeFunctionAdd, node, base, ir.Const{Value: float64(offset)})
		}
		return l.memory(storage, index, 1, 0, read, write, node)
	}
	if declaration.PersistentKind == "pointer" {
		place := placeAt(0)
		handle := lowerValue{type_: types.Typ[types.Int], slots: []ir.Expr{ir.Load{Place: place}}, places: []ir.Place{place}}
		return lowerValue{type_: declaration.Type, persistentPointer: &persistentPointerValue{
			handle: handle, storage: storage, target: declaration.Target, read: read, write: write,
		}}
	}
	if declaration.PersistentKind == "interface" {
		return l.persistentInterfaceValue(node, declaration, storage, base, read, write)
	}
	if declaration.ContainerKind != "" {
		place := placeAt(0)
		_, key, element, _ := containerTypes(declaration.Type)
		memoryBase := l.pure(resource.RuntimeFunctionAdd, node, base, ir.Const{Value: 1})
		return lowerValue{
			type_: declaration.Type, slots: []ir.Expr{ir.Load{Place: place}}, places: []ir.Place{place},
			container: &containerValue{
				kind: declaration.ContainerKind, capacity: declaration.Capacity,
				stride: declaration.KeySize + declaration.ElementSize, keySize: declaration.KeySize,
				element: element, key: key, memoryStorage: storage, memoryBaseExpr: memoryBase,
				memoryRead: read, memoryWrite: write,
			},
		}
	}
	value := lowerValue{type_: declaration.Type, slots: make([]ir.Expr, declaration.Size), places: make([]ir.Place, declaration.Size)}
	if len(declaration.Fields) != 0 || len(declaration.Elements) != 0 {
		value.levelGlobal = &levelGlobalValue{declaration: declaration, storage: storage, base: base, read: read, write: write}
	}
	for slot := range declaration.Size {
		place := placeAt(slot)
		value.places[slot], value.slots[slot] = place, ir.Load{Place: place}
	}
	return value
}

func (l *lowerer) lowerPersistentPackagePointer(declaration *LevelGlobalFieldDeclaration, storage string, read, write bool) lowerValue {
	return lowerValue{type_: types.NewPointer(declaration.Type), persistentPointer: &persistentPointerValue{
		handle:  lowerValue{type_: types.Typ[types.Int], slots: []ir.Expr{ir.Const{Value: float64(declaration.Offset + 1)}}},
		storage: storage, target: declaration, read: read, write: write,
	}}
}

func (l *lowerer) initializePackageGlobals(node ast.Node) {
	if l.phase != "preprocess" {
		return
	}
	seen := map[*LevelGlobalFieldDeclaration]bool{}
	var visit func(*LevelGlobalFieldDeclaration)
	visit = func(declaration *LevelGlobalFieldDeclaration) {
		if declaration == nil || seen[declaration] {
			return
		}
		seen[declaration] = true
		if declaration.PersistentKind == "pointer" && declaration.InitialTarget != nil {
			if target := l.packageGlobals[declaration.InitialTarget]; target != nil {
				place := l.memory(declaration.Storage, ir.Const{}, 1, declaration.Offset, false, true, node)
				l.store(lowerValue{type_: types.Typ[types.Int], places: []ir.Place{place}, slots: []ir.Expr{ir.Load{Place: place}}}, scalarValue(ir.Const{Value: float64(target.Offset + 1)}, types.Typ[types.Int]), node)
			}
		}
		if declaration.PersistentKind == "interface" && declaration.InitialInterfaceTarget != nil {
			interfaceType, ok := types.Unalias(declaration.InitialInterfaceType).Underlying().(*types.Interface)
			if ok {
				type candidate struct {
					name  string
					type_ types.Type
				}
				var candidates []candidate
				seenTypes := map[string]bool{}
				for _, pkg := range l.packages {
					for _, named := range packageNamedTypes(pkg) {
						pointer := types.NewPointer(named)
						if !types.Implements(pointer, interfaceType) {
							continue
						}
						name := types.TypeString(pointer, func(owner *types.Package) string { return owner.Path() })
						if !seenTypes[name] {
							seenTypes[name] = true
							candidates = append(candidates, candidate{name: name, type_: pointer})
						}
					}
				}
				sort.Slice(candidates, func(i, j int) bool { return candidates[i].name < candidates[j].name })
				targetType := types.NewPointer(declaration.InitialInterfaceTarget.Type)
				tag := 0
				for _, candidate := range candidates {
					pointer := candidate.type_.(*types.Pointer)
					if _, err := persistentLevelGlobalTypeNode(pointer.Elem(), declaration.GoName+"."+candidate.name, declaration.Kind, declaration.Storage); err != nil {
						continue
					}
					tag++
					if types.Identical(candidate.type_, targetType) {
						break
					}
				}
				if tag > 0 {
					if target := l.packageGlobals[declaration.InitialInterfaceTarget]; target != nil {
						for offset, value := range []float64{float64(tag), float64(target.Offset + 1)} {
							place := l.memory(declaration.Storage, ir.Const{}, 1, declaration.Offset+offset, false, true, node)
							l.store(lowerValue{type_: types.Typ[types.Int], places: []ir.Place{place}, slots: []ir.Expr{ir.Load{Place: place}}}, scalarValue(ir.Const{Value: value}, types.Typ[types.Int]), node)
						}
					}
				}
			}
		}
		if declaration.HasInitialValue && declaration.InitialValue.Exact != nil {
			if expression, ok := constantExpr(declaration.InitialValue.Exact); ok {
				place := l.memory(declaration.Storage, ir.Const{}, 1, declaration.Offset, false, true, node)
				l.store(lowerValue{type_: declaration.Type, places: []ir.Place{place}, slots: []ir.Expr{ir.Load{Place: place}}}, lowerValue{type_: declaration.Type, slots: []ir.Expr{expression}}, node)
			}
		}
		for _, child := range declaration.Fields {
			visit(child)
		}
		for _, child := range declaration.Elements {
			visit(child)
		}
	}
	for _, declaration := range l.packageGlobals {
		visit(declaration)
	}
}

type entityReferenceValue struct {
	binding archetypeBinding
}

type variadicValue struct {
	element   types.Type
	elements  []lowerValue
	callables []*staticCallable
}

type staticCallable struct {
	finiteVariant[*staticCallable]
	identity        string
	function        *types.Func
	interfaceMethod *types.Func
	literal         *ast.FuncLit
	pkg             *packages.Package
	receiver        *callArgument
	captures        map[types.Object]lowerValue
	callables       map[types.Object]*staticCallable
	substitutions   map[*types.TypeParam]types.Type
	yield           *rangeYield
	iterator        *containerIterator
	streamIter      *streamIterator
	touchIter       *touchIterator
	resultType      types.Type
	frozenTag       bool
	intrinsic       func(*ast.CallExpr, []callArgument) lowerValue
}

type containerIterator struct {
	receiver lowerValue
	offsets  []int
	types    []types.Type
	index    bool
	desc     bool
}

type streamIterator struct {
	receiver lowerValue
	kind     string
	start    ir.Expr
	desc     bool
	frame    bool
}

type touchIterator struct {
	index     bool
	touchType types.Type
}

type rangeYield struct {
	statement *ast.RangeStmt
	owner     *lowerFrame
	active    lowerValue
	label     string
}

type callArgument struct {
	value    lowerValue
	callable *staticCallable
}

func scalarValue(expr ir.Expr, t types.Type) lowerValue {
	return lowerValue{type_: t, slots: []ir.Expr{expr}}
}

func streamLeafType(t types.Type) (*types.Named, int, bool) {
	length := 1
	outer := true
	for {
		if array, ok := types.Unalias(t).Underlying().(*types.Array); ok {
			if outer {
				length = int(array.Len())
				outer = false
			}
			t = array.Elem()
			continue
		}
		named, ok := namedType(t)
		if !ok || (typeID(named) != rootID("Stream") && typeID(named) != rootID("StreamData")) {
			return nil, 0, false
		}
		return named, length, true
	}
}

func streamHandleSlots(l *lowerer, node ast.Node, base ir.Expr, width int) []ir.Expr {
	result := make([]ir.Expr, width)
	for offset := range result {
		result[offset] = base
		if offset != 0 {
			result[offset] = l.pure(resource.RuntimeFunctionAdd, node, base, ir.Const{Value: float64(offset)})
		}
	}
	return result
}

func indexedStreamValue(l *lowerer, node ast.Node, stream *streamValue, element types.Type, offset ir.Expr) *streamValue {
	base := stream.base
	if constant, ok := offset.(ir.Const); !ok || constant.Value != 0 {
		base = l.pure(resource.RuntimeFunctionAdd, node, base, offset)
	}
	length := 1
	if array, ok := types.Unalias(element).Underlying().(*types.Array); ok {
		length = int(array.Len())
	}
	return &streamValue{kind: stream.kind, valueSlots: stream.valueSlots, base: base, width: stream.width / stream.length, length: length}
}

func lowerStreamValue(l *lowerer, node ast.Node, t types.Type, stream *streamValue) lowerValue {
	value := lowerValue{type_: t, stream: stream}
	if stream.length == 1 {
		value.slots = streamHandleSlots(l, node, stream.base, stream.width)
	}
	return value
}

type recursiveCallKey struct {
	target any
	args   string
}

func (l *lowerer) callTypeArgument(call *ast.CallExpr, index int) (types.Type, bool) {
	var expressions []ast.Expr
	switch function := call.Fun.(type) {
	case *ast.IndexExpr:
		expressions = []ast.Expr{function.Index}
	case *ast.IndexListExpr:
		expressions = function.Indices
	}
	if index < 0 || index >= len(expressions) {
		return nil, false
	}
	value := l.resolveType(l.pkg.TypesInfo.TypeOf(expressions[index]))
	return value, value != nil
}

func staticCallFingerprint(args []callArgument) (string, bool) {
	var result strings.Builder
	for _, argument := range args {
		if argument.callable != nil || isContainerValue(argument.value) || argument.value.entity != nil || argument.value.variadic != nil {
			return "", false
		}
		for _, slot := range argument.value.slots {
			constant, ok := slot.(ir.Const)
			if !ok {
				return "", false
			}
			fmt.Fprintf(&result, "%016x,", math.Float64bits(constant.Value))
		}
		result.WriteByte(';')
	}
	return result.String(), true
}

type containerValue struct {
	kind           string
	capacity       int
	stride         int
	keySize        int
	element        types.Type
	key            types.Type
	dataLocal      *ir.LocalPlace
	memoryStorage  string
	memoryBase     int
	memoryBaseExpr ir.Expr
	memoryEntity   ir.Expr
	memoryRead     bool
	memoryWrite    bool
}

type archetypeBinding struct {
	id          int
	declaration *ArchetypeDeclaration
}

type inlineCallSite struct {
	function string
	pos      ir.SourcePos
}

type lowerFrame struct {
	pkg              *packages.Package
	vars             map[types.Object]lowerValue
	callables        map[types.Object]*staticCallable
	results          map[types.Object]bool
	gotoTargets      map[*types.Label]*ir.Block
	deferredCalls    map[*ast.DeferStmt]*deferredCall
	deferOrder       []*deferredCall
	hasGoto          bool
	mutableCallables map[types.Object]bool
	mutableValues    map[types.Object]bool
	reboundValues    map[types.Object]bool
	valueReads       map[types.Object]int
	result           lowerValue
	callableResult   *staticCallable
	interfaceResult  bool
	returnBlock      *ir.Block
}

type deferredCall struct {
	active   lowerValue
	repeated bool
	call     *ast.CallExpr
	callable *staticCallable
	args     []callArgument
	pkg      *packages.Package
}

type returnRedirect struct {
	owner *lowerFrame
	depth int
}

type labeledTargets struct {
	breakTarget    *ir.Block
	continueTarget *ir.Block
}

type lowerer struct {
	mode              mode.Mode
	phase             string
	packages          map[*types.Package]*packages.Package
	pkg               *packages.Package
	builder           *ir.Builder
	frames            []*lowerFrame
	callStack         map[any]bool
	resourceIDs       map[*types.Var][]int
	streamSize        int
	configuration     *ConfigurationDeclaration
	levelGlobalFields map[*types.Var]*LevelGlobalFieldDeclaration
	packageGlobals    map[*source.StaticObject]*LevelGlobalFieldDeclaration
	packagePointers   map[*types.Var]*LevelGlobalFieldDeclaration
	currentArchetype  *ArchetypeDeclaration
	archetypeFields   map[*types.Var]*FieldDeclaration
	archetypes        map[*types.Named]archetypeBinding
	breaks            []*ir.Block
	continues         []*ir.Block
	labels            map[string]labeledTargets
	fallthroughs      []*ir.Block
	inlineCalls       []inlineCallSite
	localPool         map[string][]int
	localScopes       [][]int
	returnFrames      []returnRedirect
	typeSubstitutions []map[*types.TypeParam]types.Type
	dynamicDepth      int
	checks            RuntimeChecks
	errs              []error
}

func (l *lowerer) pushLabel(name string, breakTarget, continueTarget *ir.Block) func() {
	if name == "" {
		return func() {}
	}
	if l.labels == nil {
		l.labels = map[string]labeledTargets{}
	}
	previous, existed := l.labels[name]
	l.labels[name] = labeledTargets{breakTarget: breakTarget, continueTarget: continueTarget}
	return func() {
		if existed {
			l.labels[name] = previous
		} else {
			delete(l.labels, name)
		}
	}
}

func (l *lowerer) prepareGotoTargets(frame *lowerFrame, body *ast.BlockStmt) {
	frame.gotoTargets = map[*types.Label]*ir.Block{}
	ast.Inspect(body, func(node ast.Node) bool {
		switch node := node.(type) {
		case *ast.FuncLit:
			return false
		case *ast.LabeledStmt:
			label, _ := frame.pkg.TypesInfo.Defs[node.Label].(*types.Label)
			if label != nil {
				frame.gotoTargets[label] = l.newBlock()
			}
		}
		return true
	})
}

func (l *lowerer) gotoTarget(identifier *ast.Ident, definition bool) *ir.Block {
	for index := len(l.frames) - 1; index >= 0; index-- {
		frame := l.frames[index]
		var object types.Object
		if definition {
			object = frame.pkg.TypesInfo.Defs[identifier]
		} else {
			object = frame.pkg.TypesInfo.Uses[identifier]
		}
		label, _ := object.(*types.Label)
		if label == nil {
			continue
		}
		if target := frame.gotoTargets[label]; target != nil {
			return target
		}
	}
	return nil
}

func (l *lowerer) prepareDeferredCalls(frame *lowerFrame, body *ast.BlockStmt) {
	frame.deferredCalls = map[*ast.DeferStmt]*deferredCall{}
	var statements func([]ast.Stmt, bool)
	statements = func(items []ast.Stmt, repeated bool) {
		for _, item := range items {
			switch statement := item.(type) {
			case *ast.BlockStmt:
				statements(statement.List, repeated)
			case *ast.DeferStmt:
				call := &deferredCall{active: l.allocZeroed("defer.active", types.Typ[types.Bool], statement), repeated: repeated}
				frame.deferredCalls[statement] = call
				frame.deferOrder = append(frame.deferOrder, call)
			case *ast.IfStmt:
				statements(statement.Body.List, repeated)
				if statement.Else != nil {
					statements([]ast.Stmt{statement.Else}, repeated)
				}
			case *ast.ForStmt:
				statements(statement.Body.List, true)
			case *ast.RangeStmt:
				statements(statement.Body.List, true)
			case *ast.SwitchStmt:
				for _, clause := range statement.Body.List {
					statements(clause.(*ast.CaseClause).Body, repeated)
				}
			case *ast.TypeSwitchStmt:
				for _, clause := range statement.Body.List {
					statements(clause.(*ast.CaseClause).Body, repeated)
				}
			case *ast.SelectStmt:
				for _, clause := range statement.Body.List {
					statements(clause.(*ast.CommClause).Body, repeated)
				}
			case *ast.LabeledStmt:
				statements([]ast.Stmt{statement.Stmt}, repeated)
			case *ast.BranchStmt:
				if statement.Tok == token.GOTO {
					frame.hasGoto = true
				}
			}
		}
	}
	statements(body.List, false)
}

func (l *lowerer) prepareCallableMutability(frame *lowerFrame, body *ast.BlockStmt) {
	frame.mutableCallables = map[types.Object]bool{}
	ast.Inspect(body, func(node ast.Node) bool {
		switch node := node.(type) {
		case *ast.FuncLit:
			return false
		case *ast.AssignStmt:
			if node.Tok != token.ASSIGN {
				return true
			}
			for _, target := range node.Lhs {
				identifier, ok := target.(*ast.Ident)
				if !ok {
					continue
				}
				object := frame.pkg.TypesInfo.ObjectOf(identifier)
				if object != nil && isFunctionType(object.Type()) {
					frame.mutableCallables[object] = true
				}
			}
		}
		return true
	})
}

func (l *lowerer) prepareValueParameterUsage(frame *lowerFrame, body *ast.BlockStmt) {
	frame.mutableValues = map[types.Object]bool{}
	frame.reboundValues = map[types.Object]bool{}
	frame.valueReads = map[types.Object]int{}
	markMutable := func(expr ast.Expr) {
		identifier := assignedRootIdentifier(expr)
		if identifier == nil {
			return
		}
		if object, ok := frame.pkg.TypesInfo.ObjectOf(identifier).(*types.Var); ok {
			frame.mutableValues[object] = true
		}
	}
	ast.Inspect(body, func(node ast.Node) bool {
		switch node := node.(type) {
		case *ast.FuncLit:
			return false
		case *ast.AssignStmt:
			for _, target := range node.Lhs {
				markMutable(target)
				if node.Tok == token.ASSIGN {
					if identifier, ok := target.(*ast.Ident); ok {
						if object := frame.pkg.TypesInfo.ObjectOf(identifier); object != nil {
							frame.reboundValues[object] = true
						}
					}
				}
			}
		case *ast.IncDecStmt:
			markMutable(node.X)
		case *ast.UnaryExpr:
			if node.Op == token.AND {
				markMutable(node.X)
			}
		case *ast.Ident:
			if object, ok := frame.pkg.TypesInfo.Uses[node].(*types.Var); ok {
				frame.valueReads[object]++
			}
		}
		return true
	})
}

func assignedRootIdentifier(expr ast.Expr) *ast.Ident {
	switch value := expr.(type) {
	case *ast.Ident:
		return value
	case *ast.ParenExpr:
		return assignedRootIdentifier(value.X)
	case *ast.SelectorExpr:
		return assignedRootIdentifier(value.X)
	case *ast.IndexExpr:
		return assignedRootIdentifier(value.X)
	case *ast.IndexListExpr:
		return assignedRootIdentifier(value.X)
	case *ast.StarExpr:
		return assignedRootIdentifier(value.X)
	default:
		return nil
	}
}

func (l *lowerer) callableBinding(object types.Object, name string, callable *staticCallable, node ast.Node) *staticCallable {
	frame := l.frames[len(l.frames)-1]
	if callable != nil && (callable.tag.type_ == nil || callable.frozenTag) && !frame.mutableCallables[object] {
		return callable
	}
	return l.newCallableCell(name, callable, node)
}

func (l *lowerer) deferredCall(statement *ast.DeferStmt) (*lowerFrame, *deferredCall) {
	for index := len(l.frames) - 1; index >= 0; index-- {
		frame := l.frames[index]
		if call := frame.deferredCalls[statement]; call != nil {
			return frame, call
		}
	}
	return nil, nil
}

func (l *lowerer) runDeferredCalls(frame *lowerFrame) {
	for index := len(frame.deferOrder) - 1; index >= 0; index-- {
		deferred := frame.deferOrder[index]
		if deferred.callable == nil {
			continue
		}
		invoke, next := l.newBlock(), l.newBlock()
		_ = l.builder.Branch(deferred.active.slots[0], invoke, next)
		l.setCurrent(invoke)
		previous := l.pkg
		l.pkg = deferred.pkg
		value := l.inlineStaticCallable(deferred.call, deferred.callable, deferred.args)
		l.pkg = previous
		for _, expression := range value.slots {
			if call, ok := expression.(ir.RuntimeCall); ok && !call.Pure {
				_ = l.builder.Eval(expression)
			}
		}
		l.jump(next)
		l.setCurrent(next)
	}
}

func sourcePos(pkg *packages.Package, pos token.Pos) ir.SourcePos {
	p := pkg.Fset.Position(pos)
	return ir.SourcePos{File: canonicalSourceFile(pkg, p.Filename), Line: p.Line, Column: p.Column}
}

func canonicalSourceFile(pkg *packages.Package, filename string) string {
	file := filepath.ToSlash(filename)
	if pkg == nil || pkg.Module == nil || pkg.Module.Dir == "" {
		return file
	}
	relative, err := filepath.Rel(pkg.Module.Dir, filename)
	if err != nil || relative == "." || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return file
	}
	return filepath.ToSlash(relative)
}

func (l *lowerer) errorAt(node ast.Node, format string, args ...any) {
	p := l.pkg.Fset.Position(node.Pos())
	message := fmt.Sprintf("%s:%d:%d: callback %s", canonicalSourceFile(l.pkg, p.Filename), p.Line, p.Column, l.builder.Function().Name)
	for i := len(l.inlineCalls) - 1; i >= 0; i-- {
		call := l.inlineCalls[i]
		message += fmt.Sprintf(": inlined from %s:%d:%d (%s)", call.pos.File, call.pos.Line, call.pos.Column, call.function)
	}
	l.errs = append(l.errs, fmt.Errorf("%s: %s", message, fmt.Sprintf(format, args...)))
}

func (l *lowerer) newBlock() *ir.Block        { return l.builder.NewBlock() }
func (l *lowerer) current() *ir.Block         { return l.builder.Current() }
func (l *lowerer) setCurrent(block *ir.Block) { _ = l.builder.SetCurrent(block) }

func (l *lowerer) jump(target *ir.Block) {
	_ = l.builder.JumpIfOpen(target)
}

func (l *lowerer) dynamic(fn func()) {
	l.dynamicDepth++
	defer func() { l.dynamicDepth-- }()
	fn()
}

func irTypeOf(t types.Type) ir.Type {
	if t == nil {
		return ir.Type{Name: "void"}
	}
	t = types.Unalias(t)
	if named, ok := namedType(t); ok && (typeID(named) == rootID("Stream") || typeID(named) == rootID("StreamData")) && named.TypeArgs().Len() == 1 {
		value := irTypeOf(named.TypeArgs().At(0))
		return ir.Type{Name: t.String(), Slots: value.Slots, Fields: value.Fields}
	}
	if pointer, ok := t.(*types.Pointer); ok {
		return irTypeOf(pointer.Elem())
	}
	if tuple, ok := t.(*types.Tuple); ok {
		out := ir.Type{Name: t.String()}
		for i := 0; i < tuple.Len(); i++ {
			item := irTypeOf(tuple.At(i).Type())
			out.Fields = append(out.Fields, ir.Field{Name: tuple.At(i).Name(), Offset: out.Slots, Type: item})
			out.Slots += item.Slots
		}
		return out
	}
	if basic, ok := t.Underlying().(*types.Basic); ok {
		return ir.Type{Name: basic.Name(), Slots: 1}
	}
	if array, ok := t.Underlying().(*types.Array); ok {
		e := irTypeOf(array.Elem())
		return ir.Type{Name: t.String(), Slots: int(array.Len()) * e.Slots}
	}
	if st, ok := t.Underlying().(*types.Struct); ok {
		if id := typeID(t); id == rootID("Sprite") || id == rootID("Clip") || id == rootID("Effect") || id == rootID("Text") || id == rootID("Icon") || id == rootID("Bucket") {
			return ir.Type{Name: t.String(), Slots: 1}
		}
		out := ir.Type{Name: t.String()}
		for i := 0; i < st.NumFields(); i++ {
			ft := irTypeOf(st.Field(i).Type())
			out.Fields = append(out.Fields, ir.Field{Name: st.Field(i).Name(), Offset: out.Slots, Type: ft})
			out.Slots += ft.Slots
		}
		return out
	}
	return ir.Type{Name: t.String(), Slots: 1}
}

func (l *lowerer) runtimeTypeOf(t types.Type) ir.Type {
	var build func(types.Type, bool) ir.Type
	build = func(value types.Type, nested bool) ir.Type {
		value = l.resolveType(value)
		value = types.Unalias(value)
		if _, pointer := value.(*types.Pointer); pointer {
			if _, exists := l.entityBinding(value); exists {
				return entityViewType(value)
			}
			if nested {
				return ir.Type{Name: value.String(), Slots: 1}
			}
		}
		if nested && isInterfaceType(value) {
			return ir.Type{Name: value.String(), Slots: 1}
		}
		if array, ok := value.Underlying().(*types.Array); ok {
			element := build(array.Elem(), true)
			return ir.Type{Name: value.String(), Slots: int(array.Len()) * element.Slots}
		}
		if structure, ok := value.Underlying().(*types.Struct); ok && (l.containsEntityView(value) || l.containsAggregateDescriptor(value)) {
			result := ir.Type{Name: value.String()}
			for index := 0; index < structure.NumFields(); index++ {
				fieldType := build(structure.Field(index).Type(), true)
				result.Fields = append(result.Fields, ir.Field{Name: structure.Field(index).Name(), Offset: result.Slots, Type: fieldType})
				result.Slots += fieldType.Slots
			}
			return result
		}
		return irTypeOf(value)
	}
	return build(t, false)
}

func (l *lowerer) resolveType(t types.Type) types.Type {
	if t == nil {
		return nil
	}
	value := types.Unalias(t)
	if parameter, ok := value.(*types.TypeParam); ok {
		for index := len(l.typeSubstitutions) - 1; index >= 0; index-- {
			if replacement := l.typeSubstitutions[index][parameter]; replacement != nil {
				return l.resolveType(replacement)
			}
		}
		return value
	}
	switch typed := value.(type) {
	case *types.Pointer:
		element := l.resolveType(typed.Elem())
		if types.Identical(element, typed.Elem()) {
			return typed
		}
		return types.NewPointer(element)
	case *types.Array:
		element := l.resolveType(typed.Elem())
		if types.Identical(element, typed.Elem()) {
			return typed
		}
		return types.NewArray(element, typed.Len())
	case *types.Slice:
		element := l.resolveType(typed.Elem())
		if types.Identical(element, typed.Elem()) {
			return typed
		}
		return types.NewSlice(element)
	case *types.Map:
		key, element := l.resolveType(typed.Key()), l.resolveType(typed.Elem())
		if types.Identical(key, typed.Key()) && types.Identical(element, typed.Elem()) {
			return typed
		}
		return types.NewMap(key, element)
	case *types.Chan:
		element := l.resolveType(typed.Elem())
		if types.Identical(element, typed.Elem()) {
			return typed
		}
		return types.NewChan(typed.Dir(), element)
	case *types.Named:
		if typed.TypeArgs().Len() == 0 {
			return typed
		}
		arguments := make([]types.Type, typed.TypeArgs().Len())
		changed := false
		for index := range arguments {
			arguments[index] = l.resolveType(typed.TypeArgs().At(index))
			changed = changed || !types.Identical(arguments[index], typed.TypeArgs().At(index))
		}
		if !changed {
			return typed
		}
		instance, err := types.Instantiate(nil, typed.Origin(), arguments, true)
		if err != nil {
			return typed
		}
		return instance
	case *types.Struct:
		fields := make([]*types.Var, typed.NumFields())
		tags := make([]string, typed.NumFields())
		changed := false
		for index := range fields {
			field := typed.Field(index)
			fieldType := l.resolveType(field.Type())
			fields[index] = types.NewField(field.Pos(), field.Pkg(), field.Name(), fieldType, field.Embedded())
			tags[index] = typed.Tag(index)
			changed = changed || !types.Identical(fieldType, field.Type())
		}
		if changed {
			return types.NewStruct(fields, tags)
		}
	}
	return value
}

func (l *lowerer) collectTypeSubstitutions(pattern, actual types.Type, substitutions map[*types.TypeParam]types.Type) {
	if pattern == nil || actual == nil {
		return
	}
	pattern = types.Unalias(pattern)
	actual = l.resolveType(actual)
	actual = types.Unalias(actual)
	if parameter, ok := pattern.(*types.TypeParam); ok {
		if existing := substitutions[parameter]; existing == nil || types.Identical(existing, actual) {
			substitutions[parameter] = actual
		}
		return
	}
	switch pattern := pattern.(type) {
	case *types.Pointer:
		if actual, ok := actual.(*types.Pointer); ok {
			l.collectTypeSubstitutions(pattern.Elem(), actual.Elem(), substitutions)
		}
	case *types.Array:
		if actual, ok := actual.Underlying().(*types.Array); ok && pattern.Len() == actual.Len() {
			l.collectTypeSubstitutions(pattern.Elem(), actual.Elem(), substitutions)
		}
	case *types.Slice:
		if actual, ok := actual.Underlying().(*types.Slice); ok {
			l.collectTypeSubstitutions(pattern.Elem(), actual.Elem(), substitutions)
		}
	case *types.Map:
		if actual, ok := actual.Underlying().(*types.Map); ok {
			l.collectTypeSubstitutions(pattern.Key(), actual.Key(), substitutions)
			l.collectTypeSubstitutions(pattern.Elem(), actual.Elem(), substitutions)
		}
	case *types.Named:
		actualNamed, ok := actual.(*types.Named)
		if !ok || pattern.Origin() != actualNamed.Origin() || pattern.TypeArgs().Len() != actualNamed.TypeArgs().Len() {
			return
		}
		for index := 0; index < pattern.TypeArgs().Len(); index++ {
			l.collectTypeSubstitutions(pattern.TypeArgs().At(index), actualNamed.TypeArgs().At(index), substitutions)
		}
	case *types.Struct:
		actualStruct, ok := actual.Underlying().(*types.Struct)
		if !ok || pattern.NumFields() != actualStruct.NumFields() {
			return
		}
		for index := 0; index < pattern.NumFields(); index++ {
			l.collectTypeSubstitutions(pattern.Field(index).Type(), actualStruct.Field(index).Type(), substitutions)
		}
	}
}

func (l *lowerer) inferCallTypeSubstitutions(signature, callSignature *types.Signature, args []callArgument) map[*types.TypeParam]types.Type {
	substitutions := map[*types.TypeParam]types.Type{}
	argument := 0
	if signature.Recv() != nil && len(args) != 0 {
		l.collectTypeSubstitutions(signature.Recv().Type(), args[0].value.type_, substitutions)
		argument++
	}
	for index := 0; index < signature.Params().Len() && argument < len(args); index++ {
		parameter := signature.Params().At(index).Type()
		if signature.Variadic() && index == signature.Params().Len()-1 {
			slice, _ := types.Unalias(parameter).Underlying().(*types.Slice)
			for ; argument < len(args); argument++ {
				if args[argument].value.type_ != nil {
					l.collectTypeSubstitutions(slice.Elem(), args[argument].value.type_, substitutions)
				}
			}
			break
		}
		if args[argument].value.type_ != nil {
			l.collectTypeSubstitutions(parameter, args[argument].value.type_, substitutions)
		}
		argument++
	}
	if callSignature != nil {
		offset := 0
		if signature.Recv() != nil && callSignature.Params().Len() == signature.Params().Len()+1 {
			l.collectTypeSubstitutions(signature.Recv().Type(), callSignature.Params().At(0).Type(), substitutions)
			offset = 1
		}
		for index := 0; index < signature.Params().Len() && index+offset < callSignature.Params().Len(); index++ {
			l.collectTypeSubstitutions(signature.Params().At(index).Type(), callSignature.Params().At(index+offset).Type(), substitutions)
		}
		for index := 0; index < signature.Results().Len() && index < callSignature.Results().Len(); index++ {
			l.collectTypeSubstitutions(signature.Results().At(index).Type(), callSignature.Results().At(index).Type(), substitutions)
		}
	}
	return substitutions
}

func callableInstance(pkg *packages.Package, expression ast.Expr) (types.Instance, bool) {
	switch expression := expression.(type) {
	case *ast.ParenExpr:
		return callableInstance(pkg, expression.X)
	case *ast.IndexExpr:
		return callableInstance(pkg, expression.X)
	case *ast.IndexListExpr:
		return callableInstance(pkg, expression.X)
	case *ast.Ident:
		instance, ok := pkg.TypesInfo.Instances[expression]
		return instance, ok
	case *ast.SelectorExpr:
		instance, ok := pkg.TypesInfo.Instances[expression.Sel]
		return instance, ok
	default:
		return types.Instance{}, false
	}
}

func (l *lowerer) containsEntityView(t types.Type) bool {
	var inspect func(types.Type, map[types.Type]bool) bool
	inspect = func(value types.Type, visiting map[types.Type]bool) bool {
		if value == nil {
			return false
		}
		value = types.Unalias(value)
		if _, pointer := value.(*types.Pointer); pointer {
			_, exists := l.entityBinding(value)
			return exists
		}
		if visiting[value] {
			return false
		}
		visiting[value] = true
		defer delete(visiting, value)
		switch underlying := value.Underlying().(type) {
		case *types.Array:
			return inspect(underlying.Elem(), visiting)
		case *types.Struct:
			for index := 0; index < underlying.NumFields(); index++ {
				if inspect(underlying.Field(index).Type(), visiting) {
					return true
				}
			}
		}
		return false
	}
	return inspect(t, map[types.Type]bool{})
}

func (l *lowerer) containsAggregateDescriptor(t types.Type) bool {
	if t == nil {
		return false
	}
	root := types.Unalias(l.resolveType(t))
	if _, pointer := root.(*types.Pointer); pointer || isContainerType(root) || isInterfaceType(root) {
		return false
	}
	var inspect func(types.Type, map[types.Type]bool) bool
	inspect = func(value types.Type, visiting map[types.Type]bool) bool {
		if value == nil {
			return false
		}
		value = types.Unalias(value)
		if _, pointer := value.(*types.Pointer); pointer {
			_, entity := l.entityBinding(value)
			return !entity
		}
		if isContainerType(value) {
			return true
		}
		if isInterfaceType(value) {
			return true
		}
		if visiting[value] {
			return false
		}
		visiting[value] = true
		defer delete(visiting, value)
		switch underlying := value.Underlying().(type) {
		case *types.Array:
			return inspect(underlying.Elem(), visiting)
		case *types.Struct:
			for index := 0; index < underlying.NumFields(); index++ {
				if inspect(underlying.Field(index).Type(), visiting) {
					return true
				}
			}
		}
		return false
	}
	return inspect(t, map[types.Type]bool{})
}

func (l *lowerer) attachAggregate(value lowerValue, node ast.Node) lowerValue {
	return l.attachAggregateShape(value, node, false)
}

func (l *lowerer) attachAggregateShape(value lowerValue, node ast.Node, full bool) lowerValue {
	if !full && !l.containsAggregateDescriptor(value.type_) {
		return value
	}
	var itemTypes []types.Type
	switch underlying := types.Unalias(value.type_).Underlying().(type) {
	case *types.Struct:
		itemTypes = make([]types.Type, underlying.NumFields())
		for index := range itemTypes {
			itemTypes[index] = underlying.Field(index).Type()
		}
	case *types.Array:
		itemTypes = make([]types.Type, int(underlying.Len()))
		for index := range itemTypes {
			itemTypes[index] = underlying.Elem()
		}
	default:
		return value
	}
	fields := make([]lowerValue, len(itemTypes))
	offset := 0
	for index, itemType := range itemTypes {
		fieldType := l.resolveType(itemType)
		size := l.runtimeTypeOf(fieldType).Slots
		if isPointerType(fieldType) {
			if _, entity := l.entityBinding(fieldType); !entity {
				size = 1
			}
		}
		field := lowerValue{type_: fieldType, slots: value.slots[offset : offset+size]}
		if len(value.places) != 0 {
			field.places = value.places[offset : offset+size]
		}
		if binding, exists := l.entityBinding(fieldType); exists {
			field.entity = &entityReferenceValue{binding: binding}
		} else {
			switch {
			case isPointerType(fieldType):
				if l.isAggregatePointerType(fieldType) {
					variant := finiteVariant[lowerValue]{alternatives: []lowerValue{{type_: fieldType, nilPointer: true}}, tag: field}
					field.slots = nil
					field.places = nil
					field.aggregatePointer = &variant
					l.store(variant.tag, scalarValue(ir.Const{}, types.Typ[types.Int]), node)
				} else {
					variant := finiteVariant[[]ir.Place]{alternatives: [][]ir.Place{nil}, tag: field}
					field.slots = nil
					field.places = nil
					field.pointer = &pointerValue{finiteVariant: variant}
					l.store(variant.tag, scalarValue(ir.Const{}, types.Typ[types.Int]), node)
				}
			case isContainerType(fieldType):
				variant := finiteVariant[lowerValue]{tag: field}
				field.slots = nil
				field.places = nil
				field.containerVariant = &variant
				l.store(variant.tag, scalarValue(ir.Const{Value: -1}, types.Typ[types.Int]), node)
			case isInterfaceType(fieldType):
				variant := finiteVariant[lowerValue]{tag: field}
				field.slots = nil
				field.places = nil
				field.interface_ = &interfaceValue{finiteVariant: variant}
				l.store(variant.tag, scalarValue(ir.Const{Value: -1}, types.Typ[types.Int]), node)
			default:
				field = l.attachAggregateShape(field, node, full)
			}
		}
		fields[index] = field
		offset += size
	}
	value.aggregate = &aggregateValue{fields: fields}
	return value
}

func (l *lowerer) newAggregateCell(name string, t types.Type, node ast.Node) lowerValue {
	return l.attachAggregate(l.allocZeroed(name, t, node), node)
}

func (l *lowerer) newFullAggregateCell(name string, t types.Type, node ast.Node) lowerValue {
	return l.attachAggregateShape(l.allocZeroed(name, t, node), node, true)
}

func (l *lowerer) storeAggregate(destination, source lowerValue, node ast.Node) {
	if destination.aggregate == nil || source.aggregate == nil {
		if destination.type_ != nil && source.type_ != nil && types.Identical(destination.type_, source.type_) && len(destination.slots) == len(source.slots) {
			l.store(destination, source, node)
			return
		}
		l.errorAt(node, "aggregate descriptor assignment requires identical struct types")
		return
	}
	if len(destination.aggregate.fields) != len(source.aggregate.fields) {
		l.errorAt(node, "aggregate descriptor assignment requires identical struct types")
		return
	}
	for index := range destination.aggregate.fields {
		dst, src := destination.aggregate.fields[index], source.aggregate.fields[index]
		switch {
		case dst.aggregatePointer != nil || isAggregatePointerValue(src):
			if dst.aggregatePointer == nil || (!isAggregatePointerValue(src) && !src.nilPointer) {
				l.errorAt(node, "aggregate pointer field assignment requires an aggregate address or nil")
				continue
			}
			l.mergeAggregatePointerValue(dst, src, node)
		case dst.aggregate != nil || src.aggregate != nil:
			l.storeAggregate(dst, src, node)
		case dst.entity != nil || src.entity != nil:
			l.storeEntityView(dst, src, node)
		case dst.interface_ != nil || src.interface_ != nil:
			if dst.interface_ == nil {
				l.errorAt(node, "aggregate interface field assignment requires an interface destination")
				continue
			}
			l.storeInterfaceValue(dst, src, node)
		case dst.pointer != nil || isStaticPointer(src):
			if dst.pointer == nil || !isStaticPointer(src) {
				l.errorAt(node, "aggregate pointer field assignment requires a finite pointer source")
				continue
			}
			l.mergePointerValue(dst, src, node)
		case dst.containerVariant != nil || isContainerValue(src):
			if dst.containerVariant == nil || !isContainerValue(src) {
				l.errorAt(node, "aggregate container field assignment requires a container source")
				continue
			}
			l.mergeContainerValue(dst, src, node)
		default:
			l.store(dst, src, node)
		}
	}
}

func (l *lowerer) copyAggregate(name string, source lowerValue, node ast.Node) lowerValue {
	destination := l.newAggregateCell(name, source.type_, node)
	l.storeAggregate(destination, source, node)
	return destination
}

func (l *lowerer) isAggregatePointerType(t types.Type) bool {
	pointer, ok := types.Unalias(l.resolveType(t)).(*types.Pointer)
	return ok && l.containsAggregateDescriptor(pointer.Elem())
}

func (l *lowerer) newHelperResultCell(name string, t types.Type, node ast.Node) lowerValue {
	t = l.resolveType(t)
	if isFunctionType(t) || isInterfaceType(t) {
		if isInterfaceType(t) {
			return l.newInterfaceValue(name+".interface", t, node)
		}
		return lowerValue{type_: t}
	}
	if _, callableArray := callableArrayType(t); callableArray {
		return l.newCallableArray(name+".callable", t, node)
	}
	if binding, entityView := l.entityBinding(t); entityView {
		return l.newEntityViewLocal(name+".entity", t, binding)
	}
	if l.isAggregatePointerType(t) {
		return l.newAggregatePointerCell(name+".aggregate.pointer", t, node)
	}
	if isPointerType(t) {
		return l.newPointerCell(name+".pointer", t, node)
	}
	if isContainerType(t) {
		return l.newContainerCell(name+".container", t, node)
	}
	if l.containsAggregateDescriptor(t) {
		return l.newAggregateCell(name+".aggregate", t, node)
	}
	return l.allocZeroed(name, t, node)
}

func (l *lowerer) newDescriptorMultiResult(name string, t types.Type, node ast.Node) (lowerValue, bool) {
	tuple, ok := types.Unalias(l.resolveType(t)).(*types.Tuple)
	if !ok {
		return lowerValue{}, false
	}
	needsDescriptor := false
	for index := 0; index < tuple.Len(); index++ {
		if l.isAggregatePointerType(tuple.At(index).Type()) {
			needsDescriptor = true
			break
		}
	}
	if !needsDescriptor {
		return lowerValue{}, false
	}
	result := lowerValue{type_: t, multi: make([]lowerValue, tuple.Len())}
	for index := range result.multi {
		result.multi[index] = l.newHelperResultCell(fmt.Sprintf("%s.%d", name, index), tuple.At(index).Type(), node)
	}
	return result, true
}

func isAggregatePointerValue(value lowerValue) bool {
	return value.persistentPointer != nil && levelGlobalDeclarationHasDescriptors(value.persistentPointer.target) || value.aggregatePointer != nil || value.aggregate != nil && isPointerType(value.type_)
}

func sameAggregatePointerTarget(left, right lowerValue) bool {
	if left.nilPointer || right.nilPointer {
		return left.nilPointer && right.nilPointer
	}
	if left.persistentPointer != nil || right.persistentPointer != nil {
		return left.persistentPointer != nil && right.persistentPointer != nil &&
			left.persistentPointer.storage == right.persistentPointer.storage &&
			reflect.DeepEqual(left.persistentPointer.handle.slots, right.persistentPointer.handle.slots) &&
			reflect.DeepEqual(left.persistentPointer.handle.places, right.persistentPointer.handle.places)
	}
	return left.aggregate != nil && left.aggregate == right.aggregate
}

func (l *lowerer) newAggregatePointerCell(name string, pointerType types.Type, node ast.Node) lowerValue {
	variant := newFiniteVariant[lowerValue](l, name+".aggregate.pointer", node)
	return lowerValue{type_: pointerType, aggregatePointer: &variant}
}

func (l *lowerer) mergeAggregatePointerValue(destination, source lowerValue, node ast.Node) lowerValue {
	if destination.aggregatePointer == nil {
		cell := l.newAggregatePointerCell("aggregate.pointer.variant", destination.type_, node)
		destination.aggregatePointer = cell.aggregatePointer
		if destination.aggregate != nil || destination.nilPointer {
			index, ok := destination.aggregatePointer.add(destination, sameAggregatePointerTarget)
			if !ok {
				l.errorAt(node, "finite aggregate pointer variant exceeds 256 alternatives")
				return destination
			}
			l.store(destination.aggregatePointer.tag, scalarValue(ir.Const{Value: float64(index)}, types.Typ[types.Int]), node)
		}
		destination.aggregate = nil
		destination.nilPointer = false
		destination.slots = nil
		destination.places = nil
	}
	variant := destination.aggregatePointer
	addAlternative := func(value lowerValue) int {
		index, ok := variant.add(value, sameAggregatePointerTarget)
		if !ok {
			l.errorAt(node, "finite aggregate pointer variant exceeds 256 alternatives")
			return -1
		}
		return index
	}
	if source.aggregatePointer == nil {
		if source.aggregate == nil && source.persistentPointer == nil && !source.nilPointer {
			l.errorAt(node, "aggregate pointer assignment requires an aggregate address or nil")
			return destination
		}
		index := addAlternative(source)
		if index >= 0 {
			l.store(variant.tag, scalarValue(ir.Const{Value: float64(index)}, types.Typ[types.Int]), node)
		}
		return destination
	}
	merge, invalid := l.newBlock(), l.newBlock()
	blocks := make([]*ir.Block, len(source.aggregatePointer.alternatives))
	cases := make([]ir.SwitchCase, len(blocks))
	for index := range blocks {
		blocks[index] = l.newBlock()
		cases[index] = ir.SwitchCase{Value: float64(index), Target: blocks[index].ID}
	}
	_ = l.builder.Switch(source.aggregatePointer.tag.slots[0], cases, invalid)
	for index, alternative := range source.aggregatePointer.alternatives {
		l.setCurrent(blocks[index])
		mapped := addAlternative(alternative)
		if mapped >= 0 {
			l.store(variant.tag, scalarValue(ir.Const{Value: float64(mapped)}, types.Typ[types.Int]), node)
		}
		l.jump(merge)
	}
	l.setCurrent(invalid)
	_ = l.builder.MarkUnreachable()
	l.setCurrent(merge)
	return destination
}

func (l *lowerer) copyAggregatePointerValue(name string, source lowerValue, node ast.Node) lowerValue {
	destination := l.newAggregatePointerCell(name, source.type_, node)
	return l.mergeAggregatePointerValue(destination, source, node)
}

func (l *lowerer) newDescriptorCell(name string, t types.Type, node ast.Node) lowerValue {
	t = l.resolveType(t)
	if l.containsAggregateDescriptor(t) {
		return l.newAggregateCell(name, t, node)
	}
	if binding, exists := l.entityBinding(t); exists {
		return l.newEntityViewLocal(name, t, binding)
	}
	if isInterfaceType(t) {
		return l.newInterfaceValue(name, t, node)
	}
	if isPointerType(t) {
		if l.isAggregatePointerType(t) {
			return l.newAggregatePointerCell(name, t, node)
		}
		return l.newPointerCell(name, t, node)
	}
	if isContainerType(t) {
		return l.newContainerCell(name, t, node)
	}
	return l.allocZeroed(name, t, node)
}

func (l *lowerer) storeDescriptor(destination, source lowerValue, node ast.Node) {
	switch {
	case destination.persistentPointer != nil:
		l.storePersistentPointer(destination, source, node)
	case destination.aggregatePointer != nil || isAggregatePointerValue(source):
		if destination.aggregatePointer == nil || (!isAggregatePointerValue(source) && !source.nilPointer) {
			l.errorAt(node, "descriptor aggregate pointer assignment requires an aggregate address or nil")
			return
		}
		l.mergeAggregatePointerValue(destination, source, node)
	case destination.aggregate != nil || source.aggregate != nil:
		l.storeAggregate(destination, source, node)
	case destination.entity != nil || source.entity != nil:
		l.storeEntityView(destination, source, node)
	case destination.interface_ != nil || source.interface_ != nil:
		if destination.interface_ == nil {
			l.errorAt(node, "descriptor interface assignment requires an interface destination")
			return
		}
		l.storeInterfaceValue(destination, source, node)
	case destination.pointer != nil || isStaticPointer(source):
		if destination.pointer == nil || !isStaticPointer(source) {
			l.errorAt(node, "descriptor pointer assignment requires a finite pointer source")
			return
		}
		l.mergePointerValue(destination, source, node)
	case destination.containerVariant != nil || isContainerValue(source):
		if destination.containerVariant == nil || !isContainerValue(source) {
			l.errorAt(node, "descriptor container assignment requires a container source")
			return
		}
		l.mergeContainerValue(destination, source, node)
	default:
		l.store(destination, source, node)
	}
}

func aggregatePath(value lowerValue, path []int) (lowerValue, bool) {
	for _, index := range path {
		if value.aggregate == nil || index < 0 || index >= len(value.aggregate.fields) {
			return lowerValue{}, false
		}
		value = value.aggregate.fields[index]
	}
	return value, true
}

func (l *lowerer) storeAggregateIndex(indexed *aggregateIndexValue, source lowerValue, node ast.Node) {
	if indexed == nil || indexed.array.aggregate == nil || len(indexed.index.slots) != 1 {
		l.errorAt(node, "dynamic descriptor array assignment has no indexed backing")
		return
	}
	merge, invalid := l.newBlock(), l.newBlock()
	blocks := make([]*ir.Block, len(indexed.array.aggregate.fields))
	cases := make([]ir.SwitchCase, len(blocks))
	for position := range blocks {
		blocks[position] = l.newBlock()
		cases[position] = ir.SwitchCase{Value: float64(position), Target: blocks[position].ID}
	}
	_ = l.builder.Switch(indexed.index.slots[0], cases, invalid)
	for position, block := range blocks {
		l.setCurrent(block)
		destination, ok := aggregatePath(indexed.array.aggregate.fields[position], indexed.path)
		if !ok {
			l.errorAt(node, "dynamic descriptor array assignment path is invalid")
		} else {
			l.storeDescriptor(destination, source, node)
		}
		l.jump(merge)
	}
	l.setCurrent(invalid)
	_ = l.builder.MarkUnreachable()
	l.setCurrent(merge)
}

func (l *lowerer) indexAggregateArray(base lowerValue, array *types.Array, index lowerValue, node, indexNode ast.Node) lowerValue {
	index = l.materialize("aggregate.array.index", index, indexNode)
	condition := l.pure(resource.RuntimeFunctionAnd, node,
		l.pure(resource.RuntimeFunctionGreaterOr, node, index.slots[0], ir.Const{}),
		l.pure(resource.RuntimeFunctionLess, node, index.slots[0], ir.Const{Value: float64(array.Len())}))
	l.guard(node, condition)
	result := l.newDescriptorCell("aggregate.array.value", array.Elem(), node)
	if base.aggregateLoad != nil {
		result = l.newFullAggregateCell("aggregate.array.value", array.Elem(), node)
	}
	merge, invalid := l.newBlock(), l.newBlock()
	blocks := make([]*ir.Block, len(base.aggregate.fields))
	cases := make([]ir.SwitchCase, len(blocks))
	for position := range blocks {
		blocks[position] = l.newBlock()
		cases[position] = ir.SwitchCase{Value: float64(position), Target: blocks[position].ID}
	}
	_ = l.builder.Switch(index.slots[0], cases, invalid)
	for position, block := range blocks {
		l.setCurrent(block)
		l.storeDescriptor(result, base.aggregate.fields[position], node)
		l.jump(merge)
	}
	l.setCurrent(invalid)
	_ = l.builder.MarkUnreachable()
	l.setCurrent(merge)
	result.aggregateIndex = &aggregateIndexValue{array: base, index: index}
	return result
}

func (l *lowerer) loadAggregatePointer(value lowerValue, pointer *types.Pointer, node ast.Node) lowerValue {
	if value.aggregatePointer == nil || len(value.aggregatePointer.tag.slots) != 1 {
		l.errorAt(node, "dynamic aggregate pointer has no target tag")
		return zeroValue(pointer.Elem())
	}
	if len(value.aggregatePointer.alternatives) == 1 && value.aggregatePointer.alternatives[0].persistentPointer != nil {
		l.guardWith(node, l.pure(resource.RuntimeFunctionEqual, node, value.aggregatePointer.tag.slots[0], ir.Const{}), "invalid aggregate pointer target", true)
		loaded := l.loadPersistentPointer(value.aggregatePointer.alternatives[0], pointer, node)
		return l.attachLevelGlobalAggregate(loaded, node)
	}
	result := l.newFullAggregateCell("aggregate.pointer.load", pointer.Elem(), node)
	merge, invalid := l.newBlock(), l.newBlock()
	blocks := make([]*ir.Block, len(value.aggregatePointer.alternatives))
	cases := make([]ir.SwitchCase, len(blocks))
	for index := range blocks {
		blocks[index] = l.newBlock()
		cases[index] = ir.SwitchCase{Value: float64(index), Target: blocks[index].ID}
	}
	_ = l.builder.Switch(value.aggregatePointer.tag.slots[0], cases, invalid)
	for index, target := range value.aggregatePointer.alternatives {
		l.setCurrent(blocks[index])
		if target.nilPointer {
			l.terminateRuntime(node, "nil pointer dereference")
			continue
		}
		if target.persistentPointer != nil {
			loaded := l.loadPersistentPointer(target, pointer, node)
			if loaded.levelGlobal == nil {
				l.errorAt(node, "persistent aggregate pointer target has no structured layout")
			} else {
				loaded = l.attachLevelGlobalAggregate(loaded, node)
				l.storeAggregate(result, loaded, node)
			}
		} else if target.aggregate == nil {
			l.errorAt(node, "dynamic aggregate pointer target has no aggregate descriptor")
		} else {
			l.storeAggregate(result, target, node)
		}
		l.jump(merge)
	}
	l.setCurrent(invalid)
	_ = l.builder.MarkUnreachable()
	l.setCurrent(merge)
	result.aggregateLoad = &aggregatePointerLoadValue{pointer: value.aggregatePointer}
	return result
}

func (l *lowerer) storeAggregatePointerLoad(load *aggregatePointerLoadValue, source lowerValue, node ast.Node) {
	if load == nil || load.pointer == nil || len(load.pointer.tag.slots) != 1 {
		l.errorAt(node, "dynamic aggregate pointer assignment has no target tag")
		return
	}
	merge, invalid := l.newBlock(), l.newBlock()
	blocks := make([]*ir.Block, len(load.pointer.alternatives))
	cases := make([]ir.SwitchCase, len(blocks))
	for index := range blocks {
		blocks[index] = l.newBlock()
		cases[index] = ir.SwitchCase{Value: float64(index), Target: blocks[index].ID}
	}
	_ = l.builder.Switch(load.pointer.tag.slots[0], cases, invalid)
	for index, target := range load.pointer.alternatives {
		l.setCurrent(blocks[index])
		if target.nilPointer {
			l.terminateRuntime(node, "nil pointer dereference")
			continue
		}
		l.storeAggregatePointerPath(target, load.path, source, node, true)
		l.jump(merge)
	}
	l.setCurrent(invalid)
	_ = l.builder.MarkUnreachable()
	l.setCurrent(merge)
}

func (l *lowerer) storeAggregatePointerPath(value lowerValue, path []aggregatePointerPathStep, source lowerValue, node ast.Node, dereference bool) {
	if value.persistentPointer != nil && (dereference || len(path) != 0) {
		pointer, ok := types.Unalias(value.type_).(*types.Pointer)
		if !ok {
			l.errorAt(node, "persistent aggregate pointer target has invalid type")
			return
		}
		value = l.loadPersistentPointer(value, pointer, node)
		value = l.attachLevelGlobalAggregate(value, node)
	}
	if len(path) == 0 {
		l.storeDescriptor(value, source, node)
		return
	}
	step := path[0]
	if value.aggregate == nil {
		l.errorAt(node, "dynamic aggregate pointer assignment path is invalid")
		return
	}
	if !step.isDynamic {
		if step.index < 0 || step.index >= len(value.aggregate.fields) {
			l.errorAt(node, "dynamic aggregate pointer assignment path is out of bounds")
			return
		}
		l.storeAggregatePointerPath(value.aggregate.fields[step.index], path[1:], source, node, false)
		return
	}
	if len(step.dynamic.slots) != 1 {
		l.errorAt(node, "dynamic aggregate pointer array path has no index")
		return
	}
	merge, invalid := l.newBlock(), l.newBlock()
	blocks := make([]*ir.Block, len(value.aggregate.fields))
	cases := make([]ir.SwitchCase, len(blocks))
	for index := range blocks {
		blocks[index] = l.newBlock()
		cases[index] = ir.SwitchCase{Value: float64(index), Target: blocks[index].ID}
	}
	_ = l.builder.Switch(step.dynamic.slots[0], cases, invalid)
	for index, block := range blocks {
		l.setCurrent(block)
		l.storeAggregatePointerPath(value.aggregate.fields[index], path[1:], source, node, false)
		l.jump(merge)
	}
	l.setCurrent(invalid)
	_ = l.builder.MarkUnreachable()
	l.setCurrent(merge)
}

func (l *lowerer) addressAggregatePointerLoad(load *aggregatePointerLoadValue, pointerType types.Type, node ast.Node) lowerValue {
	if load == nil || load.pointer == nil || len(load.pointer.tag.slots) != 1 {
		l.errorAt(node, "dynamic aggregate pointer address has no target tag")
		return zeroValue(pointerType)
	}
	alternatives := make([]lowerValue, len(load.pointer.alternatives))
	for index, target := range load.pointer.alternatives {
		alternatives[index] = l.addressAggregatePointerPath(target, load.path, pointerType, node, true)
	}
	return lowerValue{type_: pointerType, aggregatePointer: &finiteVariant[lowerValue]{
		alternatives: alternatives,
		tag:          load.pointer.tag,
	}}
}

func (l *lowerer) addressAggregatePointerPath(value lowerValue, path []aggregatePointerPathStep, pointerType types.Type, node ast.Node, dereference bool) lowerValue {
	if value.nilPointer {
		return lowerValue{type_: pointerType, nilPointer: true}
	}
	if value.persistentPointer != nil && (dereference || len(path) != 0) {
		pointer, ok := types.Unalias(value.type_).(*types.Pointer)
		if !ok {
			l.errorAt(node, "persistent aggregate pointer target has invalid type")
			return zeroValue(pointerType)
		}
		value = l.loadPersistentPointer(value, pointer, node)
		value = l.attachLevelGlobalAggregate(value, node)
	}
	if len(path) == 0 {
		if value.levelGlobal != nil {
			handle := scalarValue(l.pure(resource.RuntimeFunctionAdd, node, value.levelGlobal.base, ir.Const{Value: 1}), types.Typ[types.Int])
			return lowerValue{type_: pointerType, persistentPointer: &persistentPointerValue{
				handle: handle, storage: value.levelGlobal.storage, target: value.levelGlobal.declaration,
				read: value.levelGlobal.read, write: value.levelGlobal.write,
			}}
		}
		if value.persistentPointer != nil {
			l.errorAt(node, "address of a persistent pointer field is not supported")
			return zeroValue(pointerType)
		}
		value.type_ = pointerType
		return l.freezePointerValue(value, node)
	}
	step := path[0]
	if value.aggregate == nil {
		l.errorAt(node, "dynamic aggregate pointer address path is invalid")
		return zeroValue(pointerType)
	}
	if !step.isDynamic {
		if step.index < 0 || step.index >= len(value.aggregate.fields) {
			l.errorAt(node, "dynamic aggregate pointer address path is out of bounds")
			return zeroValue(pointerType)
		}
		return l.addressAggregatePointerPath(value.aggregate.fields[step.index], path[1:], pointerType, node, false)
	}
	if len(step.dynamic.slots) != 1 {
		l.errorAt(node, "dynamic aggregate pointer address array path has no index")
		return zeroValue(pointerType)
	}
	alternatives := make([]lowerValue, len(value.aggregate.fields))
	for index := range value.aggregate.fields {
		alternatives[index] = l.addressAggregatePointerPath(value.aggregate.fields[index], path[1:], pointerType, node, false)
	}
	return lowerValue{type_: pointerType, aggregatePointer: &finiteVariant[lowerValue]{
		alternatives: alternatives,
		tag:          step.dynamic,
	}}
}

func (l *lowerer) zeroRuntimeValue(t types.Type) lowerValue {
	t = l.resolveType(t)
	typ := l.runtimeTypeOf(t)
	slots := make([]ir.Expr, typ.Slots)
	for index := range slots {
		slots[index] = ir.Const{}
	}
	return lowerValue{type_: t, slots: slots}
}

func zeroValue(t types.Type) lowerValue {
	typ := irTypeOf(t)
	slots := make([]ir.Expr, typ.Slots)
	for i := range slots {
		slots[i] = ir.Const{}
	}
	return lowerValue{type_: t, slots: slots}
}

func namedResourceHandle(t types.Type) string {
	switch typeID(t) {
	case rootID("Sprite"):
		return "Sprite"
	case rootID("Clip"):
		return "Clip"
	case rootID("Effect"):
		return "Effect"
	case rootID("Text"):
		return "Text"
	case rootID("Icon"):
		return "Icon"
	default:
		return ""
	}
}

func containedResourceHandle(t types.Type, visiting map[types.Type]bool) string {
	t = types.Unalias(t)
	if kind := namedResourceHandle(t); kind != "" {
		return kind
	}
	if visiting[t] {
		return ""
	}
	visiting[t] = true
	defer delete(visiting, t)
	switch underlying := t.Underlying().(type) {
	case *types.Array:
		return containedResourceHandle(underlying.Elem(), visiting)
	case *types.Struct:
		for i := 0; i < underlying.NumFields(); i++ {
			if kind := containedResourceHandle(underlying.Field(i).Type(), visiting); kind != "" {
				return kind
			}
		}
	}
	return ""
}

func containsResourceHandle(t types.Type) string {
	return containedResourceHandle(t, map[types.Type]bool{})
}

func constantExpr(value constant.Value) (ir.Expr, bool) {
	if value == nil {
		return nil, false
	}
	if value.Kind() == constant.Bool {
		if constant.BoolVal(value) {
			return ir.Const{Value: 1}, true
		}
		return ir.Const{}, true
	}
	if value.Kind() == constant.Int || value.Kind() == constant.Float {
		f, _ := constant.Float64Val(constant.ToFloat(value))
		return ir.Const{Value: f}, true
	}
	return nil, false
}

func (l *lowerer) alloc(name string, t types.Type) lowerValue {
	t = l.resolveType(t)
	typ := l.runtimeTypeOf(t)
	poolKey := localTypePoolKey(typ)
	var value ir.Value
	var id int
	if pool := l.localPool[poolKey]; len(pool) != 0 {
		id = pool[len(pool)-1]
		l.localPool[poolKey] = pool[:len(pool)-1]
		value = l.builder.ReuseLocal(id, name, typ)
	} else {
		id = len(l.builder.Function().Locals)
		value = l.builder.NewLocal(name, typ)
	}
	if len(l.localScopes) != 0 {
		last := len(l.localScopes) - 1
		l.localScopes[last] = append(l.localScopes[last], id)
	}
	return lowerValue{type_: t, slots: value.Slots, places: ir.Places(value)}
}

func (l *lowerer) beginInlineLocalScope() {
	if l.localPool == nil {
		l.localPool = map[string][]int{}
	}
	l.localScopes = append(l.localScopes, nil)
}

func (l *lowerer) endInlineLocalScope(result lowerValue) {
	last := len(l.localScopes) - 1
	locals := l.localScopes[last]
	l.localScopes = l.localScopes[:last]
	if len(result.multi) != 0 || result.callable != nil || result.callableArray != nil || result.pointer != nil || result.pointerLoad != nil || result.container != nil || result.containerVariant != nil || result.interface_ != nil || result.entity != nil || result.variadic != nil || result.stream != nil || result.levelGlobal != nil || result.aggregate != nil || result.aggregatePointer != nil || result.aggregateLoad != nil || result.aggregateIndex != nil {
		return
	}
	escaped := map[int]bool{}
	for _, slot := range result.slots {
		collectLocalIDsExpr(slot, escaped)
	}
	for _, place := range result.places {
		collectLocalIDsPlace(place, escaped)
	}
	released := map[int]bool{}
	for _, id := range locals {
		if id < 0 || id >= len(l.builder.Function().Locals) || escaped[id] || released[id] {
			continue
		}
		released[id] = true
		typ := l.builder.Function().Locals[id]
		key := localTypePoolKey(typ)
		l.localPool[key] = append(l.localPool[key], id)
	}
}

func localTypePoolKey(typ ir.Type) string {
	var builder strings.Builder
	var write func(ir.Type)
	write = func(value ir.Type) {
		fmt.Fprintf(&builder, "%d:%s:%d[", len(value.Name), value.Name, value.Slots)
		for _, field := range value.Fields {
			fmt.Fprintf(&builder, "%d:%s:%d=", len(field.Name), field.Name, field.Offset)
			write(field.Type)
		}
		builder.WriteByte(']')
	}
	write(typ)
	return builder.String()
}

func collectLocalIDsExpr(expr ir.Expr, ids map[int]bool) {
	switch value := expr.(type) {
	case ir.Load:
		collectLocalIDsPlace(value.Place, ids)
	case ir.RuntimeCall:
		for _, argument := range value.Args {
			collectLocalIDsExpr(argument, ids)
		}
	}
}

func collectLocalIDsPlace(place ir.Place, ids map[int]bool) {
	switch value := place.(type) {
	case ir.LocalPlace:
		ids[value.ID] = true
	case ir.IndexedLocalPlace:
		ids[value.ID] = true
		collectLocalIDsExpr(value.Index, ids)
	case ir.MemoryPlace:
		collectLocalIDsExpr(value.Index, ids)
	}
}

func entityViewType(t types.Type) ir.Type {
	return ir.Type{Name: "entity-view:" + t.String(), Slots: 1}
}

func (l *lowerer) allocEntityView(name string, value lowerValue, node ast.Node) lowerValue {
	if value.entity == nil || len(value.slots) != 1 {
		l.errorAt(node, "EntityRef.Get view requires exactly one entity index slot")
		return lowerValue{}
	}
	typ := entityViewType(value.type_)
	local := l.builder.NewLocal(name, typ)
	result := lowerValue{
		type_: value.type_, slots: local.Slots, places: ir.Places(local),
		entity: &entityReferenceValue{binding: value.entity.binding},
	}
	if err := l.builder.Store(result.places, ir.Value{Type: typ, Slots: value.slots}, sourcePos(l.pkg, node.Pos())); err != nil {
		l.errorAt(node, "%v", err)
	}
	return result
}

func (l *lowerer) newEntityViewLocal(name string, t types.Type, binding archetypeBinding) lowerValue {
	typ := entityViewType(t)
	local := l.builder.NewLocal(name, typ)
	return lowerValue{
		type_: t, slots: local.Slots, places: ir.Places(local),
		entity: &entityReferenceValue{binding: binding},
	}
}

func (l *lowerer) entityBinding(t types.Type) (archetypeBinding, bool) {
	named, ok := namedType(t)
	if !ok {
		return archetypeBinding{}, false
	}
	binding, exists := l.archetypes[named]
	return binding, exists
}

func (l *lowerer) storeEntityView(dst, src lowerValue, node ast.Node) {
	if dst.entity == nil || src.entity == nil || len(dst.places) != 1 || len(src.slots) != 1 {
		l.errorAt(node, "EntityRef.Get view assignment requires one local entity index")
		return
	}
	if dst.entity.binding.declaration != src.entity.binding.declaration {
		l.errorAt(node, "EntityRef.Get view assignment cannot change target archetype from %s to %s", dst.entity.binding.declaration.Name, src.entity.binding.declaration.Name)
		return
	}
	if err := l.builder.Store(dst.places, ir.Value{Type: entityViewType(dst.type_), Slots: src.slots}, sourcePos(l.pkg, node.Pos())); err != nil {
		l.errorAt(node, "%v", err)
	}
}

func (l *lowerer) allocZeroed(name string, t types.Type, node ast.Node) lowerValue {
	value := l.alloc(name, t)
	l.store(value, l.zeroRuntimeValue(t), node)
	return value
}

func (l *lowerer) indexedLocal(base ir.LocalPlace, index ir.Expr, length, stride, offset int, node ast.Node) ir.Place {
	place, err := l.builder.IndexedLocal(base, index, length, stride, offset)
	if err != nil {
		l.errorAt(node, "%v", err)
		return nil
	}
	return place
}

func (l *lowerer) memory(storage string, index ir.Expr, stride, offset int, read, write bool, node ast.Node) ir.Place {
	place, err := l.builder.Memory(storage, index, stride, offset, read, write)
	if err != nil {
		l.errorAt(node, "%v", err)
		return nil
	}
	return place
}

func (l *lowerer) bind(obj types.Object, v lowerValue) { l.frames[len(l.frames)-1].vars[obj] = v }

func (l *lowerer) bindCallable(obj types.Object, callable *staticCallable) {
	l.frames[len(l.frames)-1].callables[obj] = callable
}

func (l *lowerer) lookupCallable(obj types.Object) (*staticCallable, bool) {
	for i := len(l.frames) - 1; i >= 0; i-- {
		if callable, ok := l.frames[i].callables[obj]; ok {
			return callable, true
		}
	}
	return nil, false
}

func (l *lowerer) captureBindings() (map[types.Object]lowerValue, map[types.Object]*staticCallable) {
	values := map[types.Object]lowerValue{}
	callables := map[types.Object]*staticCallable{}
	for _, frame := range l.frames {
		for object, value := range frame.vars {
			if value.container != nil && value.containerVariant != nil {
				value.container = nil
				value.slots = nil
				value.places = nil
			}
			values[object] = value
		}
		for object, callable := range frame.callables {
			callables[object] = callable
		}
	}
	return values, callables
}

func (l *lowerer) captureTypeSubstitutions() map[*types.TypeParam]types.Type {
	result := map[*types.TypeParam]types.Type{}
	for _, substitutions := range l.typeSubstitutions {
		for parameter, replacement := range substitutions {
			result[parameter] = l.resolveType(replacement)
		}
	}
	return result
}

func (l *lowerer) callableTypeSubstitutions(fn *types.Func, expression ast.Expr) map[*types.TypeParam]types.Type {
	result := l.captureTypeSubstitutions()
	instance, exists := callableInstance(l.pkg, expression)
	if !exists {
		return result
	}
	signature := fn.Origin().Type().(*types.Signature)
	for index := 0; index < signature.TypeParams().Len() && index < instance.TypeArgs.Len(); index++ {
		result[signature.TypeParams().At(index)] = l.resolveType(instance.TypeArgs.At(index))
	}
	return result
}

func cloneValueBindings(values map[types.Object]lowerValue) map[types.Object]lowerValue {
	result := make(map[types.Object]lowerValue, len(values))
	for object, value := range values {
		result[object] = value
	}
	return result
}

func cloneCallableBindings(values map[types.Object]*staticCallable) map[types.Object]*staticCallable {
	result := make(map[types.Object]*staticCallable, len(values))
	for object, value := range values {
		result[object] = value
	}
	return result
}

func sameCallable(left, right *staticCallable) bool {
	if left == nil || right == nil {
		return left == right
	}
	if left == right {
		return true
	}
	if left.tag.type_ != nil || right.tag.type_ != nil {
		return false
	}
	if left.identity != "" || right.identity != "" {
		return left.identity == right.identity && reflect.DeepEqual(left.receiver, right.receiver) && reflect.DeepEqual(left.captures, right.captures)
	}
	if left.intrinsic != nil || right.intrinsic != nil {
		return false
	}
	return left.function == right.function && left.literal == right.literal && left.yield == right.yield && reflect.DeepEqual(left.receiver, right.receiver) && reflect.DeepEqual(left.captures, right.captures) && sameTypeSubstitutions(left.substitutions, right.substitutions)
}

func sameTypeSubstitutions(left, right map[*types.TypeParam]types.Type) bool {
	if len(left) != len(right) {
		return false
	}
	for parameter, leftType := range left {
		rightType := right[parameter]
		if rightType == nil || !types.Identical(leftType, rightType) {
			return false
		}
	}
	return true
}

func (l *lowerer) rebind(obj types.Object, v lowerValue) {
	for i := len(l.frames) - 1; i >= 0; i-- {
		if _, ok := l.frames[i].vars[obj]; ok {
			l.frames[i].vars[obj] = v
			if l.frames[i].results[obj] {
				l.frames[i].result = v
			}
			return
		}
	}
	l.bind(obj, v)
}

func (l *lowerer) bindParameter(obj *types.Var, value lowerValue, name string, node ast.Node) {
	parameterType := l.resolveType(obj.Type())
	if l.isAggregatePointerType(parameterType) && (isAggregatePointerValue(value) || value.nilPointer) {
		value.type_ = parameterType
		frame := l.frames[len(l.frames)-1]
		if frame.reboundValues[obj] {
			l.bind(obj, l.copyAggregatePointerValue(name, value, node))
		} else {
			l.bind(obj, value)
		}
		return
	}
	if value.aggregate != nil {
		value.type_ = parameterType
		if isPointerType(parameterType) {
			l.bind(obj, value)
		} else {
			l.bind(obj, l.copyAggregate(name, value, node))
		}
		return
	}
	if value.entity != nil {
		l.bind(obj, l.allocEntityView(name, value, node))
		return
	}
	if _, typeParameter := types.Unalias(obj.Type()).(*types.TypeParam); typeParameter {
		value = l.materialize(name, value, node)
		l.bind(obj, value)
		return
	}
	if _, interfaceType := types.Unalias(parameterType).Underlying().(*types.Interface); interfaceType {
		if value.interface_ == nil {
			value = l.storeInterfaceValue(l.newInterfaceValue(name, parameterType, node), value, node)
		}
		value.type_ = parameterType
		l.bind(obj, value)
		return
	}
	if value.callableArray != nil {
		l.bind(obj, l.copyCallableArray(name, value, node))
		return
	}
	if _, pointer := types.Unalias(parameterType).(*types.Pointer); pointer {
		value.type_ = parameterType
		frame := l.frames[len(l.frames)-1]
		if !frame.mutableValues[obj] {
			l.bind(obj, value)
			return
		}
		l.bind(obj, l.copyPointerValue(name, value, node))
		return
	}
	if isContainerValue(value) {
		l.bind(obj, l.copyContainerValue(name, value, node))
		return
	}
	if value.variadic != nil || value.interface_ != nil {
		l.bind(obj, value)
		return
	}
	frame := l.frames[len(l.frames)-1]
	if !frame.mutableValues[obj] && frame.valueReads[obj] <= 1 {
		value.type_ = parameterType
		l.bind(obj, value)
		return
	}
	allConstant := len(value.slots) != 0
	for _, slot := range value.slots {
		if _, ok := slot.(ir.Const); !ok {
			allConstant = false
			break
		}
	}
	if allConstant {
		value.type_ = parameterType
		l.bind(obj, value)
		return
	}
	local := l.alloc(name, parameterType)
	l.store(local, value, node)
	l.bind(obj, local)
}

func isFunctionType(t types.Type) bool {
	if t == nil {
		return false
	}
	_, ok := types.Unalias(t).Underlying().(*types.Signature)
	return ok
}
func (l *lowerer) lookup(obj types.Object) (lowerValue, bool) {
	for i := len(l.frames) - 1; i >= 0; i-- {
		if v, ok := l.frames[i].vars[obj]; ok {
			return v, true
		}
	}
	return lowerValue{}, false
}

func (l *lowerer) store(dst, src lowerValue, node ast.Node) {
	if dst.pointerLoad != nil {
		l.storePointerLoad(dst.pointerLoad, src, node)
		return
	}
	if len(dst.places) != len(src.slots) {
		l.errorAt(node, "assignment layout mismatch: %d places for %d slots", len(dst.places), len(src.slots))
		return
	}
	for _, p := range dst.places {
		if p == nil {
			l.errorAt(node, "value is not assignable")
			return
		}
	}
	valueType := l.runtimeTypeOf(src.type_)
	if src.entity != nil {
		valueType = entityViewType(src.type_)
	}
	if err := l.builder.Store(dst.places, ir.Value{Type: valueType, Slots: src.slots}, sourcePos(l.pkg, node.Pos())); err != nil {
		l.errorAt(node, "%v", err)
	}
}

func (l *lowerer) storePointerLoad(load *pointerLoad, src lowerValue, node ast.Node) {
	if load == nil || load.pointer == nil || len(load.pointer.tag.slots) != 1 {
		l.errorAt(node, "dynamic pointer store has no target tag")
		return
	}
	if len(src.slots) != load.size {
		l.errorAt(node, "dynamic pointer store layout mismatch: %d slots for %d target slots", len(src.slots), load.size)
		return
	}
	src = l.materialize("pointer.store", src, node)
	merge, invalid := l.newBlock(), l.newBlock()
	blocks := make([]*ir.Block, len(load.pointer.alternatives))
	cases := make([]ir.SwitchCase, len(blocks))
	for index := range blocks {
		blocks[index] = l.newBlock()
		cases[index] = ir.SwitchCase{Value: float64(index), Target: blocks[index].ID}
	}
	_ = l.builder.Switch(load.pointer.tag.slots[0], cases, invalid)
	for index, target := range load.pointer.alternatives {
		l.setCurrent(blocks[index])
		if target == nil {
			l.terminateRuntime(node, "nil pointer dereference")
			continue
		}
		places := target
		if load.index != nil {
			if load.offset < 0 || load.offset+load.arrayLength*load.stride > len(target) {
				l.errorAt(node, "dynamic pointer target %d has invalid indexed layout", index)
				places = nil
			} else {
				places = make([]ir.Place, load.size)
				for slot := range places {
					places[slot] = l.dynamicArrayPlace(target[load.offset], load.index, load.arrayLength, load.stride, slot, node)
				}
			}
		}
		if load.index == nil && load.offset+load.size > len(target) {
			l.errorAt(node, "dynamic pointer target %d has invalid layout", index)
		} else {
			if load.index == nil {
				places = target[load.offset : load.offset+load.size]
			}
			if len(places) != 0 {
				if err := l.builder.Store(places, ir.Value{Type: l.runtimeTypeOf(src.type_), Slots: src.slots}, sourcePos(l.pkg, node.Pos())); err != nil {
					l.errorAt(node, "%v", err)
				}
			}
		}
		l.jump(merge)
	}
	l.setCurrent(invalid)
	_ = l.builder.MarkUnreachable()
	l.setCurrent(merge)
}

func (l *lowerer) addressPointerLoad(load *pointerLoad, pointerType types.Type, node ast.Node) lowerValue {
	if load == nil || load.pointer == nil || len(load.pointer.tag.slots) != 1 {
		l.errorAt(node, "dynamic pointer address has no target tag")
		return zeroValue(pointerType)
	}
	alternatives := make([][]ir.Place, len(load.pointer.alternatives))
	for index, target := range load.pointer.alternatives {
		if target == nil {
			continue
		}
		if load.index == nil {
			if load.offset < 0 || load.offset+load.size > len(target) {
				l.errorAt(node, "dynamic pointer target %d has invalid address layout", index)
				continue
			}
			alternatives[index] = append([]ir.Place(nil), target[load.offset:load.offset+load.size]...)
			continue
		}
		if load.offset < 0 || load.offset+load.arrayLength*load.stride > len(target) {
			l.errorAt(node, "dynamic pointer target %d has invalid indexed address layout", index)
			continue
		}
		places := make([]ir.Place, load.size)
		for slot := range places {
			places[slot] = l.dynamicArrayPlace(target[load.offset], load.index, load.arrayLength, load.stride, slot, node)
		}
		alternatives[index] = places
	}
	return lowerValue{
		type_: pointerType,
		pointer: &pointerValue{finiteVariant: finiteVariant[[]ir.Place]{
			alternatives: alternatives,
			tag:          load.pointer.tag,
		}},
	}
}

func (l *lowerer) loadPointerIndex(value lowerValue, pointer *types.Pointer, array *types.Array, index lowerValue, node ast.Node) lowerValue {
	if value.pointer == nil || len(value.pointer.tag.slots) != 1 {
		l.errorAt(node, "dynamic pointer has no target tag")
		return zeroValue(array.Elem())
	}
	index = l.materialize("pointer.index", index, node)
	condition := l.pure(resource.RuntimeFunctionAnd, node,
		l.pure(resource.RuntimeFunctionGreaterOr, node, index.slots[0], ir.Const{}),
		l.pure(resource.RuntimeFunctionLess, node, index.slots[0], ir.Const{Value: float64(array.Len())}))
	l.guard(node, condition)
	stride := l.runtimeTypeOf(types.NewArray(array.Elem(), 1)).Slots
	result := l.allocZeroed("pointer.index.load", array.Elem(), node)
	merge, invalid := l.newBlock(), l.newBlock()
	blocks := make([]*ir.Block, len(value.pointer.alternatives))
	cases := make([]ir.SwitchCase, len(blocks))
	for alternative := range blocks {
		blocks[alternative] = l.newBlock()
		cases[alternative] = ir.SwitchCase{Value: float64(alternative), Target: blocks[alternative].ID}
	}
	_ = l.builder.Switch(value.pointer.tag.slots[0], cases, invalid)
	for alternative, target := range value.pointer.alternatives {
		l.setCurrent(blocks[alternative])
		if target == nil {
			l.terminateRuntime(node, "nil pointer dereference")
			continue
		}
		if len(target) != int(array.Len())*stride {
			l.errorAt(node, "dynamic pointer target %d has %d slots; expected %d", alternative, len(target), int(array.Len())*stride)
		} else {
			slots := make([]ir.Expr, stride)
			for slot := range slots {
				place := l.dynamicArrayPlace(target[0], index.slots[0], int(array.Len()), stride, slot, node)
				slots[slot] = ir.Load{Place: place}
			}
			if err := l.builder.Store(result.places, ir.Value{Type: l.runtimeTypeOf(array.Elem()), Slots: slots}, sourcePos(l.pkg, node.Pos())); err != nil {
				l.errorAt(node, "%v", err)
			}
		}
		l.jump(merge)
	}
	l.setCurrent(invalid)
	_ = l.builder.MarkUnreachable()
	l.setCurrent(merge)
	result.type_ = array.Elem()
	result.pointerLoad = &pointerLoad{pointer: value.pointer, size: stride, index: index.slots[0], arrayLength: int(array.Len()), stride: stride}
	_ = pointer
	return result
}

func (l *lowerer) loadPointerArrayIndex(base lowerValue, array *types.Array, index lowerValue, node ast.Node) lowerValue {
	load := base.pointerLoad
	if load == nil || load.pointer == nil || len(load.pointer.tag.slots) != 1 {
		l.errorAt(node, "dynamic pointer array has no target tag")
		return zeroValue(array.Elem())
	}
	index = l.materialize("pointer.index", index, node)
	condition := l.pure(resource.RuntimeFunctionAnd, node,
		l.pure(resource.RuntimeFunctionGreaterOr, node, index.slots[0], ir.Const{}),
		l.pure(resource.RuntimeFunctionLess, node, index.slots[0], ir.Const{Value: float64(array.Len())}))
	l.guard(node, condition)
	stride := l.runtimeTypeOf(types.NewArray(array.Elem(), 1)).Slots
	result := l.allocZeroed("pointer.index.load", array.Elem(), node)
	merge, invalid := l.newBlock(), l.newBlock()
	blocks := make([]*ir.Block, len(load.pointer.alternatives))
	cases := make([]ir.SwitchCase, len(blocks))
	for alternative := range blocks {
		blocks[alternative] = l.newBlock()
		cases[alternative] = ir.SwitchCase{Value: float64(alternative), Target: blocks[alternative].ID}
	}
	_ = l.builder.Switch(load.pointer.tag.slots[0], cases, invalid)
	for alternative, target := range load.pointer.alternatives {
		l.setCurrent(blocks[alternative])
		if target == nil {
			l.terminateRuntime(node, "nil pointer dereference")
			continue
		}
		if load.offset < 0 || load.offset+int(array.Len())*stride > len(target) {
			l.errorAt(node, "dynamic pointer target %d has invalid array layout", alternative)
		} else {
			slots := make([]ir.Expr, stride)
			for slot := range slots {
				place := l.dynamicArrayPlace(target[load.offset], index.slots[0], int(array.Len()), stride, slot, node)
				slots[slot] = ir.Load{Place: place}
			}
			if err := l.builder.Store(result.places, ir.Value{Type: l.runtimeTypeOf(array.Elem()), Slots: slots}, sourcePos(l.pkg, node.Pos())); err != nil {
				l.errorAt(node, "%v", err)
			}
		}
		l.jump(merge)
	}
	l.setCurrent(invalid)
	_ = l.builder.MarkUnreachable()
	l.setCurrent(merge)
	result.type_ = array.Elem()
	result.pointerLoad = &pointerLoad{pointer: load.pointer, offset: load.offset, size: stride, index: index.slots[0], arrayLength: int(array.Len()), stride: stride}
	return result
}

func (l *lowerer) loadPointer(value lowerValue, pointer *types.Pointer, node ast.Node) lowerValue {
	if value.pointer == nil || len(value.pointer.tag.slots) != 1 {
		l.errorAt(node, "dynamic pointer has no target tag")
		return zeroValue(pointer.Elem())
	}
	result := l.allocZeroed("pointer.load", pointer.Elem(), node)
	merge, invalid := l.newBlock(), l.newBlock()
	blocks := make([]*ir.Block, len(value.pointer.alternatives))
	cases := make([]ir.SwitchCase, len(blocks))
	for index := range blocks {
		blocks[index] = l.newBlock()
		cases[index] = ir.SwitchCase{Value: float64(index), Target: blocks[index].ID}
	}
	_ = l.builder.Switch(value.pointer.tag.slots[0], cases, invalid)
	for index, target := range value.pointer.alternatives {
		l.setCurrent(blocks[index])
		if target == nil {
			l.terminateRuntime(node, "nil pointer dereference")
			continue
		}
		if len(target) != len(result.places) {
			l.errorAt(node, "dynamic pointer target %d has %d slots; expected %d", index, len(target), len(result.places))
		} else {
			slots := make([]ir.Expr, len(target))
			for slot, place := range target {
				slots[slot] = ir.Load{Place: place}
			}
			if err := l.builder.Store(result.places, ir.Value{Type: l.runtimeTypeOf(pointer.Elem()), Slots: slots}, sourcePos(l.pkg, node.Pos())); err != nil {
				l.errorAt(node, "%v", err)
			}
		}
		l.jump(merge)
	}
	l.setCurrent(invalid)
	_ = l.builder.MarkUnreachable()
	l.setCurrent(merge)
	result.type_ = pointer.Elem()
	result.pointerLoad = &pointerLoad{pointer: value.pointer, size: len(result.slots)}
	return result
}

var binaryRuntime = map[token.Token]resource.RuntimeFunction{
	token.ADD: resource.RuntimeFunctionAdd, token.SUB: resource.RuntimeFunctionSubtract,
	token.MUL: resource.RuntimeFunctionMultiply, token.QUO: resource.RuntimeFunctionDivide,
	token.REM: resource.RuntimeFunctionRem, token.EQL: resource.RuntimeFunctionEqual,
	token.NEQ: resource.RuntimeFunctionNotEqual, token.LSS: resource.RuntimeFunctionLess,
	token.LEQ: resource.RuntimeFunctionLessOr, token.GTR: resource.RuntimeFunctionGreater,
	token.GEQ: resource.RuntimeFunctionGreaterOr,
}

func runtimeBasicKind(t types.Type) (types.BasicKind, bool) {
	if t == nil {
		return types.Invalid, false
	}
	basic, ok := types.Unalias(t).Underlying().(*types.Basic)
	if !ok {
		return types.Invalid, false
	}
	switch basic.Kind() {
	case types.Bool, types.Int, types.Float64:
		return basic.Kind(), true
	default:
		return basic.Kind(), false
	}
}

func isPointerType(t types.Type) bool {
	_, ok := types.Unalias(t).(*types.Pointer)
	return ok
}

func isInterfaceType(t types.Type) bool {
	if t == nil {
		return false
	}
	if _, typeParameter := types.Unalias(t).(*types.TypeParam); typeParameter {
		return false
	}
	_, ok := types.Unalias(t).Underlying().(*types.Interface)
	return ok
}

func isStaticPointer(value lowerValue) bool {
	return value.entity == nil && (value.nilPointer || isPointerType(value.type_) && (value.persistentPointer != nil || value.aggregate != nil || value.aggregatePointer != nil || value.pointer != nil || len(value.places) != 0 && len(value.places) == len(value.slots)))
}

func samePlaces(left, right []ir.Place) bool {
	return reflect.DeepEqual(left, right)
}

func constantBool(pkg *packages.Package, expression ast.Expr) (bool, bool) {
	typed, ok := pkg.TypesInfo.Types[expression]
	if !ok || typed.Value == nil || typed.Value.Kind() != constant.Bool {
		return false, false
	}
	return constant.BoolVal(typed.Value), true
}

func foldBinaryConstants(op token.Token, left, right float64, resultType types.Type) (float64, bool) {
	switch op {
	case token.ADD:
		return left + right, true
	case token.SUB:
		return left - right, true
	case token.MUL:
		return left * right, true
	case token.QUO:
		if right == 0 {
			return 0, false
		}
		result := left / right
		if kind, ok := runtimeBasicKind(resultType); ok && kind == types.Int {
			result = math.Trunc(result)
		}
		return result, true
	case token.REM:
		if right == 0 {
			return 0, false
		}
		return math.Mod(left, right), true
	case token.EQL:
		if left == right {
			return 1, true
		}
		return 0, true
	case token.NEQ:
		if left != right {
			return 1, true
		}
		return 0, true
	case token.LSS:
		if left < right {
			return 1, true
		}
		return 0, true
	case token.LEQ:
		if left <= right {
			return 1, true
		}
		return 0, true
	case token.GTR:
		if left > right {
			return 1, true
		}
		return 0, true
	case token.GEQ:
		if left >= right {
			return 1, true
		}
		return 0, true
	default:
		return 0, false
	}
}

func (l *lowerer) expr(expr ast.Expr) lowerValue {
	_, isCall := expr.(*ast.CallExpr)
	if typed, ok := l.pkg.TypesInfo.Types[expr]; ok && typed.Value != nil && !isCall {
		value, representable := constantExpr(typed.Value)
		if !representable {
			l.errorAt(expr, "constant is not representable at runtime")
			return lowerValue{}
		}
		return lowerValue{type_: typed.Type, slots: []ir.Expr{value}}
	}
	switch n := expr.(type) {
	case *ast.ParenExpr:
		return l.expr(n.X)
	case *ast.BasicLit:
		v := l.pkg.TypesInfo.Types[n].Value
		expr, ok := constantExpr(v)
		if !ok {
			l.errorAt(n, "constant is not representable at runtime")
			return lowerValue{}
		}
		return lowerValue{type_: l.pkg.TypesInfo.TypeOf(n), slots: []ir.Expr{expr}}
	case *ast.Ident:
		if n.Name == "nil" {
			return lowerValue{type_: l.pkg.TypesInfo.TypeOf(n), nilPointer: true}
		}
		obj := l.pkg.TypesInfo.ObjectOf(n)
		if v, ok := l.lookup(obj); ok {
			return v
		}
		if value, ok := l.packageCallableArray(obj, n); ok {
			return value
		}
		if value, ok := l.packageStaticValue(obj, n); ok {
			return value
		}
		if c, ok := obj.(*types.Const); ok {
			expr, representable := constantExpr(c.Val())
			if !representable {
				l.errorAt(n, "constant %s is not representable at runtime", c.Name())
				return lowerValue{}
			}
			return lowerValue{type_: c.Type(), slots: []ir.Expr{expr}}
		}
		l.errorAt(n, "unsupported identifier %s", n.Name)
	case *ast.StarExpr:
		v := l.expr(n.X)
		if v.persistentPointer != nil {
			pointer, ok := types.Unalias(v.type_).(*types.Pointer)
			if !ok {
				l.errorAt(n, "dereference operand is not a pointer")
				return zeroValue(l.pkg.TypesInfo.TypeOf(n))
			}
			return l.loadPersistentPointer(v, pointer, n)
		}
		if v.aggregate != nil {
			pointer, ok := types.Unalias(v.type_).(*types.Pointer)
			if !ok {
				l.errorAt(n, "dereference operand is not a pointer")
				return zeroValue(l.pkg.TypesInfo.TypeOf(n))
			}
			v.type_ = pointer.Elem()
			return v
		}
		if v.aggregatePointer != nil {
			pointer, ok := types.Unalias(v.type_).(*types.Pointer)
			if !ok {
				l.errorAt(n, "dereference operand is not a pointer")
				return zeroValue(l.pkg.TypesInfo.TypeOf(n))
			}
			return l.loadAggregatePointer(v, pointer, n)
		}
		if v.entity != nil {
			l.errorAt(n, "EntityRef.Get views cannot be explicitly dereferenced")
			return zeroValue(l.pkg.TypesInfo.TypeOf(n))
		}
		pointer, ok := types.Unalias(v.type_).(*types.Pointer)
		if !ok {
			l.errorAt(n, "dereference operand is not a pointer")
			return zeroValue(l.pkg.TypesInfo.TypeOf(n))
		}
		if v.nilPointer {
			l.terminateRuntime(n, "nil pointer dereference")
			return zeroValue(pointer.Elem())
		}
		if v.pointer != nil {
			return l.loadPointer(v, pointer, n)
		}
		if len(v.places) != len(v.slots) {
			l.errorAt(n, "pointer alias has %d places for %d slots", len(v.places), len(v.slots))
			return zeroValue(l.pkg.TypesInfo.TypeOf(n))
		}
		v.type_ = pointer.Elem()
		return v
	case *ast.UnaryExpr:
		v := l.expr(n.X)
		if v.entity != nil {
			l.errorAt(n, "EntityRef.Get views cannot be addressed or used with unary operators")
			return zeroValue(l.pkg.TypesInfo.TypeOf(n))
		}
		if n.Op == token.AND {
			if l.containsEntityView(v.type_) {
				l.errorAt(n, "aggregates containing EntityRef.Get views cannot be addressed")
				return zeroValue(l.pkg.TypesInfo.TypeOf(n))
			}
			if v.entityField {
				l.errorAt(n, "EntityRef.Get fields cannot be addressed")
				return zeroValue(l.pkg.TypesInfo.TypeOf(n))
			}
			if v.levelGlobal != nil {
				handle := scalarValue(l.pure(resource.RuntimeFunctionAdd, n, v.levelGlobal.base, ir.Const{Value: 1}), types.Typ[types.Int])
				return lowerValue{type_: l.pkg.TypesInfo.TypeOf(n), persistentPointer: &persistentPointerValue{
					handle: handle, storage: v.levelGlobal.storage, target: v.levelGlobal.declaration,
					read: v.levelGlobal.read, write: v.levelGlobal.write,
				}}
			}
			if v.pointerLoad != nil {
				return l.addressPointerLoad(v.pointerLoad, l.pkg.TypesInfo.TypeOf(n), n)
			}
			if v.aggregateLoad != nil {
				return l.addressAggregatePointerLoad(v.aggregateLoad, l.pkg.TypesInfo.TypeOf(n), n)
			}
			v = l.materializeAddressable("address", v, n.X)
			if len(v.places) != len(v.slots) || len(v.places) == 0 {
				l.errorAt(n, "address operand is not representable as a DSL place")
				return zeroValue(l.pkg.TypesInfo.TypeOf(n))
			}
			v = l.freezePointerValue(v, n)
			v.type_ = l.pkg.TypesInfo.TypeOf(n)
			return v
		}
		if len(v.slots) != 1 {
			l.errorAt(n, "unary operator requires scalar value")
			return lowerValue{}
		}
		switch n.Op {
		case token.ADD:
			return v
		case token.SUB:
			return lowerValue{type_: v.type_, slots: []ir.Expr{ir.RuntimeCall{Function: resource.RuntimeFunctionNegate, Args: v.slots, Result: irTypeOf(v.type_), Pure: true, Pos: sourcePos(l.pkg, n.Pos())}}}
		case token.NOT:
			return lowerValue{type_: v.type_, slots: []ir.Expr{ir.RuntimeCall{Function: resource.RuntimeFunctionNot, Args: v.slots, Result: irTypeOf(v.type_), Pure: true, Pos: sourcePos(l.pkg, n.Pos())}}}
		default:
			l.errorAt(n, "unsupported unary operation %s", n.Op)
			return zeroValue(l.pkg.TypesInfo.TypeOf(n))
		}
	case *ast.BinaryExpr:
		if n.Op == token.LAND || n.Op == token.LOR {
			return l.shortCircuit(n)
		}
		a := l.expr(n.X)
		if a.entity != nil {
			a = l.allocEntityView("binary.left.entity", a, n.X)
		} else {
			a = l.materialize("binary.left", a, n.X)
		}
		b := l.expr(n.Y)
		if b.entity != nil {
			b = l.allocEntityView("binary.right.entity", b, n.Y)
		} else {
			b = l.materialize("binary.right", b, n.Y)
		}
		if n.Op == token.EQL || n.Op == token.NEQ {
			var variant *interfaceValue
			if a.interface_ != nil && b.nilPointer {
				variant = a.interface_
			} else if b.interface_ != nil && a.nilPointer {
				variant = b.interface_
			}
			if variant != nil {
				equal := l.pure(resource.RuntimeFunctionEqual, n, variant.tag.slots[0], ir.Const{Value: -1})
				if n.Op == token.NEQ {
					equal = l.pure(resource.RuntimeFunctionNot, n, equal)
				}
				return lowerValue{type_: l.pkg.TypesInfo.TypeOf(n), slots: []ir.Expr{equal}}
			}
		}
		if (n.Op == token.EQL || n.Op == token.NEQ) && isStaticPointer(a) && isStaticPointer(b) {
			var equal ir.Expr
			if l.isAggregatePointerType(a.type_) || l.isAggregatePointerType(b.type_) {
				equal = l.aggregatePointerEqual(a, b, n)
			} else {
				equal = l.pointerEqual(a, b, n)
			}
			if n.Op == token.NEQ {
				equal = l.pure(resource.RuntimeFunctionNot, n, equal)
			}
			return lowerValue{type_: l.pkg.TypesInfo.TypeOf(n), slots: []ir.Expr{equal}}
		}
		if a.entity != nil && b.entity != nil && (n.Op == token.EQL || n.Op == token.NEQ) {
			if a.entity.binding.declaration != b.entity.binding.declaration {
				l.errorAt(n, "EntityRef.Get view comparison requires the same target archetype")
				return zeroValue(l.pkg.TypesInfo.TypeOf(n))
			}
			op := binaryRuntime[n.Op]
			return lowerValue{type_: l.pkg.TypesInfo.TypeOf(n), slots: []ir.Expr{ir.RuntimeCall{Function: op, Args: []ir.Expr{a.slots[0], b.slots[0]}, Result: irTypeOf(l.pkg.TypesInfo.TypeOf(n)), Pure: true, Pos: sourcePos(l.pkg, n.Pos())}}}
		}
		if a.entity != nil || b.entity != nil {
			l.errorAt(n, "EntityRef.Get views cannot be compared or used with binary operators")
			return zeroValue(l.pkg.TypesInfo.TypeOf(n))
		}
		if len(a.slots) == 1 && len(b.slots) == 1 {
			left, leftConstant := a.slots[0].(ir.Const)
			right, rightConstant := b.slots[0].(ir.Const)
			if leftConstant && rightConstant {
				if value, folded := foldBinaryConstants(n.Op, left.Value, right.Value, l.pkg.TypesInfo.TypeOf(n)); folded {
					return lowerValue{type_: l.pkg.TypesInfo.TypeOf(n), slots: []ir.Expr{ir.Const{Value: value}}}
				}
			}
		}
		if (n.Op == token.EQL || n.Op == token.NEQ) && len(a.slots) == len(b.slots) && len(a.slots) != 1 && types.Comparable(a.type_) && types.Comparable(b.type_) {
			equal := ir.Expr(ir.Const{Value: 1})
			for i := range a.slots {
				slotEqual := l.pure(resource.RuntimeFunctionEqual, n, a.slots[i], b.slots[i])
				equal = l.pure(resource.RuntimeFunctionAnd, n, equal, slotEqual)
			}
			if n.Op == token.NEQ {
				equal = l.pure(resource.RuntimeFunctionNot, n, equal)
			}
			return lowerValue{type_: l.pkg.TypesInfo.TypeOf(n), slots: []ir.Expr{equal}}
		}
		op, ok := binaryRuntime[n.Op]
		if !ok || len(a.slots) != 1 || len(b.slots) != 1 {
			l.errorAt(n, "unsupported binary operation %s", n.Op)
			return lowerValue{}
		}
		resultType := l.resolveType(l.pkg.TypesInfo.TypeOf(n))
		_, resultTypeParameter := types.Unalias(resultType).(*types.TypeParam)
		if (resultTypeParameter || irTypeOf(resultType).Slots != 1) && len(a.slots) == 1 {
			resultType = a.type_
		}
		if (n.Op == token.QUO || n.Op == token.REM) && !l.requireIntegerDivisor(n, b.slots[0], resultType, n.Op) {
			return zeroValue(resultType)
		}
		result := ir.Expr(ir.RuntimeCall{Function: op, Args: []ir.Expr{a.slots[0], b.slots[0]}, Result: irTypeOf(resultType), Pure: true, Pos: sourcePos(l.pkg, n.Pos())})
		if n.Op == token.QUO {
			if kind, supported := runtimeBasicKind(resultType); !supported {
				l.errorAt(n, "runtime arithmetic does not support %s", resultType)
				return zeroValue(resultType)
			} else if kind == types.Int {
				result = ir.RuntimeCall{Function: resource.RuntimeFunctionTrunc, Args: []ir.Expr{result}, Result: irTypeOf(resultType), Pure: true, Pos: sourcePos(l.pkg, n.Pos())}
			}
		}
		return lowerValue{type_: resultType, slots: []ir.Expr{result}}
	case *ast.CompositeLit:
		return l.composite(n)
	case *ast.SelectorExpr:
		if value, ok := l.packageCallableArray(l.pkg.TypesInfo.ObjectOf(n.Sel), n); ok {
			return value
		}
		if value, ok := l.packageStaticValue(l.pkg.TypesInfo.ObjectOf(n.Sel), n); ok {
			return value
		}
		sel := l.pkg.TypesInfo.Selections[n]
		if sel == nil {
			if c, ok := l.pkg.TypesInfo.ObjectOf(n.Sel).(*types.Const); ok {
				expr, representable := constantExpr(c.Val())
				if !representable {
					l.errorAt(n, "constant %s is not representable at runtime", c.Name())
					return lowerValue{}
				}
				return lowerValue{type_: c.Type(), slots: []ir.Expr{expr}}
			}
			l.errorAt(n, "package symbol %s must be called", n.Sel.Name)
			return lowerValue{}
		}
		if field, ok := sel.Obj().(*types.Var); ok {
			if l.configuration != nil {
				if id, exists := l.configuration.OptionIDs[field]; exists {
					if l.mode == mode.ModeTutorial {
						return lowerValue{type_: field.Type(), slots: []ir.Expr{ir.Const{Value: l.configuration.Defaults[field]}}}
					}
					storage := "LevelOption"
					if l.mode == mode.ModePreview {
						storage = "PreviewOption"
					}
					place, err := l.builder.Memory(storage, ir.Const{}, 0, id, true, false)
					if err != nil {
						l.errorAt(n, "%v", err)
						return zeroValue(field.Type())
					}
					return lowerValue{type_: field.Type(), slots: []ir.Expr{ir.Load{Place: place}}, places: []ir.Place{place}}
				}
			}
			if ids, exists := l.resourceIDs[field]; exists {
				value := lowerValue{type_: l.pkg.TypesInfo.TypeOf(n)}
				if named, length, ok := streamLeafType(field.Type()); ok {
					value.stream = &streamValue{kind: typeID(named), valueSlots: irTypeOf(named.TypeArgs().At(0)).Slots, base: ir.Const{Value: float64(ids[0])}, width: len(ids), length: length}
					if length == 1 {
						value.slots = streamHandleSlots(l, n, value.stream.base, value.stream.width)
					}
				} else {
					value.slots = make([]ir.Expr, len(ids))
					for i, id := range ids {
						value.slots[i] = ir.Const{Value: float64(id)}
					}
				}
				return value
			}
			if declaration, exists := l.levelGlobalFields[field]; exists {
				storage, read, write := levelGlobalStorageAccess(declaration, l.mode, l.phase)
				return l.lowerLevelGlobalValue(n, declaration, storage, ir.Const{Value: float64(declaration.Offset)}, read, write)
			}
			base := l.expr(n.X)
			if base.persistentPointer != nil {
				pointer, ok := types.Unalias(base.type_).(*types.Pointer)
				if !ok {
					l.errorAt(n, "persistent pointer selector requires a pointer receiver")
					return zeroValue(l.pkg.TypesInfo.TypeOf(n))
				}
				base = l.loadPersistentPointer(base, pointer, n.X)
				base.type_ = pointer.Elem()
			}
			if base.levelGlobal != nil {
				global := base.levelGlobal
				declaration, baseExpr := global.declaration, global.base
				currentType := base.type_
				for _, index := range sel.Index() {
					record, ok := types.Unalias(currentType).Underlying().(*types.Struct)
					if !ok || index < 0 || index >= record.NumFields() {
						l.errorAt(n, "level global selector %s cannot be derived from %s", n.Sel.Name, currentType)
						return lowerValue{}
					}
					child := levelGlobalChild(declaration, record.Field(index))
					if child == nil {
						l.errorAt(n, "level global selector %s has no layout descriptor", n.Sel.Name)
						return lowerValue{}
					}
					if child.RelativeOffset != 0 {
						baseExpr = l.pure(resource.RuntimeFunctionAdd, n, baseExpr, ir.Const{Value: float64(child.RelativeOffset)})
					}
					declaration, currentType = child, child.Type
				}
				return l.lowerLevelGlobalValue(n, declaration, global.storage, baseExpr, global.read, global.write)
			}
			if base.aggregatePointer != nil {
				pointer, ok := types.Unalias(base.type_).(*types.Pointer)
				if !ok {
					l.errorAt(n, "aggregate pointer selector requires a pointer receiver")
					return zeroValue(l.pkg.TypesInfo.TypeOf(n))
				}
				base = l.loadAggregatePointer(base, pointer, n.X)
				base.type_ = pointer.Elem()
			}
			if base.aggregate != nil {
				if pointer, ok := types.Unalias(base.type_).(*types.Pointer); ok {
					base.type_ = pointer.Elem()
				}
				value := base
				var indexedPath []int
				if base.aggregateIndex != nil {
					indexedPath = append(indexedPath, base.aggregateIndex.path...)
				}
				var loadPath []aggregatePointerPathStep
				if base.aggregateLoad != nil {
					loadPath = append(loadPath, base.aggregateLoad.path...)
				}
				for _, index := range sel.Index() {
					if value.aggregate == nil || index < 0 || index >= len(value.aggregate.fields) {
						l.errorAt(n, "selector %s has no aggregate descriptor", n.Sel.Name)
						return zeroValue(l.pkg.TypesInfo.TypeOf(n))
					}
					value = value.aggregate.fields[index]
					indexedPath = append(indexedPath, index)
					loadPath = append(loadPath, aggregatePointerPathStep{index: index})
				}
				if base.aggregateIndex != nil {
					indexed := *base.aggregateIndex
					indexed.path = indexedPath
					value.aggregateIndex = &indexed
				}
				if base.aggregateLoad != nil {
					load := *base.aggregateLoad
					load.path = loadPath
					value.aggregateLoad = &load
				}
				return value
			}
			if field.Embedded() && l.currentArchetype != nil {
				if named, ok := namedType(field.Type()); ok {
					if binding, exists := l.archetypes[named]; exists {
						inMRO := false
						for _, member := range l.currentArchetype.MRO {
							if member == binding.declaration {
								inMRO = true
								break
							}
						}
						if inMRO {
							size := 0
							for _, inherited := range binding.declaration.Fields {
								size += inherited.Size
							}
							if size > len(base.slots) {
								l.errorAt(n, "embedded archetype base layout exceeds receiver")
								return lowerValue{}
							}
							value := lowerValue{type_: field.Type(), slots: base.slots[:size]}
							if base.places != nil {
								value.places = base.places[:size]
							}
							return value
						}
					}
				}
			}
			if base.entity != nil {
				return l.entityReferenceField(n, field, base)
			}
			if declaration, exists := l.archetypeFields[field]; exists {
				start, size := declaration.ReceiverOffset, declaration.Size
				if start+size > len(base.slots) {
					l.errorAt(n, "archetype field layout exceeds receiver")
					return lowerValue{}
				}
				value := lowerValue{type_: declaration.Type, slots: base.slots[start : start+size], places: base.places[start : start+size]}
				if declaration.ContainerKind != "" {
					_, key, element, _ := containerTypes(declaration.Type)
					stride := declaration.KeySize + declaration.ElementSize
					storage, read, write := archetypeStorageAccess(declaration.Storage, l.mode, l.phase)
					value.container = &containerValue{kind: declaration.ContainerKind, capacity: declaration.Capacity, stride: stride, keySize: declaration.KeySize, element: element, key: key, memoryStorage: storage, memoryBase: declaration.Offset + 1, memoryRead: read, memoryWrite: write}
				}
				return value
			}
			if pointer, ok := types.Unalias(base.type_).(*types.Pointer); ok {
				if base.pointer != nil {
					base = l.loadPointer(base, pointer, n.X)
				} else {
					base.type_ = pointer.Elem()
				}
			}
			offset := 0
			st, _ := types.Unalias(base.type_).Underlying().(*types.Struct)
			for _, index := range sel.Index() {
				if st == nil || index < 0 || index >= st.NumFields() {
					l.errorAt(n, "selector %s cannot be derived from runtime type %s", n.Sel.Name, base.type_)
					return zeroValue(l.pkg.TypesInfo.TypeOf(n))
				}
				for i := 0; i < index; i++ {
					offset += l.runtimeTypeOf(st.Field(i).Type()).Slots
				}
				base.type_ = st.Field(index).Type()
				st, _ = types.Unalias(base.type_).Underlying().(*types.Struct)
			}
			size := l.runtimeTypeOf(base.type_).Slots
			if offset < 0 || offset+size > len(base.slots) {
				l.errorAt(n, "selector %s layout %d:%d exceeds %s runtime slots %d", n.Sel.Name, offset, size, base.type_, len(base.slots))
				return zeroValue(l.pkg.TypesInfo.TypeOf(n))
			}
			base.slots = base.slots[offset : offset+size]
			if base.places != nil {
				base.places = base.places[offset : offset+size]
			}
			if base.pointerLoad != nil {
				load := *base.pointerLoad
				load.offset += offset
				load.size = size
				base.pointerLoad = &load
			}
			base.entity = nil
			if binding, exists := l.entityBinding(base.type_); exists {
				base.entity = &entityReferenceValue{binding: binding}
			}
			return base
		}
		l.errorAt(n, "unsupported selector %s", n.Sel.Name)
		return zeroValue(l.pkg.TypesInfo.TypeOf(n))
	case *ast.IndexExpr:
		base, index := l.expr(n.X), l.expr(n.Index)
		if base.type_ == nil || index.type_ == nil {
			return lowerValue{}
		}
		if base.aggregate != nil && base.levelGlobal == nil {
			arrayType := base.type_
			if pointer, ok := types.Unalias(arrayType).(*types.Pointer); ok {
				arrayType = pointer.Elem()
			}
			array, ok := types.Unalias(arrayType).Underlying().(*types.Array)
			if !ok {
				l.errorAt(n, "descriptor aggregate indexing requires an array")
				return lowerValue{}
			}
			constantIndex, constantOK := index.slots[0].(ir.Const)
			if !constantOK {
				result := l.indexAggregateArray(base, array, index, n, n.Index)
				if base.aggregateLoad != nil && result.aggregateIndex != nil {
					load := *base.aggregateLoad
					load.path = append(append([]aggregatePointerPathStep(nil), load.path...), aggregatePointerPathStep{dynamic: result.aggregateIndex.index, isDynamic: true})
					result.aggregateIndex = nil
					result.aggregateLoad = &load
				}
				return result
			}
			position := int(constantIndex.Value)
			if float64(position) != constantIndex.Value || position < 0 || position >= len(base.aggregate.fields) {
				l.errorAt(n.Index, "array index is out of bounds")
				return zeroValue(array.Elem())
			}
			result := base.aggregate.fields[position]
			if base.aggregateIndex != nil {
				indexed := *base.aggregateIndex
				indexed.path = append(append([]int(nil), indexed.path...), position)
				result.aggregateIndex = &indexed
			}
			if base.aggregateLoad != nil {
				load := *base.aggregateLoad
				load.path = append(append([]aggregatePointerPathStep(nil), load.path...), aggregatePointerPathStep{index: position})
				result.aggregateLoad = &load
			}
			return result
		}
		if base.variadic != nil {
			return l.indexVariadic(n, base, index)
		}
		if base.callableArray != nil {
			if len(index.slots) != 1 {
				l.errorAt(n.Index, "callable array index must be scalar")
				return lowerValue{}
			}
			if constantIndex, constantOK := index.slots[0].(ir.Const); constantOK {
				position := int(constantIndex.Value)
				if float64(position) != constantIndex.Value || position < 0 || position >= len(base.callableArray.elements) {
					l.errorAt(n.Index, "callable array index is out of bounds")
					return lowerValue{}
				}
				return lowerValue{type_: base.callableArray.element, callable: base.callableArray.elements[position]}
			}
			index = l.materialize("callable.array.index", index, n.Index)
			inBounds := l.pure(resource.RuntimeFunctionAnd, n,
				l.pure(resource.RuntimeFunctionGreaterOr, n, index.slots[0], ir.Const{}),
				l.pure(resource.RuntimeFunctionLess, n, index.slots[0], ir.Const{Value: float64(len(base.callableArray.elements))}))
			l.guard(n, inBounds)
			return lowerValue{type_: base.callableArray.element, callable: indexedCallableVariant(l, base.callableArray.elements, index, n)}
		}
		if base.levelGlobal != nil && len(base.levelGlobal.declaration.Elements) != 0 {
			array, ok := types.Unalias(base.type_).Underlying().(*types.Array)
			if !ok || len(index.slots) != 1 {
				l.errorAt(n, "nested level global array index must be scalar")
				return lowerValue{}
			}
			global := base.levelGlobal
			if constant, ok := index.slots[0].(ir.Const); ok {
				position := int(constant.Value)
				if position < 0 || position >= len(global.declaration.Elements) {
					l.errorAt(n, "array index is out of bounds")
					return lowerValue{}
				}
				element := global.declaration.Elements[position]
				baseExpr := global.base
				if element.RelativeOffset != 0 {
					baseExpr = l.pure(resource.RuntimeFunctionAdd, n, baseExpr, ir.Const{Value: float64(element.RelativeOffset)})
				}
				return l.lowerLevelGlobalValue(n, element, global.storage, baseExpr, global.read, global.write)
			}
			index = l.materialize("levelGlobal.index", index, n.Index)
			inBounds := l.pure(resource.RuntimeFunctionAnd, n,
				l.pure(resource.RuntimeFunctionGreaterOr, n, index.slots[0], ir.Const{}),
				l.pure(resource.RuntimeFunctionLess, n, index.slots[0], ir.Const{Value: float64(array.Len())}))
			l.guard(n, inBounds)
			offset := l.pure(resource.RuntimeFunctionMultiply, n, index.slots[0], ir.Const{Value: float64(global.declaration.ElementStride)})
			baseExpr := l.pure(resource.RuntimeFunctionAdd, n, global.base, offset)
			return l.lowerLevelGlobalValue(n, global.declaration.Elements[0], global.storage, baseExpr, global.read, global.write)
		}
		if pointer, ok := types.Unalias(base.type_).(*types.Pointer); ok {
			if array, isArray := types.Unalias(pointer.Elem()).Underlying().(*types.Array); isArray {
				if base.pointer != nil {
					if len(index.slots) != 1 {
						l.errorAt(n, "array index must be scalar")
						return zeroValue(array.Elem())
					}
					return l.loadPointerIndex(base, pointer, array, index, n)
				} else {
					base.type_ = pointer.Elem()
				}
			}
		}
		arr, ok := types.Unalias(base.type_).Underlying().(*types.Array)
		if !ok || len(index.slots) != 1 {
			l.errorAt(n, "only array indexing is supported")
			return lowerValue{}
		}
		handleSize := l.runtimeTypeOf(types.NewArray(arr.Elem(), 1)).Slots
		if base.stream != nil {
			handleSize = base.stream.width / base.stream.length
		}
		if c, ok := index.slots[0].(ir.Const); ok {
			size := handleSize
			start := int(c.Value) * size
			if start < 0 || int(c.Value) >= int(arr.Len()) {
				l.errorAt(n, "array index is out of bounds")
				return lowerValue{}
			}
			if base.stream != nil {
				stream := indexedStreamValue(l, n, base.stream, arr.Elem(), ir.Const{Value: float64(start)})
				return lowerStreamValue(l, n, arr.Elem(), stream)
			}
			v := lowerValue{type_: arr.Elem(), slots: base.slots[start : start+size], entityField: base.entityField, stream: base.stream, immutablePackage: base.immutablePackage}
			if base.places != nil {
				v.places = base.places[start : start+size]
			}
			if base.pointerLoad != nil {
				load := *base.pointerLoad
				load.offset += start
				load.size = size
				v.pointerLoad = &load
			}
			if binding, exists := l.entityBinding(arr.Elem()); exists {
				v.entity = &entityReferenceValue{binding: binding}
			}
			return v
		}
		if base.stream != nil {
			index = l.materialize("stream.index", index, n.Index)
			inBounds := l.pure(resource.RuntimeFunctionAnd, n,
				l.pure(resource.RuntimeFunctionGreaterOr, n, index.slots[0], ir.Const{}),
				l.pure(resource.RuntimeFunctionLess, n, index.slots[0], ir.Const{Value: float64(arr.Len())}))
			l.guard(n, inBounds)
			offset := l.pure(resource.RuntimeFunctionMultiply, n, index.slots[0], ir.Const{Value: float64(handleSize)})
			stream := indexedStreamValue(l, n, base.stream, arr.Elem(), offset)
			return lowerStreamValue(l, n, arr.Elem(), stream)
		}
		if base.pointerLoad != nil {
			return l.loadPointerArrayIndex(base, arr, index, n)
		}
		if base.immutablePackage && len(base.places) == 0 {
			return l.indexImmutablePackageArray(base, arr, index, n)
		}
		index = l.materialize("array.index", index, n.Index)
		immutablePackage := base.immutablePackage
		if len(base.places) != len(base.slots) || len(base.places) == 0 {
			base = l.materializeAddressable("array.index", base, n.X)
		}
		inBounds := l.pure(resource.RuntimeFunctionAnd, n,
			l.pure(resource.RuntimeFunctionGreaterOr, n, index.slots[0], ir.Const{}),
			l.pure(resource.RuntimeFunctionLess, n, index.slots[0], ir.Const{Value: float64(arr.Len())}))
		l.guard(n, inBounds)
		size := l.runtimeTypeOf(types.NewArray(arr.Elem(), 1)).Slots
		v := lowerValue{type_: arr.Elem(), slots: make([]ir.Expr, size), places: make([]ir.Place, size), entityField: base.entityField, immutablePackage: immutablePackage}
		for offset := 0; offset < size; offset++ {
			place := l.dynamicArrayPlace(base.places[0], index.slots[0], int(arr.Len()), size, offset, n)
			if place == nil {
				return zeroValue(arr.Elem())
			}
			v.places[offset], v.slots[offset] = place, ir.Load{Place: place}
		}
		if binding, exists := l.entityBinding(arr.Elem()); exists {
			v.entity = &entityReferenceValue{binding: binding}
		}
		return v
	case *ast.TypeAssertExpr:
		if n.Type == nil {
			l.errorAt(n, "type assertions are only valid in a static type switch")
			return lowerValue{}
		}
		value := l.expr(n.X)
		target := l.resolveType(l.pkg.TypesInfo.TypeOf(n.Type))
		if value.interface_ != nil {
			result, ok := l.interfaceAssertion(n, value, target, false)
			_ = ok
			return result
		}
		if value.type_ == nil || target == nil || (!types.AssignableTo(value.type_, target) && !types.Identical(value.type_, target)) {
			l.errorAt(n, "static type assertion from %s to %s cannot succeed", value.type_, target)
			return zeroValue(target)
		}
		value.type_ = target
		return value
	case *ast.CallExpr:
		return l.call(n)
	}
	return zeroValue(l.pkg.TypesInfo.TypeOf(expr))
}

func (l *lowerer) interfaceAssertion(node ast.Node, value lowerValue, target types.Type, commaOK bool) (lowerValue, lowerValue) {
	variant := value.interface_
	result := l.newDescriptorCell("interface.assert.value", target, node)
	ok := l.allocZeroed("interface.assert.ok", types.Typ[types.Bool], node)
	var matches ir.Expr = ir.Const{}
	matching := make([]bool, len(variant.alternatives))
	for index, alternative := range variant.alternatives {
		if types.AssignableTo(alternative.type_, target) || types.Identical(alternative.type_, target) {
			matching[index] = true
			matches = l.pure(resource.RuntimeFunctionOr, node, matches, l.pure(resource.RuntimeFunctionEqual, node, variant.tag.slots[0], ir.Const{Value: float64(index)}))
		}
	}
	if !commaOK {
		l.guardWith(node, matches, "static interface type assertion failed", true)
	}
	merge, invalid := l.newBlock(), l.newBlock()
	blocks := make([]*ir.Block, len(variant.alternatives))
	cases := make([]ir.SwitchCase, len(blocks))
	for index := range blocks {
		blocks[index] = l.newBlock()
		cases[index] = ir.SwitchCase{Value: float64(index), Target: blocks[index].ID}
	}
	_ = l.builder.Switch(variant.tag.slots[0], cases, invalid)
	for index, alternative := range variant.alternatives {
		l.setCurrent(blocks[index])
		if matching[index] {
			l.storeDescriptor(result, alternative, node)
			l.store(ok, scalarValue(ir.Const{Value: 1}, types.Typ[types.Bool]), node)
		}
		l.jump(merge)
	}
	l.setCurrent(invalid)
	if variant.persistent && commaOK {
		l.jump(merge)
	} else {
		_ = l.builder.MarkUnreachable()
	}
	l.setCurrent(merge)
	return result, ok
}

func (l *lowerer) requireIntegerDivisor(node ast.Node, divisor ir.Expr, resultType types.Type, operation token.Token) bool {
	kind, supported := runtimeBasicKind(resultType)
	if !supported {
		l.errorAt(node, "runtime arithmetic does not support %s", resultType)
		return false
	}
	if kind != types.Int {
		return true
	}
	message := "integer division by zero"
	if operation == token.REM || operation == token.REM_ASSIGN {
		message = "integer remainder by zero"
	}
	l.guardWith(node, l.pure(resource.RuntimeFunctionNotEqual, node, divisor, ir.Const{}), message, true)
	return true
}

func (l *lowerer) pointerEqual(left, right lowerValue, node ast.Node) ir.Expr {
	if left.persistentPointer != nil || right.persistentPointer != nil {
		storage := ""
		if left.persistentPointer != nil {
			storage = left.persistentPointer.storage
		} else {
			storage = right.persistentPointer.storage
		}
		leftAddress, leftOK := l.persistentPointerAddress(left, storage, node)
		rightAddress, rightOK := l.persistentPointerAddress(right, storage, node)
		if !leftOK || !rightOK {
			return ir.Const{}
		}
		return l.pure(resource.RuntimeFunctionEqual, node, leftAddress, rightAddress)
	}
	leftTargets, rightTargets := [][]ir.Place{left.places}, [][]ir.Place{right.places}
	var leftTag, rightTag ir.Expr
	if left.pointer != nil {
		leftTargets = left.pointer.alternatives
		leftTag = left.pointer.tag.slots[0]
	}
	if right.pointer != nil {
		rightTargets = right.pointer.alternatives
		rightTag = right.pointer.tag.slots[0]
	}
	result := ir.Expr(ir.Const{})
	for leftIndex, leftPlaces := range leftTargets {
		for rightIndex, rightPlaces := range rightTargets {
			if !samePlaces(leftPlaces, rightPlaces) {
				continue
			}
			condition := ir.Expr(ir.Const{Value: 1})
			if leftTag != nil {
				condition = l.pure(resource.RuntimeFunctionAnd, node, condition, l.pure(resource.RuntimeFunctionEqual, node, leftTag, ir.Const{Value: float64(leftIndex)}))
			}
			if rightTag != nil {
				condition = l.pure(resource.RuntimeFunctionAnd, node, condition, l.pure(resource.RuntimeFunctionEqual, node, rightTag, ir.Const{Value: float64(rightIndex)}))
			}
			result = l.pure(resource.RuntimeFunctionOr, node, result, condition)
		}
	}
	return result
}

func (l *lowerer) aggregatePointerEqual(left, right lowerValue, node ast.Node) ir.Expr {
	targets := func(value lowerValue) ([]lowerValue, ir.Expr) {
		if value.aggregatePointer != nil {
			return value.aggregatePointer.alternatives, value.aggregatePointer.tag.slots[0]
		}
		return []lowerValue{value}, nil
	}
	leftTargets, leftTag := targets(left)
	rightTargets, rightTag := targets(right)
	result := ir.Expr(ir.Const{})
	for leftIndex, leftTarget := range leftTargets {
		for rightIndex, rightTarget := range rightTargets {
			condition := ir.Expr(ir.Const{Value: 1})
			if leftTarget.persistentPointer != nil || rightTarget.persistentPointer != nil {
				storage := ""
				if leftTarget.persistentPointer != nil {
					storage = leftTarget.persistentPointer.storage
				} else {
					storage = rightTarget.persistentPointer.storage
				}
				leftAddress, leftOK := l.persistentPointerAddress(leftTarget, storage, node)
				rightAddress, rightOK := l.persistentPointerAddress(rightTarget, storage, node)
				if !leftOK || !rightOK {
					continue
				}
				condition = l.pure(resource.RuntimeFunctionEqual, node, leftAddress, rightAddress)
			} else if !sameAggregatePointerTarget(leftTarget, rightTarget) {
				continue
			}
			if leftTag != nil {
				condition = l.pure(resource.RuntimeFunctionAnd, node, condition, l.pure(resource.RuntimeFunctionEqual, node, leftTag, ir.Const{Value: float64(leftIndex)}))
			}
			if rightTag != nil {
				condition = l.pure(resource.RuntimeFunctionAnd, node, condition, l.pure(resource.RuntimeFunctionEqual, node, rightTag, ir.Const{Value: float64(rightIndex)}))
			}
			result = l.pure(resource.RuntimeFunctionOr, node, result, condition)
		}
	}
	return result
}

func (l *lowerer) dynamicArrayPlace(base ir.Place, index ir.Expr, length, stride, offset int, node ast.Node) ir.Place {
	scale := func(value ir.Expr, factor int) ir.Expr {
		if factor == 1 {
			return value
		}
		return l.pure(resource.RuntimeFunctionMultiply, node, value, ir.Const{Value: float64(factor)})
	}
	add := func(values ...ir.Expr) ir.Expr {
		filtered := make([]ir.Expr, 0, len(values))
		for _, value := range values {
			if constant, ok := value.(ir.Const); ok && constant.Value == 0 {
				continue
			}
			filtered = append(filtered, value)
		}
		if len(filtered) == 0 {
			return ir.Const{}
		}
		if len(filtered) == 1 {
			return filtered[0]
		}
		return l.pure(resource.RuntimeFunctionAdd, node, filtered...)
	}
	switch place := base.(type) {
	case ir.LocalPlace:
		return l.indexedLocal(place, index, length, stride, offset, node)
	case ir.IndexedLocalPlace:
		combined := add(scale(place.Index, place.Stride), scale(index, stride), ir.Const{Value: float64(place.Offset + offset)})
		localSlots := l.builder.Function().Locals[place.ID].Slots
		return l.indexedLocal(ir.LocalPlace{ID: place.ID, Name: place.Name, Offset: place.Base}, combined, localSlots-place.Base, 1, 0, node)
	case ir.MemoryPlace:
		combined := add(scale(place.Index, place.Stride), scale(index, stride))
		return l.memory(place.Storage, combined, 1, place.Offset+offset, place.Read, place.Write, node)
	default:
		l.errorAt(node, "dynamic array index does not support place %T", base)
		return nil
	}
}

func (l *lowerer) composite(n *ast.CompositeLit) lowerValue {
	t := l.resolveType(l.pkg.TypesInfo.TypeOf(n))
	if kind := containsResourceHandle(t); kind != "" {
		l.errorAt(n, "%s resource handle aggregates can only come from declared resource fields", kind)
		return zeroValue(t)
	}
	if array, ok := callableArrayType(t); ok {
		out := l.newCallableArray("literal.callable", t, n)
		index := 0
		for _, element := range n.Elts {
			valueExpression := element
			if keyed, keyedOK := element.(*ast.KeyValueExpr); keyedOK {
				constantIndex := l.pkg.TypesInfo.Types[keyed.Key].Value
				if constantIndex == nil {
					l.errorAt(keyed.Key, "callable array literal index must be a representable constant")
					continue
				}
				index64, exact := constant.Int64Val(constantIndex)
				if !exact {
					l.errorAt(keyed.Key, "callable array literal index must be a representable constant")
					continue
				}
				index = int(index64)
				valueExpression = keyed.Value
			}
			if index < 0 || index >= int(array.Len()) {
				l.errorAt(valueExpression, "callable array literal index is out of bounds")
				index++
				continue
			}
			if identifier, nilValue := valueExpression.(*ast.Ident); nilValue && identifier.Name == "nil" {
				index++
				continue
			}
			callable, callableOK := l.staticCallable(valueExpression)
			if !callableOK {
				l.errorAt(valueExpression, "callable array element requires a statically finite target")
				index++
				continue
			}
			l.storeCallableCell(out.callableArray.elements[index], callable, valueExpression)
			index++
		}
		return out
	}
	if l.containsAggregateDescriptor(t) {
		return l.aggregateComposite(n, t)
	}
	out := l.zeroRuntimeValue(t)
	if st, ok := types.Unalias(t).Underlying().(*types.Struct); ok {
		values := make([]lowerValue, st.NumFields())
		for i := range values {
			values[i] = l.zeroRuntimeValue(st.Field(i).Type())
		}
		pos := 0
		for _, elt := range n.Elts {
			if kv, ok := elt.(*ast.KeyValueExpr); ok {
				name := kv.Key.(*ast.Ident).Name
				for i := 0; i < st.NumFields(); i++ {
					if st.Field(i).Name() == name {
						value := l.expr(kv.Value)
						if value.entity != nil {
							binding, allowed := l.entityBinding(st.Field(i).Type())
							if !allowed || binding.declaration != value.entity.binding.declaration {
								l.errorAt(kv.Value, "EntityRef.Get view struct field must keep one static archetype target")
								break
							}
							values[i] = value
						} else {
							values[i] = l.materialize("literal.field", value, kv.Value)
						}
						break
					}
				}
			} else if pos < len(values) {
				value := l.expr(elt)
				if value.entity != nil {
					binding, allowed := l.entityBinding(st.Field(pos).Type())
					if !allowed || binding.declaration != value.entity.binding.declaration {
						l.errorAt(elt, "EntityRef.Get view struct field must keep one static archetype target")
					} else {
						values[pos] = value
					}
				} else {
					values[pos] = l.materialize("literal.field", value, elt)
				}
				pos++
			}
		}
		out.slots = out.slots[:0]
		for _, v := range values {
			out.slots = append(out.slots, v.slots...)
		}
		return out
	}
	if array, ok := types.Unalias(t).Underlying().(*types.Array); ok {
		elementSlots := l.runtimeTypeOf(types.NewArray(array.Elem(), 1)).Slots
		index := 0
		for _, elt := range n.Elts {
			valueExpr := elt
			if kv, ok := elt.(*ast.KeyValueExpr); ok {
				constantIndex := l.pkg.TypesInfo.Types[kv.Key].Value
				if constantIndex == nil {
					l.errorAt(kv.Key, "array literal index must be constant")
					continue
				}
				index64, exact := constant.Int64Val(constantIndex)
				if !exact {
					l.errorAt(kv.Key, "array literal index is not representable")
					continue
				}
				index = int(index64)
				valueExpr = kv.Value
			}
			value := l.expr(valueExpr)
			if value.entity != nil {
				binding, allowed := l.entityBinding(array.Elem())
				if !allowed || binding.declaration != value.entity.binding.declaration {
					l.errorAt(valueExpr, "EntityRef.Get view array element must keep one static archetype target")
					index++
					continue
				}
			}
			value = l.materialize("literal.element", value, valueExpr)
			start := index * elementSlots
			if start < 0 || start+elementSlots > len(out.slots) || len(value.slots) != elementSlots {
				l.errorAt(valueExpr, "array literal element layout is invalid")
				continue
			}
			copy(out.slots[start:start+elementSlots], value.slots)
			index++
		}
		return out
	}
	l.errorAt(n, "unsupported composite literal type %s", t)
	return out
}

func (l *lowerer) aggregateComposite(n *ast.CompositeLit, t types.Type) lowerValue {
	structure, structureOK := types.Unalias(t).Underlying().(*types.Struct)
	_, arrayOK := types.Unalias(t).Underlying().(*types.Array)
	result := l.newAggregateCell("literal.aggregate", t, n)
	position := 0
	for _, element := range n.Elts {
		valueExpression := ast.Expr(element)
		fieldIndex := position
		if keyed, ok := element.(*ast.KeyValueExpr); ok {
			switch {
			case structureOK:
				identifier, identifierOK := keyed.Key.(*ast.Ident)
				if !identifierOK {
					l.errorAt(keyed.Key, "struct literal field name must be an identifier")
					continue
				}
				fieldIndex = -1
				for index := 0; index < structure.NumFields(); index++ {
					if structure.Field(index).Name() == identifier.Name {
						fieldIndex = index
						break
					}
				}
				if fieldIndex < 0 {
					l.errorAt(keyed.Key, "unknown struct literal field %s", identifier.Name)
					continue
				}
			case arrayOK:
				constantIndex := l.pkg.TypesInfo.Types[keyed.Key].Value
				if constantIndex == nil {
					l.errorAt(keyed.Key, "array literal index must be constant")
					continue
				}
				index64, exact := constant.Int64Val(constantIndex)
				if !exact {
					l.errorAt(keyed.Key, "array literal index is not representable")
					continue
				}
				fieldIndex = int(index64)
			default:
				l.errorAt(n, "descriptor aggregate literal requires a struct or array type")
				continue
			}
			valueExpression = keyed.Value
		} else {
			position++
		}
		if fieldIndex < 0 || fieldIndex >= len(result.aggregate.fields) {
			l.errorAt(valueExpression, "descriptor aggregate literal index is out of bounds")
			continue
		}
		destination := result.aggregate.fields[fieldIndex]
		source := l.expr(valueExpression)
		switch {
		case destination.aggregate != nil:
			l.storeAggregate(destination, source, valueExpression)
		case destination.entity != nil:
			l.storeEntityView(destination, source, valueExpression)
		case destination.interface_ != nil:
			l.storeInterfaceValue(destination, source, valueExpression)
		case destination.aggregatePointer != nil:
			if !isAggregatePointerValue(source) && !source.nilPointer {
				l.errorAt(valueExpression, "aggregate pointer item requires an aggregate address or nil")
				continue
			}
			l.mergeAggregatePointerValue(destination, source, valueExpression)
		case destination.pointer != nil:
			if !isStaticPointer(source) {
				l.errorAt(valueExpression, "pointer aggregate item requires a finite pointer source")
				continue
			}
			l.mergePointerValue(destination, source, valueExpression)
		case destination.containerVariant != nil:
			if !isContainerValue(source) {
				l.errorAt(valueExpression, "container aggregate item requires a container source")
				continue
			}
			l.mergeContainerValue(destination, source, valueExpression)
		default:
			l.store(destination, source, valueExpression)
		}
	}
	return result
}

func (l *lowerer) indexVariadic(n *ast.IndexExpr, base, index lowerValue) lowerValue {
	if len(index.slots) != 1 {
		l.errorAt(n.Index, "variadic index must be scalar")
		return zeroValue(l.pkg.TypesInfo.TypeOf(n))
	}
	elements := base.variadic.elements
	if isFunctionType(base.variadic.element) {
		callables := base.variadic.callables
		if constantIndex, ok := index.slots[0].(ir.Const); ok {
			i := int(constantIndex.Value)
			if float64(i) != constantIndex.Value || i < 0 || i >= len(callables) {
				l.errorAt(n.Index, "variadic index is out of bounds")
				return lowerValue{}
			}
			return lowerValue{type_: base.variadic.element, callable: callables[i]}
		}
		if len(callables) == 0 {
			l.errorAt(n.Index, "cannot dynamically index an empty variadic parameter")
			return lowerValue{}
		}
		index = l.materialize("variadic.callable.index", index, n.Index)
		inBounds := l.pure(resource.RuntimeFunctionAnd, n,
			l.pure(resource.RuntimeFunctionGreaterOr, n, index.slots[0], ir.Const{}),
			l.pure(resource.RuntimeFunctionLess, n, index.slots[0], ir.Const{Value: float64(len(callables))}))
		l.guard(n, inBounds)
		return lowerValue{type_: base.variadic.element, callable: indexedCallableVariant(l, callables, index, n)}
	}
	if constantIndex, ok := index.slots[0].(ir.Const); ok {
		i := int(constantIndex.Value)
		if float64(i) != constantIndex.Value || i < 0 || i >= len(elements) {
			l.errorAt(n.Index, "variadic index is out of bounds")
			return zeroValue(l.pkg.TypesInfo.TypeOf(n))
		}
		return elements[i]
	}
	if len(elements) == 0 {
		l.errorAt(n.Index, "cannot dynamically index an empty variadic parameter")
		return zeroValue(l.pkg.TypesInfo.TypeOf(n))
	}
	index = l.materialize("variadic.index", index, n.Index)
	inBounds := l.pure(resource.RuntimeFunctionAnd, n,
		l.pure(resource.RuntimeFunctionGreaterOr, n, index.slots[0], ir.Const{}),
		l.pure(resource.RuntimeFunctionLess, n, index.slots[0], ir.Const{Value: float64(len(elements))}))
	l.guard(n, inBounds)
	size := irTypeOf(base.variadic.element).Slots
	backing := l.builder.NewLocal("variadic.data", ir.Type{Name: base.type_.String() + ".args", Slots: len(elements) * size})
	places := ir.Places(backing)
	for i, element := range elements {
		start := i * size
		if len(element.slots) != size {
			l.errorAt(n, "variadic element %d has %d slots; expected %d", i, len(element.slots), size)
			continue
		}
		if err := l.builder.Store(places[start:start+size], ir.Value{Type: irTypeOf(base.variadic.element), Slots: element.slots}, sourcePos(l.pkg, n.Pos())); err != nil {
			l.errorAt(n, "%v", err)
		}
	}
	first := places[0].(ir.LocalPlace)
	result := lowerValue{type_: base.variadic.element, slots: make([]ir.Expr, size), places: make([]ir.Place, size)}
	for offset := 0; offset < size; offset++ {
		place := l.indexedLocal(first, index.slots[0], len(elements), size, offset, n)
		result.places[offset], result.slots[offset] = place, ir.Load{Place: place}
	}
	return result
}

func (l *lowerer) shortCircuit(n *ast.BinaryExpr) lowerValue {
	if left, ok := constantBool(l.pkg, n.X); ok {
		if (n.Op == token.LAND && !left) || (n.Op == token.LOR && left) {
			value := 0.0
			if left {
				value = 1
			}
			return lowerValue{type_: l.pkg.TypesInfo.TypeOf(n), slots: []ir.Expr{ir.Const{Value: value}}}
		}
		return l.expr(n.Y)
	}
	result := l.alloc("shortcircuit", l.pkg.TypesInfo.TypeOf(n))
	left := l.materialize("shortcircuit.left", l.expr(n.X), n.X)
	rightBlock, shortBlock, merge := l.newBlock(), l.newBlock(), l.newBlock()
	if n.Op == token.LAND {
		_ = l.builder.Branch(left.slots[0], rightBlock, shortBlock)
	} else {
		_ = l.builder.Branch(left.slots[0], shortBlock, rightBlock)
	}
	l.setCurrent(shortBlock)
	l.store(result, left, n.X)
	l.jump(merge)
	l.setCurrent(rightBlock)
	var right lowerValue
	l.dynamic(func() { right = l.expr(n.Y) })
	l.store(result, right, n.Y)
	l.jump(merge)
	l.setCurrent(merge)
	return result
}

func calledFunc(pkg *packages.Package, call *ast.CallExpr) *types.Func {
	obj := calledObject(pkg, call.Fun)
	fn, _ := obj.(*types.Func)
	return fn
}

func (l *lowerer) staticCallable(expr ast.Expr) (*staticCallable, bool) {
	for {
		paren, ok := expr.(*ast.ParenExpr)
		if !ok {
			break
		}
		expr = paren.X
	}
	if literal, ok := expr.(*ast.FuncLit); ok {
		captures, callables := l.captureBindings()
		return &staticCallable{literal: literal, pkg: l.pkg, captures: captures, callables: callables, substitutions: l.captureTypeSubstitutions()}, true
	}
	if indexed, ok := expr.(*ast.IndexExpr); ok {
		if _, callableArray := callableArrayType(l.resolveType(l.pkg.TypesInfo.TypeOf(indexed.X))); callableArray {
			value := l.expr(indexed)
			return value.callable, value.callable != nil
		}
	}
	if selector, ok := expr.(*ast.SelectorExpr); ok {
		if selection := l.pkg.TypesInfo.Selections[selector]; selection != nil && selection.Kind() == types.MethodExpr {
			fn, _ := selection.Obj().(*types.Func)
			if fn == nil {
				return nil, false
			}
			if symbol, exists := catalog.LookupObject(fn); exists {
				return l.catalogCallable(symbol, nil), true
			}
			callable := &staticCallable{function: fn, pkg: l.packages[fn.Pkg()], substitutions: l.callableTypeSubstitutions(fn, selector)}
			if signature, ok := fn.Type().(*types.Signature); ok && signature.Recv() != nil {
				if _, interfaceReceiver := types.Unalias(signature.Recv().Type()).Underlying().(*types.Interface); interfaceReceiver {
					callable.interfaceMethod = fn
				}
			}
			return callable, true
		} else if selection != nil && selection.Kind() == types.MethodVal {
			fn, _ := selection.Obj().(*types.Func)
			if fn == nil {
				return nil, false
			}
			receiverValue := l.callReceiver(selector, fn)
			if isContainerValue(receiverValue) {
				receiverValue = l.copyContainerValue("bound.container", receiverValue, selector)
			} else if isStaticPointer(receiverValue) {
				if receiverValue.aggregatePointer != nil || receiverValue.aggregate != nil && isPointerType(receiverValue.type_) {
					receiverValue = l.copyAggregatePointerValue("bound.aggregate.pointer", receiverValue, selector)
				} else {
					receiverValue = l.copyPointerValue("bound.pointer", receiverValue, selector)
				}
			}
			if symbol, exists := catalog.LookupObject(fn); exists {
				return l.catalogCallable(symbol, &receiverValue), true
			}
			receiver := callArgument{value: receiverValue}
			return &staticCallable{function: fn, pkg: l.packages[fn.Pkg()], receiver: &receiver, substitutions: l.callableTypeSubstitutions(fn, selector)}, true
		}
	}
	if call, ok := expr.(*ast.CallExpr); ok {
		value := l.call(call)
		if value.callable != nil {
			return value.callable, true
		}
		return nil, false
	}
	object := calledObject(l.pkg, expr)
	if callable, ok := l.lookupCallable(object); ok {
		return callable, true
	}
	if variable, ok := object.(*types.Var); ok && variable.Pkg() != nil && isFunctionType(variable.Type()) {
		owner := l.packages[variable.Pkg()]
		if owner == nil {
			return nil, false
		}
		initializer, exists := variableInitializer(owner, variable)
		if !exists {
			return nil, false
		}
		previous := l.pkg
		l.pkg = owner
		callable, resolved := l.staticCallable(initializer)
		l.pkg = previous
		return callable, resolved
	}
	fn, ok := object.(*types.Func)
	if !ok || fn.Pkg() == nil || fn.Type().(*types.Signature).Recv() != nil {
		return nil, false
	}
	if symbol, exists := catalog.LookupObject(fn); exists {
		recipe := catalog.LookupRecipe(symbol)
		if supportsRecipe(recipe) && recipe.Kind != catalog.RecipeCompileTime && recipe.Kind != catalog.RecipeForbidden {
			return l.catalogCallable(symbol, nil), true
		}
	}
	if symbol, exists := intrinsic.LookupObject(fn); exists && symbol.Kind == intrinsic.RuntimeFunction {
		return &staticCallable{identity: symbol.Package + "." + symbol.Name, intrinsic: func(call *ast.CallExpr, args []callArgument) lowerValue {
			values := make([]lowerValue, len(args))
			for index, argument := range args {
				if argument.callable != nil {
					l.errorAt(call, "intrinsic function value does not accept callable arguments")
					return zeroValue(l.pkg.TypesInfo.TypeOf(call))
				}
				values[index] = argument.value
			}
			return l.runtimeCall(call, symbol.Runtime, true, symbol.Prefix, values)
		}}, true
	}
	return &staticCallable{function: fn, pkg: l.packages[fn.Pkg()], substitutions: l.callableTypeSubstitutions(fn, expr)}, true
}

func (l *lowerer) catalogCallable(symbol *catalog.Symbol, receiver *lowerValue) *staticCallable {
	return &staticCallable{identity: symbol.Key(), intrinsic: func(call *ast.CallExpr, args []callArgument) lowerValue {
		if symbol.Internal {
			l.errorAt(call, "Sonolus API %s is not part of the public callback catalog", symbol.Key())
			return zeroValue(l.pkg.TypesInfo.TypeOf(call))
		}
		if !catalog.AllowsMode(symbol, string(l.mode)) {
			l.errorAt(call, "Sonolus API %s is not available in %s mode", symbol.Key(), l.mode)
			return zeroValue(l.pkg.TypesInfo.TypeOf(call))
		}
		if !catalog.AllowsPhase(symbol, l.phase) {
			l.errorAt(call, "Sonolus API %s cannot write during %s callback", symbol.Key(), l.phase)
			return zeroValue(l.pkg.TypesInfo.TypeOf(call))
		}
		actualReceiver := receiver
		if actualReceiver == nil && symbol.Receiver != "" {
			if len(args) == 0 || args[0].callable != nil {
				l.errorAt(call, "Sonolus method expression %s requires an explicit receiver", symbol.Key())
				return zeroValue(l.pkg.TypesInfo.TypeOf(call))
			}
			value := args[0].value
			actualReceiver = &value
			args = args[1:]
		}
		if actualReceiver != nil && isContainerValue(*actualReceiver) {
			switch symbol.Key() {
			case "sonolus.VarArray.SortFunc":
				if len(args) != 1 || args[0].callable == nil {
					l.errorAt(call, "VarArray.SortFunc method value requires one static comparator")
					return lowerValue{}
				}
				return l.dispatchContainerValue(call, *actualReceiver, func(alternative lowerValue) lowerValue {
					return l.sortContainerValue(call, alternative, args[0].callable)
				})
			case "sonolus.VarArray.IndexMinFunc", "sonolus.VarArray.IndexMaxFunc", "sonolus.VarArray.MinFunc", "sonolus.VarArray.MaxFunc":
				if len(args) != 1 || args[0].callable == nil {
					l.errorAt(call, "VarArray.%s method value requires one static comparator", symbol.Name)
					return zeroValue(l.pkg.TypesInfo.TypeOf(call))
				}
				return l.dispatchContainerValue(call, *actualReceiver, func(alternative lowerValue) lowerValue {
					return l.extremeContainerValue(call, alternative, args[0].callable, symbol.Name)
				})
			case "sonolus.VarArray.Extend":
				if len(args) != 1 || args[0].callable == nil {
					l.errorAt(call, "VarArray.Extend method value requires one static sequence")
					return lowerValue{}
				}
				return l.dispatchContainerValue(call, *actualReceiver, func(alternative lowerValue) lowerValue {
					return l.extendContainerValue(call, alternative, args[0].callable)
				})
			}
		}
		values := make([]lowerValue, 0, len(args)+1)
		if actualReceiver != nil {
			values = append(values, *actualReceiver)
		}
		for _, argument := range args {
			if argument.callable != nil {
				l.errorAt(call, "Sonolus API %s does not accept a callable argument through this method value", symbol.Key())
				return zeroValue(l.pkg.TypesInfo.TypeOf(call))
			}
			values = append(values, argument.value)
		}
		return l.lowerCatalogRecipe(call, symbol, values)
	}}
}

func (l *lowerer) userCallArguments(call *ast.CallExpr, receiver []callArgument) []callArgument {
	args := append([]callArgument(nil), receiver...)
	for i, argument := range call.Args {
		if call.Ellipsis.IsValid() && i == len(call.Args)-1 {
			value := l.expr(argument)
			if value.variadic == nil {
				l.errorAt(argument, "ellipsis expansion requires a variadic helper parameter")
				continue
			}
			for index, element := range value.variadic.elements {
				if isFunctionType(value.variadic.element) {
					args = append(args, callArgument{callable: value.variadic.callables[index]})
				} else {
					args = append(args, callArgument{value: l.materialize("call.arg", element, argument)})
				}
			}
			continue
		}
		if isFunctionType(l.pkg.TypesInfo.TypeOf(argument)) {
			callable, ok := l.staticCallable(argument)
			if !ok {
				l.errorAt(argument, "function argument must have one statically known callable target")
				continue
			}
			args = append(args, callArgument{callable: callable})
			continue
		}
		args = append(args, callArgument{value: l.materialize("call.arg", l.expr(argument), argument)})
	}
	return args
}

func (l *lowerer) call(n *ast.CallExpr) lowerValue {
	if typed := l.pkg.TypesInfo.Types[n]; typed.Value != nil {
		if value, representable := constantExpr(typed.Value); representable {
			return lowerValue{type_: l.resolveType(typed.Type), slots: []ir.Expr{value}}
		}
	}
	if tv := l.pkg.TypesInfo.Types[n.Fun]; tv.IsType() {
		if typed := l.pkg.TypesInfo.Types[n]; typed.Value != nil {
			value, representable := constantExpr(typed.Value)
			if !representable {
				l.errorAt(n, "constant conversion is not representable at runtime")
				return lowerValue{}
			}
			return lowerValue{type_: typed.Type, slots: []ir.Expr{value}}
		}
		target := l.resolveType(l.pkg.TypesInfo.TypeOf(n))
		if isFunctionType(target) {
			callable, ok := l.staticCallable(n.Args[0])
			if !ok {
				l.errorAt(n, "function conversion to %s requires a statically known callable target", target)
				return lowerValue{type_: target}
			}
			return lowerValue{type_: target, callable: callable}
		}
		value := l.expr(n.Args[0])
		if value.entity != nil || l.containsEntityView(value.type_) {
			l.errorAt(n, "EntityRef.Get views cannot be converted or stored in interfaces")
			return zeroValue(target)
		}
		if _, interfaceTarget := types.Unalias(target).Underlying().(*types.Interface); interfaceTarget {
			return value
		}
		if value.nilPointer && isPointerType(target) {
			value.type_ = target
			return value
		}
		if len(value.slots) != l.runtimeTypeOf(target).Slots {
			l.errorAt(n, "conversion from %s to %s changes runtime layout", value.type_, target)
			return zeroValue(target)
		}
		targetKind, targetSupported := runtimeBasicKind(target)
		sourceKind, sourceSupported := runtimeBasicKind(value.type_)
		_, sourceTypeParameter := types.Unalias(value.type_).(*types.TypeParam)
		if targetKind != types.Invalid || sourceKind != types.Invalid || sourceTypeParameter {
			if !targetSupported || (!sourceSupported && !sourceTypeParameter) {
				l.errorAt(n, "runtime conversion from %s to %s is not supported", value.type_, target)
				return zeroValue(target)
			}
			if targetKind == types.Int && (sourceKind == types.Float64 || sourceTypeParameter) {
				value.slots[0] = ir.RuntimeCall{Function: resource.RuntimeFunctionTrunc, Args: []ir.Expr{value.slots[0]}, Result: irTypeOf(target), Pure: true, Pos: sourcePos(l.pkg, n.Pos())}
			}
		}
		value.type_ = target
		return value
	}
	funExpr := n.Fun
	for {
		if paren, ok := funExpr.(*ast.ParenExpr); ok {
			funExpr = paren.X
			continue
		}
		break
	}
	if literal, ok := funExpr.(*ast.FuncLit); ok {
		captures, callables := l.captureBindings()
		return l.inlineStaticCallable(n, &staticCallable{literal: literal, pkg: l.pkg, captures: captures, callables: callables, substitutions: l.captureTypeSubstitutions()}, l.userCallArguments(n, nil))
	}
	if selector, ok := funExpr.(*ast.SelectorExpr); ok {
		if selection := l.pkg.TypesInfo.Selections[selector]; selection != nil && selection.Kind() == types.MethodExpr {
			if callable, resolved := l.staticCallable(selector); resolved {
				return l.inlineStaticCallable(n, callable, l.userCallArguments(n, nil))
			}
		}
	}
	if _, directFunction := calledObject(l.pkg, funExpr).(*types.Func); !directFunction {
		if callable, ok := l.staticCallable(funExpr); ok && callable != nil {
			return l.inlineStaticCallable(n, callable, l.userCallArguments(n, nil))
		}
	}
	if object := calledObject(l.pkg, n.Fun); object != nil {
		if _, function := object.(*types.Func); !function {
			if callable, ok := l.staticCallable(n.Fun); ok {
				return l.inlineStaticCallable(n, callable, l.userCallArguments(n, nil))
			}
		}
	}
	if object := calledObject(l.pkg, n.Fun); object != nil {
		if builtin, ok := object.(*types.Builtin); ok {
			return l.builtinCall(n, builtin)
		}
	}
	fn := calledFunc(l.pkg, n)
	if fn == nil {
		if isFunctionType(l.pkg.TypesInfo.TypeOf(n.Fun)) {
			_ = l.expr(n.Fun)
			return zeroValue(l.pkg.TypesInfo.TypeOf(n))
		}
		l.errorAt(n, "callback call target must be a statically declared function")
		return zeroValue(l.pkg.TypesInfo.TypeOf(n))
	}
	var devirtualizedReceiver *lowerValue
	if selector, ok := n.Fun.(*ast.SelectorExpr); ok {
		if signature, ok := fn.Type().(*types.Signature); ok && signature.Recv() != nil {
			if _, interfaceReceiver := types.Unalias(signature.Recv().Type()).Underlying().(*types.Interface); interfaceReceiver {
				value := l.materialize("interface.receiver", l.expr(selector.X), selector.X)
				if value.interface_ != nil {
					return l.inlineInterfaceMethodAlternatives(n, fn, value, l.userCallArguments(n, nil))
				}
				if value.type_ != nil && !isInterfaceType(value.type_) {
					methodSet := types.NewMethodSet(value.type_)
					for index := 0; index < methodSet.Len(); index++ {
						method, _ := methodSet.At(index).Obj().(*types.Func)
						if method != nil && method.Name() == fn.Name() {
							fn = method
							devirtualizedReceiver = &value
							break
						}
					}
				}
			}
		}
	}
	if symbol, ok := catalog.LookupObject(fn); ok {
		if symbol.Internal {
			l.errorAt(n, "Sonolus API %s is not part of the public callback catalog", symbol.Key())
			return zeroValue(l.pkg.TypesInfo.TypeOf(n))
		}
		if !catalog.AllowsMode(symbol, string(l.mode)) {
			l.errorAt(n, "Sonolus API %s is not available in %s mode", symbol.Key(), l.mode)
			return zeroValue(l.pkg.TypesInfo.TypeOf(n))
		}
		if !catalog.AllowsPhase(symbol, l.phase) {
			l.errorAt(n, "Sonolus API %s cannot write during %s callback", symbol.Key(), l.phase)
			return zeroValue(l.pkg.TypesInfo.TypeOf(n))
		}
		if symbol.Key() == "sonolus.Ease" {
			return l.easeCall(n)
		}
		if symbol.Key() == "sonolus/play.debugAPI.Terminate" {
			if err := l.builder.MarkUnreachable(); err != nil {
				l.errorAt(n, "%v", err)
			}
			l.setCurrent(l.newBlock())
			return lowerValue{}
		}
		switch symbol.Key() {
		case "sonolus.Zero":
			target, exists := l.callTypeArgument(n, 0)
			if !exists {
				l.errorAt(n, "Zero requires a concrete type argument")
				return lowerValue{}
			}
			if isPointerType(target) {
				return lowerValue{type_: target, nilPointer: true}
			}
			if isFunctionType(target) || isInterfaceType(target) || isContainerType(target) || l.isCompileTimeOnlyValueType(target) || containsResourceHandle(target) != "" {
				l.errorAt(n, "Zero does not support compile-time-only type %s", target)
				return lowerValue{type_: target}
			}
			return l.zeroRuntimeValue(target)
		case "sonolus.SlotsOf":
			target, exists := l.callTypeArgument(n, 0)
			if !exists {
				l.errorAt(n, "SlotsOf requires a concrete type argument")
				return lowerValue{type_: types.Typ[types.Int], slots: []ir.Expr{ir.Const{}}}
			}
			slots, err := layoutSize(target)
			if err != nil {
				l.errorAt(n, "SlotsOf[%s]: %v", target, err)
				return lowerValue{type_: types.Typ[types.Int], slots: []ir.Expr{ir.Const{}}}
			}
			return lowerValue{type_: types.Typ[types.Int], slots: []ir.Expr{ir.Const{Value: float64(slots)}}}
		case "sonolus.RuntimeChecksEnabled":
			if len(n.Args) != 0 {
				l.errorAt(n, "RuntimeChecksEnabled does not accept arguments")
			}
			enabled := 0.0
			if l.checks != RuntimeChecksNone {
				enabled = 1
			}
			return lowerValue{type_: types.Typ[types.Bool], slots: []ir.Expr{ir.Const{Value: enabled}}}
		case "sonolus.Unreachable":
			if len(n.Args) != 1 {
				l.errorAt(n, "Unreachable requires one message")
				return lowerValue{}
			}
			messageValue := l.pkg.TypesInfo.Types[n.Args[0]].Value
			if messageValue == nil || messageValue.Kind() != constant.String {
				l.errorAt(n.Args[0], "Unreachable message must be a compile-time string")
				return lowerValue{}
			}
			l.errorAt(n, "%s", constant.StringVal(messageValue))
			return lowerValue{}
		case "sonolus.Terminate", "sonolus.Notify":
			if len(n.Args) != 1 {
				l.errorAt(n, "%s requires one message", symbol.Name)
				return lowerValue{}
			}
			messageValue := l.pkg.TypesInfo.Types[n.Args[0]].Value
			if messageValue == nil || messageValue.Kind() != constant.String {
				l.errorAt(n.Args[0], "%s message must be a compile-time string", symbol.Name)
				return lowerValue{}
			}
			message := constant.StringVal(messageValue)
			if symbol.Name == "Notify" {
				l.notifyRuntime(n, message)
			} else {
				l.terminateRuntime(n, message)
			}
			return lowerValue{}
		case "sonolus.Assert", "sonolus.Require", "sonolus.StaticAssert":
			if len(n.Args) != 2 {
				l.errorAt(n, "%s requires condition and message", symbol.Name)
				return lowerValue{}
			}
			messageValue := l.pkg.TypesInfo.Types[n.Args[1]].Value
			if messageValue == nil || messageValue.Kind() != constant.String {
				l.errorAt(n.Args[1], "%s message must be a compile-time string", symbol.Name)
				return lowerValue{}
			}
			condition := l.expr(n.Args[0])
			if len(condition.slots) != 1 {
				l.errorAt(n.Args[0], "%s condition must be scalar", symbol.Name)
				return lowerValue{}
			}
			message := constant.StringVal(messageValue)
			if symbol.Name == "StaticAssert" {
				value, ok := condition.slots[0].(ir.Const)
				if !ok {
					l.errorAt(n.Args[0], "StaticAssert condition must be a compile-time constant")
				} else if value.Value == 0 {
					l.errorAt(n, "%s", message)
				}
				return lowerValue{}
			}
			if value, ok := condition.slots[0].(ir.Const); ok {
				if value.Value == 0 {
					l.terminateRuntime(n, message)
				}
				return lowerValue{}
			}
			l.guardWith(n, condition.slots[0], message, symbol.Name == "Require")
			return lowerValue{}
		case "sonolus.VarArray.SortFunc":
			return l.sortContainerCall(n, fn)
		case "sonolus.VarArray.IndexMinFunc", "sonolus.VarArray.IndexMaxFunc", "sonolus.VarArray.MinFunc", "sonolus.VarArray.MaxFunc":
			return l.extremeContainerCall(n, fn, symbol.Name)
		case "sonolus.VarArray.Extend":
			return l.extendContainerCall(n, fn)
		case "sonolus.SortLinkedEntities", "sonolus.SortDoublyLinkedEntities":
			return l.sortLinkedEntitiesCall(n, symbol.Name == "SortDoublyLinkedEntities")
		}
	}
	_, catalogFunction := catalog.LookupObject(fn)
	_, intrinsicFunction := intrinsic.LookupObject(fn)
	if !catalogFunction && !intrinsicFunction {
		if fn.Pkg() != nil && (fn.Pkg().Path() == "math" || fn.Pkg().Path() == "math/rand") {
			l.errorAt(n, "standard library symbol %s is not a Sonolus intrinsic", fn.FullName())
			return zeroValue(l.pkg.TypesInfo.TypeOf(n))
		}
		receiver := make([]callArgument, 0, 1)
		if sel, ok := n.Fun.(*ast.SelectorExpr); ok && l.pkg.TypesInfo.Selections[sel] != nil && !l.isFacadeReceiver(sel.X) {
			if devirtualizedReceiver != nil {
				receiver = append(receiver, callArgument{value: *devirtualizedReceiver})
			} else {
				receiver = append(receiver, callArgument{value: l.callReceiver(sel, fn)})
			}
		}
		return l.inlineCallArguments(n, fn, l.userCallArguments(n, receiver))
	}
	args := make([]lowerValue, 0, len(n.Args)+1)
	if sel, ok := n.Fun.(*ast.SelectorExpr); ok && l.pkg.TypesInfo.Selections[sel] != nil {
		if !l.isFacadeReceiver(sel.X) {
			args = append(args, l.callReceiver(sel, fn))
		}
	}
	for _, arg := range n.Args {
		args = append(args, l.materialize("call.arg", l.expr(arg), arg))
	}
	if symbol, ok := catalog.LookupObject(fn); ok {
		return l.lowerCatalogRecipe(n, symbol, args)
	}
	if symbol, ok := intrinsic.LookupObject(fn); ok {
		if symbol.Kind != intrinsic.RuntimeFunction {
			l.errorAt(n, "constant intrinsic cannot be called")
			return lowerValue{}
		}
		if symbol.Package == "math" && symbol.Name == "Round" && len(args) == 1 && len(args[0].slots) == 1 {
			value := args[0].slots[0]
			magnitude := l.pure(resource.RuntimeFunctionFloor, n,
				l.pure(resource.RuntimeFunctionAdd, n,
					l.pure(resource.RuntimeFunctionAbs, n, value),
					ir.Const{Value: 0.5},
				),
			)
			return scalarValue(l.pure(resource.RuntimeFunctionMultiply, n,
				l.pure(resource.RuntimeFunctionSign, n, value),
				magnitude,
			), l.pkg.TypesInfo.TypeOf(n))
		}
		if symbol.Package == "math/rand" && symbol.Name == "Intn" && len(args) == 1 && len(args[0].slots) == 1 {
			if value, ok := args[0].slots[0].(ir.Const); ok && value.Value <= 0 {
				l.errorAt(n.Args[0], "rand.Intn constant bound must be positive")
				return zeroValue(l.pkg.TypesInfo.TypeOf(n))
			} else if !ok {
				l.guardWith(n,
					l.pure(resource.RuntimeFunctionGreater, n, args[0].slots[0], ir.Const{}),
					"rand.Intn bound must be positive",
					true,
				)
			}
		}
		return l.runtimeCall(n, symbol.Runtime, true, symbol.Prefix, args)
	}
	return zeroValue(l.pkg.TypesInfo.TypeOf(n))
}

func (l *lowerer) lowerCatalogRecipe(n *ast.CallExpr, symbol *catalog.Symbol, args []lowerValue) lowerValue {
	recipe := catalog.LookupRecipe(symbol)
	if recipe.Kind != catalog.RecipeContainer {
		for _, arg := range args {
			if arg.entity != nil {
				l.errorAt(n, "EntityRef.Get views can only be passed to inlined user helpers or local catalog containers")
				return zeroValue(l.pkg.TypesInfo.TypeOf(n))
			}
		}
	}
	if !supportsRecipe(recipe) {
		l.errorAt(n, "Sonolus API %s references unimplemented %s recipe %q", symbol.Key(), recipe.Kind, recipe.Operation)
		return zeroValue(l.pkg.TypesInfo.TypeOf(n))
	}
	switch recipe.Kind {
	case catalog.RecipeAggregate:
		return l.aggregateCall(n, recipe.Operation, args)
	case catalog.RecipeResource:
		return l.resourceCall(n, recipe.Operation, args)
	case catalog.RecipeContainer:
		return l.containerCall(n, recipe.Operation, args)
	case catalog.RecipeMemory:
		return l.memoryCall(n, recipe, args)
	case catalog.RecipeRuntime:
		return l.runtimeCall(n, recipe.Runtime, symbol.Effect != catalog.EffectWrite, recipe.Prefix, args)
	default:
		l.errorAt(n, "Sonolus API %s cannot be lowered in callbacks: %s", symbol.Key(), recipe.Reason)
		return zeroValue(l.pkg.TypesInfo.TypeOf(n))
	}
}

func (l *lowerer) inlineInterfaceMethodAlternatives(call *ast.CallExpr, method *types.Func, value lowerValue, args []callArgument) lowerValue {
	variant := value.interface_
	if variant == nil || len(variant.tag.slots) != 1 || len(variant.alternatives) == 0 {
		l.errorAt(call, "static interface variant has no alternatives")
		return zeroValue(l.pkg.TypesInfo.TypeOf(call))
	}
	resultType := l.pkg.TypesInfo.TypeOf(call)
	var result lowerValue
	if resultType != nil {
		result = l.newDescriptorCell("interface.call.result", resultType, call)
	}
	merge, invalid := l.newBlock(), l.newBlock()
	blocks := make([]*ir.Block, len(variant.alternatives))
	cases := make([]ir.SwitchCase, len(blocks))
	for index := range blocks {
		blocks[index] = l.newBlock()
		cases[index] = ir.SwitchCase{Value: float64(index), Target: blocks[index].ID}
	}
	_ = l.builder.Switch(variant.tag.slots[0], cases, invalid)
	for index, alternative := range variant.alternatives {
		l.setCurrent(blocks[index])
		methodSet := types.NewMethodSet(alternative.type_)
		var concrete *types.Func
		for methodIndex := 0; methodIndex < methodSet.Len(); methodIndex++ {
			candidate, _ := methodSet.At(methodIndex).Obj().(*types.Func)
			if candidate != nil && candidate.Name() == method.Name() {
				concrete = candidate
				break
			}
		}
		if concrete == nil {
			l.errorAt(call, "concrete type %s does not implement interface method %s", alternative.type_, method.Name())
			l.jump(merge)
			continue
		}
		arguments := append([]callArgument{{value: alternative}}, args...)
		returned := l.inlineCallArguments(call, concrete, arguments)
		if resultType != nil {
			l.storeDescriptor(result, returned, call)
		}
		l.jump(merge)
	}
	l.setCurrent(invalid)
	if variant.persistent {
		l.terminateRuntime(call, "nil interface method call")
	} else {
		_ = l.builder.MarkUnreachable()
	}
	l.setCurrent(merge)
	return result
}

func (l *lowerer) callReceiver(selector *ast.SelectorExpr, fn *types.Func) lowerValue {
	receiver := l.expr(selector.X)
	if isContainerValue(receiver) || receiver.stream != nil {
		return receiver
	}
	signature, _ := fn.Type().(*types.Signature)
	if signature == nil || signature.Recv() == nil {
		return l.materialize("call.receiver", receiver, selector.X)
	}
	declared := types.Unalias(signature.Recv().Type())
	if pointer, ok := declared.(*types.Pointer); ok {
		if _, alreadyPointer := types.Unalias(receiver.type_).(*types.Pointer); !alreadyPointer {
			if receiver.levelGlobal != nil {
				handle := scalarValue(l.pure(resource.RuntimeFunctionAdd, selector.X, receiver.levelGlobal.base, ir.Const{Value: 1}), types.Typ[types.Int])
				receiver = lowerValue{type_: pointer, persistentPointer: &persistentPointerValue{
					handle: handle, storage: receiver.levelGlobal.storage, target: receiver.levelGlobal.declaration,
					read: receiver.levelGlobal.read, write: receiver.levelGlobal.write,
				}}
				return receiver
			}
			receiver = l.materializeAddressable("call.receiver", receiver, selector.X)
			receiver.type_ = pointer
		}
		return receiver
	}
	if pointer, ok := types.Unalias(receiver.type_).(*types.Pointer); ok {
		if receiver.aggregatePointer != nil {
			receiver = l.loadAggregatePointer(receiver, pointer, selector.X)
			return l.copyAggregate("call.receiver", receiver, selector.X)
		}
		if receiver.aggregate != nil {
			receiver.type_ = pointer.Elem()
			return l.copyAggregate("call.receiver", receiver, selector.X)
		}
		if receiver.pointer != nil {
			receiver = l.loadPointer(receiver, pointer, selector.X)
		} else {
			receiver.type_ = pointer.Elem()
		}
	}
	if receiver.aggregate != nil {
		return l.copyAggregate("call.receiver", receiver, selector.X)
	}
	copy := l.alloc("call.receiver", signature.Recv().Type())
	l.store(copy, receiver, selector.X)
	return copy
}

func (l *lowerer) builtinCall(n *ast.CallExpr, builtin *types.Builtin) lowerValue {
	if builtin.Name() == "new" {
		if len(n.Args) != 1 {
			l.errorAt(n, "Go builtin new requires one type argument")
			return zeroValue(l.pkg.TypesInfo.TypeOf(n))
		}
		pointer, ok := types.Unalias(l.resolveType(l.pkg.TypesInfo.TypeOf(n))).(*types.Pointer)
		if !ok {
			l.errorAt(n, "Go builtin new result is not a pointer")
			return zeroValue(l.pkg.TypesInfo.TypeOf(n))
		}
		element := pointer.Elem()
		if l.containsAggregateDescriptor(element) {
			value := l.newAggregateCell("new.aggregate", element, n)
			value.type_ = l.pkg.TypesInfo.TypeOf(n)
			return value
		}
		if isPointerType(element) || isFunctionType(element) || isInterfaceType(element) || isContainerType(element) || l.isCompileTimeOnlyValueType(element) || l.containsEntityView(element) || containsResourceHandle(element) != "" {
			l.errorAt(n, "Go builtin new does not support compile-time-only element type %s", element)
			return zeroValue(l.pkg.TypesInfo.TypeOf(n))
		}
		value := l.allocZeroed("new", element, n)
		if len(value.places) == 0 {
			l.errorAt(n, "Go builtin new requires a non-empty runtime layout")
			return zeroValue(l.pkg.TypesInfo.TypeOf(n))
		}
		value.type_ = l.pkg.TypesInfo.TypeOf(n)
		return l.freezePointerValue(value, n)
	}
	if builtin.Name() == "min" || builtin.Name() == "max" {
		if len(n.Args) == 0 {
			l.errorAt(n, "Go builtin %s requires at least one argument", builtin.Name())
			return zeroValue(l.pkg.TypesInfo.TypeOf(n))
		}
		values := make([]ir.Expr, 0, len(n.Args))
		for _, argument := range n.Args {
			value := l.materialize("builtin.arg", l.expr(argument), argument)
			if len(value.slots) != 1 {
				l.errorAt(argument, "Go builtin %s only supports scalar numeric arguments", builtin.Name())
				return zeroValue(l.pkg.TypesInfo.TypeOf(n))
			}
			values = append(values, value.slots[0])
		}
		function := resource.RuntimeFunctionMin
		if builtin.Name() == "max" {
			function = resource.RuntimeFunctionMax
		}
		return lowerValue{type_: l.pkg.TypesInfo.TypeOf(n), slots: []ir.Expr{l.pure(function, n, values...)}}
	}
	if builtin.Name() != "len" && builtin.Name() != "cap" {
		l.errorAt(n, "Go builtin %s is not supported by the frozen callback subset", builtin.Name())
		return zeroValue(l.pkg.TypesInfo.TypeOf(n))
	}
	if len(n.Args) != 1 {
		l.errorAt(n, "Go builtin %s requires one argument", builtin.Name())
		return zeroValue(l.pkg.TypesInfo.TypeOf(n))
	}
	value := l.expr(n.Args[0])
	result := lowerValue{type_: l.pkg.TypesInfo.TypeOf(n)}
	if value.variadic != nil {
		result.slots = []ir.Expr{ir.Const{Value: float64(len(value.variadic.elements))}}
		return result
	}
	if array, ok := types.Unalias(value.type_).Underlying().(*types.Array); ok {
		result.slots = []ir.Expr{ir.Const{Value: float64(array.Len())}}
		return result
	}
	if pointer, ok := types.Unalias(value.type_).(*types.Pointer); ok {
		if array, arrayOK := types.Unalias(pointer.Elem()).Underlying().(*types.Array); arrayOK {
			result.slots = []ir.Expr{ir.Const{Value: float64(array.Len())}}
			return result
		}
	}
	if isContainerValue(value) {
		return l.dispatchContainerValue(n, value, func(alternative lowerValue) lowerValue {
			if builtin.Name() == "len" {
				return lowerValue{type_: result.type_, slots: []ir.Expr{alternative.slots[0]}}
			}
			return lowerValue{type_: result.type_, slots: []ir.Expr{ir.Const{Value: float64(alternative.container.capacity)}}}
		})
	}
	l.errorAt(n, "Go builtin %s is only supported for fixed arrays, pointers to fixed arrays, and catalog containers", builtin.Name())
	return zeroValue(l.pkg.TypesInfo.TypeOf(n))
}

func (l *lowerer) materialize(name string, value lowerValue, node ast.Node) lowerValue {
	if isContainerValue(value) || value.stream != nil || value.aggregate != nil || len(value.slots) == 0 {
		return value
	}
	if _, ok := types.Unalias(value.type_).(*types.Pointer); ok {
		return value
	}
	allConstant := true
	for _, slot := range value.slots {
		if _, ok := slot.(ir.Const); !ok {
			allConstant = false
			break
		}
	}
	if allConstant {
		return value
	}
	local := l.alloc(name, value.type_)
	l.store(local, value, node)
	return local
}

func (l *lowerer) materializeAddressable(name string, value lowerValue, node ast.Node) lowerValue {
	if value.aggregate != nil {
		return value
	}
	if len(value.places) == len(value.slots) && len(value.places) != 0 {
		return value
	}
	if isContainerValue(value) || len(value.slots) == 0 {
		return value
	}
	local := l.alloc(name, value.type_)
	l.store(local, value, node)
	return local
}

func (l *lowerer) ensureAssignable(value lowerValue, expression ast.Expr) lowerValue {
	if len(value.places) != 0 {
		return value
	}
	identifier, ok := expression.(*ast.Ident)
	if !ok {
		return value
	}
	object := l.pkg.TypesInfo.ObjectOf(identifier)
	if object == nil {
		object = l.pkg.TypesInfo.Defs[identifier]
	}
	if object == nil || len(value.slots) == 0 {
		return value
	}
	local := l.alloc(identifier.Name, object.Type())
	l.store(local, value, identifier)
	l.rebind(object, local)
	return local
}

func (l *lowerer) freezePointerValue(value lowerValue, node ast.Node) lowerValue {
	for i, raw := range value.places {
		var index ir.Expr
		switch place := raw.(type) {
		case ir.IndexedLocalPlace:
			index = place.Index
		case ir.MemoryPlace:
			index = place.Index
		default:
			continue
		}
		if _, constant := index.(ir.Const); constant {
			continue
		}
		frozen := l.materialize("pointer.index", lowerValue{type_: types.Typ[types.Int], slots: []ir.Expr{index}}, node)
		switch place := raw.(type) {
		case ir.IndexedLocalPlace:
			place.Index = frozen.slots[0]
			value.places[i] = place
			value.slots[i] = ir.Load{Place: place}
		case ir.MemoryPlace:
			place.Index = frozen.slots[0]
			value.places[i] = place
			value.slots[i] = ir.Load{Place: place}
		}
	}
	return value
}

func (l *lowerer) entityReferenceField(node ast.Node, object *types.Var, reference lowerValue) lowerValue {
	if reference.entity == nil || len(reference.slots) != 1 {
		l.errorAt(node, "EntityRef.Get view requires exactly one entity index slot")
		return zeroValue(object.Type())
	}
	var declaration *FieldDeclaration
	for _, candidate := range reference.entity.binding.declaration.Fields {
		if candidate.Object == object {
			declaration = candidate
			break
		}
	}
	if declaration == nil {
		l.errorAt(node, "EntityRef.Get field %s is not part of archetype %s", object.Name(), reference.entity.binding.declaration.Name)
		return zeroValue(object.Type())
	}
	storage := ""
	read, write := false, false
	switch declaration.Storage {
	case "imported", "data":
		storage = "EntityDataArray"
		read = true
		write = l.phase == "preprocess"
	case "shared":
		storage = "EntitySharedMemoryArray"
		read = true
		write = l.phase == "preprocess" || l.phase == "updateSequential"
		if l.mode == mode.ModePlay {
			write = write || l.phase == "touch"
		}
	case "memory":
		l.errorAt(node, "EntityRef.Get cannot access memory field %s.%s", reference.entity.binding.declaration.Name, declaration.GoName)
		return zeroValue(declaration.Type)
	case "exported":
		l.errorAt(node, "EntityRef.Get cannot access exported field %s.%s", reference.entity.binding.declaration.Name, declaration.GoName)
		return zeroValue(declaration.Type)
	default:
		l.errorAt(node, "EntityRef.Get field %s.%s has unsupported storage %q", reference.entity.binding.declaration.Name, declaration.GoName, declaration.Storage)
		return zeroValue(declaration.Type)
	}
	value := lowerValue{type_: declaration.Type, slots: make([]ir.Expr, declaration.Size), places: make([]ir.Place, declaration.Size), entityField: true}
	for offset := 0; offset < declaration.Size; offset++ {
		place := l.memory(storage, reference.slots[0], 32, declaration.Offset+offset, read, write, node)
		value.places[offset] = place
		value.slots[offset] = ir.Load{Place: place}
	}
	if declaration.ContainerKind != "" {
		_, key, element, _ := containerTypes(declaration.Type)
		stride := declaration.KeySize + declaration.ElementSize
		value.container = &containerValue{
			kind: declaration.ContainerKind, capacity: declaration.Capacity, stride: stride,
			keySize: declaration.KeySize, element: element, key: key,
			memoryStorage: storage, memoryBase: declaration.Offset + 1, memoryEntity: reference.slots[0],
			memoryRead: read, memoryWrite: write,
		}
	}
	return value
}

func (l *lowerer) easeCall(n *ast.CallExpr) lowerValue {
	if len(n.Args) != 3 {
		l.errorAt(n, "Ease requires direction, curve, and value")
		return zeroValue(l.pkg.TypesInfo.TypeOf(n))
	}
	constantString := func(expr ast.Expr) (string, bool) {
		value := l.pkg.TypesInfo.Types[expr].Value
		if value == nil || value.Kind() != constant.String {
			return "", false
		}
		return constant.StringVal(value), true
	}
	direction, okDirection := constantString(n.Args[0])
	curve, okCurve := constantString(n.Args[1])
	if !okDirection || !okCurve {
		l.errorAt(n, "Ease direction and curve must be compile-time constants")
		return zeroValue(l.pkg.TypesInfo.TypeOf(n))
	}
	name := "Ease" + direction + curve
	var runtime resource.RuntimeFunction
	for _, symbol := range catalog.Symbols {
		if symbol.Package == "sonolus/native" && symbol.Name == name && symbol.Runtime != "" {
			runtime = symbol.Runtime
			break
		}
	}
	if runtime == "" {
		l.errorAt(n, "unsupported Ease combination %s/%s", direction, curve)
		return zeroValue(l.pkg.TypesInfo.TypeOf(n))
	}
	value := l.expr(n.Args[2])
	return l.runtimeCall(n, runtime, true, nil, []lowerValue{value})
}

func (l *lowerer) isFacadeReceiver(expr ast.Expr) bool {
	var object types.Object
	switch value := expr.(type) {
	case *ast.Ident:
		object = l.pkg.TypesInfo.ObjectOf(value)
	case *ast.SelectorExpr:
		if l.pkg.TypesInfo.Selections[value] == nil {
			object = l.pkg.TypesInfo.ObjectOf(value.Sel)
		}
	}
	if object == nil {
		return false
	}
	symbol, ok := catalog.LookupObject(object)
	return ok && symbol.Kind == catalog.KindVariable && irTypeOf(object.Type()).Slots == 0
}

func facadeReceiverName(expr ast.Expr) string {
	switch value := expr.(type) {
	case *ast.Ident:
		return value.Name
	case *ast.SelectorExpr:
		return value.Sel.Name
	default:
		return ""
	}
}

func (l *lowerer) memoryCall(n *ast.CallExpr, recipe catalog.Recipe, args []lowerValue) lowerValue {
	storage, offset := recipe.Storage, recipe.Offset
	if selector, ok := n.Fun.(*ast.SelectorExpr); ok {
		if receiver := facadeReceiverName(selector.X); receiver != "" {
			switch receiver {
			case "SafeArea":
				switch l.mode {
				case mode.ModePlay, mode.ModeWatch:
					offset = 5
				case mode.ModePreview:
					offset = 2
				case mode.ModeTutorial:
					offset = 3
				}
			case "TutorialData":
				storage = "TutorialData"
			case "SkinTransform":
				storage = "SkinTransform"
			case "ParticleTransform":
				storage = "ParticleTransform"
			}
		}
	}
	index := ir.Expr(ir.Const{})
	if recipe.IndexArg >= 0 {
		if recipe.IndexArg >= len(args) || len(args[recipe.IndexArg].slots) != 1 {
			l.errorAt(n, "memory recipe requires a scalar index argument")
			return zeroValue(l.pkg.TypesInfo.TypeOf(n))
		}
		index = args[recipe.IndexArg].slots[0]
	}
	if recipe.Write {
		valueArg := recipe.IndexArg + 1
		if valueArg < 0 {
			valueArg = 0
		}
		if valueArg >= len(args) {
			l.errorAt(n, "memory write recipe requires a value argument")
			return lowerValue{}
		}
		value := args[valueArg]
		places := make([]ir.Place, len(value.slots))
		for i := range places {
			places[i] = l.memory(storage, index, recipe.Stride, memorySlotOffset(storage, offset, i), true, true, n)
		}
		if err := l.builder.Store(places, ir.Value{Type: irTypeOf(value.type_), Slots: value.slots}, sourcePos(l.pkg, n.Pos())); err != nil {
			l.errorAt(n, "%v", err)
		}
		return lowerValue{}
	}
	t := l.resolveType(l.pkg.TypesInfo.TypeOf(n))
	result := l.zeroRuntimeValue(t)
	result.places = make([]ir.Place, len(result.slots))
	for i := range result.slots {
		place := l.memory(storage, index, recipe.Stride, memorySlotOffset(storage, offset, i), true, false, n)
		result.places[i] = place
		result.slots[i] = ir.Load{Place: place}
	}
	return result
}

func memorySlotOffset(storage string, base, slot int) int {
	if storage == "SkinTransform" || storage == "ParticleTransform" {
		offsets := [...]int{0, 1, 3, 4, 5, 7, 12, 13, 15}
		if slot < len(offsets) {
			return base + offsets[slot]
		}
	}
	return base + slot
}

func containerTypes(t types.Type) (kind string, key, element types.Type, ok bool) {
	named, ok := types.Unalias(t).(*types.Named)
	if !ok {
		return "", nil, nil, false
	}
	id := typeID(named)
	args := named.TypeArgs()
	switch id {
	case rootID("VarArray"), rootID("ArraySet"):
		if args.Len() == 1 {
			return named.Obj().Name(), nil, args.At(0), true
		}
	case rootID("ArrayMap"):
		if args.Len() == 2 {
			return named.Obj().Name(), args.At(0), args.At(1), true
		}
	}
	return "", nil, nil, false
}

func isContainerType(t types.Type) bool {
	_, _, _, ok := containerTypes(t)
	return ok
}

func (l *lowerer) isCompileTimeOnlyValueType(t types.Type) bool {
	named, ok := namedType(t)
	if !ok {
		return false
	}
	id := typeID(named)
	switch id {
	case rootID("Configuration"), rootID("UIConfig"), rootID("ROMValues"), rootID("LevelFile"), rootID("StreamResource"),
		rootID("LevelMemoryResource"), rootID("LevelDataResource"),
		rootID("Stream"), rootID("StreamData"), rootID("SliderOptionConfig"), rootID("ToggleOptionConfig"), rootID("SelectOptionConfig"),
		rootID("SkinResource"), rootID("EffectResource"), rootID("ParticleResource"), rootID("BucketsResource"), rootID("InstructionResource"), rootID("InstructionIconResource"),
		rootID("AnyArchetype"):
		return true
	}
	for _, candidate := range []string{"Archetype", "CallbackOrders", "GlobalCallbacks"} {
		if id == markerID(l.mode, candidate) {
			return true
		}
	}
	return false
}

func sameContainerBacking(a, b lowerValue) bool {
	if a.container == nil || b.container == nil {
		return false
	}
	if a.container.kind != b.container.kind || a.container.capacity != b.container.capacity || a.container.stride != b.container.stride {
		return false
	}
	if a.container.dataLocal != nil || b.container.dataLocal != nil {
		if a.container.dataLocal == nil || b.container.dataLocal == nil || a.container.dataLocal.ID != b.container.dataLocal.ID {
			return false
		}
	}
	if len(a.places) == 0 || len(b.places) == 0 {
		return false
	}
	ap, aok := a.places[0].(ir.LocalPlace)
	bp, bok := b.places[0].(ir.LocalPlace)
	return aok && bok && ap.ID == bp.ID && ap.Offset == bp.Offset
}

func isContainerValue(value lowerValue) bool {
	return value.container != nil || value.containerVariant != nil
}

func directContainerAlternative(value lowerValue) lowerValue {
	value.containerVariant = nil
	return value
}

func (l *lowerer) newContainerCell(name string, containerType types.Type, node ast.Node) lowerValue {
	variant := newFiniteVariant[lowerValue](l, name+".container", node)
	return lowerValue{type_: containerType, containerVariant: &variant}
}

func (l *lowerer) copyContainerValue(name string, source lowerValue, node ast.Node) lowerValue {
	destination := l.newContainerCell(name, source.type_, node)
	if source.container == nil {
		return l.mergeContainerValue(destination, source, node)
	}
	alternative := directContainerAlternative(source)
	index, ok := destination.containerVariant.add(alternative, sameContainerBacking)
	if !ok {
		l.errorAt(node, "finite container variant exceeds 256 alternatives")
		return destination
	}
	l.store(destination.containerVariant.tag, scalarValue(ir.Const{Value: float64(index)}, types.Typ[types.Int]), node)
	result := source
	result.containerVariant = destination.containerVariant
	return result
}

func (l *lowerer) mergeContainerValue(destination, source lowerValue, node ast.Node) lowerValue {
	if !isContainerValue(source) {
		l.errorAt(node, "container assignment requires a catalog container value")
		return destination
	}
	if destination.containerVariant == nil {
		cell := l.newContainerCell("container.variant", destination.type_, node)
		destination.containerVariant = cell.containerVariant
		if destination.container != nil {
			index, ok := destination.containerVariant.add(directContainerAlternative(destination), sameContainerBacking)
			if !ok {
				l.errorAt(node, "finite container variant exceeds 256 alternatives")
				return destination
			}
			l.store(destination.containerVariant.tag, scalarValue(ir.Const{Value: float64(index)}, types.Typ[types.Int]), node)
		}
	}
	variant := destination.containerVariant
	destination.container = nil
	destination.slots = nil
	destination.places = nil
	addAlternative := func(value lowerValue) int {
		value = directContainerAlternative(value)
		index, ok := variant.add(value, sameContainerBacking)
		if !ok {
			l.errorAt(node, "finite container variant exceeds 256 alternatives")
			return -1
		}
		return index
	}
	if source.container != nil {
		index := addAlternative(source)
		if index >= 0 {
			l.store(variant.tag, scalarValue(ir.Const{Value: float64(index)}, types.Typ[types.Int]), node)
		}
		return destination
	}
	if source.containerVariant == nil || len(source.containerVariant.tag.slots) != 1 {
		l.errorAt(node, "finite container source has no runtime tag")
		return destination
	}
	merge, invalid := l.newBlock(), l.newBlock()
	blocks := make([]*ir.Block, len(source.containerVariant.alternatives))
	cases := make([]ir.SwitchCase, len(blocks))
	for index := range blocks {
		blocks[index] = l.newBlock()
		cases[index] = ir.SwitchCase{Value: float64(index), Target: blocks[index].ID}
	}
	_ = l.builder.Switch(source.containerVariant.tag.slots[0], cases, invalid)
	for index, alternative := range source.containerVariant.alternatives {
		l.setCurrent(blocks[index])
		mapped := addAlternative(alternative)
		if mapped >= 0 {
			l.store(variant.tag, scalarValue(ir.Const{Value: float64(mapped)}, types.Typ[types.Int]), node)
		}
		l.jump(merge)
	}
	l.setCurrent(invalid)
	_ = l.builder.MarkUnreachable()
	l.setCurrent(merge)
	return destination
}

func (l *lowerer) dispatchContainerValue(call *ast.CallExpr, receiver lowerValue, invoke func(lowerValue) lowerValue) lowerValue {
	if receiver.container != nil {
		return invoke(receiver)
	}
	variant := receiver.containerVariant
	if variant == nil || len(variant.tag.slots) != 1 || len(variant.alternatives) == 0 {
		l.errorAt(call, "finite container variant has no alternatives")
		return zeroValue(l.pkg.TypesInfo.TypeOf(call))
	}
	resultType := l.pkg.TypesInfo.TypeOf(call)
	if isFunctionType(resultType) {
		alternatives := make([]*staticCallable, len(variant.alternatives))
		for index, alternative := range variant.alternatives {
			value := invoke(alternative)
			if value.callable == nil {
				l.errorAt(call, "container iterator alternative %d is not callable", index)
				continue
			}
			alternatives[index] = value.callable
		}
		frozenTag := l.materialize("container.iterator.tag", variant.tag, call)
		callable := indexedCallableVariant(l, alternatives, frozenTag, call)
		callable.frozenTag = true
		return lowerValue{type_: resultType, callable: callable}
	}
	var result lowerValue
	if resultType != nil && l.runtimeTypeOf(resultType).Slots != 0 {
		result = l.allocZeroed("container.variant.result", resultType, call)
	}
	merge, invalid := l.newBlock(), l.newBlock()
	blocks := make([]*ir.Block, len(variant.alternatives))
	cases := make([]ir.SwitchCase, len(blocks))
	for index := range blocks {
		blocks[index] = l.newBlock()
		cases[index] = ir.SwitchCase{Value: float64(index), Target: blocks[index].ID}
	}
	_ = l.builder.Switch(variant.tag.slots[0], cases, invalid)
	for index, alternative := range variant.alternatives {
		l.setCurrent(blocks[index])
		value := invoke(alternative)
		if len(result.places) != 0 {
			l.store(result, value, call)
		}
		l.jump(merge)
	}
	l.setCurrent(invalid)
	_ = l.builder.MarkUnreachable()
	l.setCurrent(merge)
	return result
}

func (l *lowerer) containerCall(n *ast.CallExpr, operation string, args []lowerValue) lowerValue {
	if strings.HasSuffix(operation, ".new") {
		if len(args) != 1 || len(args[0].slots) != 1 {
			l.errorAt(n, "container constructor requires one constant capacity")
			return lowerValue{}
		}
		capacityValue, constantCapacity := args[0].slots[0].(ir.Const)
		capacity := int(capacityValue.Value)
		if !constantCapacity || capacity <= 0 || float64(capacity) != capacityValue.Value {
			l.errorAt(n.Args[0], "container capacity must be a positive integer constant")
			return lowerValue{}
		}
		resultType := l.resolveType(l.pkg.TypesInfo.TypeOf(n))
		kind, key, element, ok := containerTypes(resultType)
		if !ok {
			l.errorAt(n, "invalid container result type %s", resultType)
			return lowerValue{}
		}
		keySize := 0
		if key != nil {
			keySize = irTypeOf(key).Slots
		}
		if key != nil {
			keySize = l.runtimeTypeOf(types.NewArray(key, 1)).Slots
		}
		stride := keySize + l.runtimeTypeOf(types.NewArray(element, 1)).Slots
		size := l.allocZeroed("container.size", types.Typ[types.Int], n)
		backingType := ir.Type{Name: resultType.String() + ".backing", Slots: capacity * stride}
		backing := l.builder.NewLocal("container.data", backingType)
		result := lowerValue{type_: resultType, slots: append(append([]ir.Expr{}, size.slots...), backing.Slots...), places: append(append([]ir.Place{}, size.places...), ir.Places(backing)...)}
		var dataLocal *ir.LocalPlace
		if places := ir.Places(backing); len(places) != 0 {
			first, _ := places[0].(ir.LocalPlace)
			dataLocal = &first
		}
		result.container = &containerValue{kind: kind, capacity: capacity, stride: stride, keySize: keySize, element: element, key: key, dataLocal: dataLocal}
		return result
	}
	if len(args) != 0 && args[0].container == nil && args[0].containerVariant != nil {
		receiver := args[0]
		return l.dispatchContainerValue(n, receiver, func(alternative lowerValue) lowerValue {
			direct := append([]lowerValue(nil), args...)
			direct[0] = alternative
			return l.containerCall(n, operation, direct)
		})
	}
	if len(args) == 0 || args[0].container == nil {
		l.errorAt(n, "container method receiver has no backing layout")
		return lowerValue{}
	}
	receiver, c := args[0], args[0].container
	size := receiver.slots[0]
	scalar := func(expr ir.Expr, t types.Type) lowerValue { return lowerValue{type_: t, slots: []ir.Expr{expr}} }
	indexInRange := func(index ir.Expr, allowEnd bool) ir.Expr {
		upper := resource.RuntimeFunctionLess
		if allowEnd {
			upper = resource.RuntimeFunctionLessOr
		}
		return l.pure(resource.RuntimeFunctionAnd, n,
			l.pure(resource.RuntimeFunctionGreaterOr, n, index, ir.Const{}),
			l.pure(upper, n, index, size))
	}
	hasCapacity := func() ir.Expr {
		return l.pure(resource.RuntimeFunctionLess, n, size, ir.Const{Value: float64(c.capacity)})
	}
	indexValue := func(index ir.Expr, offset, count int, t types.Type) lowerValue {
		return l.containerElement(n, c, index, offset, count, t)
	}
	swap := func(left, right ir.Expr) {
		leftValue := l.materialize("container.swap", indexValue(left, 0, c.stride, c.element), n)
		rightValue := l.materialize("container.swap", indexValue(right, 0, c.stride, c.element), n)
		l.store(indexValue(left, 0, c.stride, c.element), rightValue, n)
		l.store(indexValue(right, 0, c.stride, c.element), leftValue, n)
	}
	switch operation {
	case "varArray.len", "arrayMap.len", "arraySet.len":
		return scalar(size, l.pkg.TypesInfo.TypeOf(n))
	case "varArray.capacity", "arrayMap.capacity", "arraySet.capacity":
		return scalar(ir.Const{Value: float64(c.capacity)}, l.pkg.TypesInfo.TypeOf(n))
	case "varArray.isFull", "arrayMap.isFull", "arraySet.isFull":
		return scalar(l.pure(resource.RuntimeFunctionEqual, n, size, ir.Const{Value: float64(c.capacity)}), l.pkg.TypesInfo.TypeOf(n))
	case "varArray.get", "varArray.getUnchecked":
		if len(args) == 2 {
			if operation == "varArray.get" {
				l.guard(n, indexInRange(args[1].slots[0], false))
			}
			return indexValue(args[1].slots[0], 0, c.stride, c.element)
		}
	case "varArray.set", "varArray.setUnchecked":
		if len(args) == 3 {
			if operation == "varArray.set" {
				l.guard(n, indexInRange(args[1].slots[0], false))
			}
			l.store(indexValue(args[1].slots[0], 0, c.stride, c.element), args[2], n)
			return lowerValue{}
		}
	case "varArray.append", "varArray.appendUnchecked":
		if len(args) == 2 {
			if operation == "varArray.append" {
				l.guard(n, hasCapacity())
			}
			l.store(indexValue(size, 0, c.stride, c.element), args[1], n)
			l.store(lowerValue{type_: types.Typ[types.Int], slots: []ir.Expr{size}, places: receiver.places[:1]}, scalar(l.pure(resource.RuntimeFunctionAdd, n, size, ir.Const{Value: 1}), types.Typ[types.Int]), n)
			return lowerValue{}
		}
	case "varArray.pop":
		l.guard(n, l.pure(resource.RuntimeFunctionGreater, n, size, ir.Const{}))
		newSize := l.pure(resource.RuntimeFunctionSubtract, n, size, ir.Const{Value: 1})
		l.store(lowerValue{type_: types.Typ[types.Int], slots: []ir.Expr{size}, places: receiver.places[:1]}, scalar(newSize, types.Typ[types.Int]), n)
		return indexValue(newSize, 0, c.stride, c.element)
	case "varArray.removeAt":
		if len(args) == 2 {
			l.guard(n, indexInRange(args[1].slots[0], false))
			result := l.materialize("container.removed", indexValue(args[1].slots[0], 0, c.stride, c.element), n)
			l.containerRemoveAt(n, receiver, args[1])
			return result
		}
	case "varArray.remove":
		if len(args) == 2 {
			index, found := l.containerFind(n, receiver, args[1], 0, c.stride)
			remove, done := l.newBlock(), l.newBlock()
			_ = l.builder.Branch(found.slots[0], remove, done)
			l.setCurrent(remove)
			l.containerRemoveAt(n, receiver, index)
			l.jump(done)
			l.setCurrent(done)
			return found
		}
	case "varArray.index":
		if len(args) == 2 {
			index, found := l.containerFind(n, receiver, args[1], 0, c.stride)
			return scalar(l.pure(resource.RuntimeFunctionIf, n, found.slots[0], index.slots[0], ir.Const{Value: -1}), types.Typ[types.Int])
		}
	case "varArray.lastIndex":
		if len(args) == 2 {
			index := l.allocZeroed("container.find.last.index", types.Typ[types.Int], n)
			result := l.alloc("container.find.last.result", types.Typ[types.Int])
			l.store(result, scalar(ir.Const{Value: -1}, types.Typ[types.Int]), n)
			header, body, match, next, exit := l.newBlock(), l.newBlock(), l.newBlock(), l.newBlock(), l.newBlock()
			l.jump(header)
			l.setCurrent(header)
			_ = l.builder.Branch(l.pure(resource.RuntimeFunctionLess, n, index.slots[0], size), body, exit)
			l.setCurrent(body)
			candidate := indexValue(index.slots[0], 0, c.stride, c.element)
			var equal ir.Expr = ir.Const{Value: 1}
			for slot := range candidate.slots {
				equal = l.pure(resource.RuntimeFunctionAnd, n, equal, l.pure(resource.RuntimeFunctionEqual, n, candidate.slots[slot], args[1].slots[slot]))
			}
			_ = l.builder.Branch(equal, match, next)
			l.setCurrent(match)
			l.store(result, index, n)
			l.jump(next)
			l.setCurrent(next)
			l.store(index, scalar(l.pure(resource.RuntimeFunctionAdd, n, index.slots[0], ir.Const{Value: 1}), types.Typ[types.Int]), n)
			l.jump(header)
			l.setCurrent(exit)
			return result
		}
	case "varArray.count":
		if len(args) == 2 {
			index := l.allocZeroed("container.count.index", types.Typ[types.Int], n)
			count := l.allocZeroed("container.count.value", types.Typ[types.Int], n)
			header, body, match, next, exit := l.newBlock(), l.newBlock(), l.newBlock(), l.newBlock(), l.newBlock()
			l.jump(header)
			l.setCurrent(header)
			_ = l.builder.Branch(l.pure(resource.RuntimeFunctionLess, n, index.slots[0], size), body, exit)
			l.setCurrent(body)
			candidate := indexValue(index.slots[0], 0, c.stride, c.element)
			var equal ir.Expr = ir.Const{Value: 1}
			for i := range candidate.slots {
				equal = l.pure(resource.RuntimeFunctionAnd, n, equal, l.pure(resource.RuntimeFunctionEqual, n, candidate.slots[i], args[1].slots[i]))
			}
			_ = l.builder.Branch(equal, match, next)
			l.setCurrent(match)
			l.store(count, scalar(l.pure(resource.RuntimeFunctionAdd, n, count.slots[0], ir.Const{Value: 1}), types.Typ[types.Int]), n)
			l.jump(next)
			l.setCurrent(next)
			l.store(index, scalar(l.pure(resource.RuntimeFunctionAdd, n, index.slots[0], ir.Const{Value: 1}), types.Typ[types.Int]), n)
			l.jump(header)
			l.setCurrent(exit)
			return count
		}
	case "varArray.swap", "varArray.swapUnchecked":
		if len(args) == 3 {
			if operation == "varArray.swap" {
				l.guard(n, indexInRange(args[1].slots[0], false))
				l.guard(n, indexInRange(args[2].slots[0], false))
			}
			swap(args[1].slots[0], args[2].slots[0])
			return lowerValue{}
		}
	case "varArray.reverse", "varArray.shuffle":
		index := l.allocZeroed("container.reorder.index", types.Typ[types.Int], n)
		header, body, exit := l.newBlock(), l.newBlock(), l.newBlock()
		l.jump(header)
		l.setCurrent(header)
		limit := l.pure(resource.RuntimeFunctionDivide, n, size, ir.Const{Value: 2})
		if operation == "varArray.shuffle" {
			limit = size
		}
		_ = l.builder.Branch(l.pure(resource.RuntimeFunctionLess, n, index.slots[0], limit), body, exit)
		l.setCurrent(body)
		other := l.pure(resource.RuntimeFunctionSubtract, n, l.pure(resource.RuntimeFunctionSubtract, n, size, ir.Const{Value: 1}), index.slots[0])
		if operation == "varArray.shuffle" {
			other = l.builder.RuntimeCall(resource.RuntimeFunctionRandomInteger, []ir.Expr{index.slots[0], size}, irTypeOf(types.Typ[types.Int]), false, sourcePos(l.pkg, n.Pos()))
		}
		swap(index.slots[0], other)
		l.store(index, scalar(l.pure(resource.RuntimeFunctionAdd, n, index.slots[0], ir.Const{Value: 1}), types.Typ[types.Int]), n)
		l.jump(header)
		l.setCurrent(exit)
		return lowerValue{}
	case "varArray.values":
		return lowerValue{type_: l.pkg.TypesInfo.TypeOf(n), callable: &staticCallable{iterator: &containerIterator{receiver: receiver, offsets: []int{0}, types: []types.Type{c.element}}}}
	case "varArray.valuesReversed":
		return lowerValue{type_: l.pkg.TypesInfo.TypeOf(n), callable: &staticCallable{iterator: &containerIterator{receiver: receiver, offsets: []int{0}, types: []types.Type{c.element}, desc: true}}}
	case "varArray.items":
		return lowerValue{type_: l.pkg.TypesInfo.TypeOf(n), callable: &staticCallable{iterator: &containerIterator{receiver: receiver, offsets: []int{0}, types: []types.Type{c.element}, index: true}}}
	case "arrayMap.keys":
		return lowerValue{type_: l.pkg.TypesInfo.TypeOf(n), callable: &staticCallable{iterator: &containerIterator{receiver: receiver, offsets: []int{0}, types: []types.Type{c.key}}}}
	case "arrayMap.values":
		return lowerValue{type_: l.pkg.TypesInfo.TypeOf(n), callable: &staticCallable{iterator: &containerIterator{receiver: receiver, offsets: []int{c.keySize}, types: []types.Type{c.element}}}}
	case "arrayMap.items":
		return lowerValue{type_: l.pkg.TypesInfo.TypeOf(n), callable: &staticCallable{iterator: &containerIterator{receiver: receiver, offsets: []int{0, c.keySize}, types: []types.Type{c.key, c.element}}}}
	case "arraySet.values":
		return lowerValue{type_: l.pkg.TypesInfo.TypeOf(n), callable: &staticCallable{iterator: &containerIterator{receiver: receiver, offsets: []int{0}, types: []types.Type{c.element}}}}
	case "varArray.clear", "arrayMap.clear", "arraySet.clear":
		l.store(lowerValue{type_: types.Typ[types.Int], slots: []ir.Expr{size}, places: receiver.places[:1]}, scalar(ir.Const{}, types.Typ[types.Int]), n)
		return lowerValue{}
	case "varArray.contains", "arraySet.contains":
		if len(args) == 2 {
			_, found := l.containerFind(n, receiver, args[1], 0, c.stride)
			return found
		}
	case "arrayMap.contains":
		if len(args) == 2 {
			_, found := l.containerFind(n, receiver, args[1], 0, c.keySize)
			return found
		}
	case "arrayMap.get":
		if len(args) == 2 {
			index, found := l.containerFind(n, receiver, args[1], 0, c.keySize)
			l.guard(n, found.slots[0])
			return indexValue(index.slots[0], c.keySize, c.stride-c.keySize, c.element)
		}
	case "arrayMap.getOK", "arrayMap.pop":
		if len(args) == 2 {
			index, found := l.containerFind(n, receiver, args[1], 0, c.keySize)
			result := l.allocZeroed("map.lookup", l.pkg.TypesInfo.TypeOf(n), n)
			present, done := l.newBlock(), l.newBlock()
			_ = l.builder.Branch(found.slots[0], present, done)
			l.setCurrent(present)
			value := l.materialize("map.value", indexValue(index.slots[0], c.keySize, c.stride-c.keySize, c.element), n)
			combined := lowerValue{type_: result.type_, slots: append(append([]ir.Expr(nil), value.slots...), ir.Const{Value: 1})}
			l.store(result, combined, n)
			if operation == "arrayMap.pop" {
				l.containerRemoveAt(n, receiver, index)
			}
			l.jump(done)
			l.setCurrent(done)
			return result
		}
	case "arrayMap.set":
		if len(args) == 3 {
			index, found := l.containerFind(n, receiver, args[1], 0, c.keySize)
			existing, newEntry, merge := l.newBlock(), l.newBlock(), l.newBlock()
			_ = l.builder.Branch(found.slots[0], existing, newEntry)
			l.setCurrent(existing)
			l.store(indexValue(index.slots[0], c.keySize, c.stride-c.keySize, c.element), args[2], n)
			l.jump(merge)
			l.setCurrent(newEntry)
			l.guard(n, hasCapacity())
			l.store(indexValue(size, 0, c.keySize, c.key), args[1], n)
			l.store(indexValue(size, c.keySize, c.stride-c.keySize, c.element), args[2], n)
			l.store(lowerValue{type_: types.Typ[types.Int], slots: []ir.Expr{size}, places: receiver.places[:1]}, scalar(l.pure(resource.RuntimeFunctionAdd, n, size, ir.Const{Value: 1}), types.Typ[types.Int]), n)
			l.jump(merge)
			l.setCurrent(merge)
			return lowerValue{}
		}
	case "arraySet.add":
		if len(args) == 2 {
			_, found := l.containerFind(n, receiver, args[1], 0, c.stride)
			result := l.alloc("set.added", types.Typ[types.Bool])
			exists, add, merge := l.newBlock(), l.newBlock(), l.newBlock()
			_ = l.builder.Branch(found.slots[0], exists, add)
			l.setCurrent(exists)
			l.store(result, scalar(ir.Const{}, types.Typ[types.Bool]), n)
			l.jump(merge)
			l.setCurrent(add)
			l.guard(n, hasCapacity())
			l.store(indexValue(size, 0, c.stride, c.element), args[1], n)
			l.store(lowerValue{type_: types.Typ[types.Int], slots: []ir.Expr{size}, places: receiver.places[:1]}, scalar(l.pure(resource.RuntimeFunctionAdd, n, size, ir.Const{Value: 1}), types.Typ[types.Int]), n)
			l.store(result, scalar(ir.Const{Value: 1}, types.Typ[types.Bool]), n)
			l.jump(merge)
			l.setCurrent(merge)
			return result
		}
	case "varArray.insert":
		if len(args) == 3 {
			l.guard(n, l.pure(resource.RuntimeFunctionAnd, n, indexInRange(args[1].slots[0], true), hasCapacity()))
			l.containerInsert(n, receiver, args[1], args[2])
			return lowerValue{}
		}
	case "arraySet.remove":
		if len(args) == 2 {
			index, found := l.containerFind(n, receiver, args[1], 0, c.stride)
			result := l.alloc("set.removed", types.Typ[types.Bool])
			remove, missing, merge := l.newBlock(), l.newBlock(), l.newBlock()
			_ = l.builder.Branch(found.slots[0], remove, missing)
			l.setCurrent(remove)
			l.containerRemoveAt(n, receiver, index)
			l.store(result, scalar(ir.Const{Value: 1}, types.Typ[types.Bool]), n)
			l.jump(merge)
			l.setCurrent(missing)
			l.store(result, scalar(ir.Const{}, types.Typ[types.Bool]), n)
			l.jump(merge)
			l.setCurrent(merge)
			return result
		}
	case "arrayMap.delete":
		if len(args) == 2 {
			index, found := l.containerFind(n, receiver, args[1], 0, c.keySize)
			result := l.alloc("map.deleted", types.Typ[types.Bool])
			remove, missing, merge := l.newBlock(), l.newBlock(), l.newBlock()
			_ = l.builder.Branch(found.slots[0], remove, missing)
			l.setCurrent(remove)
			l.containerRemoveAt(n, receiver, index)
			l.store(result, scalar(ir.Const{Value: 1}, types.Typ[types.Bool]), n)
			l.jump(merge)
			l.setCurrent(missing)
			l.store(result, scalar(ir.Const{}, types.Typ[types.Bool]), n)
			l.jump(merge)
			l.setCurrent(merge)
			return result
		}
	}
	l.errorAt(n, "internal catalog inconsistency: container recipe %s has no lowering for receiver %s with %d arguments", operation, c.kind, len(args)-1)
	return zeroValue(l.pkg.TypesInfo.TypeOf(n))
}

func (l *lowerer) sortContainerCall(n *ast.CallExpr, fn *types.Func) lowerValue {
	if len(n.Args) != 1 {
		l.errorAt(n, "VarArray.SortFunc requires one comparator")
		return lowerValue{}
	}
	selector, ok := n.Fun.(*ast.SelectorExpr)
	if !ok {
		l.errorAt(n, "VarArray.SortFunc requires a method receiver")
		return lowerValue{}
	}
	receiver := l.callReceiver(selector, fn)
	comparator, ok := l.staticCallable(n.Args[0])
	if !ok {
		l.errorAt(n.Args[0], "VarArray.SortFunc comparator must be statically callable")
		return lowerValue{}
	}
	return l.dispatchContainerValue(n, receiver, func(alternative lowerValue) lowerValue {
		return l.sortContainerValue(n, alternative, comparator)
	})
}

func (l *lowerer) sortContainerValue(n *ast.CallExpr, receiver lowerValue, comparator *staticCallable) lowerValue {
	if receiver.container == nil || receiver.container.kind != "VarArray" {
		l.errorAt(n, "VarArray.SortFunc receiver has no container backing")
		return lowerValue{}
	}
	c := receiver.container
	index := l.allocZeroed("container.sort.index", types.Typ[types.Int], n)
	cursor := l.allocZeroed("container.sort.cursor", types.Typ[types.Int], n)
	l.store(index, lowerValue{type_: types.Typ[types.Int], slots: []ir.Expr{ir.Const{Value: 1}}}, n)
	outer, loadKey, inner, compare, shift, insert, advance, exit := l.newBlock(), l.newBlock(), l.newBlock(), l.newBlock(), l.newBlock(), l.newBlock(), l.newBlock(), l.newBlock()
	l.jump(outer)
	l.setCurrent(outer)
	_ = l.builder.Branch(l.pure(resource.RuntimeFunctionLess, n, index.slots[0], receiver.slots[0]), loadKey, exit)
	l.setCurrent(loadKey)
	key := l.materialize("sort.key", l.containerElement(n, c, index.slots[0], 0, c.stride, c.element), n)
	l.store(cursor, index, n)
	l.jump(inner)
	l.setCurrent(inner)
	_ = l.builder.Branch(l.pure(resource.RuntimeFunctionGreater, n, cursor.slots[0], ir.Const{}), compare, insert)
	l.setCurrent(compare)
	previousIndex := l.pure(resource.RuntimeFunctionSubtract, n, cursor.slots[0], ir.Const{Value: 1})
	previous := l.materialize("sort.previous", l.containerElement(n, c, previousIndex, 0, c.stride, c.element), n)
	typedComparator := *comparator
	typedComparator.resultType = types.Typ[types.Bool]
	less := l.inlineStaticCallable(n, &typedComparator, []callArgument{{value: key}, {value: previous}})
	if len(less.slots) != 1 {
		l.errorAt(n, "VarArray.SortFunc comparator must return bool")
		l.jump(exit)
		l.setCurrent(exit)
		return lowerValue{}
	}
	_ = l.builder.Branch(less.slots[0], shift, insert)
	l.setCurrent(shift)
	l.store(l.containerElement(n, c, cursor.slots[0], 0, c.stride, c.element), previous, n)
	l.store(cursor, lowerValue{type_: types.Typ[types.Int], slots: []ir.Expr{previousIndex}}, n)
	l.jump(inner)
	l.setCurrent(insert)
	l.store(l.containerElement(n, c, cursor.slots[0], 0, c.stride, c.element), key, n)
	l.jump(advance)
	l.setCurrent(advance)
	l.store(index, lowerValue{type_: types.Typ[types.Int], slots: []ir.Expr{l.pure(resource.RuntimeFunctionAdd, n, index.slots[0], ir.Const{Value: 1})}}, n)
	l.jump(outer)
	l.setCurrent(exit)
	return lowerValue{}
}

func (l *lowerer) extremeContainerCall(n *ast.CallExpr, fn *types.Func, operation string) lowerValue {
	if len(n.Args) != 1 {
		l.errorAt(n, "VarArray.%s requires one comparator", operation)
		return zeroValue(l.pkg.TypesInfo.TypeOf(n))
	}
	selector, ok := n.Fun.(*ast.SelectorExpr)
	if !ok {
		l.errorAt(n, "VarArray.%s requires a method receiver", operation)
		return zeroValue(l.pkg.TypesInfo.TypeOf(n))
	}
	receiver := l.callReceiver(selector, fn)
	comparator, ok := l.staticCallable(n.Args[0])
	if !ok {
		l.errorAt(n.Args[0], "VarArray.%s comparator must be statically callable", operation)
		return zeroValue(l.pkg.TypesInfo.TypeOf(n))
	}
	return l.dispatchContainerValue(n, receiver, func(alternative lowerValue) lowerValue {
		return l.extremeContainerValue(n, alternative, comparator, operation)
	})
}

func (l *lowerer) extremeContainerValue(n *ast.CallExpr, receiver lowerValue, comparator *staticCallable, operation string) lowerValue {
	if receiver.container == nil || receiver.container.kind != "VarArray" {
		l.errorAt(n, "VarArray.%s receiver has no container backing", operation)
		return zeroValue(l.pkg.TypesInfo.TypeOf(n))
	}
	wantsIndex := strings.HasPrefix(operation, "Index")
	wantsMax := strings.Contains(operation, "Max")
	if !wantsIndex {
		l.guardWith(n, l.pure(resource.RuntimeFunctionGreater, n, receiver.slots[0], ir.Const{}), "VarArray."+operation+" requires a non-empty array", true)
	}
	best := l.alloc("container.extreme.best", types.Typ[types.Int])
	cursor := l.alloc("container.extreme.index", types.Typ[types.Int])
	l.store(best, lowerValue{type_: types.Typ[types.Int], slots: []ir.Expr{ir.Const{Value: -1}}}, n)
	l.store(cursor, lowerValue{type_: types.Typ[types.Int], slots: []ir.Expr{ir.Const{}}}, n)
	header, seed, update, advance, exit := l.newBlock(), l.newBlock(), l.newBlock(), l.newBlock(), l.newBlock()
	l.jump(header)
	l.setCurrent(header)
	_ = l.builder.Branch(l.pure(resource.RuntimeFunctionLess, n, cursor.slots[0], receiver.slots[0]), seed, exit)
	l.setCurrent(seed)
	missing := l.pure(resource.RuntimeFunctionLess, n, best.slots[0], ir.Const{})
	haveBest, evaluate := l.newBlock(), l.newBlock()
	_ = l.builder.Branch(missing, update, haveBest)
	l.setCurrent(haveBest)
	l.jump(evaluate)
	l.setCurrent(evaluate)
	candidate := l.materialize("extreme.candidate", l.containerElement(n, receiver.container, cursor.slots[0], 0, receiver.container.stride, receiver.container.element), n)
	current := l.materialize("extreme.current", l.containerElement(n, receiver.container, best.slots[0], 0, receiver.container.stride, receiver.container.element), n)
	arguments := []callArgument{{value: candidate}, {value: current}}
	if wantsMax {
		arguments[0], arguments[1] = arguments[1], arguments[0]
	}
	typedComparator := *comparator
	typedComparator.resultType = types.Typ[types.Bool]
	less := l.inlineStaticCallable(n, &typedComparator, arguments)
	if len(less.slots) != 1 {
		l.errorAt(n, "VarArray.%s comparator must return bool", operation)
		l.jump(exit)
		l.setCurrent(exit)
		return zeroValue(l.pkg.TypesInfo.TypeOf(n))
	}
	_ = l.builder.Branch(less.slots[0], update, advance)
	l.setCurrent(update)
	l.store(best, cursor, n)
	l.jump(advance)
	l.setCurrent(advance)
	l.store(cursor, lowerValue{type_: types.Typ[types.Int], slots: []ir.Expr{l.pure(resource.RuntimeFunctionAdd, n, cursor.slots[0], ir.Const{Value: 1})}}, n)
	l.jump(header)
	l.setCurrent(exit)
	if wantsIndex {
		return best
	}
	return l.containerElement(n, receiver.container, best.slots[0], 0, receiver.container.stride, receiver.container.element)
}

func (l *lowerer) sortLinkedEntitiesCall(n *ast.CallExpr, doubly bool) lowerValue {
	expected := 4
	if doubly {
		expected = 5
	}
	if len(n.Args) != expected {
		l.errorAt(n, "linked entity sort requires %d arguments", expected)
		return zeroValue(l.pkg.TypesInfo.TypeOf(n))
	}
	head := l.materialize("linked.head", l.expr(n.Args[0]), n.Args[0])
	if len(head.slots) != 1 {
		l.errorAt(n.Args[0], "linked entity sort head must be a one-slot EntityRef")
		return zeroValue(l.pkg.TypesInfo.TypeOf(n))
	}
	named, ok := namedType(head.type_)
	if !ok || typeID(named) != rootID("EntityRef") || named.TypeArgs().Len() != 1 {
		l.errorAt(n.Args[0], "linked entity sort head must be EntityRef[T]")
		return zeroValue(l.pkg.TypesInfo.TypeOf(n))
	}
	element := named.TypeArgs().At(0)
	binding, exists := l.entityBinding(element)
	if !exists {
		l.errorAt(n.Args[0], "linked entity sort target %s is not an archetype declared in %s mode", element, l.mode)
		return zeroValue(l.pkg.TypesInfo.TypeOf(n))
	}
	callables := make([]*staticCallable, expected-1)
	for index := 1; index < expected; index++ {
		callable, callableOK := l.staticCallable(n.Args[index])
		if !callableOK {
			l.errorAt(n.Args[index], "linked entity sort accessor must be statically callable")
			return zeroValue(l.pkg.TypesInfo.TypeOf(n))
		}
		callables[index-1] = callable
	}
	less, next, setNext := callables[0], callables[1], callables[2]
	var setPrevious *staticCallable
	if doubly {
		setPrevious = callables[3]
	}
	refType := head.type_
	ref := func(expression ir.Expr) lowerValue { return lowerValue{type_: refType, slots: []ir.Expr{expression}} }
	view := func(reference lowerValue) lowerValue {
		return lowerValue{type_: types.NewPointer(element), slots: []ir.Expr{reference.slots[0]}, entity: &entityReferenceValue{binding: binding}}
	}
	nextOf := func(reference lowerValue) lowerValue {
		value := l.inlineStaticCallable(n, next, []callArgument{{value: view(reference)}})
		if len(value.slots) != 1 {
			l.errorAt(n, "linked entity next accessor must return EntityRef[T]")
			return ref(ir.Const{Value: -1})
		}
		value.type_ = refType
		return value
	}
	setNextOf := func(reference, value lowerValue) {
		l.inlineStaticCallable(n, setNext, []callArgument{{value: view(reference)}, {value: value}})
	}
	lessThan := func(left, right lowerValue) ir.Expr {
		result := l.inlineStaticCallable(n, less, []callArgument{{value: view(left)}, {value: view(right)}})
		if len(result.slots) != 1 {
			l.errorAt(n, "linked entity comparator must return bool")
			return ir.Const{}
		}
		return result.slots[0]
	}
	result := l.alloc("linked.result", refType)
	l.store(result, head, n)
	width := l.alloc("linked.width", types.Typ[types.Int])
	l.store(width, scalarValue(ir.Const{Value: 1}, types.Typ[types.Int]), n)
	pass, passBody, done := l.newBlock(), l.newBlock(), l.newBlock()
	l.jump(pass)
	l.setCurrent(pass)
	_ = l.builder.Branch(l.pure(resource.RuntimeFunctionGreaterOr, n, result.slots[0], ir.Const{}), passBody, done)
	l.setCurrent(passBody)
	current := l.alloc("linked.current", refType)
	newHead := l.alloc("linked.newHead", refType)
	tail := l.alloc("linked.tail", refType)
	merges := l.allocZeroed("linked.merges", types.Typ[types.Int], n)
	l.store(current, result, n)
	l.store(newHead, ref(ir.Const{Value: -1}), n)
	l.store(tail, ref(ir.Const{Value: -1}), n)
	outer, splitLeft, splitRight, merge, appendLeft, appendRight, appended, nextRun, finishPass := l.newBlock(), l.newBlock(), l.newBlock(), l.newBlock(), l.newBlock(), l.newBlock(), l.newBlock(), l.newBlock(), l.newBlock()
	l.jump(outer)
	l.setCurrent(outer)
	_ = l.builder.Branch(l.pure(resource.RuntimeFunctionGreaterOr, n, current.slots[0], ir.Const{}), splitLeft, finishPass)
	l.setCurrent(splitLeft)
	l.store(merges, scalarValue(l.pure(resource.RuntimeFunctionAdd, n, merges.slots[0], ir.Const{Value: 1}), types.Typ[types.Int]), n)
	left := l.alloc("linked.left", refType)
	right := l.alloc("linked.right", refType)
	rest := l.alloc("linked.rest", refType)
	l.store(left, current, n)
	leftTail := l.alloc("linked.leftTail", refType)
	l.store(leftTail, left, n)
	leftCount := l.alloc("linked.leftCount", types.Typ[types.Int])
	l.store(leftCount, scalarValue(ir.Const{Value: 1}, types.Typ[types.Int]), n)
	leftLoop, leftAdvance, leftDone := l.newBlock(), l.newBlock(), l.newBlock()
	l.jump(leftLoop)
	l.setCurrent(leftLoop)
	leftNext := l.materialize("linked.leftNext", nextOf(leftTail), n)
	leftMore := l.pure(resource.RuntimeFunctionAnd, n,
		l.pure(resource.RuntimeFunctionLess, n, leftCount.slots[0], width.slots[0]),
		l.pure(resource.RuntimeFunctionGreaterOr, n, leftNext.slots[0], ir.Const{}))
	_ = l.builder.Branch(leftMore, leftAdvance, leftDone)
	l.setCurrent(leftAdvance)
	l.store(leftTail, leftNext, n)
	l.store(leftCount, scalarValue(l.pure(resource.RuntimeFunctionAdd, n, leftCount.slots[0], ir.Const{Value: 1}), types.Typ[types.Int]), n)
	l.jump(leftLoop)
	l.setCurrent(leftDone)
	l.store(right, leftNext, n)
	setNextOf(leftTail, ref(ir.Const{Value: -1}))
	l.jump(splitRight)
	l.setCurrent(splitRight)
	rightTail := l.alloc("linked.rightTail", refType)
	l.store(rightTail, right, n)
	rightCount := l.alloc("linked.rightCount", types.Typ[types.Int])
	l.store(rightCount, scalarValue(ir.Const{Value: 1}, types.Typ[types.Int]), n)
	rightLoop, rightAdvance, rightDone, noRight := l.newBlock(), l.newBlock(), l.newBlock(), l.newBlock()
	_ = l.builder.Branch(l.pure(resource.RuntimeFunctionGreaterOr, n, right.slots[0], ir.Const{}), rightLoop, noRight)
	l.setCurrent(rightLoop)
	rightNext := l.materialize("linked.rightNext", nextOf(rightTail), n)
	rightMore := l.pure(resource.RuntimeFunctionAnd, n,
		l.pure(resource.RuntimeFunctionLess, n, rightCount.slots[0], width.slots[0]),
		l.pure(resource.RuntimeFunctionGreaterOr, n, rightNext.slots[0], ir.Const{}))
	_ = l.builder.Branch(rightMore, rightAdvance, rightDone)
	l.setCurrent(rightAdvance)
	l.store(rightTail, rightNext, n)
	l.store(rightCount, scalarValue(l.pure(resource.RuntimeFunctionAdd, n, rightCount.slots[0], ir.Const{Value: 1}), types.Typ[types.Int]), n)
	l.jump(rightLoop)
	l.setCurrent(rightDone)
	l.store(rest, rightNext, n)
	setNextOf(rightTail, ref(ir.Const{Value: -1}))
	l.jump(merge)
	l.setCurrent(noRight)
	l.store(rest, ref(ir.Const{Value: -1}), n)
	l.jump(merge)
	l.setCurrent(merge)
	leftValid := l.pure(resource.RuntimeFunctionGreaterOr, n, left.slots[0], ir.Const{})
	rightValid := l.pure(resource.RuntimeFunctionGreaterOr, n, right.slots[0], ir.Const{})
	chooseRightCheck, chooseLeftCheck := l.newBlock(), l.newBlock()
	_ = l.builder.Branch(leftValid, chooseRightCheck, appendRight)
	l.setCurrent(chooseRightCheck)
	_ = l.builder.Branch(rightValid, chooseLeftCheck, appendLeft)
	l.setCurrent(chooseLeftCheck)
	_ = l.builder.Branch(lessThan(right, left), appendRight, appendLeft)
	l.setCurrent(appendLeft)
	chosen := l.alloc("linked.chosen", refType)
	l.store(chosen, left, n)
	l.store(left, nextOf(left), n)
	l.jump(appended)
	l.setCurrent(appendRight)
	l.store(chosen, right, n)
	l.store(right, nextOf(right), n)
	l.jump(appended)
	l.setCurrent(appended)
	firstAppend, appendTail := l.newBlock(), l.newBlock()
	_ = l.builder.Branch(l.pure(resource.RuntimeFunctionLess, n, tail.slots[0], ir.Const{}), firstAppend, appendTail)
	l.setCurrent(firstAppend)
	l.store(newHead, chosen, n)
	l.jump(appendTail)
	l.setCurrent(appendTail)
	setTail, afterTail := l.newBlock(), l.newBlock()
	_ = l.builder.Branch(l.pure(resource.RuntimeFunctionGreaterOr, n, tail.slots[0], ir.Const{}), setTail, afterTail)
	l.setCurrent(setTail)
	setNextOf(tail, chosen)
	l.jump(afterTail)
	l.setCurrent(afterTail)
	l.store(tail, chosen, n)
	remaining := l.pure(resource.RuntimeFunctionOr, n,
		l.pure(resource.RuntimeFunctionGreaterOr, n, left.slots[0], ir.Const{}),
		l.pure(resource.RuntimeFunctionGreaterOr, n, right.slots[0], ir.Const{}))
	_ = l.builder.Branch(remaining, merge, nextRun)
	l.setCurrent(nextRun)
	l.store(current, rest, n)
	l.jump(outer)
	l.setCurrent(finishPass)
	setNextDone, afterTerminate := l.newBlock(), l.newBlock()
	_ = l.builder.Branch(l.pure(resource.RuntimeFunctionGreaterOr, n, tail.slots[0], ir.Const{}), setNextDone, afterTerminate)
	l.setCurrent(setNextDone)
	setNextOf(tail, ref(ir.Const{Value: -1}))
	l.jump(afterTerminate)
	l.setCurrent(afterTerminate)
	l.store(result, newHead, n)
	morePasses := l.pure(resource.RuntimeFunctionGreater, n, merges.slots[0], ir.Const{Value: 1})
	continuePass := l.newBlock()
	_ = l.builder.Branch(morePasses, continuePass, done)
	l.setCurrent(continuePass)
	l.store(width, scalarValue(l.pure(resource.RuntimeFunctionMultiply, n, width.slots[0], ir.Const{Value: 2}), types.Typ[types.Int]), n)
	l.jump(pass)
	l.setCurrent(done)
	if setPrevious != nil {
		previous := l.alloc("linked.previous", refType)
		cursor := l.alloc("linked.previousCursor", refType)
		l.store(previous, ref(ir.Const{Value: -1}), n)
		l.store(cursor, result, n)
		header, body, exit := l.newBlock(), l.newBlock(), l.newBlock()
		l.jump(header)
		l.setCurrent(header)
		_ = l.builder.Branch(l.pure(resource.RuntimeFunctionGreaterOr, n, cursor.slots[0], ir.Const{}), body, exit)
		l.setCurrent(body)
		following := l.materialize("linked.following", nextOf(cursor), n)
		l.inlineStaticCallable(n, setPrevious, []callArgument{{value: view(cursor)}, {value: previous}})
		l.store(previous, cursor, n)
		l.store(cursor, following, n)
		l.jump(header)
		l.setCurrent(exit)
	}
	return result
}

func (l *lowerer) diagnosticText(n ast.Node, message string) string {
	position := l.pkg.Fset.Position(n.Pos())
	file := canonicalSourceFile(l.pkg, position.Filename)
	text := fmt.Sprintf("%s/%s/%s:%s:%d:%d: %s", l.mode, l.phase, l.builder.Function().Name, file, position.Line, position.Column, message)
	for i := len(l.inlineCalls) - 1; i >= 0; i-- {
		call := l.inlineCalls[i]
		text += fmt.Sprintf("\ninlined from %s:%d:%d (%s)", filepath.ToSlash(call.pos.File), call.pos.Line, call.pos.Column, call.function)
	}
	return text
}

func (l *lowerer) guard(n ast.Node, condition ir.Expr) {
	l.guardWith(n, condition, "runtime check failed", false)
}

func (l *lowerer) terminateRuntime(n ast.Node, message string) {
	l.notifyRuntime(n, message)
	_ = l.builder.MarkUnreachable()
}

func (l *lowerer) notifyRuntime(n ast.Node, message string) {
	if l.checks == RuntimeChecksNotify {
		log := l.builder.RuntimeCall(resource.RuntimeFunctionDebugLog, []ir.Expr{ir.Const{}}, ir.Type{Name: "void"}, false, sourcePos(l.pkg, n.Pos()))
		log.Diagnostic = l.diagnosticText(n, message)
		pause := l.builder.RuntimeCall(resource.RuntimeFunctionDebugPause, nil, ir.Type{Name: "void"}, false, sourcePos(l.pkg, n.Pos()))
		_ = l.builder.Eval(log)
		_ = l.builder.Eval(pause)
	}
}

func (l *lowerer) guardWith(n ast.Node, condition ir.Expr, message string, required bool) {
	if value, ok := condition.(ir.Const); ok {
		if value.Value != 0 {
			return
		}
		l.errorAt(n, "%s", message)
		return
	}
	if l.checks == RuntimeChecksNone && !required {
		return
	}
	valid, invalid := l.newBlock(), l.newBlock()
	_ = l.builder.Branch(condition, valid, invalid)
	l.setCurrent(invalid)
	l.terminateRuntime(n, message)
	l.setCurrent(valid)
}

func (l *lowerer) containerSizeValue(receiver lowerValue) lowerValue {
	return lowerValue{type_: types.Typ[types.Int], slots: receiver.slots[:1], places: receiver.places[:1]}
}

func (l *lowerer) containerRemoveAt(n ast.Node, receiver, index lowerValue) {
	c := receiver.container
	cursor := l.alloc("container.remove.index", types.Typ[types.Int])
	l.store(cursor, index, n)
	header, body, exit := l.newBlock(), l.newBlock(), l.newBlock()
	l.jump(header)
	l.setCurrent(header)
	last := l.pure(resource.RuntimeFunctionSubtract, n, receiver.slots[0], ir.Const{Value: 1})
	_ = l.builder.Branch(l.pure(resource.RuntimeFunctionLess, n, cursor.slots[0], last), body, exit)
	l.setCurrent(body)
	next := l.pure(resource.RuntimeFunctionAdd, n, cursor.slots[0], ir.Const{Value: 1})
	source := l.containerElement(n, c, next, 0, c.stride, types.Typ[types.Float64])
	target := l.containerElement(n, c, cursor.slots[0], 0, c.stride, types.Typ[types.Float64])
	if err := l.builder.Store(target.places, ir.Value{Type: ir.Type{Name: "container.entry", Slots: c.stride}, Slots: source.slots}, sourcePos(l.pkg, n.Pos())); err != nil {
		l.errorAt(n, "%v", err)
	}
	l.store(cursor, lowerValue{type_: types.Typ[types.Int], slots: []ir.Expr{next}}, n)
	l.jump(header)
	l.setCurrent(exit)
	l.store(l.containerSizeValue(receiver), lowerValue{type_: types.Typ[types.Int], slots: []ir.Expr{last}}, n)
}

func (l *lowerer) containerInsert(n ast.Node, receiver, index, value lowerValue) {
	c := receiver.container
	cursor := l.alloc("container.insert.index", types.Typ[types.Int])
	l.store(cursor, l.containerSizeValue(receiver), n)
	header, body, exit := l.newBlock(), l.newBlock(), l.newBlock()
	l.jump(header)
	l.setCurrent(header)
	_ = l.builder.Branch(l.pure(resource.RuntimeFunctionGreater, n, cursor.slots[0], index.slots[0]), body, exit)
	l.setCurrent(body)
	previous := l.pure(resource.RuntimeFunctionSubtract, n, cursor.slots[0], ir.Const{Value: 1})
	source := l.containerElement(n, c, previous, 0, c.stride, c.element)
	target := l.containerElement(n, c, cursor.slots[0], 0, c.stride, c.element)
	l.store(target, source, n)
	l.store(cursor, lowerValue{type_: types.Typ[types.Int], slots: []ir.Expr{previous}}, n)
	l.jump(header)
	l.setCurrent(exit)
	l.store(l.containerElement(n, c, index.slots[0], 0, c.stride, c.element), value, n)
	l.store(l.containerSizeValue(receiver), lowerValue{type_: types.Typ[types.Int], slots: []ir.Expr{l.pure(resource.RuntimeFunctionAdd, n, receiver.slots[0], ir.Const{Value: 1})}}, n)
}

func (l *lowerer) containerElement(n ast.Node, c *containerValue, index ir.Expr, offset, count int, t types.Type) lowerValue {
	v := lowerValue{type_: t, slots: make([]ir.Expr, count), places: make([]ir.Place, count)}
	for i := 0; i < count; i++ {
		var p ir.Place
		if c.dataLocal != nil {
			p = l.indexedLocal(*c.dataLocal, index, c.capacity, c.stride, offset+i, n)
		} else if c.memoryEntity != nil {
			combined := l.pure(resource.RuntimeFunctionAdd, n,
				l.pure(resource.RuntimeFunctionMultiply, n, c.memoryEntity, ir.Const{Value: 32}),
				l.pure(resource.RuntimeFunctionMultiply, n, index, ir.Const{Value: float64(c.stride)}))
			p = l.memory(c.memoryStorage, combined, 1, c.memoryBase+offset+i, c.memoryRead, c.memoryWrite, n)
		} else if c.memoryBaseExpr != nil {
			combined := l.pure(resource.RuntimeFunctionAdd, n, c.memoryBaseExpr,
				l.pure(resource.RuntimeFunctionAdd, n,
					l.pure(resource.RuntimeFunctionMultiply, n, index, ir.Const{Value: float64(c.stride)}),
					ir.Const{Value: float64(offset + i)}))
			p = l.memory(c.memoryStorage, combined, 1, 0, c.memoryRead, c.memoryWrite, n)
		} else {
			p = l.memory(c.memoryStorage, index, c.stride, c.memoryBase+offset+i, c.memoryRead, c.memoryWrite, n)
		}
		v.places[i], v.slots[i] = p, ir.Load{Place: p}
	}
	if binding, exists := l.entityBinding(t); exists {
		v.entity = &entityReferenceValue{binding: binding}
	}
	return v
}

func (l *lowerer) containerFind(n ast.Node, receiver, needle lowerValue, offset, count int) (lowerValue, lowerValue) {
	c := receiver.container
	index := l.alloc("container.find.index", types.Typ[types.Int])
	found := l.alloc("container.find.found", types.Typ[types.Bool])
	l.store(index, lowerValue{type_: types.Typ[types.Int], slots: []ir.Expr{ir.Const{}}}, n)
	l.store(found, lowerValue{type_: types.Typ[types.Bool], slots: []ir.Expr{ir.Const{}}}, n)
	header, body, match, next, exit := l.newBlock(), l.newBlock(), l.newBlock(), l.newBlock(), l.newBlock()
	l.jump(header)
	l.setCurrent(header)
	_ = l.builder.Branch(l.pure(resource.RuntimeFunctionLess, n, index.slots[0], receiver.slots[0]), body, exit)
	l.setCurrent(body)
	candidate := l.containerElement(n, c, index.slots[0], offset, count, needle.type_)
	var equal ir.Expr = ir.Const{Value: 1}
	for i := 0; i < count; i++ {
		equal = l.pure(resource.RuntimeFunctionAnd, n, equal, l.pure(resource.RuntimeFunctionEqual, n, candidate.slots[i], needle.slots[i]))
	}
	_ = l.builder.Branch(equal, match, next)
	l.setCurrent(match)
	l.store(found, lowerValue{type_: types.Typ[types.Bool], slots: []ir.Expr{ir.Const{Value: 1}}}, n)
	l.jump(exit)
	l.setCurrent(next)
	l.store(index, lowerValue{type_: types.Typ[types.Int], slots: []ir.Expr{l.pure(resource.RuntimeFunctionAdd, n, index.slots[0], ir.Const{Value: 1})}}, n)
	l.jump(header)
	l.setCurrent(exit)
	return index, found
}

func (l *lowerer) resourceCall(n *ast.CallExpr, operation string, args []lowerValue) lowerValue {
	if operation == "touch.values" || operation == "touch.items" {
		if len(args) != 0 {
			l.errorAt(n, "%s does not accept arguments", operation)
			return lowerValue{}
		}
		sequenceType := l.resolveType(l.pkg.TypesInfo.TypeOf(n))
		sequence, ok := types.Unalias(sequenceType).Underlying().(*types.Signature)
		if !ok || sequence.Params().Len() != 1 {
			l.errorAt(n, "%s has an invalid iterator signature", operation)
			return lowerValue{}
		}
		yield, ok := types.Unalias(sequence.Params().At(0).Type()).Underlying().(*types.Signature)
		touchIndex := 0
		if operation == "touch.items" {
			touchIndex = 1
		}
		if !ok || yield.Params().Len() <= touchIndex {
			l.errorAt(n, "%s has an invalid yield signature", operation)
			return lowerValue{}
		}
		return lowerValue{type_: sequenceType, callable: &staticCallable{touchIter: &touchIterator{index: operation == "touch.items", touchType: yield.Params().At(touchIndex).Type()}}}
	}
	if strings.HasPrefix(operation, "stream.") {
		if len(args) == 0 || len(args[0].slots) == 0 {
			l.errorAt(n, "%s requires a stream receiver", operation)
			return zeroValue(l.pkg.TypesInfo.TypeOf(n))
		}
		receiver := args[0]
		iteratorKind := ""
		descending, frame := false, false
		switch operation {
		case "stream.itemsFrom", "stream.itemsFromDescending", "stream.itemsSincePreviousFrame":
			iteratorKind = "items"
		case "stream.keysFrom", "stream.keysFromDescending", "stream.keysSincePreviousFrame":
			iteratorKind = "keys"
		case "stream.valuesFrom", "stream.valuesFromDescending", "stream.valuesSincePreviousFrame":
			iteratorKind = "values"
		}
		if iteratorKind != "" {
			descending = strings.HasSuffix(operation, "Descending")
			frame = strings.HasSuffix(operation, "SincePreviousFrame")
			if (!frame && (len(args) != 2 || len(args[1].slots) != 1)) || (frame && len(args) != 1) {
				l.errorAt(n, "%s has invalid arguments", operation)
				return lowerValue{}
			}
			start := ir.Expr(ir.Const{})
			if !frame {
				start = args[1].slots[0]
			}
			return lowerValue{type_: l.pkg.TypesInfo.TypeOf(n), callable: &staticCallable{streamIter: &streamIterator{receiver: receiver, kind: iteratorKind, start: start, desc: descending, frame: frame}}}
		}
		if len(args) < 2 || len(args[1].slots) != 1 {
			l.errorAt(n, "%s requires a stream receiver and key", operation)
			return zeroValue(l.pkg.TypesInfo.TypeOf(n))
		}
		key := args[1].slots[0]
		if operation == "stream.set" {
			if l.mode != mode.ModePlay {
				l.errorAt(n, "stream writes are only available in play mode")
				return lowerValue{}
			}
			if len(args) != 3 || len(args[0].slots) != len(args[2].slots) {
				l.errorAt(n, "stream Set value layout does not match stream layout")
				return lowerValue{}
			}
			for index, id := range receiver.slots {
				call := l.builder.RuntimeCall(resource.RuntimeFunctionStreamSet, []ir.Expr{id, key, args[2].slots[index]}, ir.Type{Name: "void"}, false, sourcePos(l.pkg, n.Pos()))
				_ = l.builder.Eval(call)
			}
			return lowerValue{}
		}
		if l.mode != mode.ModeWatch {
			l.errorAt(n, "stream queries are only available in watch mode")
			return zeroValue(l.pkg.TypesInfo.TypeOf(n))
		}
		switch operation {
		case "stream.has":
			return scalarValue(l.streamHas(n, receiver, key), types.Typ[types.Bool])
		case "stream.previousKey", "stream.nextKey":
			return scalarValue(l.streamKey(n, receiver, key, operation == "stream.nextKey"), l.pkg.TypesInfo.TypeOf(n))
		case "stream.get":
			return l.streamValueAt(n, receiver, key, l.pkg.TypesInfo.TypeOf(n))
		case "stream.previousKeyOrDefault", "stream.nextKeyOrDefault":
			if len(args) != 3 || len(args[2].slots) != 1 {
				l.errorAt(n, "%s requires key and fallback", operation)
				return zeroValue(l.pkg.TypesInfo.TypeOf(n))
			}
			next := strings.Contains(operation, "next")
			candidate := l.streamKey(n, receiver, key, next)
			comparison := l.pure(resource.RuntimeFunctionLess, n, candidate, key)
			if next {
				comparison = l.pure(resource.RuntimeFunctionGreater, n, candidate, key)
			}
			return scalarValue(l.pure(resource.RuntimeFunctionIf, n, comparison, candidate, args[2].slots[0]), l.pkg.TypesInfo.TypeOf(n))
		case "stream.hasPreviousKey", "stream.hasNextKey":
			next := strings.Contains(operation, "Next")
			candidate := l.streamKey(n, receiver, key, next)
			comparison := resource.RuntimeFunctionLess
			if next {
				comparison = resource.RuntimeFunctionGreater
			}
			return scalarValue(l.pure(comparison, n, candidate, key), types.Typ[types.Bool])
		case "stream.previousKeyInclusive", "stream.nextKeyInclusive":
			next := strings.Contains(operation, "next")
			candidate := l.streamKey(n, receiver, key, next)
			return scalarValue(l.pure(resource.RuntimeFunctionIf, n, l.streamHas(n, receiver, key), key, candidate), l.pkg.TypesInfo.TypeOf(n))
		case "stream.getPrevious", "stream.getNext", "stream.getPreviousInclusive", "stream.getNextInclusive":
			next := strings.Contains(operation, "Next")
			candidate := l.streamKey(n, receiver, key, next)
			if strings.HasSuffix(operation, "Inclusive") {
				candidate = l.pure(resource.RuntimeFunctionIf, n, l.streamHas(n, receiver, key), key, candidate)
			}
			return l.streamValueAt(n, receiver, candidate, l.pkg.TypesInfo.TypeOf(n))
		}
	}
	if strings.HasPrefix(operation, "streamData.") {
		if len(args) == 0 || len(args[0].slots) != 1 {
			l.errorAt(n, "%s requires a StreamData receiver", operation)
			return zeroValue(l.pkg.TypesInfo.TypeOf(n))
		}
		id := args[0].slots[0]
		switch operation {
		case "streamData.set":
			if l.mode != mode.ModePlay {
				l.errorAt(n, "StreamData writes are only available in play mode")
				return lowerValue{}
			}
			if len(args) != 2 {
				l.errorAt(n, "StreamData.Set requires one value")
				return lowerValue{}
			}
			for index, value := range args[1].slots {
				for _, key := range []float64{float64(index) - 0.5, float64(index), float64(index) + 0.5} {
					stored := value
					if key != float64(index) {
						stored = ir.Const{}
					}
					_ = l.builder.Eval(l.builder.RuntimeCall(resource.RuntimeFunctionStreamSet, []ir.Expr{id, ir.Const{Value: key}, stored}, ir.Type{Name: "void"}, false, sourcePos(l.pkg, n.Pos())))
				}
			}
			return lowerValue{}
		case "streamData.get":
			if l.mode != mode.ModeWatch {
				l.errorAt(n, "StreamData reads are only available in watch mode")
				return zeroValue(l.pkg.TypesInfo.TypeOf(n))
			}
			result := zeroValue(l.pkg.TypesInfo.TypeOf(n))
			for index := range result.slots {
				result.slots[index] = l.builder.RuntimeCall(resource.RuntimeFunctionStreamGetValue, []ir.Expr{id, ir.Const{Value: float64(index)}}, ir.Type{Name: "stream.data", Slots: 1}, false, sourcePos(l.pkg, n.Pos()))
			}
			return result
		}
	}
	if operation == "archetype.id" {
		if len(args) != 0 {
			l.errorAt(n, "ArchetypeID does not accept runtime arguments")
			return zeroValue(l.pkg.TypesInfo.TypeOf(n))
		}
		target, ok := l.callTypeArgument(n, 0)
		binding, exists := l.entityBinding(target)
		if !ok || !exists {
			l.errorAt(n, "ArchetypeID target %s is not an archetype declared in %s mode", target, l.mode)
			return zeroValue(l.pkg.TypesInfo.TypeOf(n))
		}
		if binding.id < 0 || binding.declaration.Abstract {
			l.errorAt(n, "ArchetypeID target %s is abstract and has no runtime ID", target)
			return zeroValue(l.pkg.TypesInfo.TypeOf(n))
		}
		return scalarValue(ir.Const{Value: float64(binding.id)}, l.pkg.TypesInfo.TypeOf(n))
	}
	if operation == "archetype.key" {
		if len(args) != 0 {
			l.errorAt(n, "ArchetypeKey does not accept runtime arguments")
			return zeroValue(l.pkg.TypesInfo.TypeOf(n))
		}
		target, ok := l.callTypeArgument(n, 0)
		binding, exists := l.entityBinding(target)
		if !ok || !exists {
			l.errorAt(n, "ArchetypeKey target %s is not an archetype declared in %s mode", target, l.mode)
			return zeroValue(l.pkg.TypesInfo.TypeOf(n))
		}
		if binding.declaration.Abstract {
			l.errorAt(n, "ArchetypeKey target %s is abstract and has no single runtime key", target)
			return zeroValue(l.pkg.TypesInfo.TypeOf(n))
		}
		key := -1.0
		if binding.declaration.HasKey {
			key = binding.declaration.Key
		}
		return scalarValue(ir.Const{Value: key}, l.pkg.TypesInfo.TypeOf(n))
	}
	if operation == "entity.currentRef" {
		if len(args) != 0 {
			l.errorAt(n, "CurrentEntityRef does not accept runtime arguments")
			return zeroValue(l.pkg.TypesInfo.TypeOf(n))
		}
		if l.currentArchetype == nil {
			l.errorAt(n, "CurrentEntityRef is only available in archetype callbacks")
			return zeroValue(l.pkg.TypesInfo.TypeOf(n))
		}
		target, ok := l.callTypeArgument(n, 0)
		binding, exists := l.entityBinding(target)
		anyTarget := typeID(target) == rootID("AnyArchetype")
		if !ok || (!exists && !anyTarget) {
			l.errorAt(n, "CurrentEntityRef target %s is not an archetype declared in %s mode", target, l.mode)
			return zeroValue(l.pkg.TypesInfo.TypeOf(n))
		}
		assignable := anyTarget
		if exists {
			for _, member := range l.currentArchetype.MRO {
				if member == binding.declaration {
					assignable = true
					break
				}
			}
		}
		if !assignable {
			l.errorAt(n, "CurrentEntityRef target %s is not a base of current archetype %s", target, l.currentArchetype.Name)
			return zeroValue(l.pkg.TypesInfo.TypeOf(n))
		}
		place := l.memory("CurrentEntityInfo", ir.Const{}, 0, 0, true, false, n)
		return lowerValue{type_: l.pkg.TypesInfo.TypeOf(n), slots: []ir.Expr{ir.Load{Place: place}}}
	}
	if operation == "entityRef.as" {
		if len(args) != 1 || len(args[0].slots) != 1 {
			l.errorAt(n, "EntityRefAs requires a one-slot entity reference")
			return zeroValue(l.pkg.TypesInfo.TypeOf(n))
		}
		return lowerValue{type_: l.pkg.TypesInfo.TypeOf(n), slots: []ir.Expr{args[0].slots[0]}}
	}
	if operation == "entityRef.key" {
		if len(args) != 1 || len(args[0].slots) != 1 {
			l.errorAt(n, "EntityRef.Key requires a one-slot entity reference")
			return zeroValue(l.pkg.TypesInfo.TypeOf(n))
		}
		stride := 3
		if l.mode == mode.ModePreview {
			stride = 2
		}
		place := l.memory("EntityInfo", args[0].slots[0], stride, 1, true, false, n)
		actual := ir.Expr(ir.Load{Place: place})
		key := ir.Expr(ir.Const{Value: -1})
		candidates := make([]archetypeBinding, 0, len(l.archetypes))
		for _, candidate := range l.archetypes {
			if candidate.id >= 0 {
				candidates = append(candidates, candidate)
			}
		}
		sort.Slice(candidates, func(i, j int) bool { return candidates[i].id > candidates[j].id })
		for _, candidate := range candidates {
			matches := l.pure(resource.RuntimeFunctionEqual, n, actual, ir.Const{Value: float64(candidate.id)})
			key = l.pure(resource.RuntimeFunctionIf, n, matches, ir.Const{Value: candidate.declaration.Key}, key)
		}
		return lowerValue{type_: l.pkg.TypesInfo.TypeOf(n), slots: []ir.Expr{key}}
	}
	if operation == "entityRef.matches" {
		if len(args) != 2 || len(args[0].slots) != 1 || len(args[1].slots) != 1 {
			l.errorAt(n, "EntityRefMatches requires an entity reference and strict flag")
			return zeroValue(l.pkg.TypesInfo.TypeOf(n))
		}
		target, ok := l.callTypeArgument(n, 0)
		binding, exists := l.entityBinding(target)
		anyTarget := typeID(target) == rootID("AnyArchetype")
		if !ok || (!exists && !anyTarget) {
			l.errorAt(n, "EntityRefMatches target %s is not an archetype declared in %s mode", target, l.mode)
			return zeroValue(l.pkg.TypesInfo.TypeOf(n))
		}
		stride := 3
		if l.mode == mode.ModePreview {
			stride = 2
		}
		place := l.memory("EntityInfo", args[0].slots[0], stride, 1, true, false, n)
		actual := ir.Expr(ir.Load{Place: place})
		var strict ir.Expr = ir.Const{}
		if exists {
			strict = l.pure(resource.RuntimeFunctionEqual, n, actual, ir.Const{Value: float64(binding.id)})
		}
		var derived ir.Expr = ir.Const{}
		candidates := make([]archetypeBinding, 0, len(l.archetypes))
		for _, candidate := range l.archetypes {
			candidates = append(candidates, candidate)
		}
		sort.Slice(candidates, func(i, j int) bool { return candidates[i].id < candidates[j].id })
		for _, candidate := range candidates {
			if candidate.id < 0 {
				continue
			}
			if anyTarget {
				derived = l.pure(resource.RuntimeFunctionOr, n, derived, l.pure(resource.RuntimeFunctionEqual, n, actual, ir.Const{Value: float64(candidate.id)}))
				continue
			}
			for _, member := range candidate.declaration.MRO {
				if member == binding.declaration {
					derived = l.pure(resource.RuntimeFunctionOr, n, derived, l.pure(resource.RuntimeFunctionEqual, n, actual, ir.Const{Value: float64(candidate.id)}))
					break
				}
			}
		}
		if anyTarget {
			strict = derived
		}
		result := l.pure(resource.RuntimeFunctionIf, n, args[1].slots[0], strict, derived)
		return lowerValue{type_: types.Typ[types.Bool], slots: []ir.Expr{result}}
	}
	if operation == "entityRef.get" || operation == "entityRef.getUnchecked" || operation == "entityRef.getAs" {
		if len(args) != 1 || len(args[0].slots) != 1 {
			l.errorAt(n, "%s requires a one-slot entity reference", operation)
			return zeroValue(l.pkg.TypesInfo.TypeOf(n))
		}
		target := l.resolveType(l.pkg.TypesInfo.TypeOf(n))
		if pointer, ok := types.Unalias(target).(*types.Pointer); ok {
			target = pointer.Elem()
		}
		binding, exists := l.entityBinding(target)
		if !exists {
			l.errorAt(n, "%s target %s is not an archetype declared in %s mode", operation, target, l.mode)
			return zeroValue(l.pkg.TypesInfo.TypeOf(n))
		}
		if operation != "entityRef.getUnchecked" {
			stride := 3
			if l.mode == mode.ModePreview {
				stride = 2
			}
			place := l.memory("EntityInfo", args[0].slots[0], stride, 1, true, false, n)
			actual := ir.Expr(ir.Load{Place: place})
			var matches ir.Expr = ir.Const{}
			candidates := make([]archetypeBinding, 0, len(l.archetypes))
			for _, candidate := range l.archetypes {
				candidates = append(candidates, candidate)
			}
			sort.Slice(candidates, func(i, j int) bool { return candidates[i].id < candidates[j].id })
			for _, candidate := range candidates {
				if candidate.id < 0 {
					continue
				}
				for _, member := range candidate.declaration.MRO {
					if member == binding.declaration {
						matches = l.pure(resource.RuntimeFunctionOr, n, matches, l.pure(resource.RuntimeFunctionEqual, n, actual, ir.Const{Value: float64(candidate.id)}))
						break
					}
				}
			}
			valid := l.pure(resource.RuntimeFunctionAnd, n, l.pure(resource.RuntimeFunctionGreaterOr, n, args[0].slots[0], ir.Const{}), matches)
			l.guardWith(n, valid, "EntityRef target does not match requested archetype", false)
		}
		return lowerValue{
			type_: l.pkg.TypesInfo.TypeOf(n), slots: []ir.Expr{args[0].slots[0]},
			entity: &entityReferenceValue{binding: binding},
		}
	}
	if operation == "archetype.spawn" {
		if len(args) != 1 {
			l.errorAt(n, "Spawn requires one archetype value")
			return lowerValue{}
		}
		named, ok := namedType(args[0].type_)
		binding, exists := l.archetypes[named]
		if !ok || !exists || binding.id < 0 {
			l.errorAt(n, "Spawn argument type %s is not an archetype declared in %s mode", args[0].type_, l.mode)
			return lowerValue{}
		}
		callArgs := []ir.Expr{ir.Const{Value: float64(binding.id)}}
		for _, field := range binding.declaration.Fields {
			if field.Storage != "memory" {
				continue
			}
			start, end := field.ReceiverOffset, field.ReceiverOffset+field.Size
			if start < 0 || end > len(args[0].slots) {
				l.errorAt(n, "Spawn data layout for %s is incomplete", binding.declaration.Name)
				return lowerValue{}
			}
			callArgs = append(callArgs, args[0].slots[start:end]...)
		}
		call := l.builder.RuntimeCall(resource.RuntimeFunctionSpawn, callArgs, ir.Type{Name: "void"}, false, sourcePos(l.pkg, n.Pos()))
		_ = l.builder.Eval(call)
		return lowerValue{}
	}
	if operation == "instruction.show" || operation == "instruction.clear" {
		value := ir.Expr(ir.Const{Value: -1})
		if operation == "instruction.show" {
			if len(args) != 1 || len(args[0].slots) != 1 {
				l.errorAt(n, "instruction show requires one text handle")
				return lowerValue{}
			}
			value = args[0].slots[0]
		}
		place := l.memory("TutorialInstruction", ir.Const{}, 0, 0, true, true, n)
		if err := l.builder.Store([]ir.Place{place}, ir.Value{Type: ir.Type{Name: "instruction", Slots: 1}, Slots: []ir.Expr{value}}, sourcePos(l.pkg, n.Pos())); err != nil {
			l.errorAt(n, "%v", err)
		}
		return lowerValue{}
	}
	flat := func(indices ...int) ([]ir.Expr, bool) {
		var out []ir.Expr
		for _, index := range indices {
			if index >= len(args) {
				l.errorAt(n, "resource operation %s argument mismatch", operation)
				return nil, false
			}
			out = append(out, args[index].slots...)
		}
		return out, true
	}
	var runtime resource.RuntimeFunction
	pure := false
	var callArgs []ir.Expr
	var returns bool
	switch operation {
	case "sprite.draw":
		values, ok := flat(0, 1)
		if !ok || len(values) != 9 || len(args) < 4 {
			l.errorAt(n, "sprite draw layout mismatch")
			return lowerValue{}
		}
		z, alpha := args[2].slots[0], args[3].slots[0]
		callArgs = append(values, z, alpha, z, z, z)
		runtime = resource.RuntimeFunctionDraw
	case "sprite.drawCurvedB", "sprite.drawCurvedT", "sprite.drawCurvedL", "sprite.drawCurvedR":
		values, ok := flat(0, 1)
		if !ok || len(values) != 9 || len(args) != 6 || len(args[2].slots) != 2 {
			l.errorAt(n, "single-edge curved sprite draw layout mismatch")
			return lowerValue{}
		}
		control, segments, z, alpha := args[2].slots, args[3].slots[0], args[4].slots[0], args[5].slots[0]
		callArgs = append(values, z, alpha, segments, control[0], control[1], z, z, z)
		runtime = map[string]resource.RuntimeFunction{
			"sprite.drawCurvedB": resource.RuntimeFunctionDrawCurvedB,
			"sprite.drawCurvedT": resource.RuntimeFunctionDrawCurvedT,
			"sprite.drawCurvedL": resource.RuntimeFunctionDrawCurvedL,
			"sprite.drawCurvedR": resource.RuntimeFunctionDrawCurvedR,
		}[operation]
	case "sprite.drawCurvedBT", "sprite.drawCurvedLR":
		values, ok := flat(0, 1)
		if !ok || len(values) != 9 || len(args) != 7 || len(args[2].slots) != 2 || len(args[3].slots) != 2 {
			l.errorAt(n, "two-edge curved sprite draw layout mismatch")
			return lowerValue{}
		}
		first, second := args[2].slots, args[3].slots
		segments, z, alpha := args[4].slots[0], args[5].slots[0], args[6].slots[0]
		callArgs = append(values, z, alpha, segments, first[0], first[1], second[0], second[1], z, z, z)
		if operation == "sprite.drawCurvedBT" {
			runtime = resource.RuntimeFunctionDrawCurvedBT
		} else {
			runtime = resource.RuntimeFunctionDrawCurvedLR
		}
	case "judgment.judge":
		values, ok := flat(1, 2, 0)
		if !ok || len(values) != 8 {
			l.errorAt(n, "judgment window layout mismatch")
			return lowerValue{}
		}
		callArgs = values
		runtime = resource.RuntimeFunctionJudge
		pure, returns = true, true
	case "sprite.exists":
		callArgs, _ = flat(0)
		runtime = resource.RuntimeFunctionHasSkinSprite
		pure, returns = true, true
	case "clip.play":
		callArgs, _ = flat(0, 1)
		runtime = resource.RuntimeFunctionPlay
	case "clip.playScheduled":
		callArgs, _ = flat(0, 1, 2)
		runtime = resource.RuntimeFunctionPlayScheduled
	case "clip.playLooped":
		callArgs, _ = flat(0)
		runtime = resource.RuntimeFunctionPlayLooped
		returns = true
	case "clip.playLoopedScheduled":
		callArgs, _ = flat(0, 1)
		runtime = resource.RuntimeFunctionPlayLoopedScheduled
		returns = true
	case "loop.stop":
		callArgs, _ = flat(0)
		runtime = resource.RuntimeFunctionStopLooped
	case "loop.stopScheduled":
		callArgs, _ = flat(0, 1)
		runtime = resource.RuntimeFunctionStopLoopedScheduled
	case "particle.spawn":
		callArgs, _ = flat(0, 1, 2, 3)
		runtime = resource.RuntimeFunctionSpawnParticleEffect
		returns = true
	case "particle.move":
		callArgs, _ = flat(0, 1)
		runtime = resource.RuntimeFunctionMoveParticleEffect
	case "particle.destroy":
		callArgs, _ = flat(0)
		runtime = resource.RuntimeFunctionDestroyParticleEffect
	default:
		l.errorAt(n, "unknown resource recipe %s", operation)
		return lowerValue{}
	}
	callResult := ir.Type{Name: "void"}
	if returns {
		callResult = irTypeOf(l.pkg.TypesInfo.TypeOf(n))
	}
	call := l.builder.RuntimeCall(runtime, callArgs, callResult, pure, sourcePos(l.pkg, n.Pos()))
	if !returns {
		if !pure {
			_ = l.builder.Eval(call)
		}
		return lowerValue{}
	}
	resultType := l.pkg.TypesInfo.TypeOf(n)
	if pure {
		return lowerValue{type_: resultType, slots: []ir.Expr{call}}
	}
	result := l.alloc("runtime.result", resultType)
	if err := l.builder.Store(result.places, ir.Value{Type: irTypeOf(resultType), Slots: []ir.Expr{call}}, sourcePos(l.pkg, n.Pos())); err != nil {
		l.errorAt(n, "%v", err)
	}
	return result
}

func (l *lowerer) pure(op resource.RuntimeFunction, pos ast.Node, args ...ir.Expr) ir.Expr {
	return l.builder.RuntimeCall(op, args, ir.Type{Name: "number", Slots: 1}, true, sourcePos(l.pkg, pos.Pos()))
}

func (l *lowerer) aggregateCall(n *ast.CallExpr, operation string, args []lowerValue) lowerValue {
	result := lowerValue{type_: l.pkg.TypesInfo.TypeOf(n)}
	if result.type_ == nil {
		l.errorAt(n, "aggregate operation %s has no result", operation)
		return result
	}
	require := func(index, slots int) bool {
		if index >= len(args) || len(args[index].slots) != slots {
			l.errorAt(n, "aggregate operation %s received an invalid layout", operation)
			return false
		}
		return true
	}
	op := func(fn resource.RuntimeFunction, values ...ir.Expr) ir.Expr { return l.pure(fn, n, values...) }
	add := func(a, b ir.Expr) ir.Expr { return op(resource.RuntimeFunctionAdd, a, b) }
	sub := func(a, b ir.Expr) ir.Expr { return op(resource.RuntimeFunctionSubtract, a, b) }
	mul := func(a, b ir.Expr) ir.Expr { return op(resource.RuntimeFunctionMultiply, a, b) }
	div := func(a, b ir.Expr) ir.Expr { return op(resource.RuntimeFunctionDivide, a, b) }
	rotate := func(x, y, angle ir.Expr) []ir.Expr {
		c, s := op(resource.RuntimeFunctionCos, angle), op(resource.RuntimeFunctionSin, angle)
		return []ir.Expr{sub(mul(x, c), mul(y, s)), add(mul(x, s), mul(y, c))}
	}
	identityTransform := func() []ir.Expr {
		return []ir.Expr{ir.Const{Value: 1}, ir.Const{}, ir.Const{}, ir.Const{}, ir.Const{Value: 1}, ir.Const{}, ir.Const{}, ir.Const{}, ir.Const{Value: 1}}
	}
	compose := func(a, b []ir.Expr) []ir.Expr {
		result := make([]ir.Expr, 9)
		for row := range 3 {
			for column := range 3 {
				result[row*3+column] = add(add(mul(b[row*3], a[column]), mul(b[row*3+1], a[3+column])), mul(b[row*3+2], a[6+column]))
			}
		}
		return result
	}
	translateTransform := func(value []ir.Expr, x, y ir.Expr) []ir.Expr {
		return compose(value, []ir.Expr{ir.Const{Value: 1}, ir.Const{}, x, ir.Const{}, ir.Const{Value: 1}, y, ir.Const{}, ir.Const{}, ir.Const{Value: 1}})
	}
	scaleTransform := func(value []ir.Expr, x, y ir.Expr) []ir.Expr {
		return compose(value, []ir.Expr{x, ir.Const{}, ir.Const{}, ir.Const{}, y, ir.Const{}, ir.Const{}, ir.Const{}, ir.Const{Value: 1}})
	}
	rotateTransform := func(value []ir.Expr, angle ir.Expr) []ir.Expr {
		c, s := op(resource.RuntimeFunctionCos, angle), op(resource.RuntimeFunctionSin, angle)
		return compose(value, []ir.Expr{c, op(resource.RuntimeFunctionNegate, s), ir.Const{}, s, c, ir.Const{}, ir.Const{}, ir.Const{}, ir.Const{Value: 1}})
	}
	shearXTransform := func(value []ir.Expr, amount ir.Expr) []ir.Expr {
		return compose(value, []ir.Expr{ir.Const{Value: 1}, amount, ir.Const{}, ir.Const{}, ir.Const{Value: 1}, ir.Const{}, ir.Const{}, ir.Const{}, ir.Const{Value: 1}})
	}
	shearYTransform := func(value []ir.Expr, amount ir.Expr) []ir.Expr {
		return compose(value, []ir.Expr{ir.Const{Value: 1}, ir.Const{}, ir.Const{}, amount, ir.Const{Value: 1}, ir.Const{}, ir.Const{}, ir.Const{}, ir.Const{Value: 1}})
	}
	simplePerspectiveXTransform := func(value []ir.Expr, x ir.Expr) []ir.Expr {
		return compose(value, []ir.Expr{ir.Const{Value: 1}, ir.Const{}, ir.Const{}, ir.Const{}, ir.Const{Value: 1}, ir.Const{}, div(ir.Const{Value: 1}, x), ir.Const{}, ir.Const{Value: 1}})
	}
	simplePerspectiveYTransform := func(value []ir.Expr, y ir.Expr) []ir.Expr {
		return compose(value, []ir.Expr{ir.Const{Value: 1}, ir.Const{}, ir.Const{}, ir.Const{}, ir.Const{Value: 1}, ir.Const{}, ir.Const{}, div(ir.Const{Value: 1}, y), ir.Const{Value: 1}})
	}
	perspectiveXTransform := func(value []ir.Expr, foreground ir.Expr, vanishing []ir.Expr) []ir.Expr {
		distance := sub(vanishing[0], foreground)
		value = simplePerspectiveXTransform(value, distance)
		value = shearYTransform(value, div(vanishing[1], distance))
		return translateTransform(value, foreground, ir.Const{})
	}
	perspectiveYTransform := func(value []ir.Expr, foreground ir.Expr, vanishing []ir.Expr) []ir.Expr {
		distance := sub(vanishing[1], foreground)
		value = simplePerspectiveYTransform(value, distance)
		value = shearXTransform(value, div(vanishing[0], distance))
		return translateTransform(value, ir.Const{}, foreground)
	}
	inversePerspectiveXTransform := func(value []ir.Expr, foreground ir.Expr, vanishing []ir.Expr) []ir.Expr {
		distance := sub(vanishing[0], foreground)
		value = translateTransform(value, op(resource.RuntimeFunctionNegate, foreground), ir.Const{})
		value = shearYTransform(value, op(resource.RuntimeFunctionNegate, div(vanishing[1], distance)))
		return simplePerspectiveXTransform(value, op(resource.RuntimeFunctionNegate, distance))
	}
	inversePerspectiveYTransform := func(value []ir.Expr, foreground ir.Expr, vanishing []ir.Expr) []ir.Expr {
		distance := sub(vanishing[1], foreground)
		value = translateTransform(value, ir.Const{}, op(resource.RuntimeFunctionNegate, foreground))
		value = shearXTransform(value, op(resource.RuntimeFunctionNegate, div(vanishing[0], distance)))
		return simplePerspectiveYTransform(value, op(resource.RuntimeFunctionNegate, distance))
	}
	normalizeTransform := func(value []ir.Expr) []ir.Expr {
		l.guardWith(n, op(resource.RuntimeFunctionNotEqual, value[8], ir.Const{}), "cannot normalize transform with A22 == 0", false)
		result := make([]ir.Expr, 9)
		for index := range 8 {
			result[index] = div(value[index], value[8])
		}
		result[8] = ir.Const{Value: 1}
		return result
	}
	transformVec := func(value []ir.Expr, x, y ir.Expr) []ir.Expr {
		resultX := add(add(mul(value[0], x), mul(value[1], y)), value[2])
		resultY := add(add(mul(value[3], x), mul(value[4], y)), value[5])
		weight := add(add(mul(value[6], x), mul(value[7], y)), value[8])
		return []ir.Expr{div(resultX, weight), div(resultY, weight)}
	}
	rectToQuad := func(value []ir.Expr) []ir.Expr {
		return []ir.Expr{value[3], value[2], value[3], value[0], value[1], value[0], value[1], value[2]}
	}
	switch operation {
	case "easing.linstep", "easing.smoothstep", "easing.smootherstep", "easing.stepStart", "easing.stepEnd":
		if !require(0, 1) {
			break
		}
		value := args[0].slots[0]
		switch operation {
		case "easing.linstep":
			result.slots = []ir.Expr{op(resource.RuntimeFunctionClamp, value, ir.Const{}, ir.Const{Value: 1})}
		case "easing.smoothstep":
			value = op(resource.RuntimeFunctionClamp, value, ir.Const{}, ir.Const{Value: 1})
			result.slots = []ir.Expr{mul(mul(value, value), sub(ir.Const{Value: 3}, mul(ir.Const{Value: 2}, value)))}
		case "easing.smootherstep":
			value = op(resource.RuntimeFunctionClamp, value, ir.Const{}, ir.Const{Value: 1})
			polynomial := add(mul(value, sub(mul(value, ir.Const{Value: 6}), ir.Const{Value: 15})), ir.Const{Value: 10})
			result.slots = []ir.Expr{mul(mul(mul(value, value), value), polynomial)}
		case "easing.stepStart":
			result.slots = []ir.Expr{op(resource.RuntimeFunctionGreaterOr, value, ir.Const{})}
		case "easing.stepEnd":
			result.slots = []ir.Expr{op(resource.RuntimeFunctionGreaterOr, value, ir.Const{Value: 1})}
		}
	case "entity.key":
		if len(args) != 0 {
			l.errorAt(n, "Entity.Key does not accept arguments")
			break
		}
		if l.currentArchetype == nil {
			l.errorAt(n, "Entity.Key is only available in archetype callbacks")
			break
		}
		result.slots = []ir.Expr{ir.Const{Value: l.currentArchetype.Key}}
	case "mode.isPlay", "mode.isWatch", "mode.isPreview", "mode.isTutorial", "mode.isPreprocessing":
		if len(args) != 0 {
			l.errorAt(n, "%s does not accept arguments", operation)
			break
		}
		matches := false
		switch operation {
		case "mode.isPlay":
			matches = l.mode == mode.ModePlay
		case "mode.isWatch":
			matches = l.mode == mode.ModeWatch
		case "mode.isPreview":
			matches = l.mode == mode.ModePreview
		case "mode.isTutorial":
			matches = l.mode == mode.ModeTutorial
		case "mode.isPreprocessing":
			matches = l.phase == "preprocess"
		}
		if matches {
			result.slots = []ir.Expr{ir.Const{Value: 1}}
		} else {
			result.slots = []ir.Expr{ir.Const{}}
		}
	case "touch.get":
		if !require(0, 1) {
			break
		}
		result = l.touchValue(n, args[0].slots[0], result.type_, true)
	case "screen.rect":
		aspect := ir.Load{Place: l.memory("RuntimeEnvironment", ir.Const{}, 0, 1, true, false, n)}
		receiver := ""
		if selector, ok := n.Fun.(*ast.SelectorExpr); ok {
			receiver = facadeReceiverName(selector.X)
		}
		if receiver == "Screen" {
			result.slots = []ir.Expr{ir.Const{Value: 1}, aspect, ir.Const{Value: -1}, op(resource.RuntimeFunctionNegate, aspect)}
			break
		}
		base := 0
		switch l.mode {
		case mode.ModePlay, mode.ModeWatch:
			base = 5
		case mode.ModePreview:
			base = 2
		case mode.ModeTutorial:
			base = 3
		}
		xMin := ir.Load{Place: l.memory("RuntimeEnvironment", ir.Const{}, 0, base, true, false, n)}
		xMax := ir.Load{Place: l.memory("RuntimeEnvironment", ir.Const{}, 0, base+1, true, false, n)}
		yMin := ir.Load{Place: l.memory("RuntimeEnvironment", ir.Const{}, 0, base+2, true, false, n)}
		yMax := ir.Load{Place: l.memory("RuntimeEnvironment", ir.Const{}, 0, base+3, true, false, n)}
		result.slots = []ir.Expr{yMax, xMax, yMin, xMin}
	case "time.previous", "time.offsetAdjusted":
		now := ir.Load{Place: l.memory("RuntimeUpdate", ir.Const{}, 0, 0, true, false, n)}
		offset := 1
		storage := "RuntimeUpdate"
		if operation == "time.offsetAdjusted" {
			offset = 3
			storage = "RuntimeEnvironment"
		}
		other := ir.Load{Place: l.memory(storage, ir.Const{}, 0, offset, true, false, n)}
		result.slots = []ir.Expr{op(resource.RuntimeFunctionSubtract, now, other)}
	case "range.new":
		if require(0, 1) && require(1, 1) {
			result.slots = []ir.Expr{args[0].slots[0], args[1].slots[0]}
		}
	case "range.length":
		if require(0, 2) {
			result.slots = []ir.Expr{sub(args[0].slots[1], args[0].slots[0])}
		}
	case "range.isEmpty":
		if require(0, 2) {
			result.slots = []ir.Expr{op(resource.RuntimeFunctionGreater, args[0].slots[0], args[0].slots[1])}
		}
	case "range.mid":
		if require(0, 2) {
			result.slots = []ir.Expr{div(add(args[0].slots[0], args[0].slots[1]), ir.Const{Value: 2})}
		}
	case "range.contains":
		if require(0, 2) && require(1, 1) {
			result.slots = []ir.Expr{op(resource.RuntimeFunctionAnd,
				op(resource.RuntimeFunctionLessOr, args[0].slots[0], args[1].slots[0]),
				op(resource.RuntimeFunctionLessOr, args[1].slots[0], args[0].slots[1]))}
		}
	case "range.containsRange":
		if require(0, 2) && require(1, 2) {
			result.slots = []ir.Expr{op(resource.RuntimeFunctionAnd,
				op(resource.RuntimeFunctionLessOr, args[0].slots[0], args[1].slots[0]),
				op(resource.RuntimeFunctionLessOr, args[1].slots[1], args[0].slots[1]))}
		}
	case "range.add", "range.sub", "range.mul", "range.div":
		if require(0, 2) && require(1, 1) {
			operationByName := map[string]resource.RuntimeFunction{
				"range.add": resource.RuntimeFunctionAdd, "range.sub": resource.RuntimeFunctionSubtract,
				"range.mul": resource.RuntimeFunctionMultiply, "range.div": resource.RuntimeFunctionDivide,
			}
			function := operationByName[operation]
			result.slots = []ir.Expr{op(function, args[0].slots[0], args[1].slots[0]), op(function, args[0].slots[1], args[1].slots[0])}
		}
	case "range.intersect":
		if require(0, 2) && require(1, 2) {
			result.slots = []ir.Expr{
				op(resource.RuntimeFunctionMax, args[0].slots[0], args[1].slots[0]),
				op(resource.RuntimeFunctionMin, args[0].slots[1], args[1].slots[1]),
			}
		}
	case "range.shrink", "range.expand":
		if require(0, 2) && require(1, 1) {
			if operation == "range.shrink" {
				result.slots = []ir.Expr{add(args[0].slots[0], args[1].slots[0]), sub(args[0].slots[1], args[1].slots[0])}
			} else {
				result.slots = []ir.Expr{sub(args[0].slots[0], args[1].slots[0]), add(args[0].slots[1], args[1].slots[0])}
			}
		}
	case "range.lerp", "range.lerpClamped":
		if require(0, 2) && require(1, 1) {
			function := resource.RuntimeFunctionLerp
			if operation == "range.lerpClamped" {
				function = resource.RuntimeFunctionLerpClamped
			}
			result.slots = []ir.Expr{op(function, args[0].slots[0], args[0].slots[1], args[1].slots[0])}
		}
	case "range.unlerp", "range.unlerpClamped":
		if require(0, 2) && require(1, 1) {
			function := resource.RuntimeFunctionUnlerp
			if operation == "range.unlerpClamped" {
				function = resource.RuntimeFunctionUnlerpClamped
			}
			result.slots = []ir.Expr{op(function, args[0].slots[0], args[0].slots[1], args[1].slots[0])}
		}
	case "range.clamp":
		if require(0, 2) && require(1, 1) {
			result.slots = []ir.Expr{op(resource.RuntimeFunctionClamp, args[1].slots[0], args[0].slots[0], args[0].slots[1])}
		}
	case "vec2.new":
		if require(0, 1) && require(1, 1) {
			result.slots = []ir.Expr{args[0].slots[0], args[1].slots[0]}
		}
	case "vec2.unit":
		if require(0, 1) {
			result.slots = []ir.Expr{op(resource.RuntimeFunctionCos, args[0].slots[0]), op(resource.RuntimeFunctionSin, args[0].slots[0])}
		}
	case "vec2.add", "vec2.sub":
		if require(0, 2) && require(1, 2) {
			op := resource.RuntimeFunctionAdd
			if operation == "vec2.sub" {
				op = resource.RuntimeFunctionSubtract
			}
			result.slots = []ir.Expr{l.pure(op, n, args[0].slots[0], args[1].slots[0]), l.pure(op, n, args[0].slots[1], args[1].slots[1])}
		}
	case "vec2.mul", "vec2.div":
		if require(0, 2) && require(1, 1) {
			op := resource.RuntimeFunctionMultiply
			if operation == "vec2.div" {
				op = resource.RuntimeFunctionDivide
			}
			result.slots = []ir.Expr{l.pure(op, n, args[0].slots[0], args[1].slots[0]), l.pure(op, n, args[0].slots[1], args[1].slots[0])}
		}
	case "vec2.mulVec", "vec2.divVec":
		if require(0, 2) && require(1, 2) {
			function := resource.RuntimeFunctionMultiply
			if operation == "vec2.divVec" {
				function = resource.RuntimeFunctionDivide
			}
			result.slots = []ir.Expr{op(function, args[0].slots[0], args[1].slots[0]), op(function, args[0].slots[1], args[1].slots[1])}
		}
	case "vec2.negate":
		if require(0, 2) {
			result.slots = []ir.Expr{op(resource.RuntimeFunctionNegate, args[0].slots[0]), op(resource.RuntimeFunctionNegate, args[0].slots[1])}
		}
	case "vec2.lerp", "vec2.lerpClamped":
		if require(0, 2) && require(1, 2) && require(2, 1) {
			function := resource.RuntimeFunctionLerp
			if operation == "vec2.lerpClamped" {
				function = resource.RuntimeFunctionLerpClamped
			}
			result.slots = []ir.Expr{
				op(function, args[0].slots[0], args[1].slots[0], args[2].slots[0]),
				op(function, args[0].slots[1], args[1].slots[1], args[2].slots[0]),
			}
		}
	case "vec2.dot":
		if require(0, 2) && require(1, 2) {
			result.slots = []ir.Expr{l.pure(resource.RuntimeFunctionAdd, n, l.pure(resource.RuntimeFunctionMultiply, n, args[0].slots[0], args[1].slots[0]), l.pure(resource.RuntimeFunctionMultiply, n, args[0].slots[1], args[1].slots[1]))}
		}
	case "vec2.magnitudeSquared", "vec2.magnitude":
		if require(0, 2) {
			square := l.pure(resource.RuntimeFunctionAdd, n, l.pure(resource.RuntimeFunctionMultiply, n, args[0].slots[0], args[0].slots[0]), l.pure(resource.RuntimeFunctionMultiply, n, args[0].slots[1], args[0].slots[1]))
			if operation == "vec2.magnitude" {
				square = l.pure(resource.RuntimeFunctionPower, n, square, ir.Const{Value: 0.5})
			}
			result.slots = []ir.Expr{square}
		}
	case "vec2.angle":
		if require(0, 2) {
			result.slots = []ir.Expr{l.pure(resource.RuntimeFunctionArctan2, n, args[0].slots[1], args[0].slots[0])}
		}
	case "vec2.orthogonal":
		if require(0, 2) {
			result.slots = []ir.Expr{l.pure(resource.RuntimeFunctionNegate, n, args[0].slots[1]), args[0].slots[0]}
		}
	case "vec2.normalize", "vec2.normalizeOrZero":
		if require(0, 2) {
			x, y := args[0].slots[0], args[0].slots[1]
			magnitude := op(resource.RuntimeFunctionPower, add(mul(x, x), mul(y, y)), ir.Const{Value: 0.5})
			zero := op(resource.RuntimeFunctionAnd, op(resource.RuntimeFunctionEqual, x, ir.Const{}), op(resource.RuntimeFunctionEqual, y, ir.Const{}))
			if operation == "vec2.normalizeOrZero" {
				magnitude = add(magnitude, zero)
			} else {
				l.guard(n, op(resource.RuntimeFunctionNot, zero))
			}
			result.slots = []ir.Expr{div(x, magnitude), div(y, magnitude)}
		}
	case "vec2.rotate":
		if require(0, 2) && require(1, 1) {
			result.slots = rotate(args[0].slots[0], args[0].slots[1], args[1].slots[0])
		}
	case "vec2.rotateAbout":
		if require(0, 2) && require(1, 2) && require(2, 1) {
			rotated := rotate(sub(args[0].slots[0], args[1].slots[0]), sub(args[0].slots[1], args[1].slots[1]), args[2].slots[0])
			result.slots = []ir.Expr{add(rotated[0], args[1].slots[0]), add(rotated[1], args[1].slots[1])}
		}
	case "vec2.angleDiff", "vec2.signedAngleDiff":
		if require(0, 2) && require(1, 2) {
			a := op(resource.RuntimeFunctionArctan2, args[0].slots[1], args[0].slots[0])
			b := op(resource.RuntimeFunctionArctan2, args[1].slots[1], args[1].slots[0])
			diff := sub(op(resource.RuntimeFunctionMod, add(sub(a, b), ir.Const{Value: 3.141592653589793}), ir.Const{Value: 6.283185307179586}), ir.Const{Value: 3.141592653589793})
			if operation == "vec2.angleDiff" {
				diff = op(resource.RuntimeFunctionAbs, diff)
			}
			result.slots = []ir.Expr{diff}
		}
	case "rect.fromCenter":
		if require(0, 2) && require(1, 2) {
			halfWidth, halfHeight := div(args[1].slots[0], ir.Const{Value: 2}), div(args[1].slots[1], ir.Const{Value: 2})
			result.slots = []ir.Expr{add(args[0].slots[1], halfHeight), add(args[0].slots[0], halfWidth), sub(args[0].slots[1], halfHeight), sub(args[0].slots[0], halfWidth)}
		}
	case "rect.fromMargin":
		if require(0, 1) && require(1, 1) && require(2, 1) && require(3, 1) {
			result.slots = []ir.Expr{args[0].slots[0], args[1].slots[0], op(resource.RuntimeFunctionNegate, args[2].slots[0]), op(resource.RuntimeFunctionNegate, args[3].slots[0])}
		}
	case "rect.width":
		if require(0, 4) {
			result.slots = []ir.Expr{l.pure(resource.RuntimeFunctionSubtract, n, args[0].slots[1], args[0].slots[3])}
		}
	case "rect.height":
		if require(0, 4) {
			result.slots = []ir.Expr{l.pure(resource.RuntimeFunctionSubtract, n, args[0].slots[0], args[0].slots[2])}
		}
	case "rect.center":
		if require(0, 4) {
			result.slots = []ir.Expr{l.pure(resource.RuntimeFunctionDivide, n, l.pure(resource.RuntimeFunctionAdd, n, args[0].slots[3], args[0].slots[1]), ir.Const{Value: 2}), l.pure(resource.RuntimeFunctionDivide, n, l.pure(resource.RuntimeFunctionAdd, n, args[0].slots[2], args[0].slots[0]), ir.Const{Value: 2})}
		}
	case "rect.bl", "rect.tl", "rect.tr", "rect.br":
		if require(0, 4) {
			switch operation {
			case "rect.bl":
				result.slots = []ir.Expr{args[0].slots[3], args[0].slots[2]}
			case "rect.tl":
				result.slots = []ir.Expr{args[0].slots[3], args[0].slots[0]}
			case "rect.tr":
				result.slots = []ir.Expr{args[0].slots[1], args[0].slots[0]}
			case "rect.br":
				result.slots = []ir.Expr{args[0].slots[1], args[0].slots[2]}
			}
		}
	case "rect.top", "rect.right", "rect.bottom", "rect.left":
		if require(0, 4) {
			centerX := div(add(args[0].slots[3], args[0].slots[1]), ir.Const{Value: 2})
			centerY := div(add(args[0].slots[2], args[0].slots[0]), ir.Const{Value: 2})
			switch operation {
			case "rect.top":
				result.slots = []ir.Expr{centerX, args[0].slots[0]}
			case "rect.right":
				result.slots = []ir.Expr{args[0].slots[1], centerY}
			case "rect.bottom":
				result.slots = []ir.Expr{centerX, args[0].slots[2]}
			case "rect.left":
				result.slots = []ir.Expr{args[0].slots[3], centerY}
			}
		}
	case "rect.translate":
		if require(0, 4) && require(1, 2) {
			result.slots = []ir.Expr{l.pure(resource.RuntimeFunctionAdd, n, args[0].slots[0], args[1].slots[1]), l.pure(resource.RuntimeFunctionAdd, n, args[0].slots[1], args[1].slots[0]), l.pure(resource.RuntimeFunctionAdd, n, args[0].slots[2], args[1].slots[1]), l.pure(resource.RuntimeFunctionAdd, n, args[0].slots[3], args[1].slots[0])}
		}
	case "rect.scale":
		if require(0, 4) && require(1, 1) {
			for _, slot := range args[0].slots {
				result.slots = append(result.slots, l.pure(resource.RuntimeFunctionMultiply, n, slot, args[1].slots[0]))
			}
		}
	case "rect.scaleVec":
		if require(0, 4) && require(1, 2) {
			result.slots = []ir.Expr{mul(args[0].slots[0], args[1].slots[1]), mul(args[0].slots[1], args[1].slots[0]), mul(args[0].slots[2], args[1].slots[1]), mul(args[0].slots[3], args[1].slots[0])}
		}
	case "rect.scaleAbout", "rect.scaleCentered":
		if require(0, 4) && require(1, 2) && (operation == "rect.scaleCentered" || require(2, 2)) {
			var pivot []ir.Expr
			if operation == "rect.scaleCentered" {
				pivot = []ir.Expr{div(add(args[0].slots[3], args[0].slots[1]), ir.Const{Value: 2}), div(add(args[0].slots[2], args[0].slots[0]), ir.Const{Value: 2})}
			} else {
				pivot = args[2].slots
			}
			sx, sy := args[1].slots[0], args[1].slots[1]
			result.slots = []ir.Expr{
				add(mul(sub(args[0].slots[0], pivot[1]), sy), pivot[1]),
				add(mul(sub(args[0].slots[1], pivot[0]), sx), pivot[0]),
				add(mul(sub(args[0].slots[2], pivot[1]), sy), pivot[1]),
				add(mul(sub(args[0].slots[3], pivot[0]), sx), pivot[0]),
			}
		}
	case "rect.expand", "rect.shrink":
		if require(0, 4) && require(1, 2) {
			x, y := args[1].slots[0], args[1].slots[1]
			if operation == "rect.shrink" {
				x, y = op(resource.RuntimeFunctionNegate, x), op(resource.RuntimeFunctionNegate, y)
			}
			result.slots = []ir.Expr{add(args[0].slots[0], y), add(args[0].slots[1], x), sub(args[0].slots[2], y), sub(args[0].slots[3], x)}
		}
	case "rect.toQuad":
		if require(0, 4) {
			t, r, b, left := args[0].slots[0], args[0].slots[1], args[0].slots[2], args[0].slots[3]
			result.slots = []ir.Expr{left, b, left, t, r, t, r, b}
		}
	case "rect.contains":
		if require(0, 4) && require(1, 2) {
			x, y := args[1].slots[0], args[1].slots[1]
			result.slots = []ir.Expr{op(resource.RuntimeFunctionAnd,
				op(resource.RuntimeFunctionAnd, op(resource.RuntimeFunctionGreaterOr, x, args[0].slots[3]), op(resource.RuntimeFunctionLessOr, x, args[0].slots[1])),
				op(resource.RuntimeFunctionAnd, op(resource.RuntimeFunctionGreaterOr, y, args[0].slots[2]), op(resource.RuntimeFunctionLessOr, y, args[0].slots[0])))}
		}
	case "quad.center":
		if require(0, 8) {
			result.slots = []ir.Expr{div(add(add(args[0].slots[0], args[0].slots[2]), add(args[0].slots[4], args[0].slots[6])), ir.Const{Value: 4}), div(add(add(args[0].slots[1], args[0].slots[3]), add(args[0].slots[5], args[0].slots[7])), ir.Const{Value: 4})}
		}
	case "quad.translate":
		if require(0, 8) && require(1, 2) {
			for i, value := range args[0].slots {
				result.slots = append(result.slots, add(value, args[1].slots[i%2]))
			}
		}
	case "quad.scale":
		if require(0, 8) && require(1, 1) {
			for _, value := range args[0].slots {
				result.slots = append(result.slots, mul(value, args[1].slots[0]))
			}
		}
	case "quad.scaleVec":
		if require(0, 8) && require(1, 2) {
			for index, value := range args[0].slots {
				result.slots = append(result.slots, mul(value, args[1].slots[index%2]))
			}
		}
	case "quad.scaleAbout", "quad.scaleCentered":
		if require(0, 8) && require(1, 2) && (operation == "quad.scaleCentered" || require(2, 2)) {
			var pivot []ir.Expr
			if operation == "quad.scaleCentered" {
				pivot = []ir.Expr{div(add(add(args[0].slots[0], args[0].slots[2]), add(args[0].slots[4], args[0].slots[6])), ir.Const{Value: 4}), div(add(add(args[0].slots[1], args[0].slots[3]), add(args[0].slots[5], args[0].slots[7])), ir.Const{Value: 4})}
			} else {
				pivot = args[2].slots
			}
			for index, value := range args[0].slots {
				axis := index % 2
				result.slots = append(result.slots, add(mul(sub(value, pivot[axis]), args[1].slots[axis]), pivot[axis]))
			}
		}
	case "quad.rotate":
		if require(0, 8) && require(1, 1) {
			for i := 0; i < 8; i += 2 {
				result.slots = append(result.slots, rotate(args[0].slots[i], args[0].slots[i+1], args[1].slots[0])...)
			}
		}
	case "quad.rotateAbout", "quad.rotateCentered":
		if require(0, 8) && require(1, 1) && (operation == "quad.rotateCentered" || require(2, 2)) {
			var pivot []ir.Expr
			if operation == "quad.rotateCentered" {
				pivot = []ir.Expr{div(add(add(args[0].slots[0], args[0].slots[2]), add(args[0].slots[4], args[0].slots[6])), ir.Const{Value: 4}), div(add(add(args[0].slots[1], args[0].slots[3]), add(args[0].slots[5], args[0].slots[7])), ir.Const{Value: 4})}
			} else {
				pivot = args[2].slots
			}
			for index := 0; index < 8; index += 2 {
				rotated := rotate(sub(args[0].slots[index], pivot[0]), sub(args[0].slots[index+1], pivot[1]), args[1].slots[0])
				result.slots = append(result.slots, add(rotated[0], pivot[0]), add(rotated[1], pivot[1]))
			}
		}
	case "quad.permute":
		if require(0, 8) && require(1, 1) {
			value := l.alloc("quad.permute", result.type_)
			exit := l.newBlock()
			blocks := []*ir.Block{l.newBlock(), l.newBlock(), l.newBlock(), l.newBlock()}
			cases := make([]ir.SwitchCase, 4)
			for i := range blocks {
				cases[i] = ir.SwitchCase{Value: float64(i), Target: blocks[i].ID}
			}
			rotation := op(resource.RuntimeFunctionMod, args[1].slots[0], ir.Const{Value: 4})
			_ = l.builder.Switch(rotation, cases, blocks[0])
			orders := [4][4]int{{0, 1, 2, 3}, {3, 0, 1, 2}, {2, 3, 0, 1}, {1, 2, 3, 0}}
			for i, block := range blocks {
				l.setCurrent(block)
				var slots []ir.Expr
				for _, corner := range orders[i] {
					slots = append(slots, args[0].slots[corner*2:corner*2+2]...)
				}
				l.store(value, lowerValue{type_: result.type_, slots: slots}, n)
				l.jump(exit)
			}
			l.setCurrent(exit)
			result = value
		}
	case "quad.contains":
		if require(0, 8) && require(1, 2) {
			px, py := args[1].slots[0], args[1].slots[1]
			var inside ir.Expr = ir.Const{}
			for i := 0; i < 4; i++ {
				j := (i + 3) % 4
				xi, yi := args[0].slots[i*2], args[0].slots[i*2+1]
				xj, yj := args[0].slots[j*2], args[0].slots[j*2+1]
				straddles := op(resource.RuntimeFunctionNotEqual, op(resource.RuntimeFunctionGreater, yi, py), op(resource.RuntimeFunctionGreater, yj, py))
				intersection := add(div(mul(sub(xj, xi), sub(py, yi)), sub(yj, yi)), xi)
				crosses := op(resource.RuntimeFunctionAnd, straddles, op(resource.RuntimeFunctionLess, px, intersection))
				inside = op(resource.RuntimeFunctionNotEqual, inside, crosses)
			}
			result.slots = []ir.Expr{inside}
		}
	case "quad.top":
		if require(0, 8) {
			result.slots = []ir.Expr{div(add(args[0].slots[2], args[0].slots[4]), ir.Const{Value: 2}), div(add(args[0].slots[3], args[0].slots[5]), ir.Const{Value: 2})}
		}
	case "quad.right":
		if require(0, 8) {
			result.slots = []ir.Expr{div(add(args[0].slots[4], args[0].slots[6]), ir.Const{Value: 2}), div(add(args[0].slots[5], args[0].slots[7]), ir.Const{Value: 2})}
		}
	case "quad.bottom":
		if require(0, 8) {
			result.slots = []ir.Expr{div(add(args[0].slots[0], args[0].slots[6]), ir.Const{Value: 2}), div(add(args[0].slots[1], args[0].slots[7]), ir.Const{Value: 2})}
		}
	case "quad.left":
		if require(0, 8) {
			result.slots = []ir.Expr{div(add(args[0].slots[0], args[0].slots[2]), ir.Const{Value: 2}), div(add(args[0].slots[1], args[0].slots[3]), ir.Const{Value: 2})}
		}
	case "transform.identity":
		if len(args) == 0 {
			result.slots = identityTransform()
		}
	case "transform.translate":
		if require(0, 9) && require(1, 2) {
			result.slots = translateTransform(args[0].slots, args[1].slots[0], args[1].slots[1])
		}
	case "transform.scale":
		if require(0, 9) && require(1, 2) {
			result.slots = scaleTransform(args[0].slots, args[1].slots[0], args[1].slots[1])
		}
	case "transform.rotate":
		if require(0, 9) && require(1, 1) {
			result.slots = rotateTransform(args[0].slots, args[1].slots[0])
		}
	case "transform.compose", "transform.composeBefore":
		if require(0, 9) && require(1, 9) {
			if operation == "transform.compose" {
				result.slots = compose(args[0].slots, args[1].slots)
			} else {
				result.slots = compose(args[1].slots, args[0].slots)
			}
		}
	case "transform.scaleAbout":
		if require(0, 9) && require(1, 2) && require(2, 2) {
			result.slots = translateTransform(args[0].slots, op(resource.RuntimeFunctionNegate, args[2].slots[0]), op(resource.RuntimeFunctionNegate, args[2].slots[1]))
			result.slots = scaleTransform(result.slots, args[1].slots[0], args[1].slots[1])
			result.slots = translateTransform(result.slots, args[2].slots[0], args[2].slots[1])
		}
	case "transform.rotateAbout":
		if require(0, 9) && require(1, 1) && require(2, 2) {
			result.slots = translateTransform(args[0].slots, op(resource.RuntimeFunctionNegate, args[2].slots[0]), op(resource.RuntimeFunctionNegate, args[2].slots[1]))
			result.slots = rotateTransform(result.slots, args[1].slots[0])
			result.slots = translateTransform(result.slots, args[2].slots[0], args[2].slots[1])
		}
	case "transform.shearX", "transform.shearY":
		if require(0, 9) && require(1, 1) {
			if operation == "transform.shearX" {
				result.slots = shearXTransform(args[0].slots, args[1].slots[0])
			} else {
				result.slots = shearYTransform(args[0].slots, args[1].slots[0])
			}
		}
	case "transform.simplePerspectiveX", "transform.simplePerspectiveY":
		if require(0, 9) && require(1, 1) {
			if operation == "transform.simplePerspectiveX" {
				result.slots = simplePerspectiveXTransform(args[0].slots, args[1].slots[0])
			} else {
				result.slots = simplePerspectiveYTransform(args[0].slots, args[1].slots[0])
			}
		}
	case "transform.perspectiveX", "transform.perspectiveY", "transform.inversePerspectiveX", "transform.inversePerspectiveY":
		if require(0, 9) && require(1, 1) && require(2, 2) {
			switch operation {
			case "transform.perspectiveX":
				result.slots = perspectiveXTransform(args[0].slots, args[1].slots[0], args[2].slots)
			case "transform.perspectiveY":
				result.slots = perspectiveYTransform(args[0].slots, args[1].slots[0], args[2].slots)
			case "transform.inversePerspectiveX":
				result.slots = inversePerspectiveXTransform(args[0].slots, args[1].slots[0], args[2].slots)
			case "transform.inversePerspectiveY":
				result.slots = inversePerspectiveYTransform(args[0].slots, args[1].slots[0], args[2].slots)
			}
		}
	case "transform.normalize":
		if require(0, 9) {
			result.slots = normalizeTransform(args[0].slots)
		}
	case "transform.vec":
		if require(0, 9) && require(1, 2) {
			result.slots = transformVec(args[0].slots, args[1].slots[0], args[1].slots[1])
		}
	case "transform.quad":
		if require(0, 9) && require(1, 8) {
			for i := 0; i < 8; i += 2 {
				result.slots = append(result.slots, transformVec(args[0].slots, args[1].slots[i], args[1].slots[i+1])...)
			}
		}
	case "transform.rect":
		if require(0, 9) && require(1, 4) {
			quad := rectToQuad(args[1].slots)
			for index := 0; index < 8; index += 2 {
				result.slots = append(result.slots, transformVec(args[0].slots, quad[index], quad[index+1])...)
			}
		}
	case "invertibleTransform.identity":
		if len(args) == 0 {
			result.slots = append(identityTransform(), identityTransform()...)
		}
	case "invertibleTransform.translate", "invertibleTransform.scale", "invertibleTransform.scaleAbout", "invertibleTransform.rotate", "invertibleTransform.rotateAbout", "invertibleTransform.shearX", "invertibleTransform.shearY", "invertibleTransform.simplePerspectiveX", "invertibleTransform.simplePerspectiveY", "invertibleTransform.perspectiveX", "invertibleTransform.perspectiveY":
		if !require(0, 18) {
			break
		}
		forward, inverse := args[0].slots[:9], args[0].slots[9:]
		switch operation {
		case "invertibleTransform.translate":
			if require(1, 2) {
				forward = translateTransform(forward, args[1].slots[0], args[1].slots[1])
				inverse = compose(translateTransform(identityTransform(), op(resource.RuntimeFunctionNegate, args[1].slots[0]), op(resource.RuntimeFunctionNegate, args[1].slots[1])), inverse)
			}
		case "invertibleTransform.scale":
			if require(1, 2) {
				forward = scaleTransform(forward, args[1].slots[0], args[1].slots[1])
				inverse = compose(scaleTransform(identityTransform(), div(ir.Const{Value: 1}, args[1].slots[0]), div(ir.Const{Value: 1}, args[1].slots[1])), inverse)
			}
		case "invertibleTransform.scaleAbout":
			if require(1, 2) && require(2, 2) {
				forward = translateTransform(forward, op(resource.RuntimeFunctionNegate, args[2].slots[0]), op(resource.RuntimeFunctionNegate, args[2].slots[1]))
				forward = scaleTransform(forward, args[1].slots[0], args[1].slots[1])
				forward = translateTransform(forward, args[2].slots[0], args[2].slots[1])
				inverseScale := translateTransform(identityTransform(), op(resource.RuntimeFunctionNegate, args[2].slots[0]), op(resource.RuntimeFunctionNegate, args[2].slots[1]))
				inverseScale = scaleTransform(inverseScale, div(ir.Const{Value: 1}, args[1].slots[0]), div(ir.Const{Value: 1}, args[1].slots[1]))
				inverseScale = translateTransform(inverseScale, args[2].slots[0], args[2].slots[1])
				inverse = compose(inverseScale, inverse)
			}
		case "invertibleTransform.rotate":
			if require(1, 1) {
				forward = rotateTransform(forward, args[1].slots[0])
				inverse = compose(rotateTransform(identityTransform(), op(resource.RuntimeFunctionNegate, args[1].slots[0])), inverse)
			}
		case "invertibleTransform.rotateAbout":
			if require(1, 1) && require(2, 2) {
				forward = translateTransform(forward, op(resource.RuntimeFunctionNegate, args[2].slots[0]), op(resource.RuntimeFunctionNegate, args[2].slots[1]))
				forward = rotateTransform(forward, args[1].slots[0])
				forward = translateTransform(forward, args[2].slots[0], args[2].slots[1])
				inverseRotate := translateTransform(identityTransform(), op(resource.RuntimeFunctionNegate, args[2].slots[0]), op(resource.RuntimeFunctionNegate, args[2].slots[1]))
				inverseRotate = rotateTransform(inverseRotate, op(resource.RuntimeFunctionNegate, args[1].slots[0]))
				inverseRotate = translateTransform(inverseRotate, args[2].slots[0], args[2].slots[1])
				inverse = compose(inverseRotate, inverse)
			}
		case "invertibleTransform.shearX", "invertibleTransform.shearY":
			if require(1, 1) {
				if operation == "invertibleTransform.shearX" {
					forward = shearXTransform(forward, args[1].slots[0])
					inverse = compose(shearXTransform(identityTransform(), op(resource.RuntimeFunctionNegate, args[1].slots[0])), inverse)
				} else {
					forward = shearYTransform(forward, args[1].slots[0])
					inverse = compose(shearYTransform(identityTransform(), op(resource.RuntimeFunctionNegate, args[1].slots[0])), inverse)
				}
			}
		case "invertibleTransform.simplePerspectiveX", "invertibleTransform.simplePerspectiveY":
			if require(1, 1) {
				if operation == "invertibleTransform.simplePerspectiveX" {
					forward = simplePerspectiveXTransform(forward, args[1].slots[0])
					inverse = compose(simplePerspectiveXTransform(identityTransform(), op(resource.RuntimeFunctionNegate, args[1].slots[0])), inverse)
				} else {
					forward = simplePerspectiveYTransform(forward, args[1].slots[0])
					inverse = compose(simplePerspectiveYTransform(identityTransform(), op(resource.RuntimeFunctionNegate, args[1].slots[0])), inverse)
				}
			}
		case "invertibleTransform.perspectiveX", "invertibleTransform.perspectiveY":
			if require(1, 1) && require(2, 2) {
				if operation == "invertibleTransform.perspectiveX" {
					forward = perspectiveXTransform(forward, args[1].slots[0], args[2].slots)
					inverse = compose(inversePerspectiveXTransform(identityTransform(), args[1].slots[0], args[2].slots), inverse)
				} else {
					forward = perspectiveYTransform(forward, args[1].slots[0], args[2].slots)
					inverse = compose(inversePerspectiveYTransform(identityTransform(), args[1].slots[0], args[2].slots), inverse)
				}
			}
		}
		result.slots = append(append([]ir.Expr(nil), forward...), inverse...)
	case "invertibleTransform.normalize":
		if require(0, 18) {
			result.slots = append(normalizeTransform(args[0].slots[:9]), normalizeTransform(args[0].slots[9:])...)
		}
	case "invertibleTransform.compose", "invertibleTransform.composeBefore":
		if require(0, 18) && require(1, 18) {
			left, right := args[0].slots, args[1].slots
			if operation == "invertibleTransform.composeBefore" {
				left, right = right, left
			}
			result.slots = append(compose(left[:9], right[:9]), compose(right[9:], left[9:])...)
		}
	case "invertibleTransform.vec", "invertibleTransform.inverseVec":
		if require(0, 18) && require(1, 2) {
			matrix := args[0].slots[:9]
			if operation == "invertibleTransform.inverseVec" {
				matrix = args[0].slots[9:]
			}
			result.slots = transformVec(matrix, args[1].slots[0], args[1].slots[1])
		}
	case "invertibleTransform.quad", "invertibleTransform.inverseQuad":
		if require(0, 18) && require(1, 8) {
			matrix := args[0].slots[:9]
			if operation == "invertibleTransform.inverseQuad" {
				matrix = args[0].slots[9:]
			}
			for index := 0; index < 8; index += 2 {
				result.slots = append(result.slots, transformVec(matrix, args[1].slots[index], args[1].slots[index+1])...)
			}
		}
	case "invertibleTransform.rect", "invertibleTransform.inverseRect":
		if require(0, 18) && require(1, 4) {
			matrix := args[0].slots[:9]
			if operation == "invertibleTransform.inverseRect" {
				matrix = args[0].slots[9:]
			}
			quad := rectToQuad(args[1].slots)
			for index := 0; index < 8; index += 2 {
				result.slots = append(result.slots, transformVec(matrix, quad[index], quad[index+1])...)
			}
		}
	case "transform.perspectiveApproach":
		if require(0, 1) && require(1, 1) {
			distance := op(resource.RuntimeFunctionMax, op(resource.RuntimeFunctionLerp, args[0].slots[0], ir.Const{Value: 1}, args[1].slots[0]), ir.Const{Value: 1e-6})
			result.slots = []ir.Expr{op(resource.RuntimeFunctionRemap, div(ir.Const{Value: 1}, args[0].slots[0]), ir.Const{Value: 1}, ir.Const{}, ir.Const{Value: 1}, div(ir.Const{Value: 1}, distance))}
		}
	default:
		l.errorAt(n, "unknown aggregate recipe %s", operation)
	}
	if len(result.slots) != irTypeOf(result.type_).Slots {
		l.errorAt(n, "aggregate recipe %s returned %d slots; expected %d", operation, len(result.slots), irTypeOf(result.type_).Slots)
	}
	return result
}

func (l *lowerer) inlineStaticCallable(call *ast.CallExpr, callable *staticCallable, args []callArgument) lowerValue {
	if len(callable.substitutions) != 0 {
		l.typeSubstitutions = append(l.typeSubstitutions, callable.substitutions)
		defer func() { l.typeSubstitutions = l.typeSubstitutions[:len(l.typeSubstitutions)-1] }()
	}
	if callable.intrinsic != nil {
		return callable.intrinsic(call, args)
	}
	if callable.tag.type_ != nil {
		return l.inlineCallableAlternatives(call, callable, args)
	}
	if callable.yield != nil {
		return l.inlineRangeYield(call, callable.yield, args)
	}
	if callable.iterator != nil {
		return l.inlineContainerIterator(call, callable.iterator, args)
	}
	if callable.streamIter != nil {
		return l.inlineStreamIterator(call, callable.streamIter, args)
	}
	if callable.touchIter != nil {
		return l.inlineTouchIterator(call, callable.touchIter, args)
	}
	if callable.interfaceMethod != nil {
		return l.inlineInterfaceMethodExpression(call, callable.interfaceMethod, args)
	}
	if callable.literal != nil {
		return l.inlineLiteralArguments(call, callable, args)
	}
	if callable.function != nil {
		if callable.receiver != nil {
			args = append([]callArgument{*callable.receiver}, args...)
		}
		return l.inlineCallArgumentsAs(call, callable.function, args, callable.resultType)
	}
	l.errorAt(call, "static callable has no function body")
	return zeroValue(l.pkg.TypesInfo.TypeOf(call))
}

func (l *lowerer) inlineInterfaceMethodExpression(call *ast.CallExpr, method *types.Func, args []callArgument) lowerValue {
	if len(args) == 0 || args[0].callable != nil {
		l.errorAt(call, "interface method expression %s requires an explicit receiver", method.FullName())
		return zeroValue(l.pkg.TypesInfo.TypeOf(call))
	}
	receiver := args[0].value
	remaining := args[1:]
	if receiver.interface_ != nil {
		return l.inlineInterfaceMethodAlternatives(call, method, receiver, remaining)
	}
	if receiver.type_ == nil || isInterfaceType(receiver.type_) {
		l.errorAt(call, "interface method expression %s requires a finite concrete receiver", method.FullName())
		return zeroValue(l.pkg.TypesInfo.TypeOf(call))
	}
	object, _, _ := types.LookupFieldOrMethod(receiver.type_, true, method.Pkg(), method.Name())
	concrete, _ := object.(*types.Func)
	if concrete == nil {
		l.errorAt(call, "concrete type %s does not implement interface method %s", receiver.type_, method.Name())
		return zeroValue(l.pkg.TypesInfo.TypeOf(call))
	}
	arguments := append([]callArgument{{value: receiver}}, remaining...)
	return l.inlineCallArguments(call, concrete, arguments)
}

func (l *lowerer) extendContainerCall(n *ast.CallExpr, fn *types.Func) lowerValue {
	if len(n.Args) != 1 {
		l.errorAt(n, "VarArray.Extend requires one sequence")
		return lowerValue{}
	}
	selector, ok := n.Fun.(*ast.SelectorExpr)
	if !ok {
		l.errorAt(n, "VarArray.Extend requires a method receiver")
		return lowerValue{}
	}
	receiver := l.callReceiver(selector, fn)
	sequence, ok := l.staticCallable(n.Args[0])
	if !ok {
		l.errorAt(n.Args[0], "VarArray.Extend sequence must be statically callable")
		return lowerValue{}
	}
	return l.dispatchContainerValue(n, receiver, func(alternative lowerValue) lowerValue {
		return l.extendContainerValue(n, alternative, sequence)
	})
}

func (l *lowerer) extendContainerValue(n *ast.CallExpr, receiver lowerValue, sequence *staticCallable) lowerValue {
	if receiver.container == nil || receiver.container.kind != "VarArray" {
		l.errorAt(n, "VarArray.Extend receiver has no container backing")
		return lowerValue{}
	}
	c := receiver.container
	bufferSize := l.allocZeroed("container.extend.size", types.Typ[types.Int], n)
	backing := l.builder.NewLocal("container.extend.data", ir.Type{Name: "container.extend.backing", Slots: c.capacity * c.stride})
	first, _ := ir.Places(backing)[0].(ir.LocalPlace)
	buffer := lowerValue{
		type_:     receiver.type_,
		slots:     append(append([]ir.Expr(nil), bufferSize.slots...), backing.Slots...),
		places:    append(append([]ir.Place(nil), bufferSize.places...), ir.Places(backing)...),
		container: &containerValue{kind: "VarArray", capacity: c.capacity, stride: c.stride, element: c.element, dataLocal: &first},
	}
	yield := &staticCallable{intrinsic: func(call *ast.CallExpr, arguments []callArgument) lowerValue {
		if len(arguments) != 1 || arguments[0].callable != nil {
			l.errorAt(call, "VarArray.Extend sequence must yield one value")
			return scalarValue(ir.Const{}, types.Typ[types.Bool])
		}
		condition := l.pure(resource.RuntimeFunctionLess, call, buffer.slots[0], ir.Const{Value: float64(c.capacity)})
		l.guardWith(call, condition, "VarArray.Extend sequence exceeds receiver capacity", true)
		l.store(l.containerElement(call, buffer.container, buffer.slots[0], 0, c.stride, c.element), arguments[0].value, call)
		l.store(l.containerSizeValue(buffer), scalarValue(l.pure(resource.RuntimeFunctionAdd, call, buffer.slots[0], ir.Const{Value: 1}), types.Typ[types.Int]), call)
		return scalarValue(ir.Const{Value: 1}, types.Typ[types.Bool])
	}}
	l.inlineStaticCallable(n, sequence, []callArgument{{callable: yield}})
	originalSize := l.materialize("container.extend.originalSize", l.containerSizeValue(receiver), n)
	total := l.pure(resource.RuntimeFunctionAdd, n, originalSize.slots[0], buffer.slots[0])
	l.guardWith(n, l.pure(resource.RuntimeFunctionLessOr, n, total, ir.Const{Value: float64(c.capacity)}), "VarArray.Extend exceeds receiver capacity", true)
	index := l.allocZeroed("container.extend.index", types.Typ[types.Int], n)
	header, body, exit := l.newBlock(), l.newBlock(), l.newBlock()
	l.jump(header)
	l.setCurrent(header)
	_ = l.builder.Branch(l.pure(resource.RuntimeFunctionLess, n, index.slots[0], buffer.slots[0]), body, exit)
	l.setCurrent(body)
	destination := l.pure(resource.RuntimeFunctionAdd, n, originalSize.slots[0], index.slots[0])
	value := l.containerElement(n, buffer.container, index.slots[0], 0, c.stride, c.element)
	l.store(l.containerElement(n, c, destination, 0, c.stride, c.element), value, n)
	l.store(index, scalarValue(l.pure(resource.RuntimeFunctionAdd, n, index.slots[0], ir.Const{Value: 1}), types.Typ[types.Int]), n)
	l.jump(header)
	l.setCurrent(exit)
	l.store(l.containerSizeValue(receiver), scalarValue(total, types.Typ[types.Int]), n)
	return lowerValue{}
}

func (l *lowerer) streamHas(node ast.Node, receiver lowerValue, key ir.Expr) ir.Expr {
	return l.builder.RuntimeCall(resource.RuntimeFunctionStreamHas, []ir.Expr{receiver.slots[0], key}, irTypeOf(types.Typ[types.Bool]), false, sourcePos(l.pkg, node.Pos()))
}

func (l *lowerer) streamKey(node ast.Node, receiver lowerValue, key ir.Expr, next bool) ir.Expr {
	function := resource.RuntimeFunctionStreamGetPreviousKey
	if next {
		function = resource.RuntimeFunctionStreamGetNextKey
	}
	return l.builder.RuntimeCall(function, []ir.Expr{receiver.slots[0], key}, irTypeOf(types.Typ[types.Float64]), false, sourcePos(l.pkg, node.Pos()))
}

func (l *lowerer) streamValueAt(node ast.Node, receiver lowerValue, key ir.Expr, resultType types.Type) lowerValue {
	result := lowerValue{type_: resultType, slots: make([]ir.Expr, len(receiver.slots))}
	for index, id := range receiver.slots {
		result.slots[index] = l.builder.RuntimeCall(resource.RuntimeFunctionStreamGetValue, []ir.Expr{id, key}, ir.Type{Name: "stream.value", Slots: 1}, false, sourcePos(l.pkg, node.Pos()))
	}
	return result
}

func (l *lowerer) touchValue(node ast.Node, index ir.Expr, touchType types.Type, checked bool) lowerValue {
	if checked {
		count := ir.Load{Place: l.memory("RuntimeUpdate", ir.Const{}, 0, 3, true, false, node)}
		valid := l.pure(resource.RuntimeFunctionAnd, node,
			l.pure(resource.RuntimeFunctionGreaterOr, node, index, ir.Const{}),
			l.pure(resource.RuntimeFunctionLess, node, index, count))
		l.guard(node, valid)
	}
	slots := l.runtimeTypeOf(touchType).Slots
	result := lowerValue{type_: touchType, slots: make([]ir.Expr, slots), places: make([]ir.Place, slots)}
	for slot := range slots {
		place := l.memory("RuntimeTouch", index, slots, slot, true, false, node)
		result.places[slot], result.slots[slot] = place, ir.Load{Place: place}
	}
	return result
}

func (l *lowerer) inlineTouchIterator(call *ast.CallExpr, iterator *touchIterator, args []callArgument) lowerValue {
	if len(args) != 1 || args[0].callable == nil {
		l.errorAt(call, "touch iterator requires one yield callable")
		return lowerValue{}
	}
	count := l.materialize("touch.iterator.count", scalarValue(ir.Load{Place: l.memory("RuntimeUpdate", ir.Const{}, 0, 3, true, false, call)}, types.Typ[types.Int]), call)
	index := l.allocZeroed("touch.iterator.index", types.Typ[types.Int], call)
	header, body, advance, exit := l.newBlock(), l.newBlock(), l.newBlock(), l.newBlock()
	l.jump(header)
	l.setCurrent(header)
	_ = l.builder.Branch(l.pure(resource.RuntimeFunctionLess, call, index.slots[0], count.slots[0]), body, exit)
	l.setCurrent(body)
	yielded := []callArgument{{value: l.touchValue(call, index.slots[0], iterator.touchType, false)}}
	if iterator.index {
		yielded = append([]callArgument{{value: scalarValue(index.slots[0], types.Typ[types.Int])}}, yielded...)
	}
	keepGoing := l.inlineStaticCallable(call, args[0].callable, yielded)
	if len(keepGoing.slots) != 1 {
		l.errorAt(call, "touch iterator yield must return bool")
		l.jump(exit)
		l.setCurrent(exit)
		return lowerValue{}
	}
	_ = l.builder.Branch(keepGoing.slots[0], advance, exit)
	l.setCurrent(advance)
	l.store(index, scalarValue(l.pure(resource.RuntimeFunctionAdd, call, index.slots[0], ir.Const{Value: 1}), types.Typ[types.Int]), call)
	l.jump(header)
	l.setCurrent(exit)
	return lowerValue{}
}

func (l *lowerer) inlineStreamIterator(call *ast.CallExpr, iterator *streamIterator, args []callArgument) lowerValue {
	if len(args) != 1 || args[0].callable == nil {
		l.errorAt(call, "stream iterator requires one yield callable")
		return lowerValue{}
	}
	if l.mode != mode.ModeWatch {
		l.errorAt(call, "stream iterators are only available in watch mode")
		return lowerValue{}
	}
	start := iterator.start
	var end ir.Expr
	if iterator.frame {
		now := ir.Load{Place: l.memory("RuntimeUpdate", ir.Const{}, 0, 0, true, false, call)}
		delta := ir.Load{Place: l.memory("RuntimeUpdate", ir.Const{}, 0, 1, true, false, call)}
		start = l.pure(resource.RuntimeFunctionSubtract, call, now, delta)
		end = now
	}
	current := l.alloc("stream.iterator.key", types.Typ[types.Float64])
	first := l.streamKey(call, iterator.receiver, start, !iterator.desc)
	if !iterator.frame {
		first = l.pure(resource.RuntimeFunctionIf, call, l.streamHas(call, iterator.receiver, start), start, first)
	}
	l.store(current, scalarValue(first, types.Typ[types.Float64]), call)
	header, body, advance, exit := l.newBlock(), l.newBlock(), l.newBlock(), l.newBlock()
	l.jump(header)
	l.setCurrent(header)
	condition := l.streamHas(call, iterator.receiver, current.slots[0])
	if iterator.frame {
		condition = l.pure(resource.RuntimeFunctionAnd, call, condition, l.pure(resource.RuntimeFunctionLessOr, call, current.slots[0], end))
	}
	_ = l.builder.Branch(condition, body, exit)
	l.setCurrent(body)
	streamNamed, _ := namedType(iterator.receiver.type_)
	valueType := streamNamed.TypeArgs().At(0)
	value := l.streamValueAt(call, iterator.receiver, current.slots[0], valueType)
	var yielded []callArgument
	switch iterator.kind {
	case "items":
		yielded = []callArgument{{value: scalarValue(current.slots[0], types.Typ[types.Float64])}, {value: value}}
	case "keys":
		yielded = []callArgument{{value: scalarValue(current.slots[0], types.Typ[types.Float64])}}
	case "values":
		yielded = []callArgument{{value: value}}
	}
	keepGoing := l.inlineStaticCallable(call, args[0].callable, yielded)
	if len(keepGoing.slots) != 1 {
		l.errorAt(call, "stream iterator yield must return bool")
		l.jump(exit)
		l.setCurrent(exit)
		return lowerValue{}
	}
	_ = l.builder.Branch(keepGoing.slots[0], advance, exit)
	l.setCurrent(advance)
	next := l.streamKey(call, iterator.receiver, current.slots[0], !iterator.desc)
	progress := l.pure(resource.RuntimeFunctionGreater, call, next, current.slots[0])
	if iterator.desc {
		progress = l.pure(resource.RuntimeFunctionLess, call, next, current.slots[0])
	}
	progressBlock := l.newBlock()
	_ = l.builder.Branch(progress, progressBlock, exit)
	l.setCurrent(progressBlock)
	l.store(current, scalarValue(next, types.Typ[types.Float64]), call)
	l.jump(header)
	l.setCurrent(exit)
	return lowerValue{}
}

func (l *lowerer) inlineContainerIterator(call *ast.CallExpr, iterator *containerIterator, args []callArgument) lowerValue {
	if len(args) != 1 || args[0].callable == nil {
		l.errorAt(call, "container iterator requires one yield callable")
		return lowerValue{}
	}
	index := l.allocZeroed("container.iterator.index", types.Typ[types.Int], call)
	if iterator.desc {
		l.store(index, scalarValue(l.pure(resource.RuntimeFunctionSubtract, call, iterator.receiver.slots[0], ir.Const{Value: 1}), types.Typ[types.Int]), call)
	}
	header, body, exit := l.newBlock(), l.newBlock(), l.newBlock()
	l.jump(header)
	l.setCurrent(header)
	condition := l.pure(resource.RuntimeFunctionLess, call, index.slots[0], iterator.receiver.slots[0])
	if iterator.desc {
		condition = l.pure(resource.RuntimeFunctionGreaterOr, call, index.slots[0], ir.Const{})
	}
	_ = l.builder.Branch(condition, body, exit)
	l.setCurrent(body)
	values := make([]callArgument, 0, len(iterator.offsets)+1)
	if iterator.index {
		values = append(values, callArgument{value: scalarValue(index.slots[0], types.Typ[types.Int])})
	}
	for i, offset := range iterator.offsets {
		size := l.runtimeTypeOf(types.NewArray(iterator.types[i], 1)).Slots
		values = append(values, callArgument{value: l.containerElement(call, iterator.receiver.container, index.slots[0], offset, size, iterator.types[i])})
	}
	keepGoing := l.inlineStaticCallable(call, args[0].callable, values)
	next, stop := l.newBlock(), l.newBlock()
	if len(keepGoing.slots) != 1 {
		l.errorAt(call, "container iterator yield must return bool")
		l.jump(exit)
		l.setCurrent(exit)
		return lowerValue{}
	}
	_ = l.builder.Branch(keepGoing.slots[0], next, stop)
	l.setCurrent(next)
	advance := resource.RuntimeFunctionAdd
	if iterator.desc {
		advance = resource.RuntimeFunctionSubtract
	}
	l.store(index, lowerValue{type_: types.Typ[types.Int], slots: []ir.Expr{l.pure(advance, call, index.slots[0], ir.Const{Value: 1})}}, call)
	l.jump(header)
	l.setCurrent(stop)
	l.jump(exit)
	l.setCurrent(exit)
	return lowerValue{}
}

func (l *lowerer) inlineCallableAlternatives(call *ast.CallExpr, callable *staticCallable, args []callArgument) lowerValue {
	resultType := l.pkg.TypesInfo.TypeOf(call)
	if isFunctionType(resultType) || isContainerType(resultType) || isPointerType(resultType) {
		l.errorAt(call, "runtime callable selection cannot return compile-time-only values")
		return zeroValue(resultType)
	}
	if len(callable.alternatives) == 0 {
		l.terminateRuntime(call, "call of nil function")
		return zeroValue(resultType)
	}
	valid := l.pure(resource.RuntimeFunctionAnd, call,
		l.pure(resource.RuntimeFunctionGreaterOr, call, callable.tag.slots[0], ir.Const{}),
		l.pure(resource.RuntimeFunctionLess, call, callable.tag.slots[0], ir.Const{Value: float64(len(callable.alternatives))}))
	l.guardWith(call, valid, "call of nil function", true)
	var result lowerValue
	if resultType != nil {
		result = l.allocZeroed("callable.result", resultType, call)
	}
	merge := l.newBlock()
	invalid := l.newBlock()
	blocks := make([]*ir.Block, len(callable.alternatives))
	cases := make([]ir.SwitchCase, len(blocks))
	for i := range blocks {
		blocks[i] = l.newBlock()
		cases[i] = ir.SwitchCase{Value: float64(i), Target: blocks[i].ID}
	}
	_ = l.builder.Switch(callable.tag.slots[0], cases, invalid)
	for i, alternative := range callable.alternatives {
		l.setCurrent(blocks[i])
		value := l.inlineStaticCallable(call, alternative, args)
		if resultType != nil {
			if value.type_ == nil {
				value.type_ = resultType
			}
			l.store(result, value, call)
		}
		l.jump(merge)
	}
	l.setCurrent(invalid)
	_ = l.builder.MarkUnreachable()
	l.setCurrent(merge)
	return result
}

func (l *lowerer) inlineRangeYield(call *ast.CallExpr, yield *rangeYield, args []callArgument) lowerValue {
	statement := yield.statement
	targets := []ast.Expr{}
	if statement.Key != nil {
		targets = append(targets, statement.Key)
	}
	if statement.Value != nil {
		targets = append(targets, statement.Value)
	}
	if len(args) != len(targets) {
		l.errorAt(call, "range-over-func yielded %d values; expected %d", len(args), len(targets))
		return lowerValue{type_: types.Typ[types.Bool], slots: []ir.Expr{ir.Const{}}}
	}
	if len(yield.active.slots) == 1 {
		l.guardWith(call, yield.active.slots[0], "range-over-func producer called yield after it returned false", false)
	}
	for i, target := range targets {
		if args[i].callable != nil {
			l.errorAt(call, "range-over-func cannot yield callable values")
			continue
		}
		l.assignRangeValue(target, statement.Tok, args[i].value)
	}
	result := l.alloc("range.yield", types.Typ[types.Bool])
	continued, broken, merge := l.newBlock(), l.newBlock(), l.newBlock()
	restoreLabel := l.pushLabel(yield.label, broken, continued)
	l.breaks = append(l.breaks, broken)
	l.continues = append(l.continues, continued)
	l.returnFrames = append(l.returnFrames, returnRedirect{owner: yield.owner, depth: len(l.frames)})
	l.dynamic(func() { l.stmts(statement.Body.List) })
	l.returnFrames = l.returnFrames[:len(l.returnFrames)-1]
	l.jump(continued)
	l.breaks = l.breaks[:len(l.breaks)-1]
	l.continues = l.continues[:len(l.continues)-1]
	restoreLabel()
	l.setCurrent(continued)
	l.store(result, lowerValue{type_: types.Typ[types.Bool], slots: []ir.Expr{ir.Const{Value: 1}}}, call)
	l.jump(merge)
	l.setCurrent(broken)
	if len(yield.active.places) == 1 {
		l.store(yield.active, lowerValue{type_: types.Typ[types.Bool], slots: []ir.Expr{ir.Const{}}}, call)
	}
	l.store(result, lowerValue{type_: types.Typ[types.Bool], slots: []ir.Expr{ir.Const{}}}, call)
	l.jump(merge)
	l.setCurrent(merge)
	return result
}

func (l *lowerer) inlineLiteralArguments(call *ast.CallExpr, callable *staticCallable, args []callArgument) lowerValue {
	if len(l.inlineCalls) >= 256 {
		l.errorAt(call, "closure inline depth exceeds 256 calls")
		return zeroValue(l.pkg.TypesInfo.TypeOf(call))
	}
	literal := callable.literal
	oldPkg := l.pkg
	if callable.pkg != nil {
		l.pkg = callable.pkg
	}
	defer func() { l.pkg = oldPkg }()
	sig, ok := l.pkg.TypesInfo.TypeOf(literal).Underlying().(*types.Signature)
	if !ok {
		l.errorAt(call, "immediate closure has no static signature")
		return lowerValue{}
	}
	active := l.callStack[literal]
	var recursionKey recursiveCallKey
	if active {
		fingerprint, static := staticCallFingerprint(args)
		if !static {
			l.errorAt(call, "recursive static closure call requires compile-time constant arguments")
			return zeroValue(oldPkg.TypesInfo.TypeOf(call))
		}
		recursionKey = recursiveCallKey{target: literal, args: fingerprint}
		if l.callStack[recursionKey] {
			l.errorAt(call, "recursive static closure call repeats compile-time state")
			return zeroValue(oldPkg.TypesInfo.TypeOf(call))
		}
		l.callStack[recursionKey] = true
	}
	l.callStack[literal] = true
	defer func() {
		if active {
			delete(l.callStack, recursionKey)
		} else {
			delete(l.callStack, literal)
		}
	}()
	l.beginInlineLocalScope()
	frame := &lowerFrame{
		pkg:  l.pkg,
		vars: cloneValueBindings(callable.captures), callables: cloneCallableBindings(callable.callables),
		results: map[types.Object]bool{}, returnBlock: l.newBlock(),
	}
	l.prepareGotoTargets(frame, literal.Body)
	l.prepareDeferredCalls(frame, literal.Body)
	l.prepareCallableMutability(frame, literal.Body)
	l.prepareValueParameterUsage(frame, literal.Body)
	resultType := callable.resultType
	if resultType == nil {
		resultType = oldPkg.TypesInfo.TypeOf(call)
	}
	if result, ok := l.newDescriptorMultiResult("closure.result", resultType, call); ok {
		frame.result = result
	} else if isFunctionType(resultType) {
		frame.result = lowerValue{type_: resultType}
	} else if isInterfaceType(resultType) {
		frame.result = lowerValue{type_: resultType}
		frame.interfaceResult = true
	} else if resultType != nil {
		if _, callableArray := callableArrayType(resultType); callableArray {
			frame.result = l.newCallableArray("closure.callable.result", resultType, call)
		} else if binding, entityView := l.entityBinding(resultType); entityView {
			frame.result = l.newEntityViewLocal("closure.entity.result", resultType, binding)
		} else if l.isAggregatePointerType(resultType) {
			frame.result = l.newAggregatePointerCell("closure.aggregate.pointer.result", resultType, call)
		} else if isPointerType(resultType) {
			frame.result = l.newPointerCell("closure.pointer.result", resultType, call)
		} else if isContainerType(resultType) {
			frame.result = l.newContainerCell("closure.container.result", resultType, call)
		} else if l.containsAggregateDescriptor(resultType) {
			frame.result = l.newAggregateCell("closure.aggregate.result", resultType, call)
		} else {
			frame.result = l.allocZeroed("closure.result", resultType, call)
		}
	}
	l.frames = append(l.frames, frame)
	l.inlineCalls = append(l.inlineCalls, inlineCallSite{function: "immediate closure", pos: sourcePos(l.pkg, call.Pos())})
	defer func() {
		l.inlineCalls = l.inlineCalls[:len(l.inlineCalls)-1]
		l.frames = l.frames[:len(l.frames)-1]
	}()
	arg := 0
	parameterIndex := 0
	for fieldIndex, field := range literal.Type.Params.List {
		if len(field.Names) == 0 {
			arg++
			parameterIndex++
			continue
		}
		for _, name := range field.Names {
			if sig.Variadic() && fieldIndex == len(literal.Type.Params.List)-1 {
				obj, _ := l.pkg.TypesInfo.Defs[name].(*types.Var)
				parameterType := l.resolveType(obj.Type())
				slice, _ := types.Unalias(parameterType).Underlying().(*types.Slice)
				elements := make([]lowerValue, len(args)-arg)
				callables := make([]*staticCallable, len(elements))
				for i := range elements {
					if args[arg+i].callable != nil {
						callables[i] = args[arg+i].callable
					} else {
						elements[i] = args[arg+i].value
					}
				}
				l.bind(obj, lowerValue{type_: parameterType, variadic: &variadicValue{element: slice.Elem(), elements: elements, callables: callables}})
				arg = len(args)
				parameterIndex++
				continue
			}
			if arg >= len(args) {
				l.errorAt(call, "immediate closure argument count mismatch")
				break
			}
			if obj, ok := l.pkg.TypesInfo.Defs[name].(*types.Var); ok {
				if isFunctionType(obj.Type()) {
					if args[arg].callable == nil {
						l.errorAt(call, "function parameter %s requires a static callable", name.Name)
					} else {
						l.bindCallable(obj, l.callableBinding(obj, name.Name, args[arg].callable, call))
					}
				} else {
					l.bindParameter(obj, args[arg].value, name.Name, call)
				}
			}
			arg++
			parameterIndex++
		}
	}
	if literal.Type.Results != nil && len(frame.result.multi) != 0 {
		tuple, _ := types.Unalias(l.resolveType(resultType)).(*types.Tuple)
		l.bindNamedMultiResults(frame, literal.Type.Results, tuple)
	} else if literal.Type.Results != nil && frame.result.aggregatePointer != nil {
		if sig.Results().Len() != 1 {
			l.errorAt(literal.Type.Results, "aggregate pointer helper result must be a single value")
		} else if len(literal.Type.Results.List) == 1 && len(literal.Type.Results.List[0].Names) == 1 {
			name := literal.Type.Results.List[0].Names[0]
			if obj := l.pkg.TypesInfo.Defs[name]; obj != nil {
				frame.result = l.mergeAggregatePointerValue(frame.result, lowerValue{type_: resultType, nilPointer: true}, name)
				frame.results[obj] = true
				frame.vars[obj] = frame.result
			}
		}
	} else if literal.Type.Results != nil && frame.result.aggregate != nil {
		if sig.Results().Len() != 1 {
			l.errorAt(literal.Type.Results, "aggregate helper result must be a single struct value")
		} else if len(literal.Type.Results.List) == 1 && len(literal.Type.Results.List[0].Names) == 1 {
			name := literal.Type.Results.List[0].Names[0]
			if obj := l.pkg.TypesInfo.Defs[name]; obj != nil {
				frame.results[obj] = true
				frame.vars[obj] = frame.result
			}
		}
	} else if literal.Type.Results != nil && frame.result.slots != nil {
		offset := 0
		for _, field := range literal.Type.Results.List {
			fieldType := l.resolveType(l.pkg.TypesInfo.TypeOf(field.Type))
			size := l.runtimeTypeOf(fieldType).Slots
			if _, callableArray := callableArrayType(fieldType); callableArray {
				size = 0
			}
			if frame.result.entity != nil {
				size = 1
			}
			if len(field.Names) == 0 {
				offset += size
				continue
			}
			for _, name := range field.Names {
				if obj := l.pkg.TypesInfo.Defs[name]; obj != nil {
					objectType := l.resolveType(obj.Type())
					frame.results[obj] = true
					if frame.result.callableArray != nil {
						frame.vars[obj] = frame.result
					} else if frame.result.entity != nil {
						frame.vars[obj] = lowerValue{type_: objectType, slots: frame.result.slots[offset : offset+1], places: frame.result.places[offset : offset+1], entity: frame.result.entity}
					} else if isContainerType(objectType) || isPointerType(objectType) {
						frame.vars[obj] = lowerValue{type_: objectType}
					} else {
						frame.vars[obj] = lowerValue{type_: objectType, slots: frame.result.slots[offset : offset+size], places: frame.result.places[offset : offset+size]}
					}
				}
				offset += size
			}
		}
	}
	minimumArgs := sig.Params().Len()
	if sig.Variadic() {
		minimumArgs--
	}
	if arg != len(args) || parameterIndex != sig.Params().Len() || len(args) < minimumArgs || (!sig.Variadic() && sig.Params().Len() != len(args)) {
		l.errorAt(call, "immediate closure argument count mismatch")
	}
	l.stmts(literal.Body.List)
	l.jump(frame.returnBlock)
	l.setCurrent(frame.returnBlock)
	l.runDeferredCalls(frame)
	if isFunctionType(resultType) {
		result := lowerValue{type_: resultType, callable: frame.callableResult}
		l.endInlineLocalScope(result)
		return result
	}
	result := frame.result
	l.endInlineLocalScope(result)
	return result
}

func (l *lowerer) runtimeCall(n *ast.CallExpr, op resource.RuntimeFunction, pure bool, prefix []float64, args []lowerValue) lowerValue {
	flat := make([]ir.Expr, 0)
	for _, p := range prefix {
		flat = append(flat, ir.Const{Value: p})
	}
	for _, arg := range args {
		flat = append(flat, arg.slots...)
	}
	if l.streamSize > 1 && (op == resource.RuntimeFunctionStreamSet || op == resource.RuntimeFunctionStreamHas || op == resource.RuntimeFunctionStreamGetValue || op == resource.RuntimeFunctionStreamGetPreviousKey || op == resource.RuntimeFunctionStreamGetNextKey) {
		if len(flat) == 0 {
			l.errorAt(n, "low-level stream API requires a stream ID")
		} else if id, ok := flat[0].(ir.Const); !ok {
			l.errorAt(n, "dynamic low-level stream ID may overlap typed stream IDs 1..%d", l.streamSize-1)
		} else if id.Value >= 1 && id.Value < float64(l.streamSize) {
			l.errorAt(n, "low-level stream ID %v overlaps typed stream IDs 1..%d", id.Value, l.streamSize-1)
		}
	}
	t := l.resolveType(l.pkg.TypesInfo.TypeOf(n))
	resultType := l.runtimeTypeOf(t)
	call := l.builder.RuntimeCall(op, flat, resultType, pure, sourcePos(l.pkg, n.Pos()))
	if t == nil || resultType.Slots == 0 {
		_ = l.builder.Eval(call)
		return lowerValue{}
	}
	out := l.zeroRuntimeValue(t)
	if len(out.slots) != 1 {
		l.errorAt(n, "runtime call %s cannot directly return a multi-slot value", op)
		return out
	}
	if pure {
		out.slots[0] = call
		return out
	}
	result := l.alloc("runtime.result", t)
	if err := l.builder.Store(result.places, ir.Value{Type: resultType, Slots: []ir.Expr{call}}, sourcePos(l.pkg, n.Pos())); err != nil {
		l.errorAt(n, "%v", err)
	}
	return result
}

func (l *lowerer) inlineCall(n *ast.CallExpr, fn *types.Func, args []lowerValue) lowerValue {
	arguments := make([]callArgument, len(args))
	for i, value := range args {
		arguments[i].value = value
	}
	return l.inlineCallArguments(n, fn, arguments)
}

func (l *lowerer) bindNamedMultiResults(frame *lowerFrame, fields *ast.FieldList, tuple *types.Tuple) {
	if fields == nil {
		return
	}
	resultIndex := 0
	for _, field := range fields.List {
		count := len(field.Names)
		if count == 0 {
			count = 1
		}
		for index := 0; index < count && resultIndex < len(frame.result.multi) && resultIndex < tuple.Len(); index++ {
			value := frame.result.multi[resultIndex]
			if len(field.Names) != 0 {
				name := field.Names[index]
				if value.aggregatePointer != nil {
					value = l.mergeAggregatePointerValue(value, lowerValue{type_: tuple.At(resultIndex).Type(), nilPointer: true}, name)
				} else if value.pointer != nil {
					value = l.mergePointerValue(value, lowerValue{type_: tuple.At(resultIndex).Type(), nilPointer: true}, name)
				}
				frame.result.multi[resultIndex] = value
				if object := l.pkg.TypesInfo.Defs[name]; object != nil {
					frame.results[object] = true
					frame.vars[object] = value
				}
			}
			resultIndex++
		}
	}
}

func (l *lowerer) storeHelperResultCell(destination, source lowerValue, node ast.Node) lowerValue {
	switch {
	case destination.aggregatePointer != nil:
		if !isAggregatePointerValue(source) && !source.nilPointer {
			l.errorAt(node, "aggregate pointer helper must return an aggregate address or nil")
			return destination
		}
		return l.mergeAggregatePointerValue(destination, source, node)
	case destination.aggregate != nil:
		if source.aggregate == nil {
			l.errorAt(node, "aggregate helper must return a local struct with pointer or container fields")
			return destination
		}
		l.storeAggregate(destination, source, node)
		return destination
	case destination.callableArray != nil:
		if source.callableArray == nil {
			l.errorAt(node, "callable array helper return requires a fixed callable array")
			return destination
		}
		l.storeCallableArray(destination, source, node)
		return destination
	case destination.entity != nil:
		if source.entity == nil {
			l.errorAt(node, "EntityRef.Get view helper must return an entity view")
			return destination
		}
		l.storeEntityView(destination, source, node)
		return destination
	case destination.pointer != nil:
		if !isStaticPointer(source) {
			l.errorAt(node, "pointer helper must return a statically known address")
			return destination
		}
		return l.mergePointerValue(destination, source, node)
	case destination.container != nil || destination.containerVariant != nil:
		if !isContainerValue(source) {
			l.errorAt(node, "container helper must return a catalog container with static backing storage")
			return destination
		}
		return l.mergeContainerValue(destination, source, node)
	case destination.interface_ != nil:
		return l.storeInterfaceValue(destination, source, node)
	case isFunctionType(destination.type_):
		l.errorAt(node, "function values cannot be returned as part of a multi-value helper result")
		return destination
	default:
		if len(source.multi) != 0 {
			l.errorAt(node, "nested multi-value helper result is not supported")
			return destination
		}
		source = l.materialize("return.value", source, node)
		l.store(destination, source, node)
		return destination
	}
}

func (l *lowerer) storeMultiHelperReturn(frame *lowerFrame, statement *ast.ReturnStmt) {
	if len(statement.Results) == 0 {
		return
	}
	var values []lowerValue
	if len(statement.Results) == 1 {
		value := l.expr(statement.Results[0])
		if len(value.multi) != 0 {
			values = value.multi
		} else {
			values = []lowerValue{value}
		}
	} else {
		values = make([]lowerValue, len(statement.Results))
		for index, expression := range statement.Results {
			values[index] = l.expr(expression)
		}
	}
	if len(values) != len(frame.result.multi) {
		l.errorAt(statement, "multi-value helper return produced %d values; expected %d", len(values), len(frame.result.multi))
		return
	}
	for index := range values {
		frame.result.multi[index] = l.storeHelperResultCell(frame.result.multi[index], values[index], statement.Results[0])
	}
}

func (l *lowerer) inlineCallArguments(n *ast.CallExpr, fn *types.Func, args []callArgument) lowerValue {
	return l.inlineCallArgumentsAs(n, fn, args, nil)
}

func (l *lowerer) inlineCallArgumentsAs(n *ast.CallExpr, fn *types.Func, args []callArgument, overrideResult types.Type) lowerValue {
	if len(l.inlineCalls) >= 256 {
		l.errorAt(n, "helper inline depth exceeds 256 calls")
		return zeroValue(l.pkg.TypesInfo.TypeOf(n))
	}
	active := l.callStack[fn]
	var recursionKey recursiveCallKey
	if active {
		fingerprint, static := staticCallFingerprint(args)
		if !static {
			l.errorAt(n, "recursive helper call to %s requires compile-time constant arguments", fn.FullName())
			return zeroValue(l.pkg.TypesInfo.TypeOf(n))
		}
		recursionKey = recursiveCallKey{target: fn, args: fingerprint}
		if l.callStack[recursionKey] {
			l.errorAt(n, "recursive helper call to %s repeats compile-time state", fn.FullName())
			return zeroValue(l.pkg.TypesInfo.TypeOf(n))
		}
		l.callStack[recursionKey] = true
	}
	pkg := l.packages[fn.Pkg()]
	if pkg == nil {
		l.errorAt(n, "function %s has no source body", fn.FullName())
		return zeroValue(l.pkg.TypesInfo.TypeOf(n))
	}
	decl := findFuncDecl(pkg, fn)
	if decl == nil || decl.Body == nil {
		l.errorAt(n, "function %s has no source body", fn.FullName())
		return zeroValue(l.pkg.TypesInfo.TypeOf(n))
	}
	resultType := overrideResult
	if resultType == nil {
		resultType = l.pkg.TypesInfo.TypeOf(n)
	}
	sig := fn.Type().(*types.Signature)
	sourceSig := fn.Origin().Type().(*types.Signature)
	expectedArgs := sig.Params().Len()
	if sig.Recv() != nil {
		expectedArgs++
	}
	minimumArgs := expectedArgs
	if sig.Variadic() {
		minimumArgs--
	}
	if len(args) < minimumArgs || (!sig.Variadic() && len(args) != expectedArgs) {
		l.errorAt(n, "helper %s received %d arguments; expected %d", fn.FullName(), len(args), expectedArgs)
		return zeroValue(resultType)
	}
	callSig, _ := l.pkg.TypesInfo.TypeOf(n.Fun).Underlying().(*types.Signature)
	if callSig == nil {
		callSig = sig
	}
	substitutions := l.inferCallTypeSubstitutions(sourceSig, callSig, args)
	if instance, exists := callableInstance(l.pkg, n.Fun); exists {
		for index := 0; index < sourceSig.TypeParams().Len() && index < instance.TypeArgs.Len(); index++ {
			substitutions[sourceSig.TypeParams().At(index)] = l.resolveType(instance.TypeArgs.At(index))
		}
	}
	l.typeSubstitutions = append(l.typeSubstitutions, substitutions)
	defer func() { l.typeSubstitutions = l.typeSubstitutions[:len(l.typeSubstitutions)-1] }()
	oldPkg := l.pkg
	l.pkg = pkg
	defer func() { l.pkg = oldPkg }()
	l.beginInlineLocalScope()
	frame := &lowerFrame{pkg: pkg, vars: map[types.Object]lowerValue{}, callables: map[types.Object]*staticCallable{}, results: map[types.Object]bool{}}
	if result, ok := l.newDescriptorMultiResult(fn.Name()+".result", resultType, n); ok {
		frame.result = result
	} else if isFunctionType(resultType) {
		frame.result = lowerValue{type_: resultType}
	} else if isInterfaceType(resultType) {
		frame.result = lowerValue{type_: resultType}
		frame.interfaceResult = true
	} else if resultType != nil {
		if _, callableArray := callableArrayType(resultType); callableArray {
			frame.result = l.newCallableArray(fn.Name()+".callable.result", resultType, n)
		} else if binding, entityView := l.entityBinding(resultType); entityView {
			frame.result = l.newEntityViewLocal(fn.Name()+".entity.result", resultType, binding)
		} else if l.isAggregatePointerType(resultType) {
			frame.result = l.newAggregatePointerCell(fn.Name()+".aggregate.pointer.result", resultType, n)
		} else if isPointerType(resultType) {
			frame.result = l.newPointerCell(fn.Name()+".pointer.result", resultType, n)
		} else if isContainerType(resultType) {
			frame.result = l.newContainerCell(fn.Name()+".container.result", resultType, n)
		} else if l.containsAggregateDescriptor(resultType) {
			frame.result = l.newAggregateCell(fn.Name()+".aggregate.result", resultType, n)
		} else {
			frame.result = l.allocZeroed(fn.Name()+".result", resultType, n)
		}
	}
	frame.returnBlock = l.newBlock()
	l.prepareGotoTargets(frame, decl.Body)
	l.prepareDeferredCalls(frame, decl.Body)
	l.prepareCallableMutability(frame, decl.Body)
	l.prepareValueParameterUsage(frame, decl.Body)
	l.frames = append(l.frames, frame)
	l.callStack[fn] = true
	l.inlineCalls = append(l.inlineCalls, inlineCallSite{function: fn.FullName(), pos: sourcePos(oldPkg, n.Pos())})
	defer func() {
		l.inlineCalls = l.inlineCalls[:len(l.inlineCalls)-1]
		if active {
			delete(l.callStack, recursionKey)
		} else {
			delete(l.callStack, fn)
		}
		l.frames = l.frames[:len(l.frames)-1]
	}()
	arg := 0
	if sourceSig.Recv() != nil {
		if args[arg].value.entity != nil {
			l.bindParameter(sourceSig.Recv(), args[arg].value, sourceSig.Recv().Name(), n)
		} else {
			receiver := args[arg].value
			receiver.type_ = l.resolveType(sourceSig.Recv().Type())
			frame.vars[sourceSig.Recv()] = receiver
		}
		arg++
	}
	for i := 0; i < sourceSig.Params().Len(); i++ {
		parameter := sourceSig.Params().At(i)
		if sourceSig.Variadic() && i == sourceSig.Params().Len()-1 {
			parameterType := l.resolveType(parameter.Type())
			slice, _ := types.Unalias(parameterType).Underlying().(*types.Slice)
			elements := make([]lowerValue, len(args)-arg)
			callables := make([]*staticCallable, len(elements))
			for j := range elements {
				if args[arg+j].callable != nil {
					callables[j] = args[arg+j].callable
				} else {
					elements[j] = args[arg+j].value
				}
			}
			l.bind(parameter, lowerValue{type_: parameterType, variadic: &variadicValue{element: slice.Elem(), elements: elements, callables: callables}})
			arg = len(args)
			continue
		}
		if arg >= len(args) {
			break
		}
		if isFunctionType(parameter.Type()) {
			if args[arg].callable == nil {
				l.errorAt(n, "function parameter %s requires a static callable", parameter.Name())
			} else {
				l.bindCallable(parameter, l.callableBinding(parameter, parameter.Name(), args[arg].callable, n))
			}
		} else {
			l.bindParameter(parameter, args[arg].value, parameter.Name(), n)
		}
		arg++
	}
	if decl.Type.Results != nil && len(frame.result.multi) != 0 {
		tuple, _ := types.Unalias(l.resolveType(resultType)).(*types.Tuple)
		l.bindNamedMultiResults(frame, decl.Type.Results, tuple)
	} else if decl.Type.Results != nil && frame.result.aggregatePointer != nil {
		if callSig.Results().Len() != 1 {
			l.errorAt(decl.Type.Results, "aggregate pointer helper result must be a single value")
		} else if len(decl.Type.Results.List) == 1 && len(decl.Type.Results.List[0].Names) == 1 {
			name := decl.Type.Results.List[0].Names[0]
			if result := pkg.TypesInfo.Defs[name]; result != nil {
				frame.result = l.mergeAggregatePointerValue(frame.result, lowerValue{type_: resultType, nilPointer: true}, name)
				frame.results[result] = true
				frame.vars[result] = frame.result
			}
		}
	} else if decl.Type.Results != nil && frame.result.aggregate != nil {
		if callSig.Results().Len() != 1 {
			l.errorAt(decl.Type.Results, "aggregate helper result must be a single struct value")
		} else if len(decl.Type.Results.List) == 1 && len(decl.Type.Results.List[0].Names) == 1 {
			name := decl.Type.Results.List[0].Names[0]
			if result := pkg.TypesInfo.Defs[name]; result != nil {
				frame.results[result] = true
				frame.vars[result] = frame.result
			}
		}
	} else if decl.Type.Results != nil {
		offset, resultIndex := 0, 0
		for _, field := range decl.Type.Results.List {
			count := len(field.Names)
			if count == 0 {
				count = 1
			}
			for i := 0; i < count; i++ {
				actualType := callSig.Results().At(resultIndex).Type()
				size := irTypeOf(actualType).Slots
				if _, callableArray := callableArrayType(actualType); callableArray {
					size = 0
				}
				if frame.result.entity != nil {
					size = 1
				}
				if len(field.Names) != 0 {
					result := pkg.TypesInfo.Defs[field.Names[i]]
					frame.results[result] = true
					if frame.result.callableArray != nil {
						frame.vars[result] = frame.result
					} else if frame.result.entity != nil {
						frame.vars[result] = lowerValue{type_: actualType, slots: frame.result.slots[offset : offset+1], places: frame.result.places[offset : offset+1], entity: frame.result.entity}
					} else if frame.result.aggregatePointer != nil {
						frame.vars[result] = frame.result
					} else if isPointerType(actualType) {
						if frame.result.pointer != nil && len(frame.result.pointer.alternatives) == 0 {
							frame.result = l.mergePointerValue(frame.result, lowerValue{type_: actualType, nilPointer: true}, field.Names[i])
						}
						frame.vars[result] = frame.result
					} else if isContainerType(actualType) {
						frame.vars[result] = frame.result
					} else {
						frame.vars[result] = lowerValue{type_: actualType, slots: frame.result.slots[offset : offset+size], places: frame.result.places[offset : offset+size]}
					}
				}
				offset += size
				resultIndex++
			}
		}
	}
	l.stmts(decl.Body.List)
	l.jump(frame.returnBlock)
	l.setCurrent(frame.returnBlock)
	l.runDeferredCalls(frame)
	if isFunctionType(resultType) {
		result := lowerValue{type_: resultType, callable: frame.callableResult}
		l.endInlineLocalScope(result)
		return result
	}
	result := frame.result
	l.endInlineLocalScope(result)
	return result
}

func (l *lowerer) stmt(stmt ast.Stmt) {
	if l.current() == nil {
		return
	}
	if l.current().Terminator != nil {
		labeled, ok := stmt.(*ast.LabeledStmt)
		if !ok || l.gotoTarget(labeled.Label, true) == nil {
			return
		}
	}
	switch n := stmt.(type) {
	case *ast.BlockStmt:
		l.stmts(n.List)
	case *ast.ExprStmt:
		v := l.expr(n.X)
		for _, e := range v.slots {
			if c, ok := e.(ir.RuntimeCall); ok && !c.Pure {
				_ = l.builder.Eval(e)
			}
		}
	case *ast.DeclStmt:
		decl, ok := n.Decl.(*ast.GenDecl)
		if !ok {
			l.errorAt(n, "only local var declarations are supported by the frozen callback subset")
			return
		}
		if decl.Tok == token.CONST || decl.Tok == token.TYPE {
			return
		}
		if decl.Tok != token.VAR {
			l.errorAt(n, "only local var, const, and type declarations are supported by the callback subset")
			return
		}
		for _, spec := range decl.Specs {
			vs, ok := spec.(*ast.ValueSpec)
			if !ok {
				l.errorAt(spec, "unsupported local declaration %T", spec)
				continue
			}
			if len(vs.Values) != 0 {
				l.assign(&ast.AssignStmt{Lhs: identifiersAsExprs(vs.Names), Tok: token.DEFINE, Rhs: vs.Values})
				continue
			}
			for _, name := range vs.Names {
				obj := l.pkg.TypesInfo.Defs[name]
				objectType := l.resolveType(obj.Type())
				if isInterfaceType(objectType) {
					l.bind(obj, l.newInterfaceValue(name.Name+".interface", objectType, name))
					continue
				}
				if isFunctionType(objectType) {
					l.bindCallable(obj, l.newCallableCell(name.Name, nil, name))
					continue
				}
				if _, callableArray := callableArrayType(objectType); callableArray {
					l.bind(obj, l.newCallableArray(name.Name, objectType, name))
					continue
				}
				if kind := containsResourceHandle(objectType); kind != "" {
					l.errorAt(name, "%s resource handle aggregates cannot be declared without a resource value", kind)
					continue
				}
				if l.containsAggregateDescriptor(objectType) {
					l.bind(obj, l.newAggregateCell(name.Name, objectType, name))
					continue
				}
				if isPointerType(objectType) {
					value := lowerValue{type_: objectType, nilPointer: true}
					if l.isAggregatePointerType(objectType) {
						l.bind(obj, l.copyAggregatePointerValue(name.Name, value, name))
					} else {
						l.bind(obj, l.copyPointerValue(name.Name, value, name))
					}
					continue
				}
				l.bind(obj, l.allocZeroed(name.Name, objectType, name))
			}
		}
	case *ast.AssignStmt:
		l.assign(n)
	case *ast.IncDecStmt:
		dst := l.ensureAssignable(l.expr(n.X), n.X)
		op := resource.RuntimeFunctionAdd
		if n.Tok == token.DEC {
			op = resource.RuntimeFunctionSubtract
		}
		src := lowerValue{type_: dst.type_, slots: []ir.Expr{ir.RuntimeCall{Function: op, Args: []ir.Expr{dst.slots[0], ir.Const{Value: 1}}, Result: irTypeOf(dst.type_), Pure: true, Pos: sourcePos(l.pkg, n.Pos())}}}
		l.store(dst, src, n)
	case *ast.IfStmt:
		l.ifStmt(n)
	case *ast.ForStmt:
		l.forStmt(n, "")
	case *ast.RangeStmt:
		l.rangeStmt(n, "")
	case *ast.SwitchStmt:
		l.switchStmt(n, "")
	case *ast.TypeSwitchStmt:
		l.typeSwitchStmt(n, "")
	case *ast.LabeledStmt:
		if target := l.gotoTarget(n.Label, true); target != nil {
			l.jump(target)
			l.setCurrent(target)
		}
		switch statement := n.Stmt.(type) {
		case *ast.ForStmt:
			l.forStmt(statement, n.Label.Name)
		case *ast.RangeStmt:
			l.rangeStmt(statement, n.Label.Name)
		case *ast.SwitchStmt:
			l.switchStmt(statement, n.Label.Name)
		case *ast.TypeSwitchStmt:
			l.typeSwitchStmt(statement, n.Label.Name)
		default:
			l.stmt(statement)
		}
	case *ast.BranchStmt:
		if n.Label != nil {
			if n.Tok == token.GOTO {
				target := l.gotoTarget(n.Label, false)
				if target == nil {
					l.errorAt(n, "goto label %s is not declared in this function", n.Label.Name)
				} else {
					l.jump(target)
				}
				break
			}
			targets, ok := l.labels[n.Label.Name]
			if !ok {
				l.errorAt(n, "branch label %s is not an active loop or switch", n.Label.Name)
				break
			}
			switch n.Tok {
			case token.BREAK:
				if targets.breakTarget == nil {
					l.errorAt(n, "label %s cannot be used with break", n.Label.Name)
				} else {
					l.jump(targets.breakTarget)
				}
			case token.CONTINUE:
				if targets.continueTarget == nil {
					l.errorAt(n, "label %s does not name a loop", n.Label.Name)
				} else {
					l.jump(targets.continueTarget)
				}
			default:
				l.errorAt(n, "unsupported labeled branch %s", n.Tok)
			}
			break
		}
		switch n.Tok {
		case token.BREAK:
			if len(l.breaks) == 0 {
				l.errorAt(n, "break is not inside a loop or switch")
			} else {
				l.jump(l.breaks[len(l.breaks)-1])
			}
		case token.CONTINUE:
			if len(l.continues) == 0 {
				l.errorAt(n, "continue is not inside a loop")
			} else {
				l.jump(l.continues[len(l.continues)-1])
			}
		case token.FALLTHROUGH:
			if len(l.fallthroughs) == 0 || l.fallthroughs[len(l.fallthroughs)-1] == nil {
				l.errorAt(n, "fallthrough is not inside a non-final expression switch case")
			} else {
				l.jump(l.fallthroughs[len(l.fallthroughs)-1])
			}
		default:
			l.errorAt(n, "unsupported branch %s", n.Tok)
		}
	case *ast.DeferStmt:
		frame, deferred := l.deferredCall(n)
		if frame == nil || deferred == nil {
			l.errorAt(n, "defer statement is not owned by the active function")
			return
		}
		if deferred.repeated || frame.hasGoto {
			l.errorAt(n, "defer in loops or functions containing goto requires a runtime defer stack")
			return
		}
		callable, ok := l.staticCallable(n.Call.Fun)
		if !ok {
			l.errorAt(n.Call.Fun, "defer requires a statically finite callable target")
			return
		}
		deferred.call = n.Call
		deferred.callable = l.newCallableCell("defer.target", callable, n.Call.Fun)
		deferred.args = l.userCallArguments(n.Call, nil)
		deferred.pkg = l.pkg
		l.store(deferred.active, scalarValue(ir.Const{Value: 1}, types.Typ[types.Bool]), n)
	case *ast.ReturnStmt:
		frame := l.frames[len(l.frames)-1]
		if len(l.returnFrames) != 0 {
			redirect := l.returnFrames[len(l.returnFrames)-1]
			if len(l.frames) == redirect.depth {
				frame = redirect.owner
			}
		}
		if len(frame.result.multi) != 0 {
			l.storeMultiHelperReturn(frame, n)
			l.jump(frame.returnBlock)
			return
		}
		if isFunctionType(frame.result.type_) {
			if len(n.Results) != 1 {
				l.errorAt(n, "callable helper return requires exactly one result")
			} else if callable, ok := l.staticCallable(n.Results[0]); !ok {
				l.errorAt(n.Results[0], "callable helper return requires a statically known target")
			} else {
				if frame.callableResult == nil {
					frame.callableResult = l.newCallableCell("callable.return", nil, n.Results[0])
				}
				l.storeCallableCell(frame.callableResult, callable, n.Results[0])
			}
			l.jump(frame.returnBlock)
			return
		}
		if frame.interfaceResult {
			if len(n.Results) != 1 {
				l.errorAt(n, "static interface helper return requires exactly one result")
			} else {
				value := l.expr(n.Results[0])
				if value.interface_ != nil {
					frame.result.interface_ = value.interface_
				} else if value.type_ == nil || isInterfaceType(value.type_) {
					l.errorAt(n.Results[0], "static interface helper return requires a known concrete type")
				} else {
					if frame.result.interface_ == nil {
						frame.result.interface_ = &interfaceValue{finiteVariant: newFiniteVariant[lowerValue](l, "interface.return.tag", n.Results[0])}
					}
					variant := frame.result.interface_
					alternative := -1
					for index, candidate := range variant.alternatives {
						if types.Identical(candidate.type_, value.type_) {
							alternative = index
							break
						}
					}
					if alternative == -1 {
						payload := l.allocZeroed("interface.return.value", value.type_, n.Results[0])
						var added bool
						alternative, added = variant.add(payload, func(left, right lowerValue) bool {
							return left.type_ != nil && right.type_ != nil && types.Identical(left.type_, right.type_)
						})
						if !added {
							l.errorAt(n.Results[0], "finite static interface return exceeds 256 alternatives")
							return
						}
					}
					l.store(variant.alternatives[alternative], value, n.Results[0])
					l.store(variant.tag, scalarValue(ir.Const{Value: float64(alternative)}, types.Typ[types.Int]), n.Results[0])
				}
			}
			l.jump(frame.returnBlock)
			return
		}
		if frame.result.aggregate != nil {
			if len(n.Results) == 0 {
				l.jump(frame.returnBlock)
				return
			}
			if len(n.Results) != 1 {
				l.errorAt(n, "aggregate helper return requires exactly one result")
			} else {
				value := l.expr(n.Results[0])
				if value.aggregate == nil {
					l.errorAt(n.Results[0], "aggregate helper must return a local struct with pointer or container fields")
				} else {
					l.storeAggregate(frame.result, value, n.Results[0])
				}
			}
			l.jump(frame.returnBlock)
			return
		}
		for _, result := range n.Results {
			if isFunctionType(l.pkg.TypesInfo.TypeOf(result)) {
				l.errorAt(result, "function values cannot be returned from callbacks or helpers")
				return
			}
		}
		if frame.result.callableArray != nil {
			if len(n.Results) == 0 {
				l.jump(frame.returnBlock)
				return
			}
			if len(n.Results) != 1 {
				l.errorAt(n, "callable array helper return requires exactly one result")
			} else {
				value := l.expr(n.Results[0])
				if value.callableArray == nil {
					l.errorAt(n.Results[0], "callable array helper return requires a fixed callable array")
				} else {
					l.storeCallableArray(frame.result, value, n.Results[0])
				}
			}
			l.jump(frame.returnBlock)
			return
		}
		if frame.result.entity != nil {
			if len(n.Results) != 1 {
				l.errorAt(n, "EntityRef.Get view helper return requires exactly one result")
			} else {
				value := l.expr(n.Results[0])
				if value.entity == nil {
					l.errorAt(n.Results[0], "EntityRef.Get view helper must return an entity view")
				} else {
					l.storeEntityView(frame.result, value, n.Results[0])
				}
			}
			l.jump(frame.returnBlock)
			return
		}
		if frame.result.aggregatePointer != nil {
			if len(n.Results) == 0 {
				if len(frame.result.aggregatePointer.alternatives) == 0 {
					l.errorAt(n, "named aggregate pointer result has no address or nil value")
				}
			} else if len(n.Results) != 1 {
				l.errorAt(n, "aggregate pointer helper return requires exactly one result")
			} else {
				value := l.expr(n.Results[0])
				if !isAggregatePointerValue(value) && !value.nilPointer {
					l.errorAt(n.Results[0], "aggregate pointer helper must return an aggregate address or nil")
				} else {
					frame.result = l.mergeAggregatePointerValue(frame.result, value, n.Results[0])
				}
			}
			l.jump(frame.returnBlock)
			return
		}
		if isPointerType(frame.result.type_) {
			if len(n.Results) == 0 {
				l.jump(frame.returnBlock)
				return
			}
			if len(n.Results) != 1 {
				l.errorAt(n, "pointer helper return requires exactly one result")
			} else {
				value := l.expr(n.Results[0])
				if !isStaticPointer(value) {
					l.errorAt(n.Results[0], "pointer helper must return a statically known address")
				} else {
					if len(frame.result.places) == 0 && frame.result.pointer == nil {
						frame.result = value
					} else if frame.result.pointer == nil && samePlaces(frame.result.places, value.places) {
						frame.result = value
					} else {
						frame.result = l.mergePointerValue(frame.result, value, n.Results[0])
					}
				}
			}
			l.jump(frame.returnBlock)
			return
		}
		if isContainerType(frame.result.type_) {
			if len(n.Results) > 1 {
				l.errorAt(n, "container helper return requires exactly one result")
			} else if len(n.Results) == 1 {
				value := l.expr(n.Results[0])
				if !isContainerValue(value) {
					l.errorAt(n.Results[0], "container helper must return a catalog container with static backing storage")
				} else {
					frame.result = l.mergeContainerValue(frame.result, value, n.Results[0])
				}
			} else if frame.result.container == nil && (frame.result.containerVariant == nil || len(frame.result.containerVariant.alternatives) == 0) {
				l.errorAt(n, "named container result has no static backing storage")
			}
			l.jump(frame.returnBlock)
			return
		}
		if len(n.Results) > 0 {
			combined := lowerValue{type_: frame.result.type_}
			for _, result := range n.Results {
				value := l.expr(result)
				if value.entity != nil {
					l.errorAt(result, "EntityRef.Get views cannot escape through helper returns")
					return
				}
				value = l.materialize("return.value", value, result)
				if value.variadic != nil {
					l.errorAt(result, "variadic helper parameters cannot escape through return")
					return
				}
				combined.slots = append(combined.slots, value.slots...)
			}
			l.store(frame.result, combined, n)
		}
		l.jump(frame.returnBlock)
	case *ast.EmptyStmt:
	default:
		l.errorAt(n, "unsupported callback statement %T", n)
	}
}

func (l *lowerer) typeSwitchStmt(statement *ast.TypeSwitchStmt, label string) {
	if statement.Init != nil {
		l.stmt(statement.Init)
	}
	var assertion *ast.TypeAssertExpr
	switch assignment := statement.Assign.(type) {
	case *ast.ExprStmt:
		assertion, _ = assignment.X.(*ast.TypeAssertExpr)
	case *ast.AssignStmt:
		if len(assignment.Rhs) == 1 {
			assertion, _ = assignment.Rhs[0].(*ast.TypeAssertExpr)
		}
	}
	if assertion == nil || assertion.Type != nil {
		l.errorAt(statement, "type switch must use a static interface or type-parameter value")
		return
	}
	value := l.expr(assertion.X)
	if value.interface_ != nil {
		variant := value.interface_
		exit, invalid := l.newBlock(), l.newBlock()
		restoreLabel := l.pushLabel(label, exit, nil)
		defer restoreLabel()
		blocks := make([]*ir.Block, len(variant.alternatives))
		cases := make([]ir.SwitchCase, len(blocks))
		for index := range blocks {
			blocks[index] = l.newBlock()
			cases[index] = ir.SwitchCase{Value: float64(index), Target: blocks[index].ID}
		}
		_ = l.builder.Switch(variant.tag.slots[0], cases, invalid)
		for index, alternative := range variant.alternatives {
			l.setCurrent(blocks[index])
			var selected, fallback *ast.CaseClause
			for _, item := range statement.Body.List {
				clause := item.(*ast.CaseClause)
				if len(clause.List) == 0 {
					fallback = clause
					continue
				}
				for _, expression := range clause.List {
					target := l.resolveType(l.pkg.TypesInfo.TypeOf(expression))
					if target != nil && (types.AssignableTo(alternative.type_, target) || types.Identical(alternative.type_, target)) {
						selected = clause
						break
					}
				}
				if selected != nil {
					break
				}
			}
			if selected == nil {
				selected = fallback
			}
			if selected != nil {
				frame := l.frames[len(l.frames)-1]
				if implicit := l.pkg.TypesInfo.Implicits[selected]; implicit != nil {
					frame.vars[implicit] = alternative
				}
				l.breaks = append(l.breaks, exit)
				l.dynamic(func() { l.stmts(selected.Body) })
				l.breaks = l.breaks[:len(l.breaks)-1]
			}
			l.jump(exit)
		}
		l.setCurrent(invalid)
		if variant.persistent {
			var fallback *ast.CaseClause
			for _, item := range statement.Body.List {
				clause := item.(*ast.CaseClause)
				if len(clause.List) == 0 {
					fallback = clause
					break
				}
			}
			if fallback != nil {
				frame := l.frames[len(l.frames)-1]
				if implicit := l.pkg.TypesInfo.Implicits[fallback]; implicit != nil {
					frame.vars[implicit] = value
				}
				l.breaks = append(l.breaks, exit)
				l.dynamic(func() { l.stmts(fallback.Body) })
				l.breaks = l.breaks[:len(l.breaks)-1]
			}
			l.jump(exit)
		} else {
			_ = l.builder.MarkUnreachable()
		}
		l.setCurrent(exit)
		return
	}
	var selected, fallback *ast.CaseClause
	for _, item := range statement.Body.List {
		clause := item.(*ast.CaseClause)
		if len(clause.List) == 0 {
			fallback = clause
			continue
		}
		for _, expression := range clause.List {
			target := l.resolveType(l.pkg.TypesInfo.TypeOf(expression))
			if target != nil && value.type_ != nil && (types.AssignableTo(value.type_, target) || types.Identical(value.type_, target)) {
				selected = clause
				break
			}
		}
		if selected != nil {
			break
		}
	}
	if selected == nil {
		selected = fallback
	}
	if selected == nil {
		return
	}
	frame := l.frames[len(l.frames)-1]
	if implicit := l.pkg.TypesInfo.Implicits[selected]; implicit != nil {
		frame.vars[implicit] = value
	}
	exit := l.newBlock()
	restoreLabel := l.pushLabel(label, exit, nil)
	defer restoreLabel()
	l.breaks = append(l.breaks, exit)
	l.stmts(selected.Body)
	l.breaks = l.breaks[:len(l.breaks)-1]
	l.jump(exit)
	l.setCurrent(exit)
}

func staticSwitchClause(pkg *packages.Package, n *ast.SwitchStmt) (*ast.CaseClause, bool) {
	for _, raw := range n.Body.List {
		clause := raw.(*ast.CaseClause)
		if len(clause.Body) != 0 {
			if branch, ok := clause.Body[len(clause.Body)-1].(*ast.BranchStmt); ok && branch.Tok == token.FALLTHROUGH {
				return nil, false
			}
		}
	}
	var tag constant.Value
	if n.Tag != nil {
		tag = pkg.TypesInfo.Types[n.Tag].Value
		if tag == nil {
			return nil, false
		}
	}
	var fallback *ast.CaseClause
	for _, raw := range n.Body.List {
		clause := raw.(*ast.CaseClause)
		if len(clause.List) == 0 {
			fallback = clause
			continue
		}
		for _, expression := range clause.List {
			value := pkg.TypesInfo.Types[expression].Value
			if value == nil {
				return nil, false
			}
			if tag == nil {
				if value.Kind() != constant.Bool {
					return nil, false
				}
				if constant.BoolVal(value) {
					return clause, true
				}
			} else if constant.Compare(tag, token.EQL, value) {
				return clause, true
			}
		}
	}
	return fallback, true
}

func (l *lowerer) switchStmt(n *ast.SwitchStmt, label string) {
	if n.Init != nil {
		l.stmt(n.Init)
	}
	if clause, ok := staticSwitchClause(l.pkg, n); ok {
		if clause == nil {
			return
		}
		exit := l.newBlock()
		restoreLabel := l.pushLabel(label, exit, nil)
		l.breaks = append(l.breaks, exit)
		l.stmts(clause.Body)
		l.jump(exit)
		l.breaks = l.breaks[:len(l.breaks)-1]
		restoreLabel()
		l.setCurrent(exit)
		return
	}
	allConstant := n.Tag != nil
	for _, raw := range n.Body.List {
		for _, expression := range raw.(*ast.CaseClause).List {
			if l.pkg.TypesInfo.Types[expression].Value == nil {
				allConstant = false
			}
		}
	}
	if !allConstant {
		l.dynamicSwitchStmt(n, label)
		return
	}
	value := l.materialize("switch.tag", l.expr(n.Tag), n.Tag)
	exit := l.newBlock()
	term := ir.Switch{Value: value.slots[0], Default: exit.ID}
	caseBlocks := make([]*ir.Block, len(n.Body.List))
	for i, raw := range n.Body.List {
		clause := raw.(*ast.CaseClause)
		caseBlocks[i] = l.newBlock()
		if clause.List == nil {
			term.Default = caseBlocks[i].ID
			continue
		}
		for _, expression := range clause.List {
			caseValue := l.expr(expression)
			if len(caseValue.slots) != 1 {
				l.errorAt(expression, "switch case must be scalar")
				continue
			}
			constantValue, ok := caseValue.slots[0].(ir.Const)
			if !ok {
				l.errorAt(expression, "switch case must be constant")
				continue
			}
			term.Cases = append(term.Cases, ir.SwitchCase{Value: constantValue.Value, Target: caseBlocks[i].ID})
		}
	}
	defaultBlock := exit
	for _, block := range caseBlocks {
		if block.ID == term.Default {
			defaultBlock = block
			break
		}
	}
	_ = l.builder.Switch(term.Value, term.Cases, defaultBlock)
	restoreLabel := l.pushLabel(label, exit, nil)
	defer restoreLabel()
	l.breaks = append(l.breaks, exit)
	for i, raw := range n.Body.List {
		l.setCurrent(caseBlocks[i])
		var fallthroughTarget *ir.Block
		if i+1 < len(caseBlocks) {
			fallthroughTarget = caseBlocks[i+1]
		}
		l.fallthroughs = append(l.fallthroughs, fallthroughTarget)
		l.dynamic(func() { l.stmts(raw.(*ast.CaseClause).Body) })
		l.fallthroughs = l.fallthroughs[:len(l.fallthroughs)-1]
		l.jump(exit)
	}
	l.breaks = l.breaks[:len(l.breaks)-1]
	l.setCurrent(exit)
}

func (l *lowerer) dynamicSwitchStmt(n *ast.SwitchStmt, label string) {
	exit := l.newBlock()
	restoreLabel := l.pushLabel(label, exit, nil)
	defer restoreLabel()
	caseBlocks := make([]*ir.Block, len(n.Body.List))
	var defaultBlock *ir.Block
	for i, raw := range n.Body.List {
		caseBlocks[i] = l.newBlock()
		if raw.(*ast.CaseClause).List == nil {
			defaultBlock = caseBlocks[i]
		}
	}
	if defaultBlock == nil {
		defaultBlock = exit
	}
	var tag lowerValue
	if n.Tag != nil {
		tag = l.materialize("switch.tag", l.expr(n.Tag), n.Tag)
	}
	for i, raw := range n.Body.List {
		clause := raw.(*ast.CaseClause)
		if clause.List == nil {
			continue
		}
		for _, expression := range clause.List {
			condition := l.expr(expression)
			if n.Tag != nil {
				if len(tag.slots) != 1 || len(condition.slots) != 1 {
					l.errorAt(expression, "switch comparison requires scalar values")
					continue
				}
				condition = lowerValue{type_: types.Typ[types.Bool], slots: []ir.Expr{ir.RuntimeCall{Function: resource.RuntimeFunctionEqual, Args: []ir.Expr{tag.slots[0], condition.slots[0]}, Result: irTypeOf(types.Typ[types.Bool]), Pure: true, Pos: sourcePos(l.pkg, expression.Pos())}}}
			}
			next := l.newBlock()
			_ = l.builder.Branch(condition.slots[0], caseBlocks[i], next)
			l.setCurrent(next)
		}
	}
	l.jump(defaultBlock)
	l.breaks = append(l.breaks, exit)
	for i, raw := range n.Body.List {
		l.setCurrent(caseBlocks[i])
		var fallthroughTarget *ir.Block
		if i+1 < len(caseBlocks) {
			fallthroughTarget = caseBlocks[i+1]
		}
		l.fallthroughs = append(l.fallthroughs, fallthroughTarget)
		l.dynamic(func() { l.stmts(raw.(*ast.CaseClause).Body) })
		l.fallthroughs = l.fallthroughs[:len(l.fallthroughs)-1]
		l.jump(exit)
	}
	l.breaks = l.breaks[:len(l.breaks)-1]
	l.setCurrent(exit)
}

func (l *lowerer) stmts(stmts []ast.Stmt) {
	for _, stmt := range stmts {
		l.stmt(stmt)
	}
}

func (l *lowerer) assign(n *ast.AssignStmt) {
	if len(n.Lhs) == 1 && len(n.Rhs) == 1 {
		if _, callableArray := callableArrayType(l.resolveType(l.pkg.TypesInfo.TypeOf(n.Lhs[0]))); callableArray {
			identifier, ok := n.Lhs[0].(*ast.Ident)
			if !ok {
				l.errorAt(n.Lhs[0], "whole callable arrays can only be assigned to local variables")
				return
			}
			source := l.expr(n.Rhs[0])
			if source.callableArray == nil {
				l.errorAt(n.Rhs[0], "callable array assignment requires a statically finite fixed array")
				return
			}
			switch n.Tok {
			case token.DEFINE:
				object := l.pkg.TypesInfo.Defs[identifier]
				if object == nil {
					l.errorAt(identifier, "callable array declaration has no object identity")
					return
				}
				l.bind(object, l.copyCallableArray(identifier.Name, source, n))
			case token.ASSIGN:
				destination := l.expr(n.Lhs[0])
				l.storeCallableArray(destination, source, n)
			default:
				l.errorAt(n, "unsupported callable array assignment %s", n.Tok)
			}
			return
		}
	}
	if len(n.Lhs) == 1 && len(n.Rhs) == 1 && isFunctionType(l.pkg.TypesInfo.TypeOf(n.Lhs[0])) {
		if indexed, ok := n.Lhs[0].(*ast.IndexExpr); ok {
			array := l.expr(indexed.X)
			index := l.expr(indexed.Index)
			callable, callableOK := l.staticCallable(n.Rhs[0])
			if !callableOK {
				l.errorAt(n.Rhs[0], "function assignment requires a statically finite callable target")
				return
			}
			if n.Tok != token.ASSIGN {
				l.errorAt(n, "callable array elements only support ordinary assignment")
				return
			}
			l.storeCallableArrayElement(array, index, callable, n)
			return
		}
		identifier, ok := n.Lhs[0].(*ast.Ident)
		if !ok {
			l.errorAt(n.Lhs[0], "static callable can only be stored in a local variable")
			return
		}
		callable, ok := l.staticCallable(n.Rhs[0])
		if !ok {
			l.errorAt(n.Rhs[0], "function variable initializer must have one statically known callable target")
			return
		}
		switch n.Tok {
		case token.DEFINE:
			object := l.pkg.TypesInfo.Defs[identifier]
			if object == nil {
				l.errorAt(identifier, "static callable declaration has no object identity")
				return
			}
			l.bindCallable(object, l.callableBinding(object, identifier.Name, callable, n))
		case token.ASSIGN:
			object := l.pkg.TypesInfo.ObjectOf(identifier)
			existing, exists := l.lookupCallable(object)
			if !exists || existing.tag.type_ == nil {
				l.errorAt(n, "callable assignment target is not a mutable local or parameter")
				return
			}
			l.storeCallableCell(existing, callable, n)
		default:
			l.errorAt(n, "unsupported callable assignment %s", n.Tok)
		}
		return
	}
	compound := map[token.Token]resource.RuntimeFunction{
		token.ADD_ASSIGN: resource.RuntimeFunctionAdd, token.SUB_ASSIGN: resource.RuntimeFunctionSubtract,
		token.MUL_ASSIGN: resource.RuntimeFunctionMultiply, token.QUO_ASSIGN: resource.RuntimeFunctionDivide,
		token.REM_ASSIGN: resource.RuntimeFunctionRem,
	}
	if op, ok := compound[n.Tok]; ok {
		if len(n.Lhs) != 1 || len(n.Rhs) != 1 {
			l.errorAt(n, "compound assignment requires one value")
			return
		}
		dst := l.ensureAssignable(l.expr(n.Lhs[0]), n.Lhs[0])
		rhs := l.materialize("compound.right", l.expr(n.Rhs[0]), n.Rhs[0])
		if len(dst.slots) != 1 || len(rhs.slots) != 1 {
			l.errorAt(n, "compound assignment requires scalar values")
			return
		}
		if (n.Tok == token.QUO_ASSIGN || n.Tok == token.REM_ASSIGN) && !l.requireIntegerDivisor(n, rhs.slots[0], dst.type_, n.Tok) {
			return
		}
		result := ir.Expr(ir.RuntimeCall{Function: op, Args: []ir.Expr{dst.slots[0], rhs.slots[0]}, Result: irTypeOf(dst.type_), Pure: true, Pos: sourcePos(l.pkg, n.Pos())})
		if n.Tok == token.QUO_ASSIGN {
			if kind, supported := runtimeBasicKind(dst.type_); !supported {
				l.errorAt(n, "runtime arithmetic does not support %s", dst.type_)
				return
			} else if kind == types.Int {
				result = ir.RuntimeCall{Function: resource.RuntimeFunctionTrunc, Args: []ir.Expr{result}, Result: irTypeOf(dst.type_), Pure: true, Pos: sourcePos(l.pkg, n.Pos())}
			}
		}
		value := lowerValue{type_: dst.type_, slots: []ir.Expr{result}}
		l.store(dst, value, n)
		return
	}
	destinations := make([]lowerValue, len(n.Lhs))
	skipStore := make([]bool, len(n.Lhs))
	newObjects := make([]types.Object, len(n.Lhs))
	for i, lhs := range n.Lhs {
		if id, ok := lhs.(*ast.Ident); ok && id.Name == "_" {
			skipStore[i] = true
			continue
		}
		if id, ok := lhs.(*ast.Ident); ok && n.Tok == token.DEFINE {
			obj := l.pkg.TypesInfo.Defs[id]
			if obj == nil {
				destinations[i] = l.expr(id)
				continue
			}
			newObjects[i] = obj
		} else if _, ok := lhs.(*ast.Ident); !ok {
			destinations[i] = l.expr(lhs)
		} else {
			// Identifier destinations have no evaluation side effects. Resolve
			// them after the RHS so container assignments can rebind backing.
		}
	}
	var values []lowerValue
	if len(n.Lhs) == 2 && len(n.Rhs) == 1 {
		if assertion, ok := n.Rhs[0].(*ast.TypeAssertExpr); ok && assertion.Type != nil {
			source := l.expr(assertion.X)
			target := l.resolveType(l.pkg.TypesInfo.TypeOf(assertion.Type))
			if source.interface_ != nil {
				value, present := l.interfaceAssertion(assertion, source, target, true)
				values = []lowerValue{l.materialize("assignment.rhs", value, assertion), present}
			} else {
				matches := source.type_ != nil && target != nil && (types.AssignableTo(source.type_, target) || types.Identical(source.type_, target))
				value := zeroValue(target)
				if matches {
					value = source
					value.type_ = target
				}
				boolean := 0.0
				if matches {
					boolean = 1
				}
				values = []lowerValue{l.materialize("assignment.rhs", value, assertion), {type_: types.Typ[types.Bool], slots: []ir.Expr{ir.Const{Value: boolean}}}}
			}
		}
	}
	if values == nil {
		values = make([]lowerValue, len(n.Rhs))
		for i, rhs := range n.Rhs {
			values[i] = l.expr(rhs)
			if values[i].variadic != nil {
				l.errorAt(rhs, "variadic helper parameters cannot be assigned or stored")
				return
			}
			if len(n.Lhs) > 1 {
				values[i] = l.materialize("assignment.rhs", values[i], rhs)
			}
		}
	}
	if len(values) == 1 && len(n.Lhs) > 1 {
		combined := values[0]
		if len(combined.multi) != 0 {
			if len(combined.multi) != len(n.Lhs) {
				l.errorAt(n, "multiple assignment received %d descriptor values for %d targets", len(combined.multi), len(n.Lhs))
				return
			}
			values = append([]lowerValue(nil), combined.multi...)
		} else {
			values = make([]lowerValue, len(n.Lhs))
			offset := 0
			for i, lhs := range n.Lhs {
				typ := l.resolveType(l.pkg.TypesInfo.TypeOf(lhs))
				size := l.runtimeTypeOf(typ).Slots
				if offset+size > len(combined.slots) {
					l.errorAt(n, "internal type-layout inconsistency while expanding multiple assignment: result has %d slots, target %d requires slots [%d:%d]", len(combined.slots), i+1, offset, offset+size)
					return
				}
				values[i] = lowerValue{type_: typ, slots: combined.slots[offset : offset+size]}
				offset += size
			}
			if offset != len(combined.slots) {
				l.errorAt(n, "internal type-layout inconsistency while expanding multiple assignment: consumed %d of %d result slots", offset, len(combined.slots))
				return
			}
		}
	}
	if len(values) != len(n.Lhs) {
		l.errorAt(n, "multiple assignment requires one value per target or one statically typed multi-value result")
		return
	}
	for i, object := range newObjects {
		if object == nil {
			continue
		}
		objectType := l.resolveType(object.Type())
		if values[i].interface_ != nil {
			values[i].type_ = objectType
			l.bind(object, values[i])
			skipStore[i] = true
			continue
		}
		if isInterfaceType(objectType) {
			identifier := n.Lhs[i].(*ast.Ident)
			value := l.storeInterfaceValue(l.newInterfaceValue(identifier.Name+".interface", objectType, identifier), values[i], identifier)
			l.bind(object, value)
			skipStore[i] = true
			continue
		}
		if l.isAggregatePointerType(objectType) && (isAggregatePointerValue(values[i]) || values[i].nilPointer) {
			frame := l.frames[len(l.frames)-1]
			values[i].type_ = objectType
			if frame.reboundValues[object] || values[i].aggregatePointer != nil {
				identifier := n.Lhs[i].(*ast.Ident)
				l.bind(object, l.copyAggregatePointerValue(identifier.Name, values[i], identifier))
			} else {
				l.bind(object, values[i])
			}
			skipStore[i] = true
			continue
		}
		if isStaticPointer(values[i]) {
			values[i].type_ = objectType
			identifier := n.Lhs[i].(*ast.Ident)
			l.bind(object, l.copyPointerValue(identifier.Name, values[i], identifier))
			skipStore[i] = true
			continue
		}
		if values[i].entity != nil {
			identifier := n.Lhs[i].(*ast.Ident)
			if !types.Identical(objectType, values[i].type_) {
				l.errorAt(identifier, "EntityRef.Get views cannot be stored in interfaces or converted variables")
				skipStore[i] = true
				continue
			}
			l.bind(object, l.allocEntityView(identifier.Name, values[i], identifier))
			skipStore[i] = true
			continue
		}
		if isContainerValue(values[i]) {
			identifier := n.Lhs[i].(*ast.Ident)
			l.bind(object, l.copyContainerValue(identifier.Name, values[i], identifier))
			skipStore[i] = true
			continue
		}
		if values[i].aggregate != nil {
			identifier := n.Lhs[i].(*ast.Ident)
			l.bind(object, l.copyAggregate(identifier.Name, values[i], identifier))
			skipStore[i] = true
			continue
		}
		identifier := n.Lhs[i].(*ast.Ident)
		destination := l.alloc(identifier.Name, objectType)
		l.bind(object, destination)
		destinations[i] = destination
	}
	for i, lhs := range n.Lhs {
		if skipStore[i] {
			continue
		}
		if identifier, ok := lhs.(*ast.Ident); ok {
			object := l.pkg.TypesInfo.ObjectOf(identifier)
			if object == nil {
				object = l.pkg.TypesInfo.Defs[identifier]
			}
			if object != nil && isInterfaceType(object.Type()) {
				existing, exists := l.lookup(object)
				if !exists || existing.interface_ == nil {
					existing = l.newInterfaceValue(identifier.Name+".interface", object.Type(), identifier)
				}
				existing = l.storeInterfaceValue(existing, values[i], lhs)
				l.rebind(object, existing)
				continue
			}
		}
		if values[i].interface_ != nil {
			if destinations[i].aggregateLoad != nil {
				l.storeAggregatePointerLoad(destinations[i].aggregateLoad, values[i], lhs)
				continue
			}
			if destinations[i].aggregateIndex != nil {
				l.storeAggregateIndex(destinations[i].aggregateIndex, values[i], lhs)
				continue
			}
			if destinations[i].interface_ != nil {
				l.storeInterfaceValue(destinations[i], values[i], lhs)
				continue
			}
			identifier, ok := lhs.(*ast.Ident)
			if !ok {
				l.errorAt(lhs, "finite static interface values can only be assigned to local variables")
				continue
			}
			object := l.pkg.TypesInfo.ObjectOf(identifier)
			if object == nil {
				object = l.pkg.TypesInfo.Defs[identifier]
			}
			if object == nil || !isInterfaceType(object.Type()) {
				l.errorAt(lhs, "finite static interface assignment requires an interface local")
				continue
			}
			values[i].type_ = object.Type()
			l.rebind(object, values[i])
			continue
		}
		if identifier, ok := lhs.(*ast.Ident); ok && destinations[i].aggregate == nil && destinations[i].aggregatePointer == nil {
			object := l.pkg.TypesInfo.ObjectOf(identifier)
			if object == nil {
				object = l.pkg.TypesInfo.Defs[identifier]
			}
			if existing, exists := l.lookup(object); exists {
				if existing.aggregate != nil || existing.aggregatePointer != nil {
					destinations[i] = existing
				}
			}
		}
		if destinations[i].aggregateLoad != nil {
			l.storeAggregatePointerLoad(destinations[i].aggregateLoad, values[i], lhs)
			continue
		}
		if destinations[i].aggregateIndex != nil {
			l.storeAggregateIndex(destinations[i].aggregateIndex, values[i], lhs)
			continue
		}
		if destinations[i].aggregate != nil && values[i].aggregate != nil {
			l.storeAggregate(destinations[i], values[i], lhs)
			continue
		}
		if destinations[i].interface_ != nil {
			l.storeInterfaceValue(destinations[i], values[i], lhs)
			continue
		}
		if destinations[i].persistentPointer != nil && isStaticPointer(values[i]) {
			l.storePersistentPointer(destinations[i], values[i], lhs)
			continue
		}
		if destinations[i].aggregatePointer != nil && (isAggregatePointerValue(values[i]) || values[i].nilPointer) {
			l.mergeAggregatePointerValue(destinations[i], values[i], lhs)
			continue
		}
		if destinations[i].pointer != nil && isStaticPointer(values[i]) {
			l.mergePointerValue(destinations[i], values[i], lhs)
			continue
		}
		if destinations[i].containerVariant != nil && isContainerValue(values[i]) {
			l.mergeContainerValue(destinations[i], values[i], lhs)
			continue
		}
		if isStaticPointer(values[i]) {
			identifier, ok := lhs.(*ast.Ident)
			if !ok {
				l.errorAt(lhs, "pointer aliases can only be assigned to local variables")
				continue
			}
			object := l.pkg.TypesInfo.ObjectOf(identifier)
			if object == nil {
				object = l.pkg.TypesInfo.Defs[identifier]
			}
			existing, exists := l.lookup(object)
			if !exists || !isStaticPointer(existing) {
				l.errorAt(lhs, "pointer alias assignment target has no static address binding")
				continue
			}
			if existing.pointer != nil {
				if existing.pointer == values[i].pointer {
					continue
				}
				existing = l.mergePointerValue(existing, values[i], lhs)
				l.rebind(object, existing)
				continue
			}
			if l.dynamicDepth != 0 {
				if existing.pointer == values[i].pointer && existing.pointer != nil || existing.pointer == nil && values[i].pointer == nil && samePlaces(existing.places, values[i].places) {
					continue
				}
				existing = l.mergePointerValue(existing, values[i], lhs)
				l.rebind(object, existing)
				continue
			}
			l.rebind(object, l.copyPointerValue(identifier.Name, values[i], identifier))
			continue
		}
		if identifier, ok := lhs.(*ast.Ident); ok && values[i].entity == nil {
			if object := l.pkg.TypesInfo.ObjectOf(identifier); object != nil {
				if existing, exists := l.lookup(object); exists && existing.entity != nil {
					l.errorAt(lhs, "EntityRef.Get view locals can only be assigned another EntityRef.Get view")
					continue
				}
			}
		}
		if isContainerValue(values[i]) {
			identifier, ok := lhs.(*ast.Ident)
			if !ok {
				l.errorAt(lhs, "container values can only be assigned to local variables")
				continue
			}
			object := l.pkg.TypesInfo.ObjectOf(identifier)
			if object == nil {
				object = l.pkg.TypesInfo.Defs[identifier]
			}
			if object == nil {
				l.errorAt(lhs, "container assignment target has no object identity")
				continue
			}
			existing, exists := l.lookup(object)
			if !exists || !isContainerValue(existing) {
				l.errorAt(lhs, "container assignment target has no container binding")
				continue
			}
			if existing.containerVariant != nil {
				existing = l.mergeContainerValue(existing, values[i], lhs)
				l.rebind(object, existing)
				continue
			}
			l.rebind(object, l.copyContainerValue(identifier.Name, values[i], identifier))
			continue
		}
		if values[i].entity != nil {
			identifier, ok := lhs.(*ast.Ident)
			if !ok {
				destination := l.expr(lhs)
				if destination.entity == nil || destination.entity.binding.declaration != values[i].entity.binding.declaration {
					l.errorAt(lhs, "EntityRef.Get view storage must keep one static archetype target")
					continue
				}
				l.store(destination, values[i], lhs)
				continue
			}
			object := l.pkg.TypesInfo.ObjectOf(identifier)
			if object == nil {
				object = l.pkg.TypesInfo.Defs[identifier]
			}
			if object == nil {
				l.errorAt(lhs, "EntityRef.Get proxy assignment target has no object identity")
				continue
			}
			existing, exists := l.lookup(object)
			if !exists || existing.entity == nil {
				l.errorAt(lhs, "EntityRef.Get views can only be assigned to entity-view local variables")
				continue
			}
			l.storeEntityView(existing, values[i], lhs)
			continue
		}
		if len(destinations[i].places) == 0 {
			destinations[i] = l.expr(lhs)
		}
		if destinations[i].immutablePackage {
			l.errorAt(lhs, "package static values are immutable in callbacks")
			continue
		}
		if len(destinations[i].places) == 0 {
			if identifier, ok := lhs.(*ast.Ident); ok {
				object := l.pkg.TypesInfo.ObjectOf(identifier)
				if object == nil {
					object = l.pkg.TypesInfo.Defs[identifier]
				}
				if object != nil {
					if current, exists := l.lookup(object); exists && len(current.slots) != 0 {
						local := l.alloc(identifier.Name, object.Type())
						l.store(local, current, identifier)
						l.rebind(object, local)
						destinations[i] = local
					}
				}
			}
		}
		l.store(destinations[i], values[i], lhs)
	}
}

func identifiersAsExprs(ids []*ast.Ident) []ast.Expr {
	result := make([]ast.Expr, len(ids))
	for i, id := range ids {
		result[i] = id
	}
	return result
}

func (l *lowerer) ifStmt(n *ast.IfStmt) {
	if n.Init != nil {
		l.stmt(n.Init)
	}
	if condition, ok := constantBool(l.pkg, n.Cond); ok {
		if condition {
			l.stmts(n.Body.List)
		} else if n.Else != nil {
			l.stmt(n.Else)
		}
		return
	}
	cond := l.expr(n.Cond)
	if len(cond.slots) == 1 {
		if constant, ok := cond.slots[0].(ir.Const); ok {
			if constant.Value != 0 {
				l.stmts(n.Body.List)
			} else if n.Else != nil {
				l.stmt(n.Else)
			}
			return
		}
	}
	thenBlock, elseBlock, merge := l.newBlock(), l.newBlock(), l.newBlock()
	_ = l.builder.Branch(cond.slots[0], thenBlock, elseBlock)
	l.setCurrent(thenBlock)
	l.dynamic(func() { l.stmts(n.Body.List) })
	l.jump(merge)
	l.setCurrent(elseBlock)
	if n.Else != nil {
		l.dynamic(func() { l.stmt(n.Else) })
	}
	l.jump(merge)
	l.setCurrent(merge)
}

func (l *lowerer) forStmt(n *ast.ForStmt, label string) {
	if n.Init != nil {
		l.stmt(n.Init)
	}
	if n.Cond != nil {
		if condition, ok := constantBool(l.pkg, n.Cond); ok && !condition {
			return
		}
	}
	header, body, post, exit := l.newBlock(), l.newBlock(), l.newBlock(), l.newBlock()
	restoreLabel := l.pushLabel(label, exit, post)
	defer restoreLabel()
	l.jump(header)
	l.setCurrent(header)
	if n.Cond == nil {
		l.jump(body)
	} else {
		c := l.expr(n.Cond)
		_ = l.builder.Branch(c.slots[0], body, exit)
	}
	l.breaks = append(l.breaks, exit)
	l.continues = append(l.continues, post)
	l.setCurrent(body)
	l.dynamic(func() { l.stmts(n.Body.List) })
	l.jump(post)
	l.setCurrent(post)
	if n.Post != nil {
		l.dynamic(func() { l.stmt(n.Post) })
	}
	l.jump(header)
	l.breaks = l.breaks[:len(l.breaks)-1]
	l.continues = l.continues[:len(l.continues)-1]
	l.setCurrent(exit)
}

func (l *lowerer) rangeStmt(n *ast.RangeStmt, label string) {
	if signature, ok := types.Unalias(l.pkg.TypesInfo.TypeOf(n.X)).Underlying().(*types.Signature); ok && signature.Params().Len() == 1 && signature.Results().Len() == 0 {
		yieldSignature, yieldOK := types.Unalias(signature.Params().At(0).Type()).Underlying().(*types.Signature)
		if yieldOK && yieldSignature.Results().Len() == 1 {
			if resultKind, supported := runtimeBasicKind(yieldSignature.Results().At(0).Type()); supported && resultKind == types.Bool && yieldSignature.Params().Len() <= 2 {
				callable, callableOK := l.staticCallable(n.X)
				if !callableOK {
					l.errorAt(n.X, "range-over-func requires one statically known sequence target")
					return
				}
				call := &ast.CallExpr{Fun: n.X}
				owner := l.frames[len(l.frames)-1]
				active := l.allocZeroed("range.yield.active", types.Typ[types.Bool], n)
				l.store(active, lowerValue{type_: types.Typ[types.Bool], slots: []ir.Expr{ir.Const{Value: 1}}}, n)
				l.inlineStaticCallable(call, callable, []callArgument{{callable: &staticCallable{yield: &rangeYield{statement: n, owner: owner, active: active, label: label}}}})
				return
			}
		}
	}
	collection := l.expr(n.X)
	if collection.type_ == nil {
		return
	}
	var pointerArray *types.Array
	var pointerCollection lowerValue
	if pointer, ok := types.Unalias(collection.type_).(*types.Pointer); ok {
		if array, arrayOK := types.Unalias(pointer.Elem()).Underlying().(*types.Array); arrayOK {
			pointerArray = array
			pointerCollection = collection
		}
	}
	if collection.container == nil && collection.containerVariant != nil {
		slice, ok := types.Unalias(collection.type_).Underlying().(*types.Slice)
		if !ok {
			l.errorAt(n.X, "finite container variant has no slice element layout")
			return
		}
		callables := make([]*staticCallable, len(collection.containerVariant.alternatives))
		for index, alternative := range collection.containerVariant.alternatives {
			iterator := &containerIterator{receiver: alternative, index: true}
			if n.Value != nil {
				iterator.offsets = []int{0}
				iterator.types = []types.Type{slice.Elem()}
			}
			callables[index] = &staticCallable{iterator: iterator}
		}
		frozenTag := l.materialize("range.container.tag", collection.containerVariant.tag, n)
		callable := indexedCallableVariant(l, callables, frozenTag, n)
		call := &ast.CallExpr{Fun: n.X}
		owner := l.frames[len(l.frames)-1]
		active := l.allocZeroed("range.yield.active", types.Typ[types.Bool], n)
		l.store(active, scalarValue(ir.Const{Value: 1}, types.Typ[types.Bool]), n)
		l.inlineStaticCallable(call, callable, []callArgument{{callable: &staticCallable{yield: &rangeYield{statement: n, owner: owner, active: active, label: label}}}})
		return
	}
	if basic, ok := types.Unalias(collection.type_).Underlying().(*types.Basic); ok && basic.Kind() == types.Int {
		l.integerRangeStmt(n, collection, label)
		return
	}
	array, ok := types.Unalias(collection.type_).Underlying().(*types.Array)
	if pointerArray != nil {
		array, ok = pointerArray, true
	}
	if !ok && collection.container == nil && collection.variadic == nil {
		l.errorAt(n.X, "range is only supported for int values, fixed arrays or pointers to fixed arrays, variadic parameters, and catalog containers")
		return
	}
	var length ir.Expr
	var elementType types.Type
	arrayLength := 0
	if ok {
		if collection.callableArray != nil && !blankExpr(n.Value) {
			collection = l.copyCallableArray("range.callable", collection, n.X)
		} else if collection.aggregate != nil && pointerArray == nil && !blankExpr(n.Value) {
			collection = l.copyAggregate("range.aggregate", collection, n.X)
		} else if pointerArray == nil && !blankExpr(n.Value) {
			snapshot := l.alloc("range.array", collection.type_)
			l.store(snapshot, collection, n.X)
			collection = snapshot
		}
		length = ir.Const{Value: float64(array.Len())}
		elementType = array.Elem()
		arrayLength = int(array.Len())
	} else if collection.variadic != nil {
		length = ir.Const{Value: float64(len(collection.variadic.elements))}
		elementType = collection.variadic.element
		arrayLength = len(collection.variadic.elements)
		if arrayLength > 0 && !isFunctionType(elementType) {
			size := l.runtimeTypeOf(types.NewArray(elementType, 1)).Slots
			backing := l.builder.NewLocal("range.variadic", ir.Type{Name: collection.type_.String() + ".range", Slots: arrayLength * size})
			collection.places = ir.Places(backing)
			collection.slots = make([]ir.Expr, len(collection.places))
			for i, element := range collection.variadic.elements {
				start := i * size
				if err := l.builder.Store(collection.places[start:start+size], ir.Value{Type: irTypeOf(elementType), Slots: element.slots}, sourcePos(l.pkg, n.Pos())); err != nil {
					l.errorAt(n, "%v", err)
				}
			}
			for i, place := range collection.places {
				collection.slots[i] = ir.Load{Place: place}
			}
		}
	} else {
		lengthValue := l.materialize("range.length", lowerValue{type_: types.Typ[types.Int], slots: []ir.Expr{collection.slots[0]}}, n.X)
		length = lengthValue.slots[0]
		slice, sliceOK := types.Unalias(collection.type_).Underlying().(*types.Slice)
		if !sliceOK {
			l.errorAt(n.X, "catalog container has no slice element layout")
			return
		}
		elementType = slice.Elem()
	}
	index := l.alloc("range.index", types.Typ[types.Int])
	l.store(index, lowerValue{type_: types.Typ[types.Int], slots: []ir.Expr{ir.Const{}}}, n)
	header, body, post, exit := l.newBlock(), l.newBlock(), l.newBlock(), l.newBlock()
	restoreLabel := l.pushLabel(label, exit, post)
	defer restoreLabel()
	l.jump(header)
	l.setCurrent(header)
	condition := ir.RuntimeCall{Function: resource.RuntimeFunctionLess, Args: []ir.Expr{index.slots[0], length}, Result: irTypeOf(types.Typ[types.Bool]), Pure: true, Pos: sourcePos(l.pkg, n.Pos())}
	_ = l.builder.Branch(condition, body, exit)
	l.setCurrent(body)
	if !blankExpr(n.Key) {
		l.assignRangeValue(n.Key, n.Tok, index)
	}
	if !blankExpr(n.Value) {
		size := l.runtimeTypeOf(types.NewArray(elementType, 1)).Slots
		var element lowerValue
		if collection.callableArray != nil {
			element = lowerValue{type_: elementType, callable: indexedCallableVariant(l, collection.callableArray.elements, index, n)}
		} else if collection.aggregate != nil {
			element = l.indexAggregateArray(collection, array, index, n, n.X)
		} else if collection.variadic != nil && isFunctionType(elementType) {
			element = lowerValue{type_: elementType, callable: indexedCallableVariant(l, collection.variadic.callables, index, n)}
		} else if collection.variadic != nil && arrayLength == 0 {
			element = zeroValue(elementType)
		} else if pointerArray != nil {
			pointer, _ := types.Unalias(pointerCollection.type_).(*types.Pointer)
			element = l.loadPointerIndex(pointerCollection, pointer, pointerArray, index, n)
		} else if size == 0 {
			element = l.zeroRuntimeValue(elementType)
		} else if collection.container != nil {
			element = l.containerElement(n, collection.container, index.slots[0], 0, size, elementType)
		} else {
			element = lowerValue{type_: elementType, slots: make([]ir.Expr, size), places: make([]ir.Place, size)}
			if len(collection.places) == 0 {
				l.errorAt(n.X, "ranged array must be addressable")
				return
			}
			switch first := collection.places[0].(type) {
			case ir.LocalPlace:
				for offset := 0; offset < size; offset++ {
					place := l.indexedLocal(first, index.slots[0], arrayLength, size, offset, n)
					element.places[offset], element.slots[offset] = place, ir.Load{Place: place}
				}
			case ir.MemoryPlace:
				baseIndex := first.Index
				if _, constant := baseIndex.(ir.Const); !constant {
					baseIndex = l.materialize("range.memory.index", scalarValue(baseIndex, types.Typ[types.Int]), n.X).slots[0]
				}
				baseAddress := baseIndex
				if first.Stride > 0 {
					baseAddress = l.pure(resource.RuntimeFunctionMultiply, n.X, baseAddress, ir.Const{Value: float64(first.Stride)})
				}
				if first.Offset != 0 {
					baseAddress = l.pure(resource.RuntimeFunctionAdd, n.X, baseAddress, ir.Const{Value: float64(first.Offset)})
				}
				elementOffset := l.pure(resource.RuntimeFunctionMultiply, n, index.slots[0], ir.Const{Value: float64(size)})
				address := l.pure(resource.RuntimeFunctionAdd, n, baseAddress, elementOffset)
				for offset := 0; offset < size; offset++ {
					place := l.memory(first.Storage, address, 0, offset, first.Read, first.Write, n)
					element.places[offset], element.slots[offset] = place, ir.Load{Place: place}
				}
			default:
				l.errorAt(n.X, "range does not support array place %T", collection.places[0])
				return
			}
		}
		l.assignRangeValue(n.Value, n.Tok, element)
	}
	l.breaks = append(l.breaks, exit)
	l.continues = append(l.continues, post)
	l.dynamic(func() { l.stmts(n.Body.List) })
	l.jump(post)
	l.setCurrent(post)
	next := lowerValue{type_: types.Typ[types.Int], slots: []ir.Expr{ir.RuntimeCall{Function: resource.RuntimeFunctionAdd, Args: []ir.Expr{index.slots[0], ir.Const{Value: 1}}, Result: irTypeOf(types.Typ[types.Int]), Pure: true, Pos: sourcePos(l.pkg, n.Pos())}}}
	l.store(index, next, n)
	l.jump(header)
	l.breaks = l.breaks[:len(l.breaks)-1]
	l.continues = l.continues[:len(l.continues)-1]
	l.setCurrent(exit)
}

func (l *lowerer) integerRangeStmt(n *ast.RangeStmt, length lowerValue, label string) {
	if !blankExpr(n.Value) {
		l.errorAt(n.Value, "integer range does not produce a second iteration value")
		return
	}
	length = l.materialize("range.length", length, n.X)
	index := l.alloc("range.index", types.Typ[types.Int])
	l.store(index, lowerValue{type_: types.Typ[types.Int], slots: []ir.Expr{ir.Const{}}}, n)
	header, body, post, exit := l.newBlock(), l.newBlock(), l.newBlock(), l.newBlock()
	restoreLabel := l.pushLabel(label, exit, post)
	defer restoreLabel()
	l.jump(header)
	l.setCurrent(header)
	condition := ir.RuntimeCall{Function: resource.RuntimeFunctionLess, Args: []ir.Expr{index.slots[0], length.slots[0]}, Result: irTypeOf(types.Typ[types.Bool]), Pure: true, Pos: sourcePos(l.pkg, n.Pos())}
	_ = l.builder.Branch(condition, body, exit)
	l.setCurrent(body)
	if !blankExpr(n.Key) {
		l.assignRangeValue(n.Key, n.Tok, index)
	}
	l.breaks = append(l.breaks, exit)
	l.continues = append(l.continues, post)
	l.dynamic(func() { l.stmts(n.Body.List) })
	l.jump(post)
	l.setCurrent(post)
	next := lowerValue{type_: types.Typ[types.Int], slots: []ir.Expr{ir.RuntimeCall{Function: resource.RuntimeFunctionAdd, Args: []ir.Expr{index.slots[0], ir.Const{Value: 1}}, Result: irTypeOf(types.Typ[types.Int]), Pure: true, Pos: sourcePos(l.pkg, n.Pos())}}}
	l.store(index, next, n)
	l.jump(header)
	l.breaks = l.breaks[:len(l.breaks)-1]
	l.continues = l.continues[:len(l.continues)-1]
	l.setCurrent(exit)
}

func blankExpr(expr ast.Expr) bool {
	identifier, ok := expr.(*ast.Ident)
	return expr == nil || (ok && identifier.Name == "_")
}

func (l *lowerer) assignRangeValue(target ast.Expr, op token.Token, value lowerValue) {
	if op != token.DEFINE {
		destination := l.expr(target)
		if destination.aggregateLoad != nil {
			l.storeAggregatePointerLoad(destination.aggregateLoad, value, target)
		} else if destination.aggregateIndex != nil {
			l.storeAggregateIndex(destination.aggregateIndex, value, target)
		} else if destination.aggregate != nil || destination.pointer != nil || destination.containerVariant != nil || value.aggregate != nil || isStaticPointer(value) || isContainerValue(value) {
			l.storeDescriptor(destination, value, target)
		} else {
			l.store(destination, value, target)
		}
		return
	}
	identifier, ok := target.(*ast.Ident)
	if !ok {
		l.errorAt(target, "range declaration target must be an identifier")
		return
	}
	object := l.pkg.TypesInfo.Defs[identifier]
	if object == nil {
		l.errorAt(target, "range declaration has no type object")
		return
	}
	if value.entity != nil {
		l.bind(object, l.allocEntityView(identifier.Name, value, identifier))
		return
	}
	if value.callable != nil {
		l.bindCallable(object, value.callable)
		return
	}
	if value.aggregate != nil {
		l.bind(object, l.copyAggregate(identifier.Name, value, identifier))
		return
	}
	if isStaticPointer(value) {
		l.bind(object, l.copyPointerValue(identifier.Name, value, identifier))
		return
	}
	if isContainerValue(value) {
		l.bind(object, l.copyContainerValue(identifier.Name, value, identifier))
		return
	}
	local := l.alloc(identifier.Name, object.Type())
	l.bind(object, local)
	l.store(local, value, target)
}

func lowerCallback(packagesByTypes map[*types.Package]*packages.Package, pkg *packages.Package, decl *ast.FuncDecl, fn *types.Func, fields []*FieldDeclaration, resources *ModeResources, configuration *ConfigurationDeclaration, levelGlobalFields map[*types.Var]*LevelGlobalFieldDeclaration, packageGlobals map[*source.StaticObject]*LevelGlobalFieldDeclaration, packagePointers map[*types.Var]*LevelGlobalFieldDeclaration, currentArchetype *ArchetypeDeclaration, archetypes map[*types.Named]archetypeBinding, m mode.Mode, phase string, checks RuntimeChecks) (*ir.Function, []error) {
	if decl == nil {
		return nil, nil
	}
	resultType := ir.Type{Name: "void"}
	sig := fn.Type().(*types.Signature)
	if sig.Results().Len() == 1 {
		resultType = irTypeOf(sig.Results().At(0).Type())
	}
	builder := ir.NewBuilder(fn.FullName(), resultType)
	resourceIDs := map[*types.Var][]int{}
	if resources != nil {
		resourceIDs = resources.FieldIDs
	}
	archetypeFields := map[*types.Var]*FieldDeclaration{}
	for _, field := range fields {
		archetypeFields[field.Object] = field
	}
	l := &lowerer{mode: m, phase: phase, checks: checks, packages: packagesByTypes, pkg: pkg, builder: builder, callStack: map[any]bool{fn: true}, resourceIDs: resourceIDs, streamSize: resources.StreamSize, configuration: configuration, levelGlobalFields: levelGlobalFields, packageGlobals: packageGlobals, packagePointers: packagePointers, currentArchetype: currentArchetype, archetypeFields: archetypeFields, archetypes: archetypes}
	entry := l.newBlock()
	_ = builder.SetEntry(entry)
	l.setCurrent(entry)
	l.initializePackageGlobals(decl)
	frame := &lowerFrame{pkg: pkg, vars: map[types.Object]lowerValue{}, callables: map[types.Object]*staticCallable{}, returnBlock: l.newBlock()}
	l.prepareGotoTargets(frame, decl.Body)
	l.prepareDeferredCalls(frame, decl.Body)
	l.prepareCallableMutability(frame, decl.Body)
	l.frames = append(l.frames, frame)
	if sig.Recv() != nil {
		recv := l.alloc("receiver", sig.Recv().Type())
		if len(fields) != 0 {
			recv.slots = recv.slots[:0]
			recv.places = recv.places[:0]
			for _, field := range fields {
				storage, read, write := archetypeStorageAccess(field.Storage, m, phase)
				for offset := 0; offset < field.Size; offset++ {
					place, err := builder.Memory(storage, ir.Const{}, 0, field.Offset+offset, read, write)
					if err != nil {
						l.errorAt(decl, "%v", err)
						continue
					}
					recv.places = append(recv.places, place)
					recv.slots = append(recv.slots, ir.Load{Place: place})
				}
			}
		}
		frame.vars[sig.Recv()] = recv
	}
	if sig.Results().Len() == 1 {
		frame.result = l.allocZeroed("result", sig.Results().At(0).Type(), decl)
	}
	l.stmts(decl.Body.List)
	l.jump(frame.returnBlock)
	l.setCurrent(frame.returnBlock)
	l.runDeferredCalls(frame)
	if frame.result.slots != nil {
		_ = builder.Return(ir.Value{Type: resultType, Slots: frame.result.slots})
	} else {
		_ = builder.Return(ir.Value{Type: resultType})
	}
	sort.SliceStable(l.errs, func(i, j int) bool { return l.errs[i].Error() < l.errs[j].Error() })
	if len(l.errs) > 0 {
		return nil, l.errs
	}
	result, err := builder.Finish()
	if err != nil {
		return nil, []error{fmt.Errorf("callback %s: %w", fn.FullName(), err)}
	}
	if runtimeErrs := validateRuntimeCalls(result); len(runtimeErrs) != 0 {
		for i := range runtimeErrs {
			runtimeErrs[i] = fmt.Errorf("callback %s: %w", fn.FullName(), runtimeErrs[i])
		}
		return nil, runtimeErrs
	}
	return result, nil
}

func archetypeStorageAccess(storage string, m mode.Mode, phase string) (string, bool, bool) {
	switch storage {
	case "imported", "data":
		return "data", true, phase == "preprocess"
	case "exported":
		return "exported", false, true
	case "shared":
		write := phase == "preprocess" || phase == "updateSequential"
		if m == mode.ModePlay {
			write = write || phase == "touch"
		}
		return "shared", true, write
	default:
		return storage, true, true
	}
}
