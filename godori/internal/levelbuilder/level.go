// Package levelbuilder constructs Godori LevelData from typed Go values.
package levelbuilder

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"reflect"
	"strings"
	"unicode"

	"github.com/WindowsSov8forUs/sonolus-core-go/core/resource"
)

// Archetype describes one level archetype and its imported data shape.
type Archetype[T any] struct {
	name   string
	typeOf reflect.Type
}

// Define creates a typed level archetype descriptor.
func Define[T any](name string) (Archetype[T], error) {
	typeOf := reflect.TypeOf((*T)(nil)).Elem()
	if name == "" {
		return Archetype[T]{}, errors.New("level archetype name must not be empty")
	}
	if typeOf.Kind() != reflect.Struct {
		return Archetype[T]{}, fmt.Errorf("level archetype %q data must be a struct, got %s", name, typeOf)
	}
	if _, err := rootFieldPaths(typeOf); err != nil {
		return Archetype[T]{}, fmt.Errorf("level archetype %q: %w", name, err)
	}
	return Archetype[T]{name: name, typeOf: typeOf}, nil
}

// MustDefine is Define for package-level static declarations.
func MustDefine[T any](name string) Archetype[T] {
	archetype, err := Define[T](name)
	if err != nil {
		panic(err)
	}
	return archetype
}

// Name returns the runtime archetype name.
func (a Archetype[T]) Name() string { return a.name }

// New creates an entity of this archetype.
func (a Archetype[T]) New(data T) *Entity[T] {
	entity := &Entity[T]{Data: data}
	entity.base = &entityBase{archetype: a.name}
	entity.base.value = func() reflect.Value { return reflect.ValueOf(entity.Data) }
	return entity
}

// Item is an entity accepted by Builder.
type Item interface {
	levelEntity() *entityBase
}

type entityBase struct {
	archetype    string
	explicitName string
	value        func() reflect.Value
}

// Entity is a mutable typed level entity.
type Entity[T any] struct {
	Data T
	base *entityBase
}

func (e *Entity[T]) levelEntity() *entityBase {
	if e == nil {
		return nil
	}
	return e.base
}

// Named assigns an explicit LevelData entity name.
func (e *Entity[T]) Named(name string) *Entity[T] {
	if e != nil && e.base != nil {
		e.base.explicitName = name
	}
	return e
}

// Ref returns a typed reference to this entity.
func (e *Entity[T]) Ref() Ref[T] {
	if e == nil {
		return Ref[T]{}
	}
	return Ref[T]{target: e.base}
}

type referenceValue interface {
	levelReference() *entityBase
}

// Ref is a typed reference used inside level entity data.
type Ref[T any] struct{ target *entityBase }

func (r Ref[T]) levelReference() *entityBase { return r.target }

// IsZero reports whether the reference is unset.
func (r Ref[T]) IsZero() bool { return r.target == nil }

// Builder collects typed entities into LevelData.
type Builder struct {
	bgmOffset float64
	items     []Item
}

// NewBuilder creates an empty level builder.
func NewBuilder() *Builder { return &Builder{} }

// SetBGMOffset sets the level BGM offset.
func (b *Builder) SetBGMOffset(offset float64) *Builder {
	b.bgmOffset = offset
	return b
}

// Add appends entities in stable LevelData order.
func (b *Builder) Add(items ...Item) *Builder {
	b.items = append(b.items, items...)
	return b
}

