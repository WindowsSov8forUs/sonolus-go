package engine

import (
	"fmt"
	"go/ast"
	"go/token"

	"github.com/WindowsSov8forUs/sonolus-core-go/core/resource"

	"github.com/WindowsSov8forUs/sonolus-go/compiler/frontend"
	"github.com/WindowsSov8forUs/sonolus-go/compiler/ir"
	"github.com/WindowsSov8forUs/sonolus-go/compiler/modecompile"
	"github.com/WindowsSov8forUs/sonolus-go/compiler/play"
	"github.com/WindowsSov8forUs/sonolus-go/compiler/snode"
)

var methodCallbacks = map[string]play.Callback{
	"Preprocess": play.CallbackPreprocess, "SpawnOrder": play.CallbackSpawnOrder,
	"ShouldSpawn": play.CallbackShouldSpawn, "Initialize": play.CallbackInitialize,
	"UpdateSequential": play.CallbackUpdateSequential, "Touch": play.CallbackTouch,
	"UpdateParallel": play.CallbackUpdateParallel, "Terminate": play.CallbackTerminate,
}

type parsedArchetype struct {
	name     string
	imported []ImportedField
	memory   []string
	exported []string
	data     []string
	shared   []string
	input    []string
	despawn  []string
	info     []string
	scored   bool
	lifed    bool
	methods  []parsedMethod
	helpers  map[string]*ast.FuncDecl
}

type parsedMethod struct {
	callback play.Callback
	receiver string
	body     *ast.BlockStmt
}

// CompilePlayFile compiles a Go Play-mode engine source file and returns the
// EnginePlayData and EngineConfiguration.
func CompilePlayFile(src string) (*resource.EnginePlayData, *resource.EngineConfiguration, error) {
	return CompilePlayFileWithStats(src, nil)
}

// CompilePlayFileWithStats compiles a Go Play-mode engine source file. If opts
// is non-nil and opts.Stats is non-nil, per-callback compilation timing is
// recorded.
func CompilePlayFileWithStats(src string, opts *CompileOptions) (*resource.EnginePlayData, *resource.EngineConfiguration, error) {
	pes, err := parseEngineSource(src)
	if err != nil {
		return nil, nil, err
	}

	// Run type checking for early diagnostics (D1 layer): catch undeclared
	// identifiers and wrong argument counts before compilation.
	if _, _, _, err := frontend.TypeCheck(src, nil); err != nil {
		return nil, nil, fmt.Errorf("typecheck: %w", err)
	}

	archetypes := map[string]*parsedArchetype{}
	var order []string

	get := func(name string) *parsedArchetype {
		a, ok := archetypes[name]
		if !ok {
			a = &parsedArchetype{name: name, helpers: map[string]*ast.FuncDecl{}}
			archetypes[name] = a
			order = append(order, name)
		}
		return a
	}

	for _, td := range pes.typeDecls {
		if err := parseFields(get(td.name), td.structType); err != nil {
			return nil, nil, err
		}
	}

	for _, m := range pes.methods {
		a := get(m.receiverType)
		if cb, ok := methodCallbacks[m.methodName]; ok {
			a.methods = append(a.methods, parsedMethod{callback: cb, receiver: m.receiverName, body: m.funcDecl.Body})
		} else if m.funcDecl.Body != nil {
			a.helpers[m.methodName] = m.funcDecl
		}
	}

	r, err := buildResources(pes.resources)
	if err != nil {
		return nil, nil, err
	}
	return compileParsed(pes.fset, archetypes, order, pes.funcs, r, opts)
}

func receiverInfo(field *ast.Field) (typeName, recvName string) {
	if len(field.Names) > 0 {
		recvName = field.Names[0].Name
	}
	switch t := field.Type.(type) {
	case *ast.Ident:
		typeName = t.Name
	case *ast.StarExpr:
		if id, ok := t.X.(*ast.Ident); ok {
			typeName = id.Name
		}
	}
	return
}

func parseFields(a *parsedArchetype, st *ast.StructType) error {
	tc := &tagCollector{}
	for _, f := range st.Fields.List {
		unknown, modeTag := tc.collectSonolusTags(f)
		if unknown != "" {
			// The first name in the field carries the tag.
			name := ""
			if len(f.Names) > 0 {
				name = f.Names[0].Name
			}
			return fmt.Errorf("archetype %q field %q: unknown sonolus tag %q", a.name, name, unknown)
		}
		switch modeTag {
		case "exported":
			for _, name := range f.Names {
				a.exported = append(a.exported, name.Name)
			}
		case "scored":
			a.scored = true
		case "lifed":
			a.lifed = true
		}
	}
	a.imported = tc.imported
	a.memory = tc.memory
	a.data = tc.data
	a.shared = tc.shared
	a.input = tc.input
	a.despawn = tc.despawn
	a.info = tc.info
	return nil
}

func stringLit(s string) string {
	if len(s) >= 2 {
		return s[1 : len(s)-1]
	}
	return s
}

func compileParsed(
	fset *token.FileSet,
	archetypes map[string]*parsedArchetype, order []string,
	funcs map[string]*ast.FuncDecl,
	r parsedResources,
	opts *CompileOptions,
) (*resource.EnginePlayData, *resource.EngineConfiguration, error) {
	gen := ir.NewIDGen()
	defs := make([]play.ArchetypeDef, len(order))
	bindings := make([]map[string]frontend.Binding, len(order))
	for i, name := range order {
		a := archetypes[name]
		exports := make([]resource.EngineArchetypeDataName, len(a.exported))
		imports, b := buildBindings(a.imported, a.memory, a.exported, a.data, a.shared, a.input, a.despawn, a.info,
			func(name string, idx int) {
				exports[idx] = resource.EngineArchetypeDataName(name)
			})
		if a.scored || a.lifed {
			ArchetypeScoreLife(b, a.scored, a.lifed)
		}
		bindings[i] = b
		defs[i] = play.ArchetypeDef{Name: a.name, HasInput: hasTouch(a), Imports: imports, Exports: exports}
	}

	data := play.BuildPlayData(r.skin, r.effect, r.particle, r.buckets, defs)

	var results []*modecompile.Result
	for i, name := range order {
		a := archetypes[name]
		names := frontend.ModeAccessors(ir.ModePlay)
		for k, v := range bindings[i] {
			names[k] = v
		}
		var cms []callbackMethod
		for _, m := range a.methods {
			cms = append(cms, callbackMethod{name: string(m.callback), receiver: m.receiver, body: m.body})
		}
		envBuilder := func(receiver string) frontend.Env {
			return frontend.Env{
				Names: names, Receiver: receiver, Funcs: funcs, Methods: a.helpers,
				Accessors: frontend.ModeAccessorsReadOnly(ir.ModePlay),
				Mode:      ir.ModePlay,
			}
		}
		resultFn := func(idx int, cb string, sn snode.SNode) *modecompile.Result {
			return modecompile.CompileCallbackForMode(idx, cb, sn, "play")
		}
		ctx := compileCtx{gen: gen, fset: fset, mode: ir.ModePlay, opts: opts}
		r, err := ctx.compileMethodCallbacks(cms, a.name, i, envBuilder, resultFn)
		if err != nil {
			return nil, nil, err
		}
		results = append(results, r...)
	}

	if err := play.Assemble(data, results); err != nil {
		return nil, nil, err
	}
	return data, &r.config, nil
}

func hasTouch(a *parsedArchetype) bool {
	for _, m := range a.methods {
		if m.callback == play.CallbackTouch {
			return true
		}
	}
	return false
}
