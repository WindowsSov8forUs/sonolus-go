package engine

import (
	"fmt"
	"go/ast"
	"go/token"
	"reflect"
	"strings"
	"sync"

	"github.com/WindowsSov8forUs/sonolus-core-go/core/resource"

	"github.com/WindowsSov8forUs/sonolus-go/internal/compiler/frontend"
	"github.com/WindowsSov8forUs/sonolus-go/internal/compiler/ir"
	"github.com/WindowsSov8forUs/sonolus-go/internal/compiler/modecompile"
	"github.com/WindowsSov8forUs/sonolus-go/internal/compiler/snode"
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
		panic(fmt.Sprintf("modeName: unknown mode %d", m))
	}
}

// tagCollector gathers field names grouped by sonolus struct tag. It is used by
// both parseFields (Play mode, which also handles exported/scored/lifed) and
// parseModeFile (Watch/Preview/Tutorial modes).
type tagCollector struct {
	imported   []ImportedField
	memory     []string
	data       []string
	shared     []string
	input      []string
	despawn    []string
	info       []string
	containers []ContainerFieldMeta
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
	// Extract base tag for switch matching. Extended tags like
	// "memory,capacity:64" carry additional parameters via comma.
	baseTag, _, _ := strings.Cut(tag, ",")
	for _, name := range field.Names {
		switch baseTag {
		case "imported":
			if fields, ok := resolveFieldLayout(field.Type); ok {
				for _, sf := range fields {
					tc.imported = append(tc.imported, ImportedField{Name: name.Name + "." + sf})
				}
			} else {
				tc.imported = append(tc.imported, ImportedField{Name: name.Name})
			}
		case "memory":
			if ctName := resolveFieldTypeName(field.Type); ctName != "" {
				if ct, ok := containerTypeNames[ctName]; ok {
					if capVal, hasCap := parseContainerTag(tag); hasCap {
						tc.memory = appendContainerFieldNames(tc.memory, name.Name, ctName, capVal)
						tc.containers = append(tc.containers, ContainerFieldMeta{
							Name: name.Name, TypeName: ct.recordName,
							Capacity: capVal, ElemSize: ct.elemSize,
						})
						continue
					}
					return tag + " (container type " + ctName + " requires cap=N)", ""
				}
			}
			tc.memory = appendFieldNames(tc.memory, name.Name, field.Type)
		case "data":
			tc.data = appendFieldNames(tc.data, name.Name, field.Type)
		case "shared":
			tc.shared = appendFieldNames(tc.shared, name.Name, field.Type)
		case "input":
			tc.input = appendFieldNames(tc.input, name.Name, field.Type)
		case "despawn":
			tc.despawn = appendFieldNames(tc.despawn, name.Name, field.Type)
		case "info":
			tc.info = appendFieldNames(tc.info, name.Name, field.Type)
		case "exported", "scored", "lifed":
			modeTag = tag
		case "":
			// empty tag -- silently skip
		default:
			return tag, ""
		}
	}
	return "", modeTag
}

