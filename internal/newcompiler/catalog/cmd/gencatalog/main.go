package main

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

type entry struct {
	pkg, name, receiver, kind, signature, modes, phases, effect, runtime, source string
	internal                                                                     bool
}

func semantic(item entry) entry {
	if item.kind != "KindFunction" && item.kind != "KindMethod" && item.kind != "KindNative" {
		return item
	}
	if item.kind == "KindNative" {
		// Native calls expose raw runtime operations. Their exact callback legality is
		// represented by the high-level APIs which use them; direct native calls are
		// intentionally left unrestricted for the future lowerer.
		readPrefixes := []string{"Get", "Has", "StreamGet", "StreamHas", "StackGet"}
		for _, prefix := range readPrefixes {
			if strings.HasPrefix(item.name, prefix) {
				item.effect = "EffectRead"
				return item
			}
		}
		writePrefixes := []string{"Set", "Increment", "Decrement", "Draw", "Play", "Stop", "Spawn", "Move", "Destroy", "Paint", "Print", "Export", "StreamSet", "Stack"}
		for _, prefix := range writePrefixes {
			if strings.HasPrefix(item.name, prefix) {
				item.effect = "EffectWrite"
				return item
			}
		}
		item.effect = "EffectPure"
		return item
	}
	name := item.name
	pure := (item.pkg == "sonolus" && item.kind == "KindFunction") || strings.HasPrefix(name, "New") || strings.HasPrefix(name, "From")
	if pure || strings.HasPrefix(name, "Skin") || strings.HasPrefix(name, "EffectClip") || strings.HasPrefix(name, "ParticleEffect") || strings.HasPrefix(name, "Instruction") || strings.HasPrefix(name, "JudgmentBucket") {
		item.effect = "EffectPure"
		return item
	}
	readPrefixes := []string{"Get", "Has", "Len", "Capacity", "Contains", "Exists", "Count", "Now", "Delta", "Scaled", "Previous", "Offset", "BeatTo", "TimeTo", "Rect", "Info", "Result", "Is", "Aspect", "Judge", "Archetype", "Base", "Consecutive", "Initial", "Max", "Value", "Next", "Skip", "Accuracy", "Judgment", "Direction"}
	for _, prefix := range readPrefixes {
		if strings.HasPrefix(name, prefix) {
			item.effect = "EffectRead"
			return item
		}
	}
	if item.pkg == "sonolus" {
		writeReceivers := map[string]bool{
			"Sprite": true, "Clip": true, "LoopedEffectHandle": true,
			"ScheduledLoopedEffectHandle": true, "Effect": true,
			"ParticleHandle": true, "VarArray": true, "ArrayMap": true,
			"ArraySet": true,
		}
		if !writeReceivers[item.receiver] {
			item.effect = "EffectPure"
			return item
		}
	}
	item.effect = "EffectWrite"
	all := map[string]string{
		"play":     "preprocess|spawnOrder|shouldSpawn|initialize|updateSequential|touch|updateParallel|terminate",
		"watch":    "preprocess|spawnTime|despawnTime|initialize|updateSequential|updateParallel|terminate",
		"preview":  "preprocess|render",
		"tutorial": "preprocess|navigate|update",
	}
	item.phases = all[item.modes]
	// These facades are backed by preprocess-only pointers in sonolus.js.
	if item.receiver == "screenAPI" || item.receiver == "debugAPI" || item.receiver == "uiAPI" || item.receiver == "scoreAPI" || item.receiver == "lifeAPI" {
		item.phases = "preprocess"
	}
	// Preview canvas configuration is preprocess-only; drawing happens in render.
	if item.pkg == "sonolus/preview" && item.receiver == "canvasAPI" {
		if item.name == "Print" {
			item.phases = "render"
		} else {
			item.phases = "preprocess"
		}
	}
	return item
}

func must(err error) {
	if err != nil {
		panic(err)
	}
}

func nodeString(fset *token.FileSet, node any) string {
	if node == nil {
		return "inferred"
	}
	var out bytes.Buffer
	must(format.Node(&out, fset, node))
	return out.String()
}

func modeFor(pkg string) string {
	parts := strings.Split(pkg, "/")
	last := parts[len(parts)-1]
	switch last {
	case "play", "watch", "preview", "tutorial":
		return last
	default:
		return ""
	}
}

func receiverBase(expr ast.Expr) string {
	switch expr := expr.(type) {
	case *ast.StarExpr:
		return receiverBase(expr.X)
	case *ast.Ident:
		return expr.Name
	case *ast.IndexExpr:
		return receiverBase(expr.X)
	case *ast.IndexListExpr:
		return receiverBase(expr.X)
	default:
		return ""
	}
}

