package ir

import "fmt"

type ssaDefinition struct {
	block int
	kind  uint8
}

type Validator struct {
	reachable         []bool
	queue             []int
	definitions       []ssaDefinition
	definitionSet     []bool
	sparseDefinitions map[int]ssaDefinition
	denseDefinitions  bool
}

func Validate(fn *Function) error {
	return new(Validator).Validate(fn)
}

func (validator *Validator) Validate(fn *Function) error {
	if fn == nil {
		return fmt.Errorf("function is nil")
	}
	if len(fn.Blocks) == 0 {
		return fmt.Errorf("function %s has no blocks", fn.Name)
	}
	if fn.Entry < 0 || fn.Entry >= len(fn.Blocks) {
		return fmt.Errorf("function %s has invalid entry block %d", fn.Name, fn.Entry)
	}
	if err := validateType(fn.Result); err != nil {
		return fmt.Errorf("function %s result: %w", fn.Name, err)
	}
	if cap(validator.reachable) < len(fn.Blocks) {
		validator.reachable = make([]bool, len(fn.Blocks))
	} else {
		validator.reachable = validator.reachable[:len(fn.Blocks)]
		clear(validator.reachable)
	}
	validator.queue = validator.queue[:0]
	if cap(validator.queue) < len(fn.Blocks) {
		validator.queue = make([]int, 0, len(fn.Blocks))
	}
	validator.queue = append(validator.queue, fn.Entry)
	reachable, queue := validator.reachable, validator.queue
	for head := 0; head < len(queue); head++ {
		id := queue[head]
		if id < 0 || id >= len(fn.Blocks) {
			return fmt.Errorf("block target %d does not exist", id)
		}
		if reachable[id] {
			continue
		}
		reachable[id] = true
		block := fn.Blocks[id]
		if block == nil {
			return fmt.Errorf("block %d is nil", id)
		}
		if block.ID != id {
			return fmt.Errorf("block index %d has ID %d", id, block.ID)
		}
		if block.Terminator == nil {
			return fmt.Errorf("reachable block %d has no terminator", id)
		}
		switch term := block.Terminator.(type) {
		case Jump:
			queue = append(queue, term.Target)
		case Branch:
			queue = append(queue, term.True, term.False)
		case Switch:
			queue = append(queue, term.Default)
			for _, item := range term.Cases {
				queue = append(queue, item.Target)
			}
		case Return:
		case Unreachable:
		default:
			return fmt.Errorf("block %d has unknown terminator %T", id, term)
		}
	}
	validator.queue = queue
	for id, typ := range fn.Locals {
		if err := validateType(typ); err != nil {
			return fmt.Errorf("local %d: %w", id, err)
		}
	}
	definitionCount, maxDefinitionID := 0, -1
	for _, block := range fn.Blocks {
		if block == nil {
			continue
		}
		for _, phi := range block.Phis {
			if phi.Target.ID >= 0 {
				definitionCount++
				maxDefinitionID = max(maxDefinitionID, phi.Target.ID)
			}
		}
		for _, instruction := range block.Instructions {
			if store, ok := instruction.(Store); ok {
				if place, ok := store.Place.(SSAPlace); ok && place.ID >= 0 {
					definitionCount++
					maxDefinitionID = max(maxDefinitionID, place.ID)
				}
			}
		}
	}
	validator.denseDefinitions = maxDefinitionID <= definitionCount*2+64
	if validator.denseDefinitions {
		length := maxDefinitionID + 1
		if cap(validator.definitions) < length {
			validator.definitions = make([]ssaDefinition, length)
			validator.definitionSet = make([]bool, length)
		} else {
			validator.definitions = validator.definitions[:length]
			validator.definitionSet = validator.definitionSet[:length]
			clear(validator.definitions)
			clear(validator.definitionSet)
		}
	} else {
		if validator.sparseDefinitions == nil {
			validator.sparseDefinitions = make(map[int]ssaDefinition, definitionCount)
		} else {
			clear(validator.sparseDefinitions)
		}
	}
	for _, block := range fn.Blocks {
		if block == nil {
			return fmt.Errorf("function %s contains a nil block", fn.Name)
		}
		if block.ID < 0 || block.ID >= len(fn.Blocks) || fn.Blocks[block.ID] != block {
			return fmt.Errorf("block has invalid ID %d", block.ID)
		}
		for _, phi := range block.Phis {
			if phi.Target.ID < 0 {
				return fmt.Errorf("block %d phi has invalid target", block.ID)
			}
			if previous, exists := validator.recordDefinition(phi.Target.ID, ssaDefinition{block: block.ID, kind: 1}); exists {
				return fmt.Errorf("block %d phi target %d is already defined by block %d %s", block.ID, phi.Target.ID, previous.block, ssaDefinitionKindName(previous.kind))
			}
			if err := validateLocalPlace(phi.Local, fn); err != nil {
				return fmt.Errorf("block %d phi local: %w", block.ID, err)
			}
			previousPredecessor := -1
			for _, arg := range phi.Args {
				if arg.Predecessor < 0 || arg.Predecessor >= len(fn.Blocks) || arg.Value.ID < 0 {
					return fmt.Errorf("block %d phi has invalid argument", block.ID)
				}
				if arg.Predecessor == previousPredecessor {
					return fmt.Errorf("block %d phi has duplicate predecessor %d", block.ID, arg.Predecessor)
				}
				if arg.Predecessor < previousPredecessor {
					return fmt.Errorf("block %d phi arguments are not ordered by predecessor", block.ID)
				}
				previousPredecessor = arg.Predecessor
				if !containsTarget(fn.Blocks[arg.Predecessor].Terminator, block.ID) {
					return fmt.Errorf("block %d phi argument predecessor %d has no edge to block", block.ID, arg.Predecessor)
				}
			}
		}
		for _, instruction := range block.Instructions {
			switch value := instruction.(type) {
			case Store:
				if place, ok := value.Place.(SSAPlace); ok {
					if place.ID < 0 {
						return fmt.Errorf("block %d store: SSA place has invalid ID %d", block.ID, place.ID)
					}
					if previous, exists := validator.recordDefinition(place.ID, ssaDefinition{block: block.ID, kind: 2}); exists {
						return fmt.Errorf("block %d SSA target %d is already defined by block %d %s", block.ID, place.ID, previous.block, ssaDefinitionKindName(previous.kind))
					}
				}
				if err := validatePlace(value.Place, fn); err != nil {
					return fmt.Errorf("block %d store: %w", block.ID, err)
				}
				if err := validateExpr(value.Value, fn); err != nil {
					return fmt.Errorf("block %d store value: %w", block.ID, err)
				}
			case Eval:
				call, ok := value.Value.(RuntimeCall)
				if !ok || call.Result.Slots != 0 {
					return fmt.Errorf("block %d eval requires a void runtime call", block.ID)
				}
				if err := validateRuntimeCall(call, fn); err != nil {
					return fmt.Errorf("block %d eval: %w", block.ID, err)
				}
			default:
				return fmt.Errorf("block %d has unknown instruction %T", block.ID, instruction)
			}
		}
		if block.Terminator != nil {
			if err := validateTerminator(block.Terminator, fn); err != nil {
				return fmt.Errorf("block %d terminator: %w", block.ID, err)
			}
		}
	}
	return nil
}

