package engine

import (
	"fmt"
	"go/ast"
	"go/token"

	"github.com/WindowsSov8forUs/sonolus-core-go/core/resource"

	"github.com/WindowsSov8forUs/sonolus-go/compiler/frontend"
	"github.com/WindowsSov8forUs/sonolus-go/compiler/ir"
	"github.com/WindowsSov8forUs/sonolus-go/compiler/modecompile"
	"github.com/WindowsSov8forUs/sonolus-go/compiler/snode"
)

// buildBindings builds the name→Binding map and imports slice for an archetype
// from its parsed field lists. It is shared by play-mode compilation (parse.go)
// and watch/preview-mode compilation (modes.go). The optional onExport callback
// is invoked for each exported field (play-mode only).
func buildBindings(
	imported []ImportedField,
	memory, exported, data, shared, input, despawn, info []string,
	onExport func(name string, idx int),
) ([]resource.EngineDataArchetypeImport, map[string]frontend.Binding) {
	imports := make([]resource.EngineDataArchetypeImport, len(imported))
	b := map[string]frontend.Binding{}
	for j, f := range imported {
		imports[j] = resource.EngineDataArchetypeImport{
			Name: resource.EngineArchetypeDataName(f.Name), Index: j, Def: f.Def,
		}
		b[f.Name] = frontend.Binding{Block: entityMemoryBlock, Index: j, Writable: false}
	}
	for k, m := range memory {
		b[m] = frontend.Binding{Block: entityMemoryBlock, Index: len(imported) + k, Writable: true}
	}
	for ek, en := range exported {
		if onExport != nil {
			onExport(en, ek)
		}
		b[en] = frontend.Binding{Block: -1, Index: ek, Writable: true}
	}
	for di, dn := range data {
		b[dn] = frontend.Binding{Block: entityDataBlock, Index: di, Writable: false}
	}
	for si, sn := range shared {
		b[sn] = frontend.Binding{Block: entitySharedBlock, Index: si, Writable: true}
	}
	for ii, in := range input {
		b[in] = frontend.Binding{Block: entityInputBlock, Index: ii, Writable: true}
	}
	for di, dn := range despawn {
		b[dn] = frontend.Binding{Block: entityDespawnBlock, Index: di, Writable: true}
	}
	for ii, in := range info {
		b[in] = frontend.Binding{Block: entityInfoBlock, Index: ii, Writable: false}
	}
	return imports, b
}

// archetypeData holds the per-archetype metadata produced by callback
// compilation, ready to be wrapped in a mode-specific archetype struct
// (EngineWatchDataArchetype, EnginePreviewDataArchetype, etc.).
type archetypeData struct {
	name    resource.EngineArchetypeName
	imports []resource.EngineDataArchetypeImport
}

// compileArchetypeCallbacks compiles all callbacks for all archetypes in a
// Watch/Preview mode file. It handles the shared archetype-iteration +
// callback-compilation loop, returning archetype metadata and compiled results.
//
// The callbackSet maps Go method names to Sonolus callback names (e.g.
// watchCBs {"Initialize": "initialize"} or previewCBs {"Render": "render"}).
// If opts is non-nil and opts.Stats is non-nil, per-callback timing is recorded.
func compileArchetypeCallbacks(
	gen *ir.IDGen,
	fset *token.FileSet,
	arcs map[string]*modeArch,
	order []string,
	funcs map[string]*ast.FuncDecl,
	accessors map[string]frontend.Binding,
	callbackSet map[string]string,
	mode ir.Mode,
	opts *CompileOptions,
) ([]archetypeData, []*modecompile.Result, error) {
	arcData := make([]archetypeData, 0, len(order))
	var results []*modecompile.Result

	for i, name := range order {
		a := arcs[name]
		imports, b := modeBindings(a)
		ad := archetypeData{
			name:    resource.EngineArchetypeName(name),
			imports: imports,
		}
		names := frontend.CloneBindings(accessors)
		for k, v := range b {
			names[k] = v
		}
		for _, m := range a.methods {
			cb, ok := callbackSet[m.methodName]
			if !ok {
				continue
			}
			env := frontend.Env{Names: names, Receiver: m.receiver, Funcs: funcs, Accessors: accessors, Mode: mode}
			sn, err := compileCallbackBlock(gen, fset, m.body, env, m.methodName, mode, opts)
			if err != nil {
				return nil, nil, fmt.Errorf("archetype %q callback %s: %w", name, m.methodName, err)
			}
			if r := modecompile.CompileCallback(i, cb, sn, nil); r != nil {
				results = append(results, r)
			}
		}
		arcData = append(arcData, ad)
	}
	return arcData, results, nil
}

// compileTutorialCallback compiles a single tutorial global callback body
// (Preprocess, Navigate, or Update) and returns the appended SNode index.
// It is the per-callback helper used by CompileTutorialFile.
// If opts is non-nil and opts.Stats is non-nil, per-callback timing is recorded.
func compileTutorialCallback(
	gen *ir.IDGen,
	fset *token.FileSet,
	d *ast.FuncDecl,
	funcs map[string]*ast.FuncDecl,
	accessors map[string]frontend.Binding,
	callback string,
	mode ir.Mode,
	app *snode.Appender,
	opts *CompileOptions,
) (int, error) {
	env := frontend.Env{Names: accessors, Funcs: funcs, Accessors: accessors, Mode: mode}
	sn, err := compileCallbackBlock(gen, fset, d.Body, env, d.Name.Name, mode, opts)
	if err != nil {
		return 0, fmt.Errorf("tutorial %q: %w", d.Name.Name, err)
	}
	if r := modecompile.CompileCallback(-1, callback, sn, nil); r != nil {
		return app.Append(r.Node)
	}
	return 0, nil
}
