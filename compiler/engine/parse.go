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
	"github.com/WindowsSov8forUs/sonolus-go/compiler/ir/optimize"
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

func CompilePlayFile(src string) (*resource.EnginePlayData, error) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "engine.go", src, 0)
	if err != nil {
		return nil, fmt.Errorf("parse: %w", err)
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
					return nil, err
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

	r := buildResources(resourceASTs)
	return compileParsed(fset, archetypes, order, funcs, r)
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
	for _, f := range st.Fields.List {
		if f.Tag == nil || len(f.Names) == 0 {
			continue
		}
		tag := reflect.StructTag(stringLit(f.Tag.Value)).Get("sonolus")
		for _, name := range f.Names {
			switch tag {
			case "imported":
				a.imported = append(a.imported, ImportedField{Name: name.Name})
			case "memory":
				a.memory = append(a.memory, name.Name)
			case "exported":
				a.exported = append(a.exported, name.Name)
			case "data":
				a.data = append(a.data, name.Name)
			case "shared":
				a.shared = append(a.shared, name.Name)
			case "input":
				a.input = append(a.input, name.Name)
			case "despawn":
				a.despawn = append(a.despawn, name.Name)
			case "info":
				a.info = append(a.info, name.Name)
			case "scored":
				a.scored = true
			case "lifed":
				a.lifed = true
			case "":
			default:
				return fmt.Errorf("archetype %q field %q: unknown sonolus tag %q", a.name, name.Name, tag)
			}
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
) (*resource.EnginePlayData, error) {
	defs := make([]play.ArchetypeDef, len(order))
	bindings := make([]map[string]frontend.Binding, len(order))
	for i, name := range order {
		a := archetypes[name]
		imports := make([]resource.EngineDataArchetypeImport, len(a.imported))
		exports := make([]resource.EngineArchetypeDataName, len(a.exported))
		b := map[string]frontend.Binding{}
		for j, f := range a.imported {
			imports[j] = resource.EngineDataArchetypeImport{
				Name: resource.EngineArchetypeDataName(f.Name), Index: j, Def: f.Def,
			}
			b[f.Name] = frontend.Binding{Block: entityMemoryBlock, Index: j, Writable: false}
		}
		for k, m := range a.memory {
			b[m] = frontend.Binding{Block: entityMemoryBlock, Index: len(a.imported) + k, Writable: true}
		}
		for ek, en := range a.exported {
			exports[ek] = resource.EngineArchetypeDataName(en)
			b[en] = frontend.Binding{Block: -1, Index: ek, Writable: true}
		}
		for di, dn := range a.data {
			b[dn] = frontend.Binding{Block: entityDataBlock, Index: di, Writable: false}
		}
		for si, sn := range a.shared {
			b[sn] = frontend.Binding{Block: entitySharedBlock, Index: si, Writable: true}
		}
		for ii, in := range a.input {
			b[in] = frontend.Binding{Block: entityInputBlock, Index: ii, Writable: true}
		}
		for di, dn := range a.despawn {
			b[dn] = frontend.Binding{Block: entityDespawnBlock, Index: di, Writable: true}
		}
		for ii, in := range a.info {
			b[in] = frontend.Binding{Block: entityInfoBlock, Index: ii, Writable: false}
		}
		if a.scored || a.lifed {
			ArchetypeScoreLife(b, a.scored, a.lifed)
		}
		bindings[i] = b
		defs[i] = play.ArchetypeDef{Name: a.name, HasInput: hasTouch(a), Imports: imports, Exports: exports}
	}

	data := play.BuildPlayData(r.skin, r.effect, r.particle, r.buckets, defs)

	var results []*play.CompileResult
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
			}
			entry, err := frontend.CompileBlock(fset, m.body, env)
			if err != nil {
				return nil, fmt.Errorf("archetype %q callback %q: %w", a.name, m.callback, err)
			}
			entry = optimize.Optimize(entry, ir.ModePlay, string(m.callback), ir.DefaultTempMemoryBlock)
			results = append(results, play.CompileCallback(i, m.callback, ir.CFGToSNode(entry), 0))
		}
	}

	if err := play.Assemble(data, results); err != nil {
		return nil, err
	}
	return data, nil
}

func hasTouch(a *parsedArchetype) bool {
	for _, m := range a.methods {
		if m.callback == play.CallbackTouch {
			return true
		}
	}
	return false
}