// Build validates and converts all entities to Sonolus LevelData.
func (b *Builder) Build() (*resource.LevelData, error) {
	if b == nil {
		return nil, errors.New("nil level builder")
	}
	if math.IsNaN(b.bgmOffset) || math.IsInf(b.bgmOffset, 0) {
		return nil, errors.New("level BGM offset must be finite")
	}
	bases := make([]*entityBase, len(b.items))
	names := make(map[*entityBase]string, len(b.items))
	usedNames := make(map[string]bool, len(b.items))
	for index, item := range b.items {
		if item == nil || item.levelEntity() == nil {
			return nil, fmt.Errorf("level entity %d is nil", index)
		}
		base := item.levelEntity()
		if base.archetype == "" || base.value == nil {
			return nil, fmt.Errorf("level entity %d has an invalid archetype descriptor", index)
		}
		if _, exists := names[base]; exists {
			return nil, fmt.Errorf("level entity %d was added more than once", index)
		}
		name := base.explicitName
		if name == "" {
			name = fmt.Sprintf("%d_%s", index, base.archetype)
		}
		if usedNames[name] {
			return nil, fmt.Errorf("level entity %d has duplicate name %q", index, name)
		}
		usedNames[name] = true
		names[base] = name
		bases[index] = base
	}
	entities := make([]resource.LevelDataEntity, len(bases))
	for index, base := range bases {
		data, err := flattenRootValue(base.value(), names)
		if err != nil {
			return nil, fmt.Errorf("level entity %d (%s): %w", index, base.archetype, err)
		}
		entities[index] = resource.LevelDataEntity{
			Name: names[base], Archetype: resource.EngineArchetypeName(base.archetype), Data: data,
		}
	}
	return &resource.LevelData{BGMOffset: b.bgmOffset, Entities: entities}, nil
}

// Marshal returns canonical indented JSON suitable for go:embed.
func Marshal(data *resource.LevelData) ([]byte, error) {
	if data == nil {
		return nil, errors.New("nil level data")
	}
	encoded, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(encoded, '\n'), nil
}

type fieldOptions struct {
	name      string
	omitEmpty bool
	skip      bool
}

func parseFieldOptions(field reflect.StructField) (fieldOptions, error) {
	raw, exists := field.Tag.Lookup("level")
	if !exists {
		return fieldOptions{name: lowerFieldName(field.Name)}, nil
	}
	parts := strings.Split(raw, ",")
	if parts[0] == "-" {
		return fieldOptions{skip: true}, nil
	}
	options := fieldOptions{name: parts[0]}
	if options.name == "" {
		options.name = lowerFieldName(field.Name)
	}
	for _, option := range parts[1:] {
		switch option {
		case "omitempty":
			options.omitEmpty = true
		case "":
		default:
			return fieldOptions{}, fmt.Errorf("field %s has unknown level tag option %q", field.Name, option)
		}
	}
	if options.name == "" {
		return fieldOptions{}, fmt.Errorf("field %s has an empty level data name", field.Name)
	}
	return options, nil
}

func rootFieldPaths(typeOf reflect.Type) ([]string, error) {
	var result []string
	seen := map[string]bool{}
	for index := 0; index < typeOf.NumField(); index++ {
		field := typeOf.Field(index)
		if field.PkgPath != "" {
			return nil, fmt.Errorf("field %s must be exported", field.Name)
		}
		options, err := parseFieldOptions(field)
		if err != nil {
			return nil, err
		}
		if options.skip {
			continue
		}
		paths, err := valuePaths(field.Type, options.name)
		if err != nil {
			return nil, fmt.Errorf("field %s: %w", field.Name, err)
		}
		for _, path := range paths {
			if seen[path] {
				return nil, fmt.Errorf("duplicate flattened level data name %q", path)
			}
			seen[path] = true
			result = append(result, path)
		}
	}
	return result, nil
}

var referenceValueType = reflect.TypeOf((*referenceValue)(nil)).Elem()

func valuePaths(typeOf reflect.Type, prefix string) ([]string, error) {
	if typeOf.Implements(referenceValueType) {
		return []string{prefix}, nil
	}
	if typeOf.Kind() == reflect.Pointer {
		return valuePaths(typeOf.Elem(), prefix)
	}
	switch typeOf.Kind() {
	case reflect.Bool, reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr,
		reflect.Float32, reflect.Float64:
		return []string{prefix}, nil
	case reflect.Array:
		var result []string
		for index := 0; index < typeOf.Len(); index++ {
			paths, err := valuePaths(typeOf.Elem(), fmt.Sprintf("%s[%d]", prefix, index))
			if err != nil {
				return nil, err
			}
			result = append(result, paths...)
		}
		return result, nil
	case reflect.Struct:
		var result []string
		for index := 0; index < typeOf.NumField(); index++ {
			field := typeOf.Field(index)
			if field.PkgPath != "" {
				return nil, fmt.Errorf("nested field %s must be exported", field.Name)
			}
			options, err := parseFieldOptions(field)
			if err != nil {
				return nil, err
			}
			if options.skip {
				continue
			}
			paths, err := valuePaths(field.Type, prefix+"."+options.name)
			if err != nil {
				return nil, err
			}
			result = append(result, paths...)
		}
		if len(result) == 1 {
			return []string{prefix}, nil
		}
		return result, nil
	default:
		return nil, fmt.Errorf("unsupported level data type %s", typeOf)
	}
}

