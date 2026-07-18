package backend

import (
	"encoding/binary"
	"math"

	"github.com/WindowsSov8forUs/sonolus-go/v2/internal/compiler/frontend"
	"github.com/WindowsSov8forUs/sonolus-go/v2/internal/compiler/ir"
)

func needsROM(project *frontend.Project) bool {
	if project.ROMDeclared || len(project.ROM) != 0 {
		return true
	}
	for _, declarations := range project.Modes {
		if declarations == nil {
			continue
		}
		callbacks := append([]*frontend.CallbackDeclaration(nil), declarations.Globals...)
		for _, archetype := range declarations.Archetypes {
			if archetype == nil {
				continue
			}
			callbacks = append(callbacks, archetype.Callbacks...)
		}
		for _, callback := range callbacks {
			if callback != nil && functionUsesROM(callback.IR) {
				return true
			}
		}
	}
	return false
}

func functionUsesROM(function *ir.Function) bool {
	if function == nil {
		return false
	}
	for _, block := range function.Blocks {
		for _, instruction := range block.Instructions {
			switch instruction := instruction.(type) {
			case ir.Store:
				if placeUsesROM(instruction.Place) || expressionUsesROM(instruction.Value) {
					return true
				}
			case ir.Eval:
				if expressionUsesROM(instruction.Value) {
					return true
				}
			}
		}
		switch terminator := block.Terminator.(type) {
		case ir.Branch:
			if expressionUsesROM(terminator.Condition) {
				return true
			}
		case ir.Switch:
			if expressionUsesROM(terminator.Value) {
				return true
			}
		case ir.Return:
			for _, expression := range terminator.Value.Slots {
				if expressionUsesROM(expression) {
					return true
				}
			}
		}
	}
	return false
}

func expressionUsesROM(expression ir.Expr) bool {
	switch expression := expression.(type) {
	case ir.Const:
		return math.IsNaN(expression.Value) || math.IsInf(expression.Value, 0)
	case ir.Load:
		return placeUsesROM(expression.Place)
	case ir.RuntimeCall:
		for _, argument := range expression.Args {
			if expressionUsesROM(argument) {
				return true
			}
		}
	}
	return false
}

func placeUsesROM(place ir.Place) bool {
	switch place := place.(type) {
	case ir.IndexedLocalPlace:
		return expressionUsesROM(place.Index)
	case ir.MemoryPlace:
		return place.Storage == "EngineRom" || expressionUsesROM(place.Index)
	default:
		return false
	}
}

func buildROM(user []byte) []byte {
	result := make([]byte, 12+len(user))
	binary.LittleEndian.PutUint32(result[0:4], math.Float32bits(float32(math.NaN())))
	binary.LittleEndian.PutUint32(result[4:8], math.Float32bits(float32(math.Inf(1))))
	binary.LittleEndian.PutUint32(result[8:12], math.Float32bits(float32(math.Inf(-1))))
	copy(result[12:], user)
	return result
}