func publicEntries(root string) []entry {
	var result []entry
	for _, rel := range []string{"sonolus", "sonolus/play", "sonolus/watch", "sonolus/preview", "sonolus/tutorial"} {
		dir := filepath.Join(root, filepath.FromSlash(rel))
		files, err := filepath.Glob(filepath.Join(dir, "*.go"))
		must(err)
		for _, path := range files {
			if strings.HasSuffix(path, "_test.go") || strings.HasSuffix(path, "generated.go") {
				continue
			}
			fset := token.NewFileSet()
			file, err := parser.ParseFile(fset, path, nil, parser.ParseComments)
			must(err)
			for _, decl := range file.Decls {
				switch decl := decl.(type) {
				case *ast.GenDecl:
					for _, spec := range decl.Specs {
						switch spec := spec.(type) {
						case *ast.TypeSpec:
							if ast.IsExported(spec.Name.Name) {
								result = append(result, entry{pkg: rel, name: spec.Name.Name, kind: "KindType", signature: nodeString(fset, spec.Type), modes: modeFor(rel), effect: "EffectPure", source: "sonolus.py|sonolus.js|wiki"})
							}
						case *ast.ValueSpec:
							kind := "KindVariable"
							if decl.Tok == token.CONST {
								kind = "KindConstant"
							}
							for _, name := range spec.Names {
								if ast.IsExported(name.Name) {
									result = append(result, entry{pkg: rel, name: name.Name, kind: kind, signature: nodeString(fset, spec.Type), modes: modeFor(rel), effect: "EffectRead", source: "sonolus.py|sonolus.js|wiki"})
								}
							}
						}
					}
				case *ast.FuncDecl:
					if !ast.IsExported(decl.Name.Name) {
						continue
					}
					receiver := ""
					kind := "KindFunction"
					if decl.Recv != nil && len(decl.Recv.List) == 1 {
						kind = "KindMethod"
						receiver = receiverBase(decl.Recv.List[0].Type)
					}
					result = append(result, entry{pkg: rel, name: decl.Name.Name, receiver: receiver, kind: kind, signature: nodeString(fset, decl.Type), modes: modeFor(rel), effect: "EffectWrite", source: "sonolus.py|sonolus.js|wiki"})
				}
			}
		}
	}
	return result
}

func nativeEntries(root string) []entry {
	js, err := os.ReadFile(filepath.Join(root, "..", "sonolus.js-compiler", "src", "lib", "shared", "native.ts"))
	must(err)
	core, err := os.ReadFile(filepath.Join(root, "..", "sonolus-core-go", "core", "resource", "runtimes.go"))
	must(err)
	fnRE := regexp.MustCompile(`(?ms)^\s{4}([A-Za-z0-9_]+)\((.*?)\):\s*(number|boolean|void)\s*$`)
	argRE := regexp.MustCompile(`^\.\.\.([A-Za-z0-9_]+):\s*number\[\]$|^([A-Za-z0-9_]+):\s*(.+)$`)
	known := map[string]entry{}
	for _, match := range fnRE.FindAllStringSubmatch(string(js), -1) {
		name, rawArgs, rawResult := match[1], strings.TrimSpace(match[2]), match[3]
		var args []string
		if rawArgs != "" {
			for _, raw := range strings.Split(rawArgs, ",") {
				raw = strings.TrimSpace(raw)
				if raw == "" {
					continue
				}
				m := argRE.FindStringSubmatch(raw)
				if m == nil {
					panic("unsupported native argument: " + raw)
				}
				if m[1] != "" {
					args = append(args, m[1]+" ...float64")
					continue
				}
				typ := "float64"
				if strings.TrimSpace(m[3]) == "boolean" {
					typ = "bool"
				}
				args = append(args, m[2]+" "+typ)
			}
		}
		result := ""
		if rawResult == "number" {
			result = " float64"
		}
		if rawResult == "boolean" {
			result = " bool"
		}
		signature := "func(" + strings.Join(args, ", ") + ")" + result
		known[name] = entry{pkg: "sonolus/native", name: name, kind: "KindNative", signature: signature, effect: "EffectWrite", runtime: name, source: "sonolus.js native|sonolus-core-go"}
	}
	constRE := regexp.MustCompile(`RuntimeFunction([A-Za-z0-9_]+)\s+RuntimeFunction\s*=\s*"([A-Za-z0-9_]+)"`)
	var result []entry
	for _, match := range constRE.FindAllStringSubmatch(string(core), -1) {
		name := match[2]
		if item, ok := known[name]; ok {
			result = append(result, item)
		} else {
			result = append(result, entry{pkg: "sonolus/native", name: name, kind: "KindInternal", runtime: name, effect: "EffectWrite", source: "sonolus-core-go", internal: true})
		}
	}
	return result
}

