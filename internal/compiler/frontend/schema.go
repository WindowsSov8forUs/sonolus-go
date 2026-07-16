package frontend

import (
	"fmt"
	"go/types"
	"sort"
	"strings"

	"golang.org/x/tools/go/packages"

	"github.com/WindowsSov8forUs/sonolus-go/v2/internal/compiler/mode"
)

// ParseArchetypeDeclarations parses archetype markers and field layouts without
// lowering callbacks or evaluating unrelated static declarations.
func ParseArchetypeDeclarations(pkg *packages.Package, m mode.Mode) ([]*ArchetypeDeclaration, error) {
	if !m.Valid() {
		return nil, fmt.Errorf("invalid Sonolus mode %q; expected play, watch, preview, or tutorial", m)
	}
	if pkg == nil {
		return nil, fmt.Errorf("%s mode package is nil", m)
	}
	userPackages := collectPackages(pkg)
	packagesByTypes := make(map[*types.Package]*packages.Package, len(userPackages))
	for _, p := range userPackages {
		packagesByTypes[p.Types] = p
	}
	names := make(map[string]bool)
	var declarations []*ArchetypeDeclaration
	var errs []error
	for _, p := range userPackages {
		for _, named := range packageNamedTypes(p) {
			marker, ok, markerErrs := primaryDeclarationMarker(named)
			errs = append(errs, markerErrs...)
			if !ok || marker.id != markerID(m, "Archetype") {
				continue
			}
			declaration, parseErrs := parseArchetype(packagesByTypes, p, named, m, marker.tag)
			errs = append(errs, parseErrs...)
			if names[declaration.Name] {
				errs = append(errs, fmt.Errorf("duplicate archetype %q", declaration.Name))
				continue
			}
			names[declaration.Name] = true
			declarations = append(declarations, declaration)
		}
	}
	sort.Slice(declarations, func(i, j int) bool {
		if declarations[i].Name == declarations[j].Name {
			return declarations[i].PackagePath < declarations[j].PackagePath
		}
		return declarations[i].Name < declarations[j].Name
	})
	errs = append(errs, resolveArchetypeInheritance(declarations)...)
	if len(errs) != 0 {
		messages := make([]string, len(errs))
		for i, err := range errs {
			messages[i] = err.Error()
		}
		sort.Strings(messages)
		return nil, fmt.Errorf("%s", strings.Join(messages, "\n"))
	}
	return declarations, nil
}
