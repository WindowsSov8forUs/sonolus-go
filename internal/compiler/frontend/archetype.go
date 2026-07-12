package frontend

import (
	"fmt"
	"go/ast"
	"go/types"
	"sort"
	"strconv"
	"strings"
	"sync"

	"golang.org/x/tools/go/packages"

	"github.com/WindowsSov8forUs/sonolus-core-go/core/resource"
	"github.com/WindowsSov8forUs/sonolus-go/internal/compiler/mode"
)

func layoutSize(t types.Type) (int, error) {
	if basic, ok := types.Unalias(t).Underlying().(*types.Basic); ok {
		if basic.Info()&(types.IsBoolean|types.IsInteger|types.IsFloat) != 0 {
			return 1, nil
		}
	}
	switch typeID(t) {
	case rootID("Vec2"), rootID("Range"):
		return 2, nil
	case rootID("Rect"):
		return 4, nil
	case rootID("Transform2D"), rootID("JudgmentWindow"), rootID("JudgmentWindows"):
		return 6, nil
	case rootID("Quad"):
		return 8, nil
	}
	if strings.HasPrefix(typeID(t), rootID("EntityRef")) {
		return 1, nil
	}
	if array, ok := types.Unalias(t).Underlying().(*types.Array); ok {
		element, err := layoutSize(array.Elem())
		if err != nil {
			return 0, err
		}
		return int(array.Len()) * element, nil
	}
	if record, ok := types.Unalias(t).Underlying().(*types.Struct); ok {
		size := 0
		for i := 0; i < record.NumFields(); i++ {
			field, err := layoutSize(record.Field(i).Type())
			if err != nil {
				return 0, err
			}
			size += field
		}
		return size, nil
	}
	return 0, fmt.Errorf("unsupported runtime field type %s", t.String())
}

func parseBool(value string) (bool, error) { return strconv.ParseBool(value) }

func parseArchetype(packagesByTypes map[*types.Package]*packages.Package, pkg *packages.Package, named *types.Named, resources *ModeResources, m mode.Mode, marker tagValue) (*ArchetypeDeclaration, []error) {
	var errs []error
	errs = append(errs, validateTag(named.Obj().Name(), marker, nil, []string{"name", "hasInput"})...)
	name := marker.Items["name"]
	if name == "" {
		name = named.Obj().Name()
	}
	hasInput := false
	if raw, ok := marker.Items["hasInput"]; ok {
		value, err := parseBool(raw)
		if err != nil {
			errs = append(errs, fmt.Errorf("%s: invalid hasInput %q", named.Obj().Name(), raw))
		} else {
			hasInput = value
		}
	}
	result := &ArchetypeDeclaration{PackagePath: pkg.PkgPath, TypeName: named.Obj().Name(), Name: name, HasInput: hasInput, Named: named, CallbackOrders: map[string]int{}}
	st := named.Underlying().(*types.Struct)
	offsets := map[string]int{"data": 0, "memory": 0, "shared": 0, "exported": 0}
	receiverOffset := 0
	external := map[string]bool{}
	for i := 0; i < st.NumFields(); i++ {
		field := st.Field(i)
		id := typeID(field.Type())
		if field.Embedded() {
			if id == markerID(m, "CallbackOrders") {
				tag, _ := sonolusTag(st.Tag(i))
				for key, raw := range tag.Items {
					order, err := strconv.Atoi(raw)
					if err != nil {
						errs = append(errs, fmt.Errorf("%s.%s: invalid callback order %q", named.Obj().Name(), key, raw))
						continue
					}
					result.CallbackOrders[key] = order
				}
			}
			continue
		}
		tag, ok := sonolusTag(st.Tag(i))
		if !ok {
			continue
		}
		errs = append(errs, validateTag(named.Obj().Name()+"."+field.Name(), tag,
			[]string{"imported", "data", "memory", "shared", "exported"}, []string{"name", "default", "cap"})...)
		storage := ""
		for _, candidate := range []string{"imported", "data", "memory", "shared", "exported"} {
			if tag.Flags[candidate] {
				if storage != "" {
					errs = append(errs, fmt.Errorf("%s.%s: multiple storage classes", named.Obj().Name(), field.Name()))
				}
				storage = candidate
			}
		}
		if storage == "" {
			errs = append(errs, fmt.Errorf("%s.%s: missing storage class", named.Obj().Name(), field.Name()))
			continue
		}
		offsetStorage := storage
		if storage == "imported" || storage == "data" {
			offsetStorage = "data"
		}
		if storage == "exported" && m != mode.ModePlay {
			errs = append(errs, fmt.Errorf("%s.%s: exports are only available in play mode", named.Obj().Name(), field.Name()))
			continue
		}
		size, err := layoutSize(field.Type())
		containerKind, keyType, elementType, isContainer := containerTypes(field.Type())
		capacity, keySize, elementSize := 0, 0, 0
		if isContainer {
			raw, exists := tag.Items["cap"]
			if !exists {
				errs = append(errs, fmt.Errorf("%s.%s: container field requires cap", named.Obj().Name(), field.Name()))
				continue
			}
			capacity, err = strconv.Atoi(raw)
			if err != nil || capacity <= 0 {
				errs = append(errs, fmt.Errorf("%s.%s: container cap must be a positive integer", named.Obj().Name(), field.Name()))
				continue
			}
			if keyType != nil {
				keySize, err = layoutSize(keyType)
				if err != nil {
					errs = append(errs, fmt.Errorf("%s.%s key: %w", named.Obj().Name(), field.Name(), err))
					continue
				}
			}
			elementSize, err = layoutSize(elementType)
			if err != nil {
				errs = append(errs, fmt.Errorf("%s.%s element: %w", named.Obj().Name(), field.Name(), err))
				continue
			}
			size = 1 + capacity*(keySize+elementSize)
		} else if _, exists := tag.Items["cap"]; exists {
			errs = append(errs, fmt.Errorf("%s.%s: cap is only valid for container fields", named.Obj().Name(), field.Name()))
		}
		if err != nil {
			errs = append(errs, fmt.Errorf("%s.%s: %w", named.Obj().Name(), field.Name(), err))
			continue
		}
		externalName := tag.Items["name"]
		if externalName == "" {
			externalName = field.Name()
		}
		if (storage == "imported" || storage == "exported") && external[externalName] {
			errs = append(errs, fmt.Errorf("%s: duplicate external field name %q", named.Obj().Name(), externalName))
			continue
		}
		if storage == "imported" || storage == "exported" {
			external[externalName] = true
		}
		def := 0.0
		if raw, ok := tag.Items["default"]; ok {
			if storage != "imported" {
				errs = append(errs, fmt.Errorf("%s.%s: default is only valid for imported fields", named.Obj().Name(), field.Name()))
			} else if value, err := strconv.ParseFloat(raw, 64); err != nil {
				errs = append(errs, fmt.Errorf("%s.%s: invalid default %q", named.Obj().Name(), field.Name(), raw))
			} else {
				def = value
			}
		}
		fd := &FieldDeclaration{GoName: field.Name(), ExternalName: externalName, Storage: storage, Offset: offsets[offsetStorage], Size: size, Default: def, Type: field.Type(), Object: field, ReceiverOffset: receiverOffset, ContainerKind: containerKind, Capacity: capacity, KeySize: keySize, ElementSize: elementSize}
		result.Fields = append(result.Fields, fd)
		receiverOffset += size
		if storage == "imported" {
			result.Imports = append(result.Imports, resource.EngineDataArchetypeImport{Name: resource.EngineArchetypeDataName(externalName), Index: offsets[offsetStorage], Def: def})
		}
		if storage == "exported" {
			result.Exports = append(result.Exports, resource.EngineArchetypeDataName(externalName))
		}
		offsets[offsetStorage] += size
		capacityLimit := map[string]int{"data": 32, "memory": 64, "shared": 32, "exported": 32}[offsetStorage]
		if offsets[offsetStorage] > capacityLimit {
			errs = append(errs, fmt.Errorf("%s.%s: %s storage exceeds capacity %d", named.Obj().Name(), field.Name(), offsetStorage, capacityLimit))
		}
	}

	return result, errs
}

