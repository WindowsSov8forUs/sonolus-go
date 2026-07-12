package tracer

import (
	"fmt"
	"go/ast"
	"go/types"

	"golang.org/x/tools/go/packages"
)

type staticEvalState uint8

const (
	staticUnseen staticEvalState = iota
	staticEvaluating
	staticDone
	staticFailed
)

type staticInitializer struct {
	pkg    *staticPackageState
	init   *types.Initializer
	state  staticEvalState
	values []StaticValue
	err    error
}

type staticVarState struct {
	object  *types.Var
	storage *StaticObject
	init    *staticInitializer
	index   int
}

type staticPackageState struct {
	pkg  *packages.Package
	vars map[*types.Var]*staticVarState
}

type staticEvaluator struct {
	tracer            *ASTTracer
	packages          map[*packages.Package]*staticPackageState
	packageErrs       map[*packages.Package]error
	nextObjectID      uint64
	nextMapID         uint64
	allocatedElements int64
}

func newStaticEvaluator(tracer *ASTTracer) *staticEvaluator {
	return &staticEvaluator{
		tracer:      tracer,
		packages:    make(map[*packages.Package]*staticPackageState),
		packageErrs: make(map[*packages.Package]error),
	}
}

func (e *staticEvaluator) newObject(typ types.Type, value StaticValue) *StaticObject {
	e.nextObjectID++
	return &StaticObject{ID: e.nextObjectID, Type: typ, Value: value}
}

func (e *staticEvaluator) newMap(typ *types.Map) *StaticMap {
	e.nextMapID++
	return &StaticMap{ID: e.nextMapID, Type: typ}
}

func (e *staticEvaluator) packageState(pkg *packages.Package) (*staticPackageState, error) {
	if state, ok := e.packages[pkg]; ok {
		return state, nil
	}
	if err, ok := e.packageErrs[pkg]; ok {
		return nil, err
	}
	if !hasStaticTypeInfo(pkg) {
		err := e.errorAt(pkg, nil, nil, ErrMissingTypeInfo)
		e.packageErrs[pkg] = err
		return nil, err
	}
	state := &staticPackageState{
		pkg:  pkg,
		vars: make(map[*types.Var]*staticVarState),
	}
	e.packages[pkg] = state

	scope := pkg.Types.Scope()
	for _, name := range scope.Names() {
		variable, ok := scope.Lookup(name).(*types.Var)
		if !ok {
			continue
		}
		zero, err := e.zeroValue(pkg, variable.Type())
		if err != nil {
			delete(e.packages, pkg)
			e.packageErrs[pkg] = err
			return nil, err
		}
		storage := e.newObject(variable.Type(), zero)
		variableState := &staticVarState{object: variable, storage: storage, index: -1}
		storage.owner = variableState
		state.vars[variable] = variableState
	}

	for _, initializer := range pkg.TypesInfo.InitOrder {
		initState := &staticInitializer{pkg: state, init: initializer}
		for index, variable := range initializer.Lhs {
			variableState, ok := state.vars[variable]
			if !ok {
				continue
			}
			variableState.init = initState
			variableState.index = index
		}
	}
	return state, nil
}

func hasStaticTypeInfo(pkg *packages.Package) bool {
	return pkg != nil && pkg.Types != nil && pkg.TypesInfo != nil && pkg.TypesSizes != nil && len(pkg.Syntax) > 0
}

func (e *staticEvaluator) packageForObject(object types.Object) (*packages.Package, error) {
	if object == nil || object.Pkg() == nil {
		return nil, nil
	}
	pkg, ok := e.tracer.packageForObject(object)
	if !ok {
		return nil, e.errorAt(e.tracer.pkg, nil, object, ErrMissingTypeInfo)
	}
	return pkg, nil
}

func (e *staticEvaluator) evalObject(object types.Object) (StaticBinding, error) {
	switch object := object.(type) {
	case *types.Const:
		return StaticBinding{Name: object.Name(), Object: object, Value: staticConstant(object.Type(), object.Val())}, nil
	case *types.Var:
		pkg, err := e.packageForObject(object)
		if err != nil {
			return StaticBinding{}, err
		}
		state, err := e.packageState(pkg)
		if err != nil {
			return StaticBinding{}, err
		}
		variableState, ok := state.vars[object]
		if !ok {
			return StaticBinding{}, e.errorAt(pkg, nil, object, ErrNotStatic)
		}
		if err := e.ensureVar(variableState); err != nil {
			return StaticBinding{}, e.associateError(pkg, object, err)
		}
		if err := e.finalizeValueGraph(pkg, variableState.storage.Value, make(map[*StaticObject]bool)); err != nil {
			return StaticBinding{}, e.associateError(pkg, object, err)
		}
		return StaticBinding{
			Name:    object.Name(),
			Object:  object,
			Storage: variableState.storage,
			Value:   cloneStaticValue(variableState.storage.Value),
		}, nil
	default:
		return StaticBinding{}, e.errorAt(e.tracer.pkg, nil, object, ErrNotStatic)
	}
}

