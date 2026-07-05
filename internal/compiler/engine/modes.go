package engine

import (
	"errors"
	"fmt"
	"go/ast"
	"go/token"
	"sort"
	"sync"
	"time"

	"github.com/WindowsSov8forUs/sonolus-core-go/core/resource"

	"github.com/WindowsSov8forUs/sonolus-go/internal/compiler/frontend"
	"github.com/WindowsSov8forUs/sonolus-go/internal/compiler/ir"
	"github.com/WindowsSov8forUs/sonolus-go/internal/compiler/ir/optimize"
	"github.com/WindowsSov8forUs/sonolus-go/internal/compiler/modecompile"
	"github.com/WindowsSov8forUs/sonolus-go/internal/compiler/preview"
	"github.com/WindowsSov8forUs/sonolus-go/internal/compiler/snode"
	"github.com/WindowsSov8forUs/sonolus-go/internal/compiler/tutorial"
	"github.com/WindowsSov8forUs/sonolus-go/internal/compiler/watch"
)

type modeMethod struct {
	methodName string
	receiver   string
	body       *ast.BlockStmt
}

type modeArch struct {
	name       string
	imported   []ImportedField
	memory     []string
	data       []string
	shared     []string
	input      []string
	despawn    []string
	info       []string
	methods    []modeMethod
	containers []ContainerFieldMeta
}

var watchCallbacks = map[string]string{
	"Preprocess": "preprocess", "SpawnTime": "spawnTime", "DespawnTime": "despawnTime",
	"Initialize": "initialize", "UpdateSequential": "updateSequential",
	"UpdateParallel": "updateParallel", "Terminate": "terminate",
}

var previewCallbacks = map[string]string{
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
		a.containers = tc.containers
	}

	for _, m := range pes.methods {
		a := get(m.receiverType)
		if m.funcDecl.Body != nil {
			a.methods = append(a.methods, modeMethod{methodName: m.methodName, receiver: m.receiverName, body: m.funcDecl.Body})
		}
	}

	return &parsedModeFile{fset: pes.fset, arcs: arcs, order: order, funcs: pes.funcs, resources: pes.resources, uiVar: pes.uiVar}, nil
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
	stats := (*CompileStats)(nil)
	if opts != nil {
		stats = opts.Stats
	}
	defer func() {
		if stats != nil {
			stats.Record(methodName, time.Since(t0))
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
	p, err := runNonPlayPipeline(src, watchCallbacks, ir.ModeWatch, opts)
	if err != nil {
		return nil, err
	}
	return finishWatch(p, opts)
}

// CompileWatchSources compiles a multi-file engine source tree for Watch mode.
func CompileWatchSources(ess *EngineSources, opts *CompileOptions) (*resource.EngineWatchData, error) {
	p, err := runNonPlayPipelineSources(ess, watchCallbacks, ir.ModeWatch, opts)
	if err != nil {
		return nil, err
	}
	return finishWatch(p, opts)
}

func finishWatch(p nonPlayPipelineResult, opts *CompileOptions) (*resource.EngineWatchData, error) {
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
	p, err := runNonPlayPipeline(src, previewCallbacks, ir.ModePreview, opts)
	if err != nil {
		return nil, err
	}
	return finishPreview(p)
}

// CompilePreviewSources compiles a multi-file engine source tree for Preview mode.
func CompilePreviewSources(ess *EngineSources, opts *CompileOptions) (*resource.EnginePreviewData, error) {
	p, err := runNonPlayPipelineSources(ess, previewCallbacks, ir.ModePreview, opts)
	if err != nil {
		return nil, err
	}
	return finishPreview(p)
}

func finishPreview(p nonPlayPipelineResult) (*resource.EnginePreviewData, error) {
	outArcs := make([]resource.EnginePreviewDataArchetype, len(p.arcData))
	for i, ad := range p.arcData {
		outArcs[i] = resource.EnginePreviewDataArchetype{Name: ad.name, Imports: ad.imports}
	}

	if err := modecompile.Assemble(&p.nodes, outArcs, p.results, modecompile.NewCallbackSetter(preview.Setters)); err != nil {
		return nil, err
	}

	return &resource.EnginePreviewData{Skin: p.r.skin, Archetypes: outArcs, Nodes: p.nodes}, nil
}

// callbackJob describes one callback body to compile in parallel.
type callbackJob struct {
	name string         // Go method name (for error messages)
	cb   string         // Sonolus callback name (e.g. "Preprocess")
	body *ast.BlockStmt // function body to compile
	env  frontend.Env   // compilation environment
}

// compileCallbacksParallel compiles a set of callback bodies in parallel.
// Each callback gets its own IDGen for deterministic compilation. Results
// are collected into a map keyed by callback name, suitable for
// deterministic (sorted) assembly after all goroutines complete.
func compileCallbacksParallel(
	fset *token.FileSet,
	jobs []callbackJob,
	mode ir.Mode,
	opts *CompileOptions,
) (map[string][]snode.SNode, error) {
	type compiled struct {
		cb string
		sn snode.SNode
	}
	ch := make(chan compiled, len(jobs))
	errCh := make(chan error, len(jobs))
	var wg sync.WaitGroup

	for _, job := range jobs {
		wg.Add(1)
		go func(j callbackJob) {
			defer wg.Done()
			sn, err := compileCallbackBlock(
				ir.NewIDGen(), fset, j.body, j.env,
				j.name, mode, opts,
			)
			if err != nil {
				errCh <- fmt.Errorf("%s %q: %w", j.cb, j.name, err)
				return
			}
			ch <- compiled{cb: j.cb, sn: sn}
		}(job)
	}
	wg.Wait()
	close(ch)
	close(errCh)

	var errs []error
	for err := range errCh {
		errs = append(errs, err)
	}
	if len(errs) > 0 {
		return nil, errors.Join(errs...)
	}

	result := make(map[string][]snode.SNode)
	for c := range ch {
		result[c.cb] = append(result[c.cb], c.sn)
	}
	return result, nil
}

// sortedFuncDecls returns the function declarations from a map sorted by name
// for deterministic iteration order.
func sortedFuncDecls(funcs map[string]*ast.FuncDecl) []*ast.FuncDecl {
	sorted := make([]*ast.FuncDecl, 0, len(funcs))
	for _, d := range funcs {
		sorted = append(sorted, d)
	}
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Name.Name < sorted[j].Name.Name
	})
	return sorted
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
	if _, err := buildResources(pf.resources, pf.uiVar); err != nil {
		return nil, err
	}
	return finishTutorial(pf.fset, pf.funcs, pf.resources, opts)
}

