// Package catalog is the single semantic index for the public Sonolus Go DSL.
//
//go:generate go run ./cmd/gencatalog
package catalog

import (
	"go/types"
	"strings"

	"github.com/WindowsSov8forUs/sonolus-core-go/core/resource"
	"github.com/WindowsSov8forUs/sonolus-go/v2/internal/compiler/mode"
)

type Kind string

const (
	KindType     Kind = "type"
	KindFunction Kind = "function"
	KindMethod   Kind = "method"
	KindVariable Kind = "variable"
	KindConstant Kind = "constant"
	KindNative   Kind = "native"
	KindInternal Kind = "internal"
)

type Effect string

const (
	EffectPure  Effect = "pure"
	EffectRead  Effect = "read"
	EffectWrite Effect = "write"
)

type Symbol struct {
	Package   string
	Name      string
	Receiver  string
	Kind      Kind
	Signature string
	Modes     []string
	Phases    []string
	Effect    Effect
	Runtime   resource.RuntimeFunction
	Source    string
	Internal  bool
}

type RuntimeSignature struct {
	MinArgs, MaxArgs int
	ResultSlots      int
}

type SimulationClass string

const (
	SimulationControl SimulationClass = "control"
	SimulationPure    SimulationClass = "pure"
	SimulationMemory  SimulationClass = "memory"
	SimulationRandom  SimulationClass = "random"
	SimulationEffect  SimulationClass = "effect"
	SimulationHandler SimulationClass = "handler"
)

type RuntimeSimulation struct {
	Class        SimulationClass
	Signature    RuntimeSignature
	Effect       Effect
	Strategy     string
	SpecialShape bool
	Shape        string
	Arguments    string
}

func LookupRuntimeSimulation(runtime resource.RuntimeFunction) (RuntimeSimulation, bool) {
	metadata, ok := RuntimeSimulations[runtime]
	return metadata, ok
}

func MemoryReadonly(currentMode mode.Mode, callback, storage string) bool {
	for _, recipes := range []map[string]memoryRecipe{memoryRecipes, uiMemoryRecipes} {
		for key, recipe := range recipes {
			if recipe.storage != storage || !recipe.write {
				continue
			}
			var symbol *Symbol
			for index := range Symbols {
				if Symbols[index].Key() == key {
					symbol = &Symbols[index]
					break
				}
			}
			if symbol == nil {
				continue
			}
			modeAllowed := len(symbol.Modes) == 0
			for _, candidate := range symbol.Modes {
				if candidate == string(currentMode) {
					modeAllowed = true
					break
				}
			}
			if !modeAllowed {
				continue
			}
			phaseAllowed := len(symbol.Phases) == 0
			for _, phase := range symbol.Phases {
				if phase == callback {
					phaseAllowed = true
					break
				}
			}
			if phaseAllowed {
				return false
			}
		}
	}
	return true
}

func LookupRuntimeSignature(runtime resource.RuntimeFunction) (RuntimeSignature, bool) {
	signature, ok := RuntimeSignatures[runtime]
	if !ok {
		signature, ok = internalRuntimeSignatures[runtime]
	}
	return signature, ok
}

var internalRuntimeSignatures = map[resource.RuntimeFunction]RuntimeSignature{
	resource.RuntimeFunctionAnd:      {MinArgs: 0, MaxArgs: -1, ResultSlots: 1},
	resource.RuntimeFunctionOr:       {MinArgs: 0, MaxArgs: -1, ResultSlots: 1},
	resource.RuntimeFunctionIf:       {MinArgs: 3, MaxArgs: 3, ResultSlots: 1},
	resource.RuntimeFunctionSubtract: {MinArgs: 2, MaxArgs: 2, ResultSlots: 1},
}

func (s Symbol) Key() string {
	if s.Receiver != "" {
		return s.Package + "." + s.Receiver + "." + s.Name
	}
	return s.Package + "." + s.Name
}

var byKey map[string]*Symbol

func init() {
	byKey = make(map[string]*Symbol, len(Symbols))
	for i := range Symbols {
		byKey[Symbols[i].Key()] = &Symbols[i]
	}
}

func normalizePackage(path string) string {
	if index := strings.LastIndex(path, "/sonolus"); index >= 0 {
		return path[index+1:]
	}
	return path
}

func receiverName(fn *types.Func) string {
	signature, _ := fn.Type().(*types.Signature)
	if signature == nil || signature.Recv() == nil {
		return ""
	}
	typeName := signature.Recv().Type()
	if pointer, ok := typeName.(*types.Pointer); ok {
		typeName = pointer.Elem()
	}
	if named, ok := types.Unalias(typeName).(*types.Named); ok {
		return named.Obj().Name()
	}
	return ""
}

func LookupObject(object types.Object) (*Symbol, bool) {
	if object == nil || object.Pkg() == nil {
		return nil, false
	}
	pkg := normalizePackage(object.Pkg().Path())
	receiver := ""
	if fn, ok := object.(*types.Func); ok {
		receiver = receiverName(fn)
	}
	key := pkg + "." + object.Name()
	if receiver != "" {
		key = pkg + "." + receiver + "." + object.Name()
	}
	symbol, ok := byKey[key]
	return symbol, ok
}

func AllowsMode(symbol *Symbol, mode string) bool {
	if symbol == nil || len(symbol.Modes) == 0 {
		return true
	}
	for _, allowed := range symbol.Modes {
		if allowed == mode {
			return true
		}
	}
	return false
}

func AllowsPhase(symbol *Symbol, phase string) bool {
	if symbol == nil || symbol.Effect != EffectWrite || len(symbol.Phases) == 0 {
		return true
	}
	for _, allowed := range symbol.Phases {
		if allowed == phase {
			return true
		}
	}
	return false
}