func (validator *Validator) recordDefinition(id int, definition ssaDefinition) (ssaDefinition, bool) {
	if validator.denseDefinitions {
		if validator.definitionSet[id] {
			return validator.definitions[id], true
		}
		validator.definitions[id], validator.definitionSet[id] = definition, true
		return ssaDefinition{}, false
	}
	previous, exists := validator.sparseDefinitions[id]
	if !exists {
		validator.sparseDefinitions[id] = definition
	}
	return previous, exists
}

func ssaDefinitionKindName(kind uint8) string {
	if kind == 1 {
		return "phi"
	}
	return "store"
}

func containsTarget(terminator Terminator, target int) bool {
	switch value := terminator.(type) {
	case Jump:
		return value.Target == target
	case Branch:
		return value.True == target || value.False == target
	case Switch:
		if value.Default == target {
			return true
		}
		for _, item := range value.Cases {
			if item.Target == target {
				return true
			}
		}
	}
	return false
}

func validateTerminator(term Terminator, fn *Function) error {
	target := func(id int) error {
		if id < 0 || id >= len(fn.Blocks) {
			return fmt.Errorf("block target %d does not exist", id)
		}
		return nil
	}
	switch value := term.(type) {
	case Jump:
		return target(value.Target)
	case Branch:
		if err := validateExpr(value.Condition, fn); err != nil {
			return err
		}
		if err := target(value.True); err != nil {
			return err
		}
		return target(value.False)
	case Switch:
		if err := validateExpr(value.Value, fn); err != nil {
			return err
		}
		if err := target(value.Default); err != nil {
			return err
		}
		for _, item := range value.Cases {
			if err := target(item.Target); err != nil {
				return err
			}
		}
		return nil
	case Return:
		if len(value.Value.Slots) != fn.Result.Slots {
			return fmt.Errorf("returns %d slots; expected %d", len(value.Value.Slots), fn.Result.Slots)
		}
		if !sameTypeLayout(value.Value.Type, fn.Result) {
			return fmt.Errorf("returns type %q; expected %q", value.Value.Type.Name, fn.Result.Name)
		}
		for _, slot := range value.Value.Slots {
			if err := validateExpr(slot, fn); err != nil {
				return err
			}
		}
		return nil
	case Unreachable:
		return nil
	default:
		return fmt.Errorf("unknown terminator %T", term)
	}
}

