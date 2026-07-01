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
		tc := &tagCollector{}
		for _, f := range td.structType.Fields.List {
			unknown, _ := tc.collectSonolusTags(f)
			if unknown != "" {
				name := ""
				if len(f.Names) > 0 {
					name = f.Names[0].Name
				}
				return nil, fmt.Errorf("%s: unknown sonolus tag %q on field %q", td.name, unknown, name)
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

	return &parsedModeFile{fset: pes.fset, arcs: arcs, order: order, funcs: pes.funcs, resources: pes.resources}, nil
}

func modeBindings(a *modeArch) ([]resource.EngineDataArchetypeImport, map[string]frontend.Binding) {
	return buildBindings(a.imported, a.memory, nil, a.data, a.shared, a.input, a.despawn, a.info, nil)
}

// compileCallbackBlock is the shared compile+optimize+lower pipeline for a single
// callback body. It is used by all non-Play modes (Play uses parse.go which goes
// through the play sub-package).
// If opts is non-nil and opts.Stats is non-nil, per-callback timing is recorded.
func compileCallbackBlock(gen *ir.IDGen, fset *token.FileSet, body *ast.BlockStmt, env frontend.Env, methodName string, mode ir.Mode, opts *CompileOptions) (sn snode.SNode, err error) {
	t0 := time.Now()
	defer func() {
		if opts != nil && opts.Stats != nil {
			opts.Stats.Record(methodName, time.Since(t0))
		}
	}()
	entry, err := frontend.CompileBlock(fset, gen, body, env)
	if err != nil {
		return nil, err
	}
	entry, err = optimize.OptimizeCtx(gen, entry, mode, methodName, ir.DefaultTempMemoryBlock, optsLevel(opts), optsCtx(opts))
	if err != nil {
		return nil, err
	}
	sn, err = ir.CFGToSNode(gen, entry)
	return sn, err
}

// CompileWatchFile compiles a Go Watch-mode engine source file.
func CompileWatchFile(src string) (*resource.EngineWatchData, error) {
	return CompileWatchFileWithStats(src, nil)
}

// CompileWatchFileWithStats compiles a Go Watch-mode engine source file.
// If opts is non-nil and opts.Stats is non-nil, per-callback timing is recorded.
func CompileWatchFileWithStats(src string, opts *CompileOptions) (*resource.EngineWatchData, error) {
	p, err := runNonPlayPipeline(src, watchCBs, ir.ModeWatch, opts)
	if err != nil {
		return nil, err
	}

	outArcs := make([]resource.EngineWatchDataArchetype, len(p.arcData))
	for i, ad := range p.arcData {
		outArcs[i] = resource.EngineWatchDataArchetype{Name: ad.name, Imports: ad.imports}
	}

	updateSpawn, err := compileUpdateSpawn(p.gen, p.pf.fset, p.pf.funcs, p.accessors, opts, &p.nodes)
	if err != nil {
		return nil, err
	}
	if updateSpawn < 0 {
		updateSpawn = 0
	}

	if err := modecompile.Assemble(&p.nodes, outArcs, p.results, modecompile.NewCallbackSetter(watch.Setters)); err != nil {
		return nil, err
	}

	return &resource.EngineWatchData{
		Skin: p.r.skin, Effect: p.r.effect, Particle: p.r.particle, Buckets: p.r.buckets,
		Archetypes: outArcs, Nodes: p.nodes, UpdateSpawn: updateSpawn,
	}, nil
}

// CompilePreviewFile compiles a Go Preview-mode engine source file.
func CompilePreviewFile(src string) (*resource.EnginePreviewData, error) {
	return CompilePreviewFileWithStats(src, nil)
}

// CompilePreviewFileWithStats compiles a Go Preview-mode engine source file.
// If opts is non-nil and opts.Stats is non-nil, per-callback timing is recorded.
func CompilePreviewFileWithStats(src string, opts *CompileOptions) (*resource.EnginePreviewData, error) {
	p, err := runNonPlayPipeline(src, previewCBs, ir.ModePreview, opts)
	if err != nil {
		return nil, err
	}

	outArcs := make([]resource.EnginePreviewDataArchetype, len(p.arcData))
	for i, ad := range p.arcData {
		outArcs[i] = resource.EnginePreviewDataArchetype{Name: ad.name, Imports: ad.imports}
	}

	if err := modecompile.Assemble(&p.nodes, outArcs, p.results, modecompile.NewCallbackSetter(preview.Setters)); err != nil {
		return nil, err
	}

	return &resource.EnginePreviewData{Skin: p.r.skin, Archetypes: outArcs, Nodes: p.nodes}, nil
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

	if pp < 0 {
		pp = 0
	}
	if nav < 0 {
		nav = 0
	}
	if upd < 0 {
		upd = 0
	}

	return &resource.EngineTutorialData{
		Skin: r.skin, Effect: r.effect, Particle: r.particle, Instruction: r.instruction,
		Preprocess: pp, Navigate: nav, Update: upd, Nodes: nodes,
	}, nil
}
