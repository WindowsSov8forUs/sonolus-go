// Package engine is the top-level integration layer: it compiles a Go-authored
// engine description end-to-end into Sonolus EngineData. It wires together the
// front end (go/ast tracer), the optimizer, finalization, and play-mode
// assembly.
//
// This is the foundation slice: callbacks are authored as Go statement bodies
// using the get/set memory builtins and ordinary control flow. A richer value
// system and runtime library are layered on later.
package engine

import (
	"fmt"

	"github.com/WindowsSov8forUs/sonolus-core-go/core/resource"

	"github.com/WindowsSov8forUs/sonolus-go/compiler/frontend"
	"github.com/WindowsSov8forUs/sonolus-go/compiler/ir"
	"github.com/WindowsSov8forUs/sonolus-go/compiler/ir/optimize"
	"github.com/WindowsSov8forUs/sonolus-go/compiler/play"
	"github.com/WindowsSov8forUs/sonolus-go/compiler/snode"
)

// Callback is one archetype callback, authored as a Go statement body.
type Callback struct {
	Name  play.Callback
	Order int
	Body  string // Go statements; wrapped into a function before tracing
}

// ImportedField is a per-entity field imported from level data. It is read-only
// in callbacks and occupies an entity-memory slot.
type ImportedField struct {
	Name string
	Def  float64
}

// Archetype is a play-mode archetype with named entity fields and callbacks.
//
// Field layout in EntityMemory (block 4000): imported fields occupy slots
// 0..len(Imported)-1 (and generate the archetype's import table); entity-memory
// fields follow. Imported fields are read-only; entity-memory fields are
// read/write. Callbacks reference fields by bare name.
type Archetype struct {
	Name      string
	HasInput  bool
	Imported  []ImportedField
	Memory    []string // entity-memory field names (read/write)
	Exports   []resource.EngineArchetypeDataName
	Callbacks []Callback
}

// entityMemoryBlock is the Play-mode EntityMemory block.
const entityMemoryBlock = 4000

// PlayEngine describes a play-mode engine to compile.
type PlayEngine struct {
	Skin       resource.EngineSkinData
	Effect     resource.EngineEffectData
	Particle   resource.EngineParticleData
	Buckets    []resource.EngineDataBucket
	Archetypes []Archetype
}

// CompilePlay compiles the engine to EnginePlayData: each callback is traced,
// optimized, finalized, and assembled (with deduplicated nodes) onto its
// archetype.
func CompilePlay(e PlayEngine) (*resource.EnginePlayData, error) {
	defs := make([]play.ArchetypeDef, len(e.Archetypes))
	fieldBindings := make([]map[string]frontend.Binding, len(e.Archetypes))
	for i, a := range e.Archetypes {
		imports := make([]resource.EngineDataArchetypeImport, len(a.Imported))
		bindings := map[string]frontend.Binding{}
		for j, f := range a.Imported {
			imports[j] = resource.EngineDataArchetypeImport{
				Name:  resource.EngineArchetypeDataName(f.Name),
				Index: j,
				Def:   f.Def,
			}
			bindings[f.Name] = frontend.Binding{Block: entityMemoryBlock, Index: j, Writable: false}
		}
		for k, name := range a.Memory {
			bindings[name] = frontend.Binding{Block: entityMemoryBlock, Index: len(a.Imported) + k, Writable: true}
		}
		fieldBindings[i] = bindings
		defs[i] = play.ArchetypeDef{
			Name:     a.Name,
			HasInput: a.HasInput,
			Imports:  imports,
			Exports:  a.Exports,
		}
	}
	data := play.BuildPlayData(e.Skin, e.Effect, e.Particle, e.Buckets, defs)

	var results []*play.CompileResult
	for i, a := range e.Archetypes {
		for _, cb := range a.Callbacks {
			node, err := compileCallback(ir.ModePlay, cb, fieldBindings[i])
			if err != nil {
				return nil, fmt.Errorf("archetype %q callback %q: %w", a.Name, cb.Name, err)
			}
			results = append(results, play.CompileCallback(i, cb.Name, node, cb.Order))
		}
	}

	if err := play.Assemble(data, results); err != nil {
		return nil, err
	}
	return data, nil
}

// compileCallback runs one callback body through the full pipeline up to a
// finalized SNode (play.CompileCallback applies the snode peephole + no-op rules).
func compileCallback(mode ir.Mode, cb Callback, fields map[string]frontend.Binding) (snode.SNode, error) {
	src := "package engine\nfunc callback() {\n" + cb.Body + "\n}\n"
	// Archetype fields shadow runtime accessors of the same name.
	names := frontend.ModeAccessors(mode)
	for k, v := range fields {
		names[k] = v
	}
	entry, err := frontend.Compile(src, frontend.Env{Names: names})
	if err != nil {
		return nil, err
	}
	entry = optimize.Optimize(entry, mode, string(cb.Name), ir.DefaultTempMemoryBlock)
	return ir.CFGToSNode(entry), nil
}
