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

// modeName returns the canonical mode name string for an ir.Mode value,
// used to look up registered OmitFunc in modecompile.
func modeName(m ir.Mode) string {
	switch m {
	case ir.ModePlay:
		return "play"
	case ir.ModeWatch:
		return "watch"
	case ir.ModePreview:
		return "preview"
	case ir.ModeTutorial:
		return "tutorial"
	default:
		return ""
	}
}

// tagCollector gathers field names grouped by sonolus struct tag. It is used by
// both parseFields (Play mode, which also handles exported/scored/lifed) and
// parseModeFile (Watch/Preview/Tutorial modes).
type tagCollector struct {
	imported []ImportedField
	memory   []string
	data     []string
	shared   []string
	input    []string
	despawn  []string
	info     []string
}

// collectSonolusTags reads sonolus struct tags from a field and appends field
// names to the appropriate collector slices. Mode-specific tags (exported,
// scored, lifed) are reported via modeTag so callers can handle them without
// re-reading the struct tag. An empty tag is silently skipped. Any other
// unrecognized tag is reported via unknownTag (empty string means no error).
func (tc *tagCollector) collectSonolusTags(field *ast.Field) (unknownTag, modeTag string) {
	if field.Tag == nil || len(field.Names) == 0 {
		return "", ""
	}
	tag := reflect.StructTag(stringLit(field.Tag.Value)).Get("sonolus")
	for _, name := range field.Names {
		switch tag {
		case "imported":
			tc.imported = append(tc.imported, ImportedField{Name: name.Name})
		case "memory":
			tc.memory = append(tc.memory, name.Name)
		case "data":
			tc.data = append(tc.data, name.Name)
		case "shared":
			tc.shared = append(tc.shared, name.Name)
		case "input":
			tc.input = append(tc.input, name.Name)
		case "despawn":
			tc.despawn = append(tc.despawn, name.Name)
		case "info":
			tc.info = append(tc.info, name.Name)
		case "exported", "scored", "lifed":
			modeTag = tag
		case "":
			// empty tag — silently skip
		default:
			return tag, ""
		}
	}
	return "", modeTag
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

// archetypeData holds the per-archetype metadata produced by callback
// compilation, ready to be wrapped in a mode-specific archetype struct
// (EngineWatchDataArchetype, EnginePreviewDataArchetype, etc.).
type archetypeData struct {
	name    resource.EngineArchetypeName
	imports []resource.EngineDataArchetypeImport
}

// callbackMethod is a unified representation of a single archetype callback,
// used by compileMethodCallbacks to abstract over parsedMethod (Play) and
// modeMethod (Watch/Preview/Tutorial).
type callbackMethod struct {
	name     string
	receiver string
	body     *ast.BlockStmt
}

// compileCtx bundles the common per-compilation-session parameters shared across
// callback compilation helpers, reducing parameter list duplication.
type compileCtx struct {
	gen  *ir.IDGen
	fset *token.FileSet
	mode ir.Mode
	opts *CompileOptions
}

// compileMethodCallbacks compiles a list of callback methods for one archetype.
// The envBuilder callback constructs the frontend.Env for each method (this
// differs between Play and non-Play modes). The resultFn callback wraps the
// compiled SNode into a mode-specific Result.
func (ctx compileCtx) compileMethodCallbacks(
	methods []callbackMethod,
	archetypeName string,
	archetypeIndex int,
	envBuilder func(receiver string) frontend.Env,
	resultFn func(idx int, cb string, sn snode.SNode) *modecompile.Result,
) ([]*modecompile.Result, error) {
	var results []*modecompile.Result
	for _, m := range methods {
		env := envBuilder(m.receiver)
		sn, err := compileCallbackBlock(ctx.gen, ctx.fset, m.body, env, m.name, ctx.mode, ctx.opts)
		if err != nil {
			return nil, fmt.Errorf("archetype %q callback %s: %w", archetypeName, m.name, err)
		}
		if r := resultFn(archetypeIndex, m.name, sn); r != nil {
			results = append(results, r)
		}
	}
	return results, nil
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
		var cms []callbackMethod
		for _, m := range a.methods {
			if cb, ok := callbackSet[m.methodName]; ok {
				cms = append(cms, callbackMethod{name: cb, receiver: m.receiver, body: m.body})
			}
		}
		envBuilder := func(receiver string) frontend.Env {
			return frontend.Env{Names: names, Receiver: receiver, Funcs: funcs, Accessors: accessors, Mode: mode}
		}
		resultFn := func(idx int, cb string, sn snode.SNode) *modecompile.Result {
			return modecompile.CompileCallbackForMode(idx, cb, sn, modeName(mode))
		}
		ctx := compileCtx{gen: gen, fset: fset, mode: mode, opts: opts}
		r, err := ctx.compileMethodCallbacks(cms, name, i, envBuilder, resultFn)
		if err != nil {
			return nil, nil, err
		}
		results = append(results, r...)
		arcData = append(arcData, ad)
	}
	return arcData, results, nil
}

