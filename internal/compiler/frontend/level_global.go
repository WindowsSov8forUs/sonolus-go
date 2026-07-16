package frontend

import (
	"fmt"
	"go/constant"
	"go/types"
	"math"

	"golang.org/x/tools/go/packages"

	"github.com/WindowsSov8forUs/sonolus-go/v2/internal/compiler/catalog"
	"github.com/WindowsSov8forUs/sonolus-go/v2/internal/compiler/mode"
	"github.com/WindowsSov8forUs/sonolus-go/v2/internal/compiler/source"
)

func levelGlobalStorage(kind string, currentMode mode.Mode) (string, bool) {
	switch kind {
	case "memory":
		switch currentMode {
		case mode.ModePlay, mode.ModeWatch:
			return "LevelMemory", true
		case mode.ModeTutorial:
			return "TutorialMemory", true
		}
	case "data":
		switch currentMode {
		case mode.ModePlay, mode.ModeWatch:
			return "LevelData", true
		case mode.ModePreview:
			return "PreviewData", true
		case mode.ModeTutorial:
			return "TutorialData", true
		}
	}
	return "", false
}

func levelGlobalStorageAccess(declaration *LevelGlobalFieldDeclaration, currentMode mode.Mode, phase string) (string, bool, bool) {
	storage, kind := declaration.Storage, declaration.Kind
	write := false
	switch kind {
	case "data":
		write = phase == "preprocess"
	case "memory":
		switch currentMode {
		case mode.ModePlay:
			write = phase == "preprocess" || phase == "updateSequential" || phase == "touch"
		case mode.ModeWatch:
			write = phase == "preprocess" || phase == "updateSequential"
		case mode.ModeTutorial:
			write = phase == "preprocess" || phase == "navigate" || phase == "update"
		}
	}
	return storage, true, write
}

func staticZero(value source.StaticValue) bool {
	value = dereferenceStatic(value)
	switch value.Kind {
	case source.StaticNil:
		return true
	case source.StaticConstant:
		if value.Exact == nil {
			return false
		}
		switch value.Exact.Kind() {
		case constant.Bool:
			return !constant.BoolVal(value.Exact)
		case constant.String:
			return constant.StringVal(value.Exact) == ""
		default:
			return constant.Sign(value.Exact) == 0
		}
	case source.StaticArray:
		for _, element := range value.Elements {
			if !staticZero(element) {
				return false
			}
		}
		return true
	case source.StaticStruct:
		for _, field := range value.Fields {
			if !staticZero(field.Value) {
				return false
			}
		}
		return true
	default:
		return false
	}
}

func levelGlobalContainerCapacity(value source.StaticValue, expected types.Type, name, kind string) (int, error) {
	value = dereferenceStatic(value)
	if value.Kind != source.StaticFunctionCall || value.Call == nil || value.Call.Object == nil || value.Call.Receiver != nil || len(value.Call.Args) != 1 {
		return 0, fmt.Errorf("%s: initialize the container with sonolus.New%s(capacity)", name, kind)
	}
	call := value.Call
	symbol, ok := catalog.LookupObject(call.Object)
	want := map[string]string{"VarArray": "NewVarArray", "ArrayMap": "NewArrayMap", "ArraySet": "NewArraySet"}[kind]
	if !ok || symbol.Package != "sonolus" || symbol.Name != want || symbol.Kind != catalog.KindFunction {
		return 0, fmt.Errorf("%s: initialize the container with sonolus.%s(capacity)", name, want)
	}
	if call.Signature == nil || call.Signature.Results().Len() != 1 || !types.Identical(call.Signature.Results().At(0).Type(), expected) {
		return 0, fmt.Errorf("%s: container constructor has an invalid result type", name)
	}
	capacity, ok := staticInt(call.Args[0])
	if !ok || capacity <= 0 {
		return 0, fmt.Errorf("%s: container capacity must be a positive static integer", name)
	}
	return capacity, nil
}

func containsLevelGlobalContainer(t types.Type) bool {
	if _, _, _, ok := containerTypes(t); ok {
		return true
	}
	if array, ok := types.Unalias(t).Underlying().(*types.Array); ok {
		return containsLevelGlobalContainer(array.Elem())
	}
	if record, ok := types.Unalias(t).Underlying().(*types.Struct); ok {
		for index := 0; index < record.NumFields(); index++ {
			if containsLevelGlobalContainer(record.Field(index).Type()) {
				return true
			}
		}
	}
	return false
}

func sameLevelGlobalLayout(left, right *LevelGlobalFieldDeclaration) bool {
	if left == nil || right == nil || left.Size != right.Size || left.ContainerKind != right.ContainerKind || left.Capacity != right.Capacity || left.KeySize != right.KeySize || left.ElementSize != right.ElementSize || left.ElementStride != right.ElementStride || len(left.Fields) != len(right.Fields) || len(left.Elements) != len(right.Elements) {
		return false
	}
	for index := range left.Fields {
		if !sameLevelGlobalLayout(left.Fields[index], right.Fields[index]) {
			return false
		}
	}
	for index := range left.Elements {
		if !sameLevelGlobalLayout(left.Elements[index], right.Elements[index]) {
			return false
		}
	}
	return true
}

