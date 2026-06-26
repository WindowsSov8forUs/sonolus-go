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

// methodCallbacks maps Go method names to play-mode callbacks.
var methodCallbacks = map[string]play.Callback{
	"Preprocess":       play.CallbackPreprocess,
	"SpawnOrder":       play.CallbackSpawnOrder,
	"ShouldSpawn":      play.CallbackShouldSpawn,
	"Initialize":       play.CallbackInitialize,
	"UpdateSequential": play.CallbackUpdateSequential,
	"Touch":            play.CallbackTouch,
	"UpdateParallel":   play.CallbackUpdateParallel,
	"Terminate":        play.CallbackTerminate,
}

type parsedArchetype struct {
	name     string
	imported []ImportedField
	memory   []string
	methods  []parsedMethod
}

type parsedMethod struct {
	callback play.Callback
	receiver string
	body     *ast.BlockStmt
}

// CompilePlayFile parses a Go engine source file and compiles it to
// EnginePlayData. Each struct type is an archetype: fields tagged
// `sonolus:"imported"` / `sonolus:"memory"` become entity fields, and methods
// named after callbacks (Initialize, UpdateParallel, ...) become callbacks whose
// bodies reference fields via the receiver (e.g. n.TargetTime).
func CompilePlayFile(src string) (*resource.EnginePlayData, error) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "engine.go", src, 0)
	if err != nil {
		return nil, fmt.Errorf("parse: %w", err)
	}

	archetypes := map[string]*parsedArchetype{}
	var order []string

	get := func(name string) *parsedArchetype {
		a, ok := archetypes[name]
		if !ok {
			a = &parsedArchetype{name: name}
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
				a := get(ts.Name.Name)
				if err := parseFields(a, st); err != nil {
					return nil, err
				}
			}
		case *ast.FuncDecl:
			if d.Recv == nil || len(d.Recv.List) == 0 {
				continue
			}
			cb, ok := methodCallbacks[d.Name.Name]
			if !ok {
				continue // not a callback method
			}
			typeName, recvName := receiverInfo(d.Recv.List[0])
			if typeName == "" {
				continue
			}
			a := get(typeName)
			a.methods = append(a.methods, parsedMethod{callback: cb, receiver: recvName, body: d.Body})
		}
	}

	return compileParsed(fset, archetypes, order)
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
	return typeName, recvName
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
			case "":
				// untagged field: ignored
			default:
				return fmt.Errorf("archetype %q field %q: unknown sonolus tag %q", a.name, name.Name, tag)
			}
		}
	}
	return nil
}

// stringLit strips the surrounding backticks/quotes from a raw tag literal.
func stringLit(s string) string {
	if len(s) >= 2 {
		return s[1 : len(s)-1]
	}
	return s
}

func compileParsed(fset *token.FileSet, archetypes map[string]*parsedArchetype, order []string) (*resource.EnginePlayData, error) {
	defs := make([]play.ArchetypeDef, len(order))
	bindings := make([]map[string]frontend.Binding, len(order))
	for i, name := range order {
		a := archetypes[name]
		imports := make([]resource.EngineDataArchetypeImport, len(a.imported))
		b := map[string]frontend.Binding{}
		for j, f := range a.imported {
			imports[j] = resource.EngineDataArchetypeImport{
				Name:  resource.EngineArchetypeDataName(f.Name),
				Index: j,
				Def:   f.Def,
			}
			b[f.Name] = frontend.Binding{Block: entityMemoryBlock, Index: j, Writable: false}
		}
		for k, m := range a.memory {
			b[m] = frontend.Binding{Block: entityMemoryBlock, Index: len(a.imported) + k, Writable: true}
		}
		bindings[i] = b
		defs[i] = play.ArchetypeDef{Name: a.name, HasInput: hasTouch(a), Imports: imports}
	}

	data := play.BuildPlayData(resource.EngineSkinData{}, resource.EngineEffectData{}, resource.EngineParticleData{}, nil, defs)

	var results []*play.CompileResult
	for i, name := range order {
		a := archetypes[name]
		names := frontend.ModeAccessors(ir.ModePlay)
		for k, v := range bindings[i] {
			names[k] = v
		}
		for _, m := range a.methods {
			env := frontend.Env{Names: names, Receiver: m.receiver}
			entry, err := frontend.CompileBlock(fset, m.body, env)
			if err != nil {
				return nil, fmt.Errorf("archetype %q callback %q: %w", a.name, m.callback, err)
			}
			entry = optimize.Optimize(entry, ir.ModePlay, string(m.callback), ir.DefaultTempMemoryBlock)
			node := ir.CFGToSNode(entry)
			results = append(results, play.CompileCallback(i, m.callback, node, 0))
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