// CompileTutorialSources compiles a multi-file engine source tree for Tutorial mode.
func CompileTutorialSources(ess *EngineSources, opts *CompileOptions) (*resource.EngineTutorialData, error) {
	if err := ess.ResolveImports(); err != nil {
		return nil, err
	}
	if _, _, _, err := frontend.TypeCheckEngine(ess.Access()); err != nil {
		return nil, fmt.Errorf("typecheck: %w", err)
	}

	mainPES, err := parseEngineSourceFiles(ess.Main, true)
	if err != nil {
		return nil, err
	}

	// Merge free functions from imported packages.
	allFuncs := make(map[string]*ast.FuncDecl)
	for k, v := range mainPES.funcs {
		allFuncs[k] = v
	}
	for impPath, files := range ess.Imports {
		_, impPES, err := parseImportedPackage(files)
		if err != nil {
			return nil, fmt.Errorf("import %q: %w", impPath, err)
		}
		for k, v := range impPES.funcs {
			if _, dup := allFuncs[k]; dup {
				return nil, fmt.Errorf("duplicate function %q in import %q and main package", k, impPath)
			}
			allFuncs[k] = v
		}
	}

	return finishTutorial(mainPES.fset, allFuncs, mainPES.resources, opts)
}

func finishTutorial(fset *token.FileSet, funcs map[string]*ast.FuncDecl, resources map[string]*ast.StructType, opts *CompileOptions) (*resource.EngineTutorialData, error) {
	r, err := buildResources(resources, nil)
	if err != nil {
		return nil, err
	}
	accessors := frontend.ModeAccessorsReadOnly(ir.ModeTutorial)
	nodes := []resource.EngineDataNode{}
	app := snode.NewAppender(&nodes)

	// Build job list in deterministic (sorted) order.
	var jobs []callbackJob
	for _, d := range sortedFuncDecls(funcs) {
		var cb string
		switch d.Name.Name {
		case "Preprocess":
			cb = string(tutorial.CallbackPreprocess)
		case "Navigate":
			cb = string(tutorial.CallbackNavigate)
		case "Update":
			cb = string(tutorial.CallbackUpdate)
		default:
			continue
		}
		if d.Body == nil {
			continue
		}
		jobs = append(jobs, callbackJob{
			name: d.Name.Name, cb: cb, body: d.Body,
			env: frontend.Env{
				Names: accessors, Funcs: funcs,
				Accessors: accessors, Mode: ir.ModeTutorial,
					Constants: map[string]float64{
						"entityStateWaiting":   0,
						"entityStateActive":    1,
						"entityStateDespawned": 2,
					},
			},
		})
	}

	compiled, err := compileCallbacksParallel(fset, jobs, ir.ModeTutorial, opts)
	if err != nil {
		return nil, err
	}

	// Deterministic assembly by callback name order.
	var ppIdxs, navIdxs, updIdxs []int
	for _, cb := range []string{string(tutorial.CallbackPreprocess), string(tutorial.CallbackNavigate), string(tutorial.CallbackUpdate)} {
		for _, sn := range compiled[cb] {
			r := modecompile.CompileCallbackForMode(-1, cb, sn, "tutorial")
			if r == nil {
				continue
			}
			idx, err := app.Append(r.Node)
			if err != nil {
				return nil, err
			}
			switch cb {
			case string(tutorial.CallbackPreprocess):
				ppIdxs = append(ppIdxs, idx)
			case string(tutorial.CallbackNavigate):
				navIdxs = append(navIdxs, idx)
			case string(tutorial.CallbackUpdate):
				updIdxs = append(updIdxs, idx)
			}
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