func parseLevelGlobalNode(value source.StaticValue, t types.Type, object *types.Var, name, kind, storage string, absoluteOffset int) (*LevelGlobalFieldDeclaration, []error) {
	declaration := &LevelGlobalFieldDeclaration{GoName: name, Kind: kind, Storage: storage, Offset: absoluteOffset, Type: t, Object: object}
	containerKind, keyType, elementType, container := containerTypes(t)
	if container {
		capacity, err := levelGlobalContainerCapacity(value, t, name, containerKind)
		if err != nil {
			return declaration, []error{fmt.Errorf("%s: %w", name, err)}
		}
		keySize := 0
		if keyType != nil {
			keySize, err = layoutSize(keyType)
			if err != nil {
				return declaration, []error{fmt.Errorf("%s key: %w", name, err)}
			}
		}
		elementSize, err := layoutSize(elementType)
		if err != nil {
			return declaration, []error{fmt.Errorf("%s element: %w", name, err)}
		}
		stride := keySize + elementSize
		if stride < 0 || capacity > (math.MaxInt-1)/max(1, stride) {
			return declaration, []error{fmt.Errorf("%s: container layout is too large", name)}
		}
		declaration.ContainerKind = containerKind
		declaration.Capacity = capacity
		declaration.KeySize = keySize
		declaration.ElementSize = elementSize
		declaration.Size = 1 + capacity*stride
		return declaration, nil
	}
	if containsLevelGlobalContainer(t) {
		value = dereferenceStatic(value)
		if record, ok := types.Unalias(t).Underlying().(*types.Struct); ok {
			if value.Kind != source.StaticStruct {
				return declaration, []error{fmt.Errorf("%s: nested level global record must be statically evaluable", name)}
			}
			offset := 0
			var errs []error
			for index := 0; index < record.NumFields(); index++ {
				field := record.Field(index)
				fieldValue, found := staticField(value, field)
				if !found {
					errs = append(errs, fmt.Errorf("%s.%s: field value is not static", name, field.Name()))
					continue
				}
				child, childErrs := parseLevelGlobalNode(fieldValue, field.Type(), field, name+"."+field.Name(), kind, storage, absoluteOffset+offset)
				child.RelativeOffset = offset
				declaration.Fields = append(declaration.Fields, child)
				errs = append(errs, childErrs...)
				offset += child.Size
			}
			declaration.Size = offset
			return declaration, errs
		}
		if array, ok := types.Unalias(t).Underlying().(*types.Array); ok {
			if value.Kind != source.StaticArray || len(value.Elements) != int(array.Len()) {
				return declaration, []error{fmt.Errorf("%s: nested level global array must be statically evaluable", name)}
			}
			var errs []error
			for index, elementValue := range value.Elements {
				child, childErrs := parseLevelGlobalNode(elementValue, array.Elem(), nil, fmt.Sprintf("%s[%d]", name, index), kind, storage, absoluteOffset+declaration.Size)
				child.RelativeOffset = declaration.Size
				if index > 0 && !sameLevelGlobalLayout(declaration.Elements[0], child) {
					errs = append(errs, fmt.Errorf("%s: array elements must use identical container layouts", name))
				}
				declaration.Elements = append(declaration.Elements, child)
				errs = append(errs, childErrs...)
				declaration.Size += child.Size
			}
			if len(declaration.Elements) != 0 {
				declaration.ElementStride = declaration.Elements[0].Size
			}
			return declaration, errs
		}
	}
	if !staticZero(value) {
		return declaration, []error{fmt.Errorf("%s: runtime level global fields must have zero initial values", name)}
	}
	size, err := layoutSize(t)
	if err != nil {
		return declaration, []error{fmt.Errorf("%s: %w", name, err)}
	}
	declaration.Size = size
	return declaration, nil
}

func parseLevelGlobal(pkg *packages.Package, named *types.Named, singleton *types.Var, tracer *source.ASTTracer, kind, storage string, base int) (*LevelGlobalDeclaration, []error) {
	result := &LevelGlobalDeclaration{PackagePath: pkg.PkgPath, TypeName: named.Obj().Name(), Variable: singleton.Name(), Kind: kind, Storage: storage, Offset: base}
	st, ok := named.Underlying().(*types.Struct)
	if !ok {
		return result, []error{fmt.Errorf("%s: level global marker requires a named struct", named.Obj().Name())}
	}
	binding, err := tracer.EvalObject(singleton)
	if err != nil {
		return result, []error{fmt.Errorf("%s: level global singleton must be statically evaluable: %w", singleton.Name(), err)}
	}
	value := binding.Value
	var errs []error
	offset := base
	for index := 0; index < st.NumFields(); index++ {
		field := st.Field(index)
		id := typeID(field.Type())
		if field.Embedded() && ((kind == "memory" && id == rootID("LevelMemoryResource")) || (kind == "data" && id == rootID("LevelDataResource"))) {
			if _, direct := types.Unalias(field.Type()).(*types.Named); !direct {
				errs = append(errs, fmt.Errorf("%s.%s: level global marker must be embedded as a value", named.Obj().Name(), field.Name()))
			}
			continue
		}
		if field.Embedded() {
			errs = append(errs, fmt.Errorf("%s.%s: level global declarations do not allow additional embedded fields", named.Obj().Name(), field.Name()))
			continue
		}
		if _, exists := archetypeTag(st.Tag(index)); exists {
			errs = append(errs, fmt.Errorf("%s.%s: archetype tags are not valid on level global fields", named.Obj().Name(), field.Name()))
		}
		fieldValue, found := staticField(value, field)
		if !found {
			errs = append(errs, fmt.Errorf("%s.%s: singleton field value is not static", named.Obj().Name(), field.Name()))
			continue
		}
		declaration, fieldErrs := parseLevelGlobalNode(fieldValue, field.Type(), field, named.Obj().Name()+"."+field.Name(), kind, storage, offset)
		errs = append(errs, fieldErrs...)
		result.Fields = append(result.Fields, declaration)
		offset += declaration.Size
	}
	result.Size = offset - base
	return result, errs
}