func flattenRootValue(value reflect.Value, names map[*entityBase]string) ([]resource.LevelDataEntityData, error) {
	typeOf := value.Type()
	result := []resource.LevelDataEntityData{}
	for index := 0; index < typeOf.NumField(); index++ {
		field := typeOf.Field(index)
		options, err := parseFieldOptions(field)
		if err != nil {
			return nil, err
		}
		if options.skip {
			continue
		}
		fieldValue := value.Field(index)
		if options.omitEmpty && fieldValue.IsZero() {
			continue
		}
		entries, err := flattenValue(fieldValue, options.name, names)
		if err != nil {
			return nil, fmt.Errorf("field %s: %w", field.Name, err)
		}
		result = append(result, entries...)
	}
	return result, nil
}

func flattenValue(value reflect.Value, prefix string, names map[*entityBase]string) ([]resource.LevelDataEntityData, error) {
	if value.CanInterface() {
		if reference, ok := value.Interface().(referenceValue); ok {
			target := reference.levelReference()
			if target == nil {
				return nil, nil
			}
			name, exists := names[target]
			if !exists {
				return nil, errors.New("reference target is not part of this level")
			}
			return []resource.LevelDataEntityData{resource.LevelDataEntityRefData{Name: resource.EngineArchetypeDataName(prefix), Ref: name}}, nil
		}
	}
	if value.Kind() == reflect.Pointer {
		if value.IsNil() {
			return nil, nil
		}
		return flattenValue(value.Elem(), prefix, names)
	}
	switch value.Kind() {
	case reflect.Bool:
		number := 0.0
		if value.Bool() {
			number = 1
		}
		return valueEntry(prefix, number)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return valueEntry(prefix, float64(value.Int()))
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		return valueEntry(prefix, float64(value.Uint()))
	case reflect.Float32, reflect.Float64:
		return valueEntry(prefix, value.Float())
	case reflect.Array:
		result := []resource.LevelDataEntityData{}
		for index := 0; index < value.Len(); index++ {
			entries, err := flattenValue(value.Index(index), fmt.Sprintf("%s[%d]", prefix, index), names)
			if err != nil {
				return nil, err
			}
			result = append(result, entries...)
		}
		return result, nil
	case reflect.Struct:
		result := []resource.LevelDataEntityData{}
		for index := 0; index < value.NumField(); index++ {
			field := value.Type().Field(index)
			options, err := parseFieldOptions(field)
			if err != nil {
				return nil, err
			}
			if options.skip {
				continue
			}
			fieldValue := value.Field(index)
			if options.omitEmpty && fieldValue.IsZero() {
				continue
			}
			entries, err := flattenValue(fieldValue, prefix+"."+options.name, names)
			if err != nil {
				return nil, err
			}
			result = append(result, entries...)
		}
		if len(result) == 1 {
			switch entry := result[0].(type) {
			case resource.LevelDataEntityValueData:
				entry.Name = resource.EngineArchetypeDataName(prefix)
				result[0] = entry
			case resource.LevelDataEntityRefData:
				entry.Name = resource.EngineArchetypeDataName(prefix)
				result[0] = entry
			}
		}
		return result, nil
	default:
		return nil, fmt.Errorf("unsupported level data type %s", value.Type())
	}
}

func valueEntry(name string, value float64) ([]resource.LevelDataEntityData, error) {
	if math.IsNaN(value) || math.IsInf(value, 0) {
		return nil, fmt.Errorf("level data %q must be finite", name)
	}
	return []resource.LevelDataEntityData{resource.LevelDataEntityValueData{Name: resource.EngineArchetypeDataName(name), Value: value}}, nil
}

func lowerFieldName(name string) string {
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