// buildBindings builds the name->Binding map and imports slice for an archetype
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
		b[f.Name] = frontend.Binding{Block: entityMemoryBlock, Index: j, Writable: true}
	}
	for k, m := range memory {
		b[m] = frontend.Binding{Block: entityMemoryBlock, Index: len(imported) + k, Writable: true}
	}
	for ek, en := range exported {
		if onExport != nil {
			onExport(en, ek)
		}
		b[en] = frontend.Binding{Block: ExportedBlock, Index: ek, Writable: true}
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
	uiVar     *ast.CompositeLit
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
//
// Callbacks are compiled in parallel via goroutines since they share no mutable
// state (each callback has its own IDGen, Env, and compilation pipeline). The
// results slice preserves the original method order.
func (ctx compileCtx) compileMethodCallbacks(
	methods []callbackMethod,
	archetypeName string,
	archetypeIndex int,
	envBuilder func(receiver string) frontend.Env,
	resultFn func(idx int, cb string, sn snode.SNode) *modecompile.Result,
) ([]*modecompile.Result, error) {
	if len(methods) == 0 {
		return nil, nil
	}

	type compiled struct {
		idx int
		r   *modecompile.Result
		err error
	}

	ch := make(chan compiled, len(methods))
	var wg sync.WaitGroup

	for i, m := range methods {
		wg.Add(1)
		go func(idx int, method callbackMethod) {
			defer wg.Done()
			// Each goroutine gets its own IDGen -- the compilation pipeline
			// is instance-isolated and safe for concurrent use.
			mCtx := compileCtx{
				gen:  ir.NewIDGen(),
				fset: ctx.fset,
				mode: ctx.mode,
				opts: ctx.opts,
			}
			env := envBuilder(method.receiver)
			sn, err := compileCallbackBlock(mCtx.gen, mCtx.fset, method.body, env, method.name, mCtx.mode, mCtx.opts)
			if err != nil {
				ch <- compiled{idx: idx, err: fmt.Errorf("archetype %q callback %s: %w", archetypeName, method.name, err)}
				return
			}
			ch <- compiled{idx: idx, r: resultFn(archetypeIndex, method.name, sn)}
		}(i, m)
	}

	wg.Wait()
	close(ch)

	// Collect results in original method order.
	out := make([]*compiled, len(methods))
	for c := range ch {
		out[c.idx] = &c
	}

	results := make([]*modecompile.Result, 0, len(methods))
	for _, c := range out {
		if c.err != nil {
			return nil, c.err
		}
		if c.r != nil {
			results = append(results, c.r)
		}
	}
	return results, nil
}

// compileArchetypeCallbacks compiles all callbacks for all archetypes in a
// Watch/Preview mode file. It handles the shared archetype-iteration +
// callback-compilation loop, returning archetype metadata and compiled results.
//
// The callbackSet maps Go method names to Sonolus callback names (e.g.
// watchCallbacks {"Initialize": "initialize"} or previewCallbacks {"Render": "render"}).
// If opts is non-nil and opts.Stats is non-nil, per-callback timing is recorded.
func compileArchetypeCallbacks(spriteIndex map[string]float64,
	gen *ir.IDGen,
	fset *token.FileSet,
	arcs map[string]*modeArch,
	order []string,
	funcs map[string]*ast.FuncDecl,
	accessors map[string]frontend.Binding,
	callbackSet map[string]string,
	mode ir.Mode,
	opts *CompileOptions,
	configOptionIndices map[string]int,
) ([]archetypeData, []*modecompile.Result, error) {
	arcData := make([]archetypeData, 0, len(order))
	results := make([]*modecompile.Result, 0, len(order)*3)

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
		for optName, optIdx := range configOptionIndices {
			names[frontend.LowerFirst(optName)] = frontend.Binding{Block: 1007, Index: optIdx, Writable: false}
		}
		cms := make([]callbackMethod, 0, len(a.methods))
		for _, m := range a.methods {
			if cb, ok := callbackSet[m.methodName]; ok {
				cms = append(cms, callbackMethod{name: cb, receiver: m.receiver, body: m.body})
			}
		}
		envBuilder := func(receiver string) frontend.Env {
			return frontend.Env{Names: names, Receiver: receiver, Funcs: funcs, Accessors: accessors, Mode: mode, ContainerFields: frontendContainerFieldMeta(a.containers),
				Constants: map[string]float64{
					"entityStateWaiting":   0,
					"entityStateActive":    1,
					"entityStateDespawned": 2,
				},
			}
		}
		resultFn := func(idx int, cb string, sn snode.SNode) *modecompile.Result {
			mn := modeName(mode)
			return modecompile.CompileCallbackForMode(idx, cb, sn, mn)
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
// parse -> build resources -> compile callbacks. Each mode function then performs
// mode-specific finalisation (UpdateSpawn for Watch, different output types).
func runNonPlayPipeline(src string, cbSet map[string]string, mode ir.Mode, opts *CompileOptions) (nonPlayPipelineResult, error) {
	var res nonPlayPipelineResult
	pf, err := parseModeFile(src)
	if err != nil {
		return res, fmt.Errorf("parse: %w", err)
	}
	res.pf = pf
	res.gen = ir.NewIDGen()
	r, err := buildResources(pf.resources, pf.uiVar)
	if err != nil {
		return res, fmt.Errorf("build resources: %w", err)
	}
	res.r = r
	res.accessors = frontend.ModeAccessorsReadOnly(mode)
	res.nodes = []resource.EngineDataNode{}
	arcData, results, err := compileArchetypeCallbacks(buildSpriteIndex(r.skin, r.skinST), res.gen, pf.fset, pf.arcs, pf.order, pf.funcs, res.accessors, cbSet, mode, opts, r.configOptionIndices)
	if err != nil {
		return res, fmt.Errorf("compile callbacks: %w", err)
	}
	res.arcData = arcData
	res.results = results
	return res, nil
}

// runNonPlayPipelineSources is the multi-file variant of runNonPlayPipeline.
// It resolves imports, type-checks, parses main + imported packages, and
// merges archetypes and free functions from all packages before compilation.
func runNonPlayPipelineSources(ess *EngineSources, cbSet map[string]string, mode ir.Mode, opts *CompileOptions) (nonPlayPipelineResult, error) {
	var res nonPlayPipelineResult

	// 1. Resolve imports and type-check.
	if err := ess.ResolveImports(); err != nil {
		return res, err
	}
	if _, _, _, err := frontend.TypeCheckEngine(ess.Access()); err != nil {
		return res, fmt.Errorf("typecheck: %w", err)
	}

	// 2. Parse main package.
	mainPES, err := parseEngineSourceFiles(ess.Main, true)
	if err != nil {
		return res, err
	}

	// 3. Build resources (main package only).
	r, err := buildResources(mainPES.resources, mainPES.uiVar)
	if err != nil {
		return res, fmt.Errorf("build resources: %w", err)
	}
	res.r = r

	// 4. Collect archetypes and free functions from all packages.
	arcs := map[string]*modeArch{}
	var order []string
	allFuncs := make(map[string]*ast.FuncDecl)

	get := func(name string) *modeArch {
		a, ok := arcs[name]
		if !ok {
			a = &modeArch{name: name}
			arcs[name] = a
			order = append(order, name)
		}
		return a
	}

	collectFromParsed := func(pes *parsedEngineSource, pkgLabel string) error {
		for _, td := range pes.typeDecls {
			a := get(td.name)
			tc := &tagCollector{}
			for _, f := range td.structType.Fields.List {
				unknown, _ := tc.collectSonolusTags(f)
				if unknown != "" {
					name := ""
					if len(f.Names) > 0 {
						name = f.Names[0].Name
					}
					return fmt.Errorf("%s: %s: unknown sonolus tag %q on field %q", pkgLabel, td.name, unknown, name)
				}
			}
			a.imported = tc.imported
			a.memory = tc.memory
			a.data = tc.data
			a.shared = tc.shared
			a.input = tc.input
			a.despawn = tc.despawn
			a.info = tc.info
		}
		for _, m := range pes.methods {
			a := get(m.receiverType)
			if m.funcDecl.Body != nil {
				a.methods = append(a.methods, modeMethod{methodName: m.methodName, receiver: m.receiverName, body: m.funcDecl.Body})
			}
		}
		for k, v := range pes.funcs {
			if _, dup := allFuncs[k]; dup {
				return fmt.Errorf("duplicate function %q in %s", k, pkgLabel)
			}
			allFuncs[k] = v
		}
		return nil
	}

	// Collect from main package.
	if err := collectFromParsed(mainPES, "main"); err != nil {
		return res, err
	}

	// Collect from imported packages.
	for impPath, files := range ess.Imports {
		_, impPES, err := parseImportedPackage(files)
		if err != nil {
			return res, fmt.Errorf("import %q: %w", impPath, err)
		}
		if err := collectFromParsed(impPES, "import "+impPath); err != nil {
			return res, err
		}
	}

	res.gen = ir.NewIDGen()
	res.accessors = frontend.ModeAccessorsReadOnly(mode)
	res.nodes = []resource.EngineDataNode{}

	pf := &parsedModeFile{
		fset:      mainPES.fset,
		arcs:      arcs,
		order:     order,
		funcs:     allFuncs,
		resources: mainPES.resources,
	}
	res.pf = pf

	arcData, results, err := compileArchetypeCallbacks(buildSpriteIndex(r.skin, r.skinST), res.gen, pf.fset, pf.arcs, pf.order, pf.funcs, res.accessors, cbSet, mode, opts, r.configOptionIndices)
	if err != nil {
		return res, fmt.Errorf("compile callbacks: %w", err)
	}
	res.arcData = arcData
	res.results = results
	return res, nil
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
			env := frontend.Env{Names: accessors, Funcs: funcs, Accessors: accessors, Mode: ir.ModeWatch,
				Constants: map[string]float64{
					"entityStateWaiting":   0,
					"entityStateActive":    1,
					"entityStateDespawned": 2,
				},
			}
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
	// Compose: Execute(idx1, idx2, ...) -- each callback's individual return
	// value has already been processed by ignoreReturn during compilation.
	// The Execute composition preserves the last callback's return value,
	// matching sonolus.js-compiler's tutorial assemble.ts behaviour.
	node := resource.EngineDataFunctionNode{
		Func: resource.RuntimeFunctionExecute,
		Args: idxs,
	}
	*nodes = append(*nodes, node)
	return len(*nodes) - 1
}
