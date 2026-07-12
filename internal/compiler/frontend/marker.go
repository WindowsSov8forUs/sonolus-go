package frontend

import (
	"fmt"
	"go/types"
	"strings"

	"github.com/WindowsSov8forUs/sonolus-go/internal/compiler/mode"
	"github.com/WindowsSov8forUs/sonolus-go/internal/compiler/source"
	compilerTag "github.com/WindowsSov8forUs/sonolus-go/internal/compiler/tag"
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

var resourceMarkerKinds = map[string]string{
	rootID("SkinResource"):            "Skin",
	rootID("EffectResource"):          "Effect",
	rootID("ParticleResource"):        "Particle",
	rootID("BucketsResource"):         "Buckets",
	rootID("InstructionResource"):     "Instruction",
	rootID("InstructionIconResource"): "InstructionIcon",
}

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

func primaryDeclarationMarker(named *types.Named) (markerField, bool, []error) {
	var primary []markerField
	var errs []error
	for _, candidate := range structMarkers(named) {
		isPrimary := candidate.id == rootID("Configuration")
		if _, ok := resourceMarkerKinds[candidate.id]; ok {
			isPrimary = true
		}
		isCallbackOrders := false
		for _, candidateMode := range orderedModes {
			if candidate.id == markerID(candidateMode, "Archetype") || candidate.id == markerID(candidateMode, "GlobalCallbacks") {
				isPrimary = true
			}
			if candidate.id == markerID(candidateMode, "CallbackOrders") {
				isCallbackOrders = true
			}
		}
		if isPrimary {
			primary = append(primary, candidate)
			continue
		}
		separator := strings.LastIndex(candidate.id, ".")
		candidatePackage := ""
		if separator >= 0 {
			candidatePackage = candidate.id[:separator]
		}
		if source.IsSonolusPkgPath(candidatePackage) && (len(candidate.tag.Flags) != 0 || len(candidate.tag.Items) != 0) && !isCallbackOrders {
			errs = append(errs, fmt.Errorf("%s.%s: unknown Sonolus declaration marker %s", named.Obj().Name(), candidate.field.Name(), candidate.id))
		}
	}
	if len(primary) > 1 {
		errs = append(errs, fmt.Errorf("%s: multiple declaration markers are not allowed", named.Obj().Name()))
		return markerField{}, false, errs
	}
	if len(primary) == 0 {
		return markerField{}, false, errs
	}
	return primary[0], true, errs
}