func (e *staticEvaluator) finalizeValueGraph(
	pkg *packages.Package,
	value StaticValue,
	visited map[*StaticObject]bool,
) error {
	visitObject := func(object *StaticObject) error {
		if object == nil {
			return nil
		}
		if visited[object] {
			return nil
		}
		visited[object] = true
		if object.owner != nil {
			if err := e.ensureVar(object.owner); err != nil {
				return err
			}
		}
		return e.finalizeValueGraph(pkg, object.Value, visited)
	}

	switch value.Kind {
	case StaticArray:
		for _, element := range value.Elements {
			if err := e.finalizeValueGraph(pkg, element, visited); err != nil {
				return err
			}
		}
	case StaticStruct:
		for _, field := range value.Fields {
			if err := e.finalizeValueGraph(pkg, field.Value, visited); err != nil {
				return err
			}
		}
	case StaticSliceValue:
		if value.Slice != nil {
			return visitObject(value.Slice.Backing)
		}
	case StaticMapValue:
		if value.Map != nil {
			for _, entry := range value.Map.Entries {
				if err := e.finalizeValueGraph(pkg, entry.Key, visited); err != nil {
					return err
				}
				if err := e.finalizeValueGraph(pkg, entry.Value, visited); err != nil {
					return err
				}
			}
		}
	case StaticPointer:
		if value.Pointer != nil {
			return visitObject(value.Pointer.Object)
		}
	case StaticInterface:
		if value.Dynamic != nil {
			return e.finalizeValueGraph(pkg, *value.Dynamic, visited)
		}
	case StaticFunctionCall:
		if value.Call != nil {
			if value.Call.Receiver != nil {
				if err := e.finalizeValueGraph(pkg, *value.Call.Receiver, visited); err != nil {
					return err
				}
			}
			for _, arg := range value.Call.Args {
				if err := e.finalizeValueGraph(pkg, arg, visited); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

// EvalPackageValue evaluates one package-scope constant or variable on demand.
func (t *ASTTracer) EvalPackageValue(name string) (StaticBinding, error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if !hasStaticTypeInfo(t.pkg) {
		return StaticBinding{}, t.evaluator.errorAt(t.pkg, nil, nil, ErrMissingTypeInfo)
	}
	object := t.pkg.Types.Scope().Lookup(name)
	if object == nil {
		return StaticBinding{}, t.evaluator.errorAt(t.pkg, nil, nil, fmt.Errorf("package value %q not found: %w", name, ErrNotStatic))
	}
	binding, err := t.evaluator.evalObject(object)
	if err != nil {
		return StaticBinding{}, err
	}
	return binding, nil
}

// EvalObject evaluates a package-scope constant or variable from any package
// registered in this tracer's user package graph.
func (t *ASTTracer) EvalObject(object types.Object) (StaticBinding, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if object == nil || object.Pkg() == nil {
		return StaticBinding{}, ErrNotStatic
	}
	pkg, ok := t.packageForObject(object)
	if !ok || !hasStaticTypeInfo(pkg) {
		return StaticBinding{}, t.evaluator.errorAt(pkg, nil, object, ErrMissingTypeInfo)
	}
	return t.evaluator.evalObject(object)
}

// EvalValueSpec evaluates all names in declaration order and fails atomically.
func (t *ASTTracer) EvalValueSpec(vs *ast.ValueSpec) ([]StaticBinding, error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if vs == nil || !hasStaticTypeInfo(t.pkg) {
		return nil, t.evaluator.errorAt(t.pkg, nil, nil, ErrMissingTypeInfo)
	}
	bindings := make([]StaticBinding, 0, len(vs.Names))
	for _, name := range vs.Names {
		object := t.pkg.TypesInfo.Defs[name]
		if object == nil {
			return nil, t.evaluator.errorAt(t.pkg, name, nil, ErrNotStatic)
		}
		binding, err := t.evaluator.evalObject(object)
		if err != nil {
			return nil, err
		}
		binding.Name = name.Name
		bindings = append(bindings, binding)
	}
	return bindings, nil
}