func validateExpr(expr Expr, fn *Function) error {
	switch value := expr.(type) {
	case Const:
		return nil
	case Load:
		if err := validatePlace(value.Place, fn); err != nil {
			return err
		}
		if memory, ok := value.Place.(MemoryPlace); ok && !memory.Read {
			return fmt.Errorf("%s storage is write-only", memory.Storage)
		}
		return nil
	case RuntimeCall:
		if value.Result.Slots != 1 {
			return fmt.Errorf("runtime expression returns %d slots; expected 1", value.Result.Slots)
		}
		return validateRuntimeCall(value, fn)
	default:
		return fmt.Errorf("unknown expression %T", expr)
	}
}

func validateRuntimeCall(value RuntimeCall, fn *Function) error {
	if value.Function == "" {
		return fmt.Errorf("runtime call has no function")
	}
	if value.Diagnostic != "" && value.Function != "DebugLog" {
		return fmt.Errorf("runtime diagnostic marker requires DebugLog")
	}
	if err := validateType(value.Result); err != nil {
		return fmt.Errorf("runtime result: %w", err)
	}
	for i, arg := range value.Args {
		if err := validateExpr(arg, fn); err != nil {
			return fmt.Errorf("runtime argument %d: %w", i, err)
		}
	}
	return nil
}

func validatePlace(place Place, fn *Function) error {
	switch p := place.(type) {
	case SSAPlace:
		if p.ID < 0 {
			return fmt.Errorf("SSA place has invalid ID %d", p.ID)
		}
	case LocalPlace:
		return validateLocalPlace(p, fn)
	case IndexedLocalPlace:
		if p.ID < 0 || p.ID >= len(fn.Locals) || p.Base < 0 || p.Length <= 0 || p.Stride <= 0 || p.Offset < 0 || p.Offset >= p.Stride || p.Base+p.Length*p.Stride > fn.Locals[p.ID].Slots {
			return fmt.Errorf("indexed local place %d:%d has an invalid layout (base=%d length=%d stride=%d localSlots=%d)", p.ID, p.Offset, p.Base, p.Length, p.Stride, localSlots(fn, p.ID))
		}
		if err := validateExpr(p.Index, fn); err != nil {
			return fmt.Errorf("indexed local place %d:%d index: %w", p.ID, p.Offset, err)
		}
	case MemoryPlace:
		if p.Storage == "" || p.Offset < 0 || p.Stride < 0 || (!p.Read && !p.Write) {
			return fmt.Errorf("memory place has an invalid semantic layout")
		}
		if err := validateExpr(p.Index, fn); err != nil {
			return fmt.Errorf("%s memory index: %w", p.Storage, err)
		}
	default:
		return fmt.Errorf("unknown place %T", place)
	}
	return nil
}

