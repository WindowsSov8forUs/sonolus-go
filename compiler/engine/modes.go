package engine

import (
	"fmt"
	"go/ast"
	"go/token"
	"sort"
	"time"

	"github.com/WindowsSov8forUs/sonolus-core-go/core/resource"

	"github.com/WindowsSov8forUs/sonolus-go/compiler/frontend"
	"github.com/WindowsSov8forUs/sonolus-go/compiler/ir"
	"github.com/WindowsSov8forUs/sonolus-go/compiler/ir/optimize"
	"github.com/WindowsSov8forUs/sonolus-go/compiler/modecompile"
	"github.com/WindowsSov8forUs/sonolus-go/compiler/preview"
	"github.com/WindowsSov8forUs/sonolus-go/compiler/snode"
	"github.com/WindowsSov8forUs/sonolus-go/compiler/watch"
)

type modeMethod struct {
	methodName string
	receiver   string
	body       *ast.BlockStmt
}

type modeArch struct {
	name     string
	imported []ImportedField
	memory   []string
	data     []string
	shared   []string
	input    []string
	despawn  []string
	info     []string
	methods  []modeMethod
}

var watchCBs = map[string]string{
	"Preprocess": "preprocess", "SpawnTime": "spawnTime", "DespawnTime": "despawnTime",
	"Initialize": "initialize", "UpdateSequential": "updateSequential",
	"UpdateParallel": "updateParallel", "Terminate": "terminate",
}

var previewCBs = map[string]string{
	"Preprocess": "preprocess", "Render": "render",
}

func parseModeFile(src string) (*parsedModeFile, error) {
	pes, err := parseEngineSource(src)
	if err != nil {
		return nil, err
	}

	// Run type checking for early diagnostics (D1 layer): catch undeclared
	// identifiers and wrong argument counts before compilation.
	if _, _, _, err := frontend.TypeCheck(src, nil); err != nil {
		return nil, fmt.Errorf("typecheck: %w", err)
	}

	arcs := map[string]*modeArch{}
	var order []string

	get := func(name string) *modeArch {
		a, ok := arcs[name]
		if !ok {
			a = &modeArch{name: name}
			arcs[name] = a
			order = append(order, name)
		}
		return a
	}

	for _, td := range pes.typeDecls {
		a := get(td.name)
		tc := &tagCollector{
			imported: &a.imported,
			memory:   &a.memory,
			data:     &a.data,
			shared:   &a.shared,
			input:    &a.input,
			despawn:  &a.despawn,
			info:     &a.info,
		}
		for _, f := range td.structType.Fields.List {
			collectSonolusTags(f, tc) // non-Play modes ignore modeTag (exported/scored/lifed are Play-only)
		}
	}

	for _, m := range pes.methods {
		a := get(m.receiverType)
		if m.funcDecl.Body != nil {
			a.methods = append(a.methods, modeMethod{methodName: m.methodName, receiver: m.receiverName, body: m.funcDecl.Body})
		}
	}

	return &parsedModeFile{fset: pes.fset, arcs: arcs, order: order, funcs: pes.funcs, resources: pes.resources}, nil
}

func modeBindings(a *modeArch) ([]resource.EngineDataArchetypeImport, map[string]frontend.Binding) {
	return buildBindings(a.imported, a.memory, nil, a.data, a.shared, a.input, a.despawn, a.info, nil)
}

// compileCallbackBlock is the shared compile+optimize+lower pipeline for a single
// callback body. It is used by all non-Play modes (Play uses parse.go which goes
// through the play sub-package).
// If opts is non-nil and opts.Stats is non-nil, per-callback timing is recorded.
func compileCallbackBlock(gen *ir.IDGen, fset *token.FileSet, body *ast.BlockStmt, env frontend.Env, methodName string, mode ir.Mode, opts *CompileOptions) (snode.SNode, error) {
	t0 := time.Now()
	entry, err := frontend.CompileBlock(fset, gen, body, env)
	if err != nil {
		return nil, err
	}
	entry, err = optimize.OptimizeCtx(gen, entry, mode, methodName, ir.DefaultTempMemoryBlock, optimize.LevelStandard, optsCtx(opts))
	if err != nil {
		return nil, err
	}
	sn, err := ir.CFGToSNode(gen, entry)
	if opts != nil && opts.Stats != nil {
		opts.Stats.Record(methodName, time.Since(t0))
	}
	return sn, err
}

// CompileWatchFile compiles a Go Watch-mode engine source file.
func CompileWatchFile(src string) (*resource.EngineWatchData, error) {
	return CompileWatchFileWithStats(src, nil)
}