func standardNameEntries(root string) ([]entry, string) {
	types := []struct {
		file, corePrefix, goPrefix string
	}{
		{"skin.go", "SkinSpriteName", "StandardSprite"},
		{"effect.go", "EffectClipName", "StandardClip"},
		{"particle.go", "ParticleEffectName", "StandardEffect"},
		{"instruction.go", "InstructionIconName", "StandardIcon"},
	}
	var entries []entry
	var declarations strings.Builder
	declarations.WriteString("// Code generated by gencatalog; DO NOT EDIT.\npackage sonolus\n\nconst (\n")
	for _, typ := range types {
		data, err := os.ReadFile(filepath.Join(root, "..", "sonolus-core-go", "core", "resource", typ.file))
		must(err)
		re := regexp.MustCompile(`(?m)^\s*` + regexp.QuoteMeta(typ.corePrefix) + `([A-Za-z0-9_]+)\s+` + regexp.QuoteMeta(typ.corePrefix) + `\s*=\s*("[^"]+")`)
		for _, match := range re.FindAllStringSubmatch(string(data), -1) {
			name := typ.goPrefix + match[1]
			fmt.Fprintf(&declarations, "\t%s = %s\n", name, match[2])
			entries = append(entries, entry{pkg: "sonolus", name: name, kind: "KindConstant", signature: "untyped string", effect: "EffectRead", source: "sonolus-core-go"})
		}
	}
	declarations.WriteString(")\n")
	return entries, declarations.String()
}

func main() {
	wd, err := os.Getwd()
	must(err)
	root := wd
	for filepath.Base(root) != "sonolus-go" {
		parent := filepath.Dir(root)
		if parent == root {
			panic("repository root not found")
		}
		root = parent
	}
	standards, standardSource := standardNameEntries(root)
	entries := append(publicEntries(root), standards...)
	entries = append(entries, nativeEntries(root)...)
	for i := range entries {
		entries[i] = semantic(entries[i])
	}
	sort.Slice(entries, func(i, j int) bool {
		left := entries[i].pkg + entries[i].receiver + entries[i].name
		right := entries[j].pkg + entries[j].receiver + entries[j].name
		return left < right
	})
	var out strings.Builder
	out.WriteString("// Code generated by gencatalog; DO NOT EDIT.\npackage catalog\n\nimport \"github.com/WindowsSov8forUs/sonolus-core-go/core/resource\"\n\nvar Symbols = []Symbol{\n")
	for _, item := range entries {
		fmt.Fprintf(&out, "{Package:%q, Name:%q, Receiver:%q, Kind:%s, Signature:%q, ", item.pkg, item.name, item.receiver, item.kind, item.signature)
		if item.modes != "" {
			fmt.Fprintf(&out, "Modes:[]string{%q}, ", item.modes)
		}
		if item.phases != "" {
			fmt.Fprintf(&out, "Phases:[]string{%s}, ", strings.Join(quoteList(strings.Split(item.phases, "|")), ","))
		}
		fmt.Fprintf(&out, "Effect:%s, Source:%q, Internal:%t", item.effect, item.source, item.internal)
		if item.runtime != "" {
			fmt.Fprintf(&out, ", Runtime:resource.RuntimeFunction(%q)", item.runtime)
		}
		out.WriteString("},\n")
	}
	out.WriteString("}\n")
	formatted, err := format.Source([]byte(out.String()))
	must(err)
	must(os.WriteFile(filepath.Join(root, "internal", "newcompiler", "catalog", "generated.go"), formatted, 0o644))

	var native strings.Builder
	native.WriteString("// Code generated by gencatalog; DO NOT EDIT.\npackage native\n\n")
	for _, item := range entries {
		if item.kind != "KindNative" {
			continue
		}
		fmt.Fprintf(&native, "func %s%s {", item.name, strings.TrimPrefix(item.signature, "func"))
		if strings.HasSuffix(item.signature, " float64") {
			native.WriteString(" return 0 ")
		}
		if strings.HasSuffix(item.signature, " bool") {
			native.WriteString(" return false ")
		}
		native.WriteString("}\n")
	}
	nativeFormatted, err := format.Source([]byte(native.String()))
	must(err)
	must(os.WriteFile(filepath.Join(root, "sonolus", "native", "generated.go"), nativeFormatted, 0o644))
	standardFormatted, err := format.Source([]byte(standardSource))
	must(err)
	must(os.WriteFile(filepath.Join(root, "sonolus", "standard_names_generated.go"), standardFormatted, 0o644))
}

func quoteList(values []string) []string {
	result := make([]string, len(values))
	for i, value := range values {
		result[i] = fmt.Sprintf("%q", value)
	}
	return result
}
