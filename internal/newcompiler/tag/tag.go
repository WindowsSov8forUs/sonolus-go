// Package tag parses namespaced Go struct tags used by the new compiler.
// It owns syntax only; declaration-specific validation remains in frontend.
package tag

import (
	"go/ast"
	"reflect"
	"sort"
	"strings"
)

type Value struct {
	Name  string
	Flags map[string]bool
	Items map[string]string
}

func ParseValue(name, raw string) Value {
	result := Value{Name: name, Flags: map[string]bool{}, Items: map[string]string{}}
	for _, part := range strings.Split(raw, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		key, value, ok := strings.Cut(part, "=")
		key = strings.TrimSpace(key)
		if ok {
			result.Items[key] = strings.TrimSpace(value)
		} else {
			result.Flags[key] = true
		}
	}
	return result
}

func Parse(rawStructTag, name string) (Value, bool) {
	raw, ok := reflect.StructTag(rawStructTag).Lookup(name)
	if !ok {
		return Value{}, false
	}
	return ParseValue(name, raw), true
}

func ParseLiteral(literal *ast.BasicLit, name string) (Value, bool) {
	if literal == nil {
		return Value{}, false
	}
	return Parse(strings.Trim(literal.Value, "`"), name)
}

func (v Value) Unknown(allowedFlags, allowedItems []string) []string {
	flags := make(map[string]bool, len(allowedFlags))
	items := make(map[string]bool, len(allowedItems))
	for _, key := range allowedFlags {
		flags[key] = true
	}
	for _, key := range allowedItems {
		items[key] = true
	}
	var result []string
	for key := range v.Flags {
		if !flags[key] {
			result = append(result, key)
		}
	}
	for key := range v.Items {
		if !items[key] {
			result = append(result, key)
		}
	}
	sort.Strings(result)
	return result
}
