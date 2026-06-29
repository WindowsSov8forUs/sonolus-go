package engine

import (
	"fmt"
	"go/ast"
	"sort"
	"go/parser"
	"go/token"
	"reflect"

	"github.com/WindowsSov8forUs/sonolus-core-go/core/resource"

	"github.com/WindowsSov8forUs/sonolus-go/compiler/frontend"
	"github.com/WindowsSov8forUs/sonolus-go/compiler/ir"
	"github.com/WindowsSov8forUs/sonolus-go/compiler/ir/optimize"
	"github.com/WindowsSov8forUs/sonolus-go/compiler/snode"
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

func parseModeFile(src string) (*token.FileSet, map[string]*modeArch, []string, map[string]*ast.FuncDecl, map[string]*ast.StructType, error) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "engine.go", src, 0)
	if err != nil {
		return nil, nil, nil, nil, nil, fmt.Errorf("parse: %w", err)
	}

	arcs := map[string]*modeArch{}
	var order []string
	funcs := map[string]*ast.FuncDecl{}
	resources := map[string]*ast.StructType{}

	get := func(name string) *modeArch {
		a, ok := arcs[name]
		if !ok {
			a = &modeArch{name: name}
			arcs[name] = a
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
					resources[ts.Name.Name] = st
					continue
				}
				a := get(ts.Name.Name)
				for _, f := range st.Fields.List {
					if f.Tag == nil || len(f.Names) == 0 {
						continue
					}
					switch reflect.StructTag(stringLit(f.Tag.Value)).Get("sonolus") {
					case "imported":
						for _, n := range f.Names {
							a.imported = append(a.imported, ImportedField{Name: n.Name})
						}
					case "memory":
						for _, n := range f.Names {
							a.memory = append(a.memory, n.Name)
						}
					case "data":
						for _, n := range f.Names {
							a.data = append(a.data, n.Name)
						}
					case "shared":
						for _, n := range f.Names {
							a.shared = append(a.shared, n.Name)
						}
					case "input":
						for _, n := range f.Names {
							a.input = append(a.input, n.Name)
						}
					case "despawn":
						for _, n := range f.Names {
							a.despawn = append(a.despawn, n.Name)
						}
					case "info":
						for _, n := range f.Names {
							a.info = append(a.info, n.Name)
						}
					}
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
			if d.Body != nil {
				a.methods = append(a.methods, modeMethod{methodName: d.Name.Name, receiver: recvName, body: d.Body})
			}
		}
	}
	return fset, arcs, order, funcs, resources, nil
}

