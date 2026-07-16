package compiler

import (
	"fmt"
	"maps"

	"golang.org/x/tools/go/packages"

	"github.com/WindowsSov8forUs/sonolus-go/internal/compiler/frontend"
	"github.com/WindowsSov8forUs/sonolus-go/internal/compiler/mode"
	compilerschema "github.com/WindowsSov8forUs/sonolus-go/internal/compiler/schema"
)

type ProjectSchema = compilerschema.Project
type ArchetypeSchema = compilerschema.Archetype

var schemaModes = []mode.Mode{mode.ModePlay, mode.ModeWatch, mode.ModePreview}

// Schema returns the Play, Watch, and Preview level archetype schema without
// lowering callbacks or running optimization and backend compilation.
func (c *Compiler) Schema() (*ProjectSchema, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.schemaResult != nil {
		return cloneProjectSchema(c.schemaResult), nil
	}
	candidate := make(map[mode.Mode]*packages.Package, len(schemaModes))
	var pending []mode.Mode
	for _, m := range schemaModes {
		candidate[m] = c.packages[m]
		if candidate[m] == nil {
			candidate[m] = c.schemaPackages[m]
		}
		if candidate[m] == nil {
			pending = append(pending, m)
		}
	}
	loaded, _, err := c.loadModes(pending)
	if err != nil {
		return nil, err
	}
	maps.Copy(candidate, loaded)
	declarations := make(map[mode.Mode][]*frontend.ArchetypeDeclaration, len(schemaModes))
	for _, m := range schemaModes {
		items, err := frontend.ParseArchetypeDeclarations(candidate[m], m)
		if err != nil {
			return nil, fmt.Errorf("compiler: parse %s schema declarations: %w", m, err)
		}
		declarations[m] = items
	}
	contract := compilerschema.Build(
		modeArchetypes(declarations[mode.ModePlay]),
		modeArchetypes(declarations[mode.ModeWatch]),
		modeArchetypes(declarations[mode.ModePreview]),
	)
	c.schemaPackages = candidate
	c.schemaResult = cloneProjectSchema(&contract.Project)
	return cloneProjectSchema(c.schemaResult), nil
}

func modeArchetypes(declarations []*frontend.ArchetypeDeclaration) []compilerschema.ModeArchetype {
	result := make([]compilerschema.ModeArchetype, len(declarations))
	for i, declaration := range declarations {
		item := compilerschema.ModeArchetype{Name: declaration.Name}
		item.Imports = make([]string, len(declaration.Imports))
		for j, imported := range declaration.Imports {
			item.Imports[j] = string(imported.Name)
		}
		item.Exports = make([]string, len(declaration.Exports))
		for j, exported := range declaration.Exports {
			item.Exports[j] = string(exported)
		}
		result[i] = item
	}
	return result
}

func cloneProjectSchema(value *ProjectSchema) *ProjectSchema {
	if value == nil {
		return nil
	}
	result := &ProjectSchema{Archetypes: make([]ArchetypeSchema, len(value.Archetypes))}
	for i, archetype := range value.Archetypes {
		result.Archetypes[i] = ArchetypeSchema{Name: archetype.Name, Fields: append([]string(nil), archetype.Fields...)}
	}
	return result
}
