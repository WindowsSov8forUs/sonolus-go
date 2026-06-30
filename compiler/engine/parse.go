package engine

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"reflect"

	"github.com/WindowsSov8forUs/sonolus-core-go/core/resource"

	"github.com/WindowsSov8forUs/sonolus-go/compiler/frontend"
	"github.com/WindowsSov8forUs/sonolus-go/compiler/ir"
	"github.com/WindowsSov8forUs/sonolus-go/compiler/modecompile"
	"github.com/WindowsSov8forUs/sonolus-go/compiler/play"
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
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "engine.go", src, 0)
	if err != nil {
		return nil, nil, fmt.Errorf("parse: %w", err)
	}

	archetypes := map[string]*parsedArchetype{}
	var order []string
	funcs := map[string]*ast.FuncDecl{}
	resourceASTs := map[string]*ast.StructType{}

	get := func(name string) *parsedArchetype {
		a, ok := archetypes[name]
		if !ok {
			a = &parsedArchetype{name: name, helpers: map[string]*ast.FuncDecl{}}
			archetypes[name] = a
			order = append(order, name)
		}
		return a
	}

	for _, decl := range file.Decls {
		switch d := decl.(type) {
		case *ast.GenDecl:
			for _, spec := range d.Specs {
				ts, ok := spec.(*ast.TypeSpec)
				if !ok {
					continue
				}
				st, ok := ts.Type.(*ast.StructType)
				if !ok {
					continue
				}
				if resourceRole(ts.Name.Name) != "" {
					resourceASTs[ts.Name.Name] = st
				} else if err := parseFields(get(ts.Name.Name), st); err != nil {
					return nil, nil, err
				}
			}
		case *ast.FuncDecl:
			if d.Recv == nil || len(d.Recv.List) == 0 {
				if d.Body != nil {
					funcs[d.Name.Name] = d
				}
				continue
			}
			typeName, recvName := receiverInfo(d.Recv.List[0])
			if typeName == "" {
				continue
			}
			a := get(typeName)
			if cb, ok := methodCallbacks[d.Name.Name]; ok {
				a.methods = append(a.methods, parsedMethod{callback: cb, receiver: recvName, body: d.Body})
			} else if d.Body != nil {
				a.helpers[d.Name.Name] = d
			}
		}
	}

	r, err := buildResources(resourceASTs)
	if err != nil {
		return nil, nil, err
	}
	return compileParsed(fset, archetypes, order, funcs, r, opts)
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
	tc := &tagCollector{
		imported: &a.imported,
		memory:   &a.memory,
		data:     &a.data,
		shared:   &a.shared,
		input:    &a.input,
		despawn:  &a.despawn,
		info:     &a.info,
	}
	for _, f := range st.Fields.List {
		unknown := collectSonolusTags(f, tc)
		if unknown != "" {
			// The first name in the field carries the tag.
			name := ""
			if len(f.Names) > 0 {
				name = f.Names[0].Name
			}
			return fmt.Errorf("archetype %q field %q: unknown sonolus tag %q", a.name, name, unknown)
		}
		if f.Tag == nil || len(f.Names) == 0 {
			continue
		}
		tag := reflect.StructTag(stringLit(f.Tag.Value)).Get("sonolus")
		switch tag {
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
		for _, m := range a.methods {
			env := frontend.Env{
				Names: names, Receiver: m.receiver, Funcs: funcs, Methods: a.helpers,
				Accessors: frontend.ModeAccessors(ir.ModePlay),
				Mode:      ir.ModePlay,
			}
			sn, err := compileCallbackBlock(gen, fset, m.body, env, string(m.callback), ir.ModePlay, opts)
			if err != nil {
				return nil, nil, fmt.Errorf("archetype %q callback %q: %w", a.name, m.callback, err)
			}
			results = append(results, play.CompileCallback(i, m.callback, sn))
		}
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