func modeBindings(a *modeArch) ([]resource.EngineDataArchetypeImport, map[string]frontend.Binding) {
	imports := make([]resource.EngineDataArchetypeImport, len(a.imported))
	b := map[string]frontend.Binding{}
	for j, f := range a.imported {
		imports[j] = resource.EngineDataArchetypeImport{Name: resource.EngineArchetypeDataName(f.Name), Index: j, Def: f.Def}
		b[f.Name] = frontend.Binding{Block: entityMemoryBlock, Index: j, Writable: false}
	}
	for k, m := range a.memory {
		b[m] = frontend.Binding{Block: entityMemoryBlock, Index: len(a.imported) + k, Writable: true}
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
	return imports, b
}

func CompileWatchFile(src string) (*resource.EngineWatchData, error) {
	fset, arcs, order, funcs, resources, err := parseModeFile(src)
	if err != nil {
		return nil, err
	}
	gen := ir.NewIDGen()
	r, err := buildResources(resources)
	if err != nil {
		return nil, err
	}
	accessors := frontend.ModeAccessors(ir.ModeWatch)
	nodes := []resource.EngineDataNode{}
	app := snode.NewAppender(&nodes)
	outArcs := make([]resource.EngineWatchDataArchetype, 0, len(order))

	for _, name := range order {
		a := arcs[name]
		imports, b := modeBindings(a)
		arc := resource.EngineWatchDataArchetype{Name: resource.EngineArchetypeName(name), Imports: imports}
		names := copyBC(accessors)
		for k, v := range b {
			names[k] = v
		}
		for _, m := range a.methods {
			cb, ok := watchCBs[m.methodName]
			if !ok {
				continue
			}
			env := frontend.Env{Names: names, Receiver: m.receiver, Funcs: funcs, Accessors: accessors, Mode: ir.ModeWatch}
			entry, err := frontend.CompileBlock(fset, gen, m.body, env)
			if err != nil {
				return nil, fmt.Errorf("archetype %q: %w", name, err)
			}
			entry, err = optimize.Optimize(gen, entry, ir.ModeWatch, m.methodName, ir.DefaultTempMemoryBlock, optimize.LevelStandard)
			if err != nil {
				return nil, fmt.Errorf("archetype %q callback %s: %w", name, m.methodName, err)
			}
			idx, _ := app.Append(ir.CFGToSNode(gen, entry))
			c := resource.EngineWatchDataArchetypeCallback{Index: idx}
			switch cb {
			case "preprocess":
				arc.Preprocess = &c
			case "spawnTime":
				arc.SpawnTime = &c
			case "despawnTime":
				arc.DespawnTime = &c
			case "initialize":
				arc.Initialize = &c
			case "updateSequential":
				arc.UpdateSequential = &c
			case "updateParallel":
				arc.UpdateParallel = &c
			case "terminate":
				arc.Terminate = &c
			}
		}
		outArcs = append(outArcs, arc)
	}

	// Global callbacks (UpdateSpawn).
	var updateSpawn int
	for _, d := range funcs {
		if d.Name.Name == "UpdateSpawn" && d.Body != nil {
			env := frontend.Env{Names: accessors, Funcs: funcs, Accessors: accessors, Mode: ir.ModeWatch}
			entry, err := frontend.CompileBlock(fset, gen, d.Body, env)
			if err != nil {
				return nil, fmt.Errorf("UpdateSpawn: %w", err)
			}
			entry, err = optimize.Optimize(gen, entry, ir.ModeWatch, "UpdateSpawn", ir.DefaultTempMemoryBlock, optimize.LevelStandard)
			if err != nil {
				return nil, fmt.Errorf("UpdateSpawn: %w", err)
			}
			updateSpawn, _ = app.Append(ir.CFGToSNode(gen, entry))
			break
		}
	}

	return &resource.EngineWatchData{
		Skin: r.skin, Effect: r.effect, Particle: r.particle, Buckets: r.buckets,
		Archetypes: outArcs, Nodes: nodes, UpdateSpawn: updateSpawn,
	}, nil
}

func CompilePreviewFile(src string) (*resource.EnginePreviewData, error) {
	fset, arcs, order, funcs, resources, err := parseModeFile(src)
	if err != nil {
		return nil, err
	}
	gen := ir.NewIDGen()
	r, err := buildResources(resources)
	if err != nil {
		return nil, err
	}
	accessors := frontend.ModeAccessors(ir.ModePreview)
	nodes := []resource.EngineDataNode{}
	app := snode.NewAppender(&nodes)
	outArcs := make([]resource.EnginePreviewDataArchetype, 0, len(order))

	for _, name := range order {
		a := arcs[name]
		imports, b := modeBindings(a)
		arc := resource.EnginePreviewDataArchetype{Name: resource.EngineArchetypeName(name), Imports: imports}
		names := copyBC(accessors)
		for k, v := range b {
			names[k] = v
		}
		for _, m := range a.methods {
			cb, ok := previewCBs[m.methodName]
			if !ok {
				continue
			}
			env := frontend.Env{Names: names, Receiver: m.receiver, Funcs: funcs, Accessors: accessors, Mode: ir.ModePreview}
			entry, err := frontend.CompileBlock(fset, gen, m.body, env)
			if err != nil {
				return nil, fmt.Errorf("archetype %q: %w", name, err)
			}
			entry, err = optimize.Optimize(gen, entry, ir.ModePreview, m.methodName, ir.DefaultTempMemoryBlock, optimize.LevelStandard)
			if err != nil {
				return nil, fmt.Errorf("archetype %q callback %s: %w", name, m.methodName, err)
			}
			idx, _ := app.Append(ir.CFGToSNode(gen, entry))
			c := resource.EnginePreviewDataArchetypeCallback{Index: idx}
			switch cb {
			case "preprocess":
				arc.Preprocess = &c
			case "render":
				arc.Render = &c
			}
		}
		outArcs = append(outArcs, arc)
	}

	return &resource.EnginePreviewData{Skin: r.skin, Archetypes: outArcs, Nodes: nodes}, nil
}

func CompileTutorialFile(src string) (*resource.EngineTutorialData, error) {
	fset, _, _, funcs, resources, err := parseModeFile(src)
	if err != nil {
		return nil, err
	}
	gen := ir.NewIDGen()
	r, err := buildResources(resources)
	if err != nil {
		return nil, err
	}
	accessors := frontend.ModeAccessors(ir.ModeTutorial)
	nodes := []resource.EngineDataNode{}
	app := snode.NewAppender(&nodes)

	var pp, nav, upd int
	sortedFuncs := make([]*ast.FuncDecl, 0, len(funcs))
	for _, d := range funcs {
		sortedFuncs = append(sortedFuncs, d)
	}
	sort.Slice(sortedFuncs, func(i, j int) bool {
		return sortedFuncs[i].Name.Name < sortedFuncs[j].Name.Name
	})
	for _, d := range sortedFuncs {
		cb := ""
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
		env := frontend.Env{Names: accessors, Funcs: funcs, Accessors: accessors, Mode: ir.ModeTutorial}
		entry, err := frontend.CompileBlock(fset, gen, d.Body, env)
		if err != nil {
			return nil, fmt.Errorf("tutorial %q: %w", d.Name.Name, err)
		}
		entry, err = optimize.Optimize(gen, entry, ir.ModeTutorial, d.Name.Name, ir.DefaultTempMemoryBlock, optimize.LevelStandard)
		if err != nil {
			return nil, fmt.Errorf("tutorial %q: %w", d.Name.Name, err)
		}
		idx, _ := app.Append(ir.CFGToSNode(gen, entry))
		switch cb {
		case "Preprocess":
			pp = idx
		case "Navigate":
			nav = idx
		case "Update":
			upd = idx
		}
	}

	return &resource.EngineTutorialData{
		Skin: r.skin, Effect: r.effect, Particle: r.particle, Instruction: r.instruction,
		Preprocess: pp, Navigate: nav, Update: upd, Nodes: nodes,
	}, nil
}

func copyBC(src map[string]frontend.Binding) map[string]frontend.Binding {
	out := map[string]frontend.Binding{}
	for k, v := range src {
		out[k] = v
	}
	return out
}