// CompileWatchFileWithStats compiles a Go Watch-mode engine source file.
// If opts is non-nil and opts.Stats is non-nil, per-callback timing is recorded.
func CompileWatchFileWithStats(src string, opts *CompileOptions) (*resource.EngineWatchData, error) {
	pf, err := parseModeFile(src)
	if err != nil {
		return nil, err
	}
	gen := ir.NewIDGen()
	r, err := buildResources(pf.resources)
	if err != nil {
		return nil, err
	}
	accessors := frontend.ModeAccessorsReadOnly(ir.ModeWatch)
	nodes := []resource.EngineDataNode{}

	arcData, results, err := compileArchetypeCallbacks(gen, pf.fset, pf.arcs, pf.order, pf.funcs, accessors, watchCBs, ir.ModeWatch, opts)
	if err != nil {
		return nil, err
	}

	outArcs := make([]resource.EngineWatchDataArchetype, len(arcData))
	for i, ad := range arcData {
		outArcs[i] = resource.EngineWatchDataArchetype{Name: ad.name, Imports: ad.imports}
	}

	updateSpawn, err := compileUpdateSpawn(gen, pf.fset, pf.funcs, accessors, opts, &nodes)
	if err != nil {
		return nil, err
	}

	if err := modecompile.Assemble(&nodes, outArcs, results, watch.SetWatchCallback); err != nil {
		return nil, err
	}

	return &resource.EngineWatchData{
		Skin: r.skin, Effect: r.effect, Particle: r.particle, Buckets: r.buckets,
		Archetypes: outArcs, Nodes: nodes, UpdateSpawn: updateSpawn,
	}, nil
}

// CompilePreviewFile compiles a Go Preview-mode engine source file.
func CompilePreviewFile(src string) (*resource.EnginePreviewData, error) {
	return CompilePreviewFileWithStats(src, nil)
}

// CompilePreviewFileWithStats compiles a Go Preview-mode engine source file.
// If opts is non-nil and opts.Stats is non-nil, per-callback timing is recorded.
func CompilePreviewFileWithStats(src string, opts *CompileOptions) (*resource.EnginePreviewData, error) {
	pf, err := parseModeFile(src)
	if err != nil {
		return nil, err
	}
	gen := ir.NewIDGen()
	r, err := buildResources(pf.resources)
	if err != nil {
		return nil, err
	}
	accessors := frontend.ModeAccessorsReadOnly(ir.ModePreview)
	nodes := []resource.EngineDataNode{}

	arcData, results, err := compileArchetypeCallbacks(gen, pf.fset, pf.arcs, pf.order, pf.funcs, accessors, previewCBs, ir.ModePreview, opts)
	if err != nil {
		return nil, err
	}

	outArcs := make([]resource.EnginePreviewDataArchetype, len(arcData))
	for i, ad := range arcData {
		outArcs[i] = resource.EnginePreviewDataArchetype{Name: ad.name, Imports: ad.imports}
	}

	if err := modecompile.Assemble(&nodes, outArcs, results, preview.SetPreviewCallback); err != nil {
		return nil, err
	}

	return &resource.EnginePreviewData{Skin: r.skin, Archetypes: outArcs, Nodes: nodes}, nil
}

// CompileTutorialFile compiles a Go Tutorial-mode engine source file.
func CompileTutorialFile(src string) (*resource.EngineTutorialData, error) {
	return CompileTutorialFileWithStats(src, nil)
}

// CompileTutorialFileWithStats compiles a Go Tutorial-mode engine source file.
// If opts is non-nil and opts.Stats is non-nil, per-callback timing is recorded.
func CompileTutorialFileWithStats(src string, opts *CompileOptions) (*resource.EngineTutorialData, error) {
	pf, err := parseModeFile(src)
	if err != nil {
		return nil, err
	}
	gen := ir.NewIDGen()
	r, err := buildResources(pf.resources)
	if err != nil {
		return nil, err
	}
	accessors := frontend.ModeAccessorsReadOnly(ir.ModeTutorial)
	nodes := []resource.EngineDataNode{}
	app := snode.NewAppender(&nodes)

	// Sort for deterministic output.
	sortedFuncs := make([]*ast.FuncDecl, 0, len(pf.funcs))
	for _, d := range pf.funcs {
		sortedFuncs = append(sortedFuncs, d)
	}
	sort.Slice(sortedFuncs, func(i, j int) bool {
		return sortedFuncs[i].Name.Name < sortedFuncs[j].Name.Name
	})

	var ppIdxs, navIdxs, updIdxs []int
	for _, d := range sortedFuncs {
		var cb string
		switch d.Name.Name {
		case "Preprocess":
			cb = "Preprocess"
		case "Navigate":
			cb = "Navigate"
		case "Update":
			cb = "Update"
		default:
			continue
		}
		if d.Body == nil {
			continue
		}
		tutCtx := compileCtx{gen: gen, fset: pf.fset, mode: ir.ModeTutorial, opts: opts}
		idx, err := tutCtx.compileTutorialCallback(d, pf.funcs, accessors, cb, app)
		if err != nil {
			return nil, err
		}
		if idx == 0 {
			continue
		}
		switch cb {
		case "Preprocess":
			ppIdxs = append(ppIdxs, idx)
		case "Navigate":
			navIdxs = append(navIdxs, idx)
		case "Update":
			updIdxs = append(updIdxs, idx)
		}
	}

	pp := composeOrFirst(ppIdxs, &nodes)
	nav := composeOrFirst(navIdxs, &nodes)
	upd := composeOrFirst(updIdxs, &nodes)

	return &resource.EngineTutorialData{
		Skin: r.skin, Effect: r.effect, Particle: r.particle, Instruction: r.instruction,
		Preprocess: pp, Navigate: nav, Update: upd, Nodes: nodes,
	}, nil
}
