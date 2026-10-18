package frontend

import (
	"encoding/binary"
	"fmt"
	"go/ast"
	"go/types"
	"math"
	"sort"
	"strings"

	"github.com/WindowsSov8forUs/sonolus-core-go/core/resource"

	"github.com/WindowsSov8forUs/sonolus-go/v2/internal/compiler/ir"
	"github.com/WindowsSov8forUs/sonolus-go/v2/internal/compiler/mode"
)

// indexConstantROMArray indexes a non-addressable array of constants through
// EngineRom, as sonolus.py does for constant arrays with a non-constant index,
// instead of materialising a switch over every element (or a copy of the
// whole table) at each access site. EngineRom stores binary32, so a table is
// placed there only when every value is exactly representable; other tables
// keep the exact switch lowering.
func (l *lowerer) indexConstantROMArray(base lowerValue, array *types.Array, index lowerValue, node *ast.IndexExpr) (lowerValue, bool) {
	length := int(array.Len())
	elementSlots := l.runtimeTypeOf(types.NewArray(array.Elem(), 1)).Slots
	if len(base.places) != 0 || length < 2 || elementSlots == 0 || len(base.slots) != length*elementSlots {
		return lowerValue{}, false
	}
	if _, exists := l.entityBinding(array.Elem()); exists {
		return lowerValue{}, false
	}
	values := make([]float64, len(base.slots))
	for position, slot := range base.slots {
		constant, ok := slot.(ir.Const)
		if !ok || !float32Exact(constant.Value) {
			return lowerValue{}, false
		}
		values[position] = constant.Value
	}
	storage := ir.ConstantTableStorage(values)
	index = l.materialize("array.index", index, node.Index)
	inBounds := l.pure(resource.RuntimeFunctionAnd, node,
		l.pure(resource.RuntimeFunctionGreaterOr, node, index.slots[0], ir.Const{}),
		l.pure(resource.RuntimeFunctionLess, node, index.slots[0], ir.Const{Value: float64(length)}))
	l.guard(node, inBounds)
	result := lowerValue{type_: array.Elem(), slots: make([]ir.Expr, elementSlots), entityField: base.entityField, immutablePackage: base.immutablePackage}
	for slot := range result.slots {
		place := l.memory(storage, index.slots[0], elementSlots, slot, true, false, node)
		if place == nil {
			return zeroValue(array.Elem()), true
		}
		result.slots[slot] = ir.Load{Place: place}
	}
	return result, true
}

func float32Exact(value float64) bool {
	return math.IsNaN(value) || float64(float32(value)) == value
}

// PlaceConstantTables appends the constant tables still read by any callback
// after the user ROM (sorted by contents, each table once) and rewrites their
// placeholder places to EngineRom. It runs after optimization, which folds
// reads at constant indexes, and after the user or fallback ROM is final,
// because the table offsets follow it.
func PlaceConstantTables(project *Project) error {
	if project == nil {
		return fmt.Errorf("constant tables: project is nil")
	}
	tables := map[string]int{}
	visitProjectPlaces(project, func(place ir.MemoryPlace) ir.MemoryPlace {
		if strings.HasPrefix(place.Storage, ir.ConstantTablePrefix) {
			tables[place.Storage] = 0
		}
		return place
	})
	if len(tables) == 0 {
		return nil
	}
	if len(project.ROM)%4 != 0 {
		return fmt.Errorf("constant tables: ROM length %d is not divisible by 4", len(project.ROM))
	}
	keys := make([]string, 0, len(tables))
	for key := range tables {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	// buildROM prefixes NaN, +Inf and -Inf before the user ROM.
	const romPrefix = 3
	rom := append([]byte(nil), project.ROM...)
	for _, key := range keys {
		values, ok := ir.ConstantTable(key)
		if !ok {
			return fmt.Errorf("constant tables: invalid table storage %q", key)
		}
		tables[key] = romPrefix + len(rom)/4
		for _, value := range values {
			rom = binary.LittleEndian.AppendUint32(rom, math.Float32bits(float32(value)))
		}
	}
	project.ROM = rom
	visitProjectPlaces(project, func(place ir.MemoryPlace) ir.MemoryPlace {
		if offset, ok := tables[place.Storage]; ok {
			place.Storage = "EngineRom"
			place.Offset += offset
		}
		return place
	})
	return nil
}

// visitProjectPlaces rewrites every memory place in every callback of the
// project with rewrite.
func visitProjectPlaces(project *Project, rewrite func(ir.MemoryPlace) ir.MemoryPlace) {
	for _, currentMode := range []mode.Mode{mode.ModePlay, mode.ModeWatch, mode.ModePreview, mode.ModeTutorial} {
		declarations := project.Modes[currentMode]
		if declarations == nil {
			continue
		}
		for _, callback := range modeCallbacks(declarations) {
			if callback != nil && callback.IR != nil {
				rewriteFunctionPlaces(callback.IR, rewrite)
			}
		}
	}
}

func rewriteFunctionPlaces(function *ir.Function, rewrite func(ir.MemoryPlace) ir.MemoryPlace) {
	var expression func(ir.Expr) ir.Expr
	var place func(ir.Place) ir.Place
	place = func(value ir.Place) ir.Place {
		switch value := value.(type) {
		case ir.IndexedLocalPlace:
			value.Index = expression(value.Index)
			return value
		case ir.MemoryPlace:
			value.Index = expression(value.Index)
			return rewrite(value)
		default:
			return value
		}
	}
	expression = func(value ir.Expr) ir.Expr {
		switch value := value.(type) {
		case ir.Load:
			value.Place = place(value.Place)
			return value
		case ir.RuntimeCall:
			for index, argument := range value.Args {
				value.Args[index] = expression(argument)
			}
			return value
		default:
			return value
		}
	}
	for _, block := range function.Blocks {
		if block == nil {
			continue
		}
		for index, instruction := range block.Instructions {
			switch value := instruction.(type) {
			case ir.Store:
				value.Place = place(value.Place)
				value.Value = expression(value.Value)
				block.Instructions[index] = value
			case ir.Eval:
				value.Value = expression(value.Value)
				block.Instructions[index] = value
			}
		}
		switch value := block.Terminator.(type) {
		case ir.Branch:
			value.Condition = expression(value.Condition)
			block.Terminator = value
		case ir.Switch:
			value.Value = expression(value.Value)
			block.Terminator = value
		case ir.Return:
			for index, slot := range value.Value.Slots {
				value.Value.Slots[index] = expression(slot)
			}
			block.Terminator = value
		}
	}
}
