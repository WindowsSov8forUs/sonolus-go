package frontend

import (
	"fmt"
	"go/ast"
	"go/types"
	"math"
	"sort"
	"strconv"
	"strings"
	"sync"
	"unicode"

	"golang.org/x/tools/go/packages"

	"github.com/WindowsSov8forUs/sonolus-core-go/core/resource"
	"github.com/WindowsSov8forUs/sonolus-go/v2/internal/compiler/mode"
)

func layoutSize(t types.Type) (int, error) {
	if named, ok := namedType(t); ok && (typeID(named) == rootID("Stream") || typeID(named) == rootID("StreamData")) {
		if named.TypeArgs().Len() != 1 {
			return 0, fmt.Errorf("stream type requires one type argument")
		}
		return layoutSize(named.TypeArgs().At(0))
	}
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
	case rootID("JudgmentWindow"), rootID("JudgmentWindows"):
		return 6, nil
	case rootID("Transform2D"):
		return 9, nil
	case rootID("InvertibleTransform2D"):
		return 18, nil
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

func flattenedFieldNames(t types.Type, prefix string) ([]string, error) {
	if named, ok := namedType(t); ok && (typeID(named) == rootID("Stream") || typeID(named) == rootID("StreamData")) {
		if named.TypeArgs().Len() != 1 {
			return nil, fmt.Errorf("stream type requires one type argument")
		}
		return flattenedFieldNames(named.TypeArgs().At(0), prefix)
	}
	if basic, ok := types.Unalias(t).Underlying().(*types.Basic); ok && basic.Info()&(types.IsBoolean|types.IsInteger|types.IsFloat) != 0 {
		return []string{prefix}, nil
	}
	if strings.HasPrefix(typeID(t), rootID("EntityRef")) {
		return []string{prefix}, nil
	}
	if array, ok := types.Unalias(t).Underlying().(*types.Array); ok {
		var result []string
		for index := range int(array.Len()) {
			names, err := flattenedFieldNames(array.Elem(), fmt.Sprintf("%s[%d]", prefix, index))
			if err != nil {
				return nil, err
			}
			result = append(result, names...)
		}
		return result, nil
	}
	if record, ok := types.Unalias(t).Underlying().(*types.Struct); ok {
		var result []string
		for index := 0; index < record.NumFields(); index++ {
			field := record.Field(index)
			names, err := flattenedFieldNames(field.Type(), prefix+"."+lowerSlotFieldName(field.Name()))
			if err != nil {
				return nil, err
			}
			result = append(result, names...)
		}
		if len(result) == 1 {
			return []string{prefix}, nil
		}
		return result, nil
	}
	return nil, fmt.Errorf("unsupported runtime field type %s", t.String())
}

func lowerSlotFieldName(name string) string {
	runes := []rune(name)
	uppercase := 0
	for uppercase < len(runes) && unicode.IsUpper(runes[uppercase]) {
		uppercase++
	}
	if uppercase == 0 {
		return name
	}
	if uppercase > 1 && uppercase < len(runes) && unicode.IsLower(runes[uppercase]) {
		uppercase--
	}
	for index := 0; index < uppercase; index++ {
		runes[index] = unicode.ToLower(runes[index])
	}
	return string(runes)
}

func parseBool(value string) (bool, error) { return strconv.ParseBool(value) }

func parseArchetype(packagesByTypes map[*types.Package]*packages.Package, pkg *packages.Package, named *types.Named, m mode.Mode, marker tagValue) (*ArchetypeDeclaration, []error) {
	var errs []error
	errs = append(errs, validateTag(named.Obj().Name(), marker, []string{"abstract"}, []string{"name", "hasInput", "key"})...)
	abstract := marker.Flags["abstract"]
	name := marker.Items["name"]
	if name == "" {
		name = named.Obj().Name()
	}
	if abstract {
		if _, exists := marker.Items["name"]; exists {
			errs = append(errs, fmt.Errorf("%s: abstract archetype cannot declare a runtime name", named.Obj().Name()))
		}
		if _, exists := marker.Items["hasInput"]; exists {
			errs = append(errs, fmt.Errorf("%s: abstract archetype cannot declare hasInput", named.Obj().Name()))
		}
		if _, exists := marker.Items["key"]; exists {
			errs = append(errs, fmt.Errorf("%s: abstract archetype cannot declare a key", named.Obj().Name()))
		}
	}
	key := -1.0
	keyText, hasKey := marker.Items["key"]
	if hasKey {
		parsed, err := strconv.ParseFloat(keyText, 64)
		if err != nil || math.IsNaN(parsed) || math.IsInf(parsed, 0) {
			errs = append(errs, fmt.Errorf("%s: archetype key must be a finite number", named.Obj().Name()))
		} else {
			key = parsed
		}
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
	result := &ArchetypeDeclaration{PackagePath: pkg.PkgPath, TypeName: named.Obj().Name(), Name: name, Abstract: abstract, Key: key, HasKey: hasKey, HasInput: hasInput, Named: named, CallbackOrders: map[string]int{}}
	st := named.Underlying().(*types.Struct)
	offsets := map[string]int{"data": 0, "memory": 0, "shared": 0, "exported": 0}
	receiverOffset := 0
	external := map[string]bool{}
	for i := 0; i < st.NumFields(); i++ {
		field := st.Field(i)
		id := typeID(field.Type())
		if _, ok := sonolusTag(st.Tag(i)); ok {
			replacement := `archetype:"memory"`
			switch id {
			case markerID(m, "Archetype"):
				replacement = `archetype:"name=Note,hasInput=true"`
			case markerID(m, "CallbackOrders"):
				replacement = `archetype:"preprocess=-10"`
			}
			errs = append(errs, fmt.Errorf("%s: %s.%s: sonolus struct tags are no longer supported for archetypes; use %s", pkg.Fset.Position(field.Pos()), named.Obj().Name(), field.Name(), replacement))
		}
		if field.Embedded() {
			if tag, ok := archetypeTag(st.Tag(i)); ok && tag.Flags["base"] {
				errs = append(errs, validateTag(named.Obj().Name()+"."+field.Name(), tag, []string{"base"}, nil)...)
				base, ok := types.Unalias(field.Type()).(*types.Named)
				if !ok || field.Type() != base {
					errs = append(errs, fmt.Errorf("%s.%s: archetype base must be a directly embedded named struct type", named.Obj().Name(), field.Name()))
				} else if result.BaseNamed != nil {
					errs = append(errs, fmt.Errorf("%s: multiple archetype bases are not allowed", named.Obj().Name()))
				} else {
					result.BaseNamed = base
				}
				continue
			}
			if id == markerID(m, "CallbackOrders") {
				tag, _ := archetypeTag(st.Tag(i))
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
		tag, ok := archetypeTag(st.Tag(i))
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
		externalNames := []string(nil)
		if storage == "imported" || storage == "exported" {
			externalNames, err = flattenedFieldNames(field.Type(), externalName)
			if err != nil {
				errs = append(errs, fmt.Errorf("%s.%s: %w", named.Obj().Name(), field.Name(), err))
				continue
			}
			duplicate := false
			for _, name := range externalNames {
				if external[name] {
					errs = append(errs, fmt.Errorf("%s: duplicate external field name %q", named.Obj().Name(), name))
					duplicate = true
				}
			}
			if duplicate {
				continue
			}
			for _, name := range externalNames {
				external[name] = true
			}
		}
		def := 0.0
		if raw, ok := tag.Items["default"]; ok {
			if storage != "imported" {
				errs = append(errs, fmt.Errorf("%s.%s: default is only valid for imported fields", named.Obj().Name(), field.Name()))
			} else if size != 1 {
				errs = append(errs, fmt.Errorf("%s.%s: default is only valid for single-slot imported fields", named.Obj().Name(), field.Name()))
			} else if value, err := strconv.ParseFloat(raw, 64); err != nil {
				errs = append(errs, fmt.Errorf("%s.%s: invalid default %q", named.Obj().Name(), field.Name(), raw))
			} else {
				def = value
			}
		}
		fd := &FieldDeclaration{GoName: field.Name(), ExternalName: externalName, ExternalNames: externalNames, Storage: storage, Offset: offsets[offsetStorage], Size: size, Default: def, Type: field.Type(), Object: field, ReceiverOffset: receiverOffset, ContainerKind: containerKind, Capacity: capacity, KeySize: keySize, ElementSize: elementSize}
		result.Fields = append(result.Fields, fd)
		receiverOffset += size
		if storage == "imported" {
			for index, name := range externalNames {
				result.Imports = append(result.Imports, resource.EngineDataArchetypeImport{Name: resource.EngineArchetypeDataName(name), Index: offsets[offsetStorage] + index, Def: def})
			}
		}
		if storage == "exported" {
			for _, name := range externalNames {
				result.Exports = append(result.Exports, resource.EngineArchetypeDataName(name))
			}
		}
		offsets[offsetStorage] += size
		capacityLimit := map[string]int{"data": 32, "memory": 64, "shared": 32, "exported": 32}[offsetStorage]
		if offsets[offsetStorage] > capacityLimit {
			errs = append(errs, fmt.Errorf("%s.%s: %s storage exceeds capacity %d", named.Obj().Name(), field.Name(), offsetStorage, capacityLimit))
		}
	}

	return result, errs
}

func resolveArchetypeInheritance(declarations []*ArchetypeDeclaration) []error {
	byType := make(map[*types.Named]*ArchetypeDeclaration, len(declarations))
	for _, declaration := range declarations {
		byType[declaration.Named] = declaration
	}
	states := make(map[*ArchetypeDeclaration]int, len(declarations))
	var errs []error
	var resolve func(*ArchetypeDeclaration) bool
	resolve = func(declaration *ArchetypeDeclaration) bool {
		switch states[declaration] {
		case 1:
			errs = append(errs, fmt.Errorf("%s: archetype inheritance cycle", declaration.TypeName))
			return false
		case 2:
			return true
		}
		states[declaration] = 1
		if declaration.BaseNamed != nil {
			base := byType[declaration.BaseNamed]
			if base == nil {
				errs = append(errs, fmt.Errorf("%s: archetype base %s is not declared in the current mode", declaration.TypeName, declaration.BaseNamed.Obj().Name()))
				states[declaration] = 2
				return false
			}
			if !resolve(base) {
				states[declaration] = 2
				return false
			}
			declaration.Base = base
			if !declaration.HasKey && base.HasKey {
				declaration.Key, declaration.HasKey = base.Key, true
			}
			if !inheritArchetypeLayout(declaration, base, &errs) {
				states[declaration] = 2
				return false
			}
			declaration.MRO = append([]*ArchetypeDeclaration{declaration}, base.MRO...)
		} else {
			declaration.MRO = []*ArchetypeDeclaration{declaration}
		}
		states[declaration] = 2
		return true
	}
	for _, declaration := range declarations {
		resolve(declaration)
	}
	return errs
}

func inheritArchetypeLayout(derived, base *ArchetypeDeclaration, errs *[]error) bool {
	offsets := map[string]int{"data": 0, "memory": 0, "shared": 0, "exported": 0}
	receiverOffset := 0
	external := map[string]bool{}
	fields := make([]*FieldDeclaration, 0, len(base.Fields)+len(derived.Fields))
	appendField := func(source *FieldDeclaration, inherited bool) {
		field := *source
		storage := field.Storage
		if storage == "imported" || storage == "data" {
			storage = "data"
		}
		field.Offset = offsets[storage]
		field.ReceiverOffset = receiverOffset
		fields = append(fields, &field)
		receiverOffset += field.Size
		offsets[storage] += field.Size
		if field.Storage == "imported" || field.Storage == "exported" {
			for _, name := range field.ExternalNames {
				if external[name] && !inherited {
					*errs = append(*errs, fmt.Errorf("%s: duplicate inherited external field name %q", derived.TypeName, name))
				}
				external[name] = true
			}
		}
	}
	for _, field := range base.Fields {
		appendField(field, true)
	}
	for _, field := range derived.Fields {
		for _, inherited := range base.Fields {
			if field.GoName == inherited.GoName {
				*errs = append(*errs, fmt.Errorf("%s: field %q duplicates inherited archetype field", derived.TypeName, field.GoName))
				return false
			}
		}
		appendField(field, false)
	}
	for storage, size := range offsets {
		limit := map[string]int{"data": 32, "memory": 64, "shared": 32, "exported": 32}[storage]
		if size > limit {
			*errs = append(*errs, fmt.Errorf("%s: inherited %s storage exceeds capacity %d", derived.TypeName, storage, limit))
			return false
		}
	}
	derived.Fields = fields
	derived.Imports = nil
	derived.Exports = nil
	for _, field := range fields {
		if field.Storage == "imported" {
			for index, name := range field.ExternalNames {
				derived.Imports = append(derived.Imports, resource.EngineDataArchetypeImport{Name: resource.EngineArchetypeDataName(name), Index: field.Offset + index, Def: field.Default})
			}
		}
		if field.Storage == "exported" {
			for _, name := range field.ExternalNames {
				derived.Exports = append(derived.Exports, resource.EngineArchetypeDataName(name))
			}
		}
	}
	orders := make(map[string]int, len(base.CallbackOrders)+len(derived.CallbackOrders))
	for key, order := range base.CallbackOrders {
		orders[key] = order
	}
	for key, order := range derived.CallbackOrders {
		orders[key] = order
	}
	derived.CallbackOrders = orders
	return true
}

func lowerArchetypeCallbacks(packagesByTypes map[*types.Package]*packages.Package, pkg *packages.Package, result *ArchetypeDeclaration, resources *ModeResources, configuration *ConfigurationDeclaration, levelGlobalFields map[*types.Var]*LevelGlobalFieldDeclaration, archetypes map[*types.Named]archetypeBinding, m mode.Mode, checks RuntimeChecks) []error {
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
			bodyIR, lowerErrs := lowerCallback(packagesByTypes, job.pkg, job.decl, job.fn, result.Fields, resources, configuration, levelGlobalFields, result, archetypes, m, job.key, checks)
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