func lowerArchetypeCallbacks(packagesByTypes map[*types.Package]*packages.Package, pkg *packages.Package, result *ArchetypeDeclaration, resources *ModeResources, archetypes map[*types.Named]archetypeBinding, m mode.Mode) []error {
	var errs []error
	methodSet := types.NewMethodSet(types.NewPointer(result.Named))
	foundOrders := map[string]bool{}
	type callbackJob struct {
		key  string
		fn   *types.Func
		pkg  *packages.Package
		decl *ast.FuncDecl
	}
	var jobs []callbackJob
	for i := 0; i < methodSet.Len(); i++ {
		fn, ok := methodSet.At(i).Obj().(*types.Func)
		if !ok {
			continue
		}
		want, isCallback := callbacks[m][fn.Name()]
		if !isCallback {
			continue
		}
		if err := validCallbackSignature(fn, want, true); err != nil {
			errs = append(errs, fmt.Errorf("%s.%s: %w", result.Named.Obj().Name(), fn.Name(), err))
			continue
		}
		key := callbackKey(fn.Name())
		foundOrders[key] = true
		callbackPkg := packagesByTypes[fn.Pkg()]
		if callbackPkg == nil {
			callbackPkg = pkg
		}
		decl := findFuncDecl(callbackPkg, fn)
		jobs = append(jobs, callbackJob{key: key, fn: fn, pkg: callbackPkg, decl: decl})
	}
	callbacks := make([]*CallbackDeclaration, len(jobs))
	jobErrs := make([][]error, len(jobs))
	var wg sync.WaitGroup
	for i, job := range jobs {
		wg.Add(1)
		go func(i int, job callbackJob) {
			defer wg.Done()
			bodyIR, lowerErrs := lowerCallback(packagesByTypes, job.pkg, job.decl, job.fn, result.Fields, resources, archetypes, m, job.key)
			callbacks[i] = &CallbackDeclaration{Name: job.key, Order: result.CallbackOrders[job.key], Function: job.fn, Decl: job.decl, IR: bodyIR}
			jobErrs[i] = lowerErrs
		}(i, job)
	}
	wg.Wait()
	for i := range jobs {
		result.Callbacks = append(result.Callbacks, callbacks[i])
		errs = append(errs, jobErrs[i]...)
	}
	for key := range result.CallbackOrders {
		if !foundOrders[key] {
			errs = append(errs, fmt.Errorf("%s: callback order for missing or invalid callback %q", result.Named.Obj().Name(), key))
		}
	}
	sort.Slice(result.Callbacks, func(i, j int) bool { return result.Callbacks[i].Name < result.Callbacks[j].Name })
	return errs
}

func staticName(tag tagValue, fallback string) string {
	if value := tag.Items["name"]; value != "" {
		return value
	}
	return fallback
}
