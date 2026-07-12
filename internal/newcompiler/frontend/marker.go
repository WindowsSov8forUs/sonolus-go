package frontend

import (
	"fmt"
	"go/types"

	"github.com/WindowsSov8forUs/sonolus-go/internal/newcompiler/mode"
	"github.com/WindowsSov8forUs/sonolus-go/internal/newcompiler/source"
	compilerTag "github.com/WindowsSov8forUs/sonolus-go/internal/newcompiler/tag"
)

type tagValue = compilerTag.Value

func sonolusTag(tag string) (tagValue, bool) {
	return compilerTag.Parse(tag, "sonolus")
}

func configurationTag(tag string) (tagValue, bool) {
	return compilerTag.Parse(tag, "configuration")
}

func validateTag(where string, tag tagValue, flags, items []string) []error {
	var errs []error
	for _, key := range tag.Unknown(flags, items) {
		errs = append(errs, fmt.Errorf("%s: unknown %s tag %q", where, tag.Name, key))
	}
	return errs
}

func namedType(t types.Type) (*types.Named, bool) {
	if p, ok := t.(*types.Pointer); ok {
		t = p.Elem()
	}
	n, ok := types.Unalias(t).(*types.Named)
	return n, ok
}

func typeID(t types.Type) string {
	n, ok := namedType(t)
	if !ok || n.Obj().Pkg() == nil {
		return ""
	}
	return n.Obj().Pkg().Path() + "." + n.Obj().Name()
}

func markerID(m mode.Mode, name string) string {
	return source.SonolusPkgPath() + "/" + string(m) + "." + name
}

func rootID(name string) string { return source.SonolusPkgPath() + "." + name }

type markerField struct {
	id    string
	tag   tagValue
	field *types.Var
}

func structMarkers(named *types.Named) []markerField {
	st, ok := named.Underlying().(*types.Struct)
	if !ok {
		return nil
	}
	var result []markerField
	for i := 0; i < st.NumFields(); i++ {
		field := st.Field(i)
		if !field.Embedded() {
			continue
		}
		id := typeID(field.Type())
		if id == "" {
			continue
		}
		tag, _ := sonolusTag(st.Tag(i))
		result = append(result, markerField{id: id, tag: tag, field: field})
	}
	return result
}
