package source

import (
	"go/ast"
	"reflect"
	"strings"
)

type StructTag struct {
	Name    string
	Options []string
	Map     map[string]string
}

func ParseStructTag(tag *ast.BasicLit, name string) *StructTag {
	ref := reflect.StructTag(strings.Trim(tag.Value, "`"))
	value, ok := ref.Lookup(name)
	if !ok {
		return nil
	}

	result := &StructTag{Name: name, Map: make(map[string]string)}
	if len(value) <= 0 {
		return result
	}

	options := strings.SplitSeq(value, ",")
	for option := range options {
		option = strings.TrimSpace(option)
		if option == "" {
			continue
		}
		parts := strings.SplitN(option, "=", 2)
		if len(parts) < 2 {
			result.Options = append(result.Options, parts...)
		} else {
			result.Map[parts[0]] = parts[1]
		}
	}

	return result
}
