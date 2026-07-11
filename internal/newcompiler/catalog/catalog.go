// Package catalog is the single semantic index for the public Sonolus Go DSL.
//
//go:generate go run ./cmd/gencatalog
package catalog

import (
	"go/types"
	"strings"

	"github.com/WindowsSov8forUs/sonolus-core-go/core/resource"
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
