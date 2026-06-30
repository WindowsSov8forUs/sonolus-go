package engine

import (
	"fmt"
	"go/ast"
	"go/token"
	"reflect"

	"github.com/WindowsSov8forUs/sonolus-core-go/core/resource"

	"github.com/WindowsSov8forUs/sonolus-go/compiler/frontend"
	"github.com/WindowsSov8forUs/sonolus-go/compiler/ir"
	"github.com/WindowsSov8forUs/sonolus-go/compiler/modecompile"
	"github.com/WindowsSov8forUs/sonolus-go/compiler/snode"
)

// tagCollector gathers field names grouped by sonolus struct tag. It is used by
// both parseFields (Play mode, which also handles exported/scored/lifed) and
// parseModeFile (Watch/Preview/Tutorial modes).
type tagCollector struct {
	imported *[]ImportedField
	memory   *[]string
	data     *[]string
	shared   *[]string
	input    *[]string
	despawn  *[]string
	info     *[]string
}

// collectSonolusTags reads sonolus struct tags from a field and appends field
// names to the appropriate collector slices. It returns false for tags that are
// mode-specific (exported, scored, lifed) so callers can handle them separately.
// An empty tag is silently skipped. Any other unrecognized tag is reported via
// unknownTag (empty string means no unknown tag).
func collectSonolusTags(field *ast.Field, tc *tagCollector) (unknownTag string) {
	if field.Tag == nil || len(field.Names) == 0 {
		return ""
	}
	tag := reflect.StructTag(stringLit(field.Tag.Value)).Get("sonolus")
	for _, name := range field.Names {
		switch tag {
		case "imported":
			*tc.imported = append(*tc.imported, ImportedField{Name: name.Name})
		case "memory":
			*tc.memory = append(*tc.memory, name.Name)
		case "data":
			*tc.data = append(*tc.data, name.Name)
		case "shared":
			*tc.shared = append(*tc.shared, name.Name)
		case "input":
			*tc.input = append(*tc.input, name.Name)
		case "despawn":
			*tc.despawn = append(*tc.despawn, name.Name)
		case "info":
			*tc.info = append(*tc.info, name.Name)
		case "exported", "scored", "lifed", "":
			// mode-specific or empty — caller handles
		default:
			return tag
		}
	}
	return ""
}

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
// parsedModeFile holds the parsed state of a Watch/Preview/Tutorial mode engine
// source file: the file set (for error positions), archetypes in declaration order,
// free functions, and resource struct definitions.
type parsedModeFile struct {
	fset      *token.FileSet
	arcs      map[string]*modeArch
	order     []string
	funcs     map[string]*ast.FuncDecl
	resources map[string]*ast.StructType
}

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

// compileUpdateSpawn compiles the standalone UpdateSpawn global function for
// Watch mode. It returns the appended SNode index, or 0 if the function is
// absent or compiles to a no-op.
func compileUpdateSpawn(
	gen *ir.IDGen,
	fset *token.FileSet,
	funcs map[string]*ast.FuncDecl,
	accessors map[string]frontend.Binding,
	opts *CompileOptions,
	nodes *[]resource.EngineDataNode,
) (int, error) {
	for _, d := range funcs {
		if d.Name.Name == "UpdateSpawn" && d.Body != nil {
			env := frontend.Env{Names: accessors, Funcs: funcs, Accessors: accessors, Mode: ir.ModeWatch}
			sn, err := compileCallbackBlock(gen, fset, d.Body, env, "UpdateSpawn", ir.ModeWatch, opts)
			if err != nil {
				return 0, fmt.Errorf("UpdateSpawn: %w", err)
			}
			if r := modecompile.CompileCallback(-1, "UpdateSpawn", sn, nil); r != nil {
				app := snode.NewAppender(nodes)
				idx, err := app.Append(r.Node)
				if err != nil {
					return 0, fmt.Errorf("UpdateSpawn append: %w", err)
				}
				return idx, nil
			}
			break
		}
	}
	return 0, nil
}
