package frontend

import (
	"fmt"
	"go/ast"
	"go/types"
	"sort"
	"strings"
	"sync"

	"golang.org/x/tools/go/packages"

	"github.com/WindowsSov8forUs/sonolus-go/internal/newcompiler/mode"
)

var callbacks = map[mode.Mode]map[string]string{
	mode.ModePlay: {
		"Preprocess": "void", "SpawnOrder": "float", "ShouldSpawn": "bool",
		"Initialize": "void", "UpdateSequential": "void", "Touch": "void",
		"UpdateParallel": "void", "Terminate": "void",
	},
	mode.ModeWatch: {
		"Preprocess": "void", "SpawnTime": "float", "DespawnTime": "float",
		"Initialize": "void", "UpdateSequential": "void", "UpdateParallel": "void", "Terminate": "void",
	},
	mode.ModePreview: {"Preprocess": "void", "Render": "void"},
}

func callbackKey(goName string) string {
	if goName == "" {
		return ""
	}
	return strings.ToLower(goName[:1]) + goName[1:]
}

func validCallbackSignature(fn *types.Func, result string, receiver bool) error {
	sig, ok := fn.Type().(*types.Signature)
	if !ok {
		return fmt.Errorf("not a function")
	}
	if receiver != (sig.Recv() != nil) {
		return fmt.Errorf("invalid receiver")
	}
	if sig.Params().Len() != 0 {
		return fmt.Errorf("callback must not have parameters")
	}
	switch result {
	case "void":
		if sig.Results().Len() != 0 {
			return fmt.Errorf("callback must not return a value")
		}
	case "float":
		if sig.Results().Len() != 1 || !types.Identical(sig.Results().At(0).Type(), types.Typ[types.Float64]) {
			return fmt.Errorf("callback must return float64")
		}
	case "bool":
		if sig.Results().Len() != 1 || !types.Identical(sig.Results().At(0).Type(), types.Typ[types.Bool]) {
			return fmt.Errorf("callback must return bool")
		}
	}
	return nil
}

func findFuncDecl(pkg *packages.Package, fn *types.Func) *ast.FuncDecl {
	if pkg == nil {
		return nil
	}
	for _, file := range pkg.Syntax {
		for _, decl := range file.Decls {
			if fd, ok := decl.(*ast.FuncDecl); ok && pkg.TypesInfo.Defs[fd.Name] == fn {
				return fd
			}
		}
	}
	return nil
}

func calledObject(pkg *packages.Package, expr ast.Expr) types.Object {
	switch expr := expr.(type) {
	case *ast.ParenExpr:
		return calledObject(pkg, expr.X)
	case *ast.IndexExpr:
		return calledObject(pkg, expr.X)
	case *ast.IndexListExpr:
		return calledObject(pkg, expr.X)
	case *ast.Ident:
		return pkg.TypesInfo.ObjectOf(expr)
	case *ast.SelectorExpr:
		return pkg.TypesInfo.ObjectOf(expr.Sel)
	default:
		return nil
	}
}

func globalCallbacks(packagesByTypes map[*types.Package]*packages.Package, pkg *packages.Package, resources *ModeResources, m mode.Mode, hasMarker bool) ([]*CallbackDeclaration, []error) {
	if !hasMarker {
		return nil, nil
	}
	wanted := map[string]string{}
	if m == mode.ModeWatch {
		wanted["UpdateSpawn"] = "float"
	}
	if m == mode.ModeTutorial {
		wanted["Preprocess"], wanted["Navigate"], wanted["Update"] = "void", "void", "void"
	}
	var result []*CallbackDeclaration
	var errs []error
	knownGlobalNames := map[string]bool{
		"UpdateSpawn": true,
		"Preprocess":  true,
		"Navigate":    true,
		"Update":      true,
	}
	for _, name := range pkg.Types.Scope().Names() {
		object, ok := pkg.Types.Scope().Lookup(name).(*types.Func)
		if !ok || !object.Exported() {
			continue
		}
		signature, _ := object.Type().(*types.Signature)
		if signature != nil && signature.Recv() == nil {
			if !knownGlobalNames[name] {
				errs = append(errs, fmt.Errorf("%s: unknown %s global callback", name, m))
			}
		}
	}
	names := make([]string, 0, len(wanted))
	for name := range wanted {
		names = append(names, name)
	}
	sort.Strings(names)
	type callbackJob struct {
		name string
		fn   *types.Func
		decl *ast.FuncDecl
	}
	var jobs []callbackJob
	for _, name := range names {
		signature := wanted[name]
		obj, ok := pkg.Types.Scope().Lookup(name).(*types.Func)
		if !ok {
			continue
		}
		if err := validCallbackSignature(obj, signature, false); err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", name, err))
			continue
		}
		decl := findFuncDecl(pkg, obj)
		jobs = append(jobs, callbackJob{name: name, fn: obj, decl: decl})
	}
	callbacks := make([]*CallbackDeclaration, len(jobs))
	jobErrs := make([][]error, len(jobs))
	var wg sync.WaitGroup
	for i, job := range jobs {
		wg.Add(1)
		go func(i int, job callbackJob) {
			defer wg.Done()
			key := callbackKey(job.name)
			bodyIR, lowerErrs := lowerCallback(packagesByTypes, pkg, job.decl, job.fn, nil, resources, nil, m, key)
			callbacks[i] = &CallbackDeclaration{Name: key, Function: job.fn, Decl: job.decl, IR: bodyIR}
			jobErrs[i] = lowerErrs
		}(i, job)
	}
	wg.Wait()
	for i := range jobs {
		result = append(result, callbacks[i])
		errs = append(errs, jobErrs[i]...)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Name < result[j].Name })
	return result, errs
}
