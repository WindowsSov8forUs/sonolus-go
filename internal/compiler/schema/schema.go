// Package schema builds the level archetype field contract shared by the CLI
// schema output and development-level validation.
package schema

import (
	"sort"

	"github.com/WindowsSov8forUs/sonolus-go/v2/internal/compiler/mode"
)

type Project struct {
	Archetypes []Archetype `json:"archetypes"`
}

type Archetype struct {
	Name   string   `json:"name"`
	Fields []string `json:"fields"`
}

type ModeArchetype struct {
	Name    string
	Imports []string
	Exports []string
}

type FieldSources struct {
	PlayImport    bool
	PlayExport    bool
	WatchImport   bool
	PreviewImport bool
}

type Contract struct {
	Project Project
	modes   map[mode.Mode]map[string]bool
	fields  map[string]map[string]FieldSources
}

func Build(play, watch, preview []ModeArchetype) *Contract {
	contract := &Contract{
		Project: Project{Archetypes: []Archetype{}},
		modes: map[mode.Mode]map[string]bool{
			mode.ModePlay: {}, mode.ModeWatch: {}, mode.ModePreview: {},
		},
		fields: make(map[string]map[string]FieldSources),
	}
	type accumulated struct {
		fields []string
		seen   map[string]bool
	}
	byName := make(map[string]*accumulated)
	addArchetypes := func(m mode.Mode, archetypes []ModeArchetype, includeExports bool) {
		for _, archetype := range archetypes {
			contract.modes[m][archetype.Name] = true
			entry := byName[archetype.Name]
			if entry == nil {
				entry = &accumulated{seen: make(map[string]bool)}
				byName[archetype.Name] = entry
			}
			if contract.fields[archetype.Name] == nil {
				contract.fields[archetype.Name] = make(map[string]FieldSources)
			}
			add := func(name string, update func(*FieldSources)) {
				sources := contract.fields[archetype.Name][name]
				update(&sources)
				contract.fields[archetype.Name][name] = sources
				if !entry.seen[name] {
					entry.seen[name] = true
					entry.fields = append(entry.fields, name)
				}
			}
			if includeExports {
				for _, name := range archetype.Exports {
					add(name, func(s *FieldSources) { s.PlayExport = true })
				}
			}
			for _, name := range archetype.Imports {
				if m == mode.ModeWatch && (name == "#ACCURACY" || name == "#JUDGMENT") {
					sources := contract.fields[archetype.Name][name]
					sources.WatchImport = true
					contract.fields[archetype.Name][name] = sources
					continue
				}
				add(name, func(s *FieldSources) {
					switch m {
					case mode.ModePlay:
						s.PlayImport = true
					case mode.ModeWatch:
						s.WatchImport = true
					case mode.ModePreview:
						s.PreviewImport = true
					}
				})
			}
		}
	}
	addArchetypes(mode.ModePlay, play, true)
	addArchetypes(mode.ModeWatch, watch, false)
	addArchetypes(mode.ModePreview, preview, false)
	names := make([]string, 0, len(byName))
	for name := range byName {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		fields := append([]string{}, byName[name].fields...)
		contract.Project.Archetypes = append(contract.Project.Archetypes, Archetype{Name: name, Fields: fields})
	}
	return contract
}

func (c *Contract) HasArchetype(m mode.Mode, name string) bool {
	return c != nil && c.modes[m][name]
}

func (c *Contract) IsImportedByAll(name, field string) bool {
	if c == nil {
		return false
	}
	sources := c.fields[name][field]
	return sources.PlayImport && sources.WatchImport && sources.PreviewImport
}

func (c *Contract) IsImported(m mode.Mode, name, field string) bool {
	if c == nil {
		return false
	}
	sources := c.fields[name][field]
	switch m {
	case mode.ModePlay:
		return sources.PlayImport
	case mode.ModeWatch:
		return sources.WatchImport
	case mode.ModePreview:
		return sources.PreviewImport
	default:
		return false
	}
}