func validateLocalPlace(place LocalPlace, fn *Function) error {
	if place.ID < 0 || place.ID >= len(fn.Locals) || place.Offset < 0 || place.Offset >= fn.Locals[place.ID].Slots {
		return fmt.Errorf("local place %d:%d is outside its layout", place.ID, place.Offset)
	}
	return nil
}

// ValidateFinal verifies the backend-facing subset after optimization.
func ValidateFinal(fn *Function) error {
	return new(Validator).ValidateFinal(fn)
}

func (validator *Validator) ValidateFinal(fn *Function) error {
	if err := validator.Validate(fn); err != nil {
		return err
	}
	if !fn.Allocated {
		return fmt.Errorf("function locals have not been allocated")
	}
	for _, block := range fn.Blocks {
		if len(block.Phis) != 0 {
			return fmt.Errorf("block %d still contains phi nodes", block.ID)
		}
		var checkExpr func(Expr) error
		var checkPlace func(Place) error
		checkPlace = func(place Place) error {
			switch p := place.(type) {
			case SSAPlace:
				return fmt.Errorf("SSA place %s.%d remains", p.Name, p.ID)
			case IndexedLocalPlace:
				return checkExpr(p.Index)
			case MemoryPlace:
				return checkExpr(p.Index)
			default:
				return nil
			}
		}
		checkExpr = func(expr Expr) error {
			switch value := expr.(type) {
			case Load:
				return checkPlace(value.Place)
			case RuntimeCall:
				if value.Diagnostic != "" {
					return fmt.Errorf("unresolved runtime diagnostic marker remains")
				}
				for _, arg := range value.Args {
					if err := checkExpr(arg); err != nil {
						return err
					}
				}
			}
			return nil
		}
		for _, instruction := range block.Instructions {
			switch value := instruction.(type) {
			case Store:
				if err := checkPlace(value.Place); err != nil {
					return err
				}
				if err := checkExpr(value.Value); err != nil {
					return err
				}
			case Eval:
				if err := checkExpr(value.Value); err != nil {
					return err
				}
			}
		}
		switch term := block.Terminator.(type) {
		case Branch:
			if err := checkExpr(term.Condition); err != nil {
				return fmt.Errorf("block %d branch: %w", block.ID, err)
			}
		case Switch:
			if err := checkExpr(term.Value); err != nil {
				return fmt.Errorf("block %d switch: %w", block.ID, err)
			}
		case Return:
			for _, value := range term.Value.Slots {
				if err := checkExpr(value); err != nil {
					return fmt.Errorf("block %d return: %w", block.ID, err)
				}
			}
		}
	}
	return nil
}

func localSlots(fn *Function, id int) int {
	if id < 0 || id >= len(fn.Locals) {
		return -1
	}
	return fn.Locals[id].Slots
}

func validateType(typ Type) error {
	if typ.Slots < 0 {
		return fmt.Errorf("type %q has invalid slot count", typ.Name)
	}
	for _, field := range typ.Fields {
		if field.Offset < 0 || field.Offset+field.Type.Slots > typ.Slots {
			return fmt.Errorf("field %q is outside the %d-slot layout", field.Name, typ.Slots)
		}
		if err := validateType(field.Type); err != nil {
			return fmt.Errorf("field %q: %w", field.Name, err)
		}
	}
	return nil
}

func sameTypeLayout(a, b Type) bool {
	if a.Name != b.Name || a.Slots != b.Slots || len(a.Fields) != len(b.Fields) {
		return false
	}
	for i := range a.Fields {
		if a.Fields[i].Name != b.Fields[i].Name || a.Fields[i].Offset != b.Fields[i].Offset || !sameTypeLayout(a.Fields[i].Type, b.Fields[i].Type) {
			return false
		}
	}
	return true
}