// nonPlayPipelineResult bundles the common intermediate results produced by
// runNonPlayPipeline for Watch and Preview mode compilation.
type nonPlayPipelineResult struct {
	pf        *parsedModeFile
	gen       *ir.IDGen
	r         parsedResources
	accessors map[string]frontend.Binding
	nodes     []resource.EngineDataNode
	arcData   []archetypeData
	results   []*modecompile.Result
}

// runNonPlayPipeline executes the shared Watch/Preview compilation pipeline:
// parse → build resources → compile callbacks. Each mode function then performs
// mode-specific finalisation (UpdateSpawn for Watch, different output types).
func runNonPlayPipeline(src string, cbSet map[string]string, mode ir.Mode, opts *CompileOptions) (nonPlayPipelineResult, error) {
	var res nonPlayPipelineResult
	pf, err := parseModeFile(src)
	if err != nil {
		return res, err
	}
	res.pf = pf
	res.gen = ir.NewIDGen()
	r, err := buildResources(pf.resources)
	if err != nil {
		return res, err
	}
	res.r = r
	res.accessors = frontend.ModeAccessorsReadOnly(mode)
	res.nodes = []resource.EngineDataNode{}
	arcData, results, err := compileArchetypeCallbacks(res.gen, pf.fset, pf.arcs, pf.order, pf.funcs, res.accessors, cbSet, mode, opts)
	if err != nil {
		return res, err
	}
	res.arcData = arcData
	res.results = results
	return res, nil
}

// compileTutorialCallback compiles a single tutorial global callback body
// (Preprocess, Navigate, or Update) and returns the appended SNode index.
// It is the per-callback helper used by CompileTutorialFile.
// If opts is non-nil and opts.Stats is non-nil, per-callback timing is recorded.
func (ctx compileCtx) compileTutorialCallback(
	d *ast.FuncDecl,
	funcs map[string]*ast.FuncDecl,
	accessors map[string]frontend.Binding,
	callback string,
	app *snode.Appender,
) (int, error) {
	env := frontend.Env{Names: accessors, Funcs: funcs, Accessors: accessors, Mode: ctx.mode}
	sn, err := compileCallbackBlock(ctx.gen, ctx.fset, d.Body, env, d.Name.Name, ctx.mode, ctx.opts)
	if err != nil {
		return -1, fmt.Errorf("tutorial %q: %w", d.Name.Name, err)
	}
	if r := modecompile.CompileCallbackForMode(-1, callback, sn, "tutorial"); r != nil {
		return app.Append(r.Node)
	}
	return -1, nil
}
// compileUpdateSpawn compiles the standalone UpdateSpawn global function for
// Watch mode. It returns the appended SNode index, or -1 if the function is
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
				return -1, fmt.Errorf("UpdateSpawn: %w", err)
			}
			if r := modecompile.CompileCallbackForMode(-1, "UpdateSpawn", sn, "watch"); r != nil {
				app := snode.NewAppender(nodes)
				idx, err := app.Append(r.Node)
				if err != nil {
					return -1, fmt.Errorf("UpdateSpawn append: %w", err)
				}
				return idx, nil
			}
			break
		}
	}
	return -1, nil
}

// composeOrFirst composes multiple callback node indices into a single Execute
// node when there is more than one, matching sonolus.js-compiler's behaviour of
// composing multiple tutorial callback functions of the same type. When there is
// exactly one index, it is returned directly. When the slice is empty, -1 is
// returned (the callback is omitted).
func composeOrFirst(idxs []int, nodes *[]resource.EngineDataNode) int {
	if len(idxs) == 0 {
		return -1
	}
	if len(idxs) == 1 {
		return idxs[0]
	}
	// Compose: Execute(idx1, idx2, ..., 0) — the trailing 0 discards the return
	// value of the last callback, matching the existing Execute convention in
	// modecompile.ignoreReturn.
	args := make([]int, len(idxs)+1)
	copy(args, idxs)
	args[len(args)-1] = 0
	node := resource.EngineDataFunctionNode{
		Func: resource.RuntimeFunctionExecute,
		Args: args,
	}
	*nodes = append(*nodes, node)
	return len(*nodes) - 1
}
