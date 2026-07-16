package frontend

import (
	"go/types"
	"sort"

	"golang.org/x/tools/go/packages"

	"github.com/WindowsSov8forUs/sonolus-go/internal/compiler/source"
)

func collectPackages(root *packages.Package) []*packages.Package {
	seen := map[string]bool{}
	var result []*packages.Package
	var visit func(*packages.Package)
	visit = func(pkg *packages.Package) {
		if pkg == nil || seen[pkg.PkgPath] || source.IsSonolusPkg(pkg) {
			return
		}
		if pkg.Module == nil || !pkg.Module.Main {
			return
		}
		seen[pkg.PkgPath] = true
		result = append(result, pkg)
		paths := make([]string, 0, len(pkg.Imports))
		for path := range pkg.Imports {
			paths = append(paths, path)
		}
		sort.Strings(paths)
		for _, path := range paths {
			visit(pkg.Imports[path])
		}
	}
	visit(root)
	sort.Slice(result, func(i, j int) bool { return result[i].PkgPath < result[j].PkgPath })
	return result
}

func packageNamedTypes(pkg *packages.Package) []*types.Named {
	names := pkg.Types.Scope().Names()
	result := make([]*types.Named, 0)
	for _, name := range names {
		obj, ok := pkg.Types.Scope().Lookup(name).(*types.TypeName)
		if !ok {
			continue
		}
		if named, ok := namedType(obj.Type()); ok {
			result = append(result, named)
		}
	}
	return result
}

func packageVariables(pkg *packages.Package) []*types.Var {
	var result []*types.Var
	for _, name := range pkg.Types.Scope().Names() {
		if v, ok := pkg.Types.Scope().Lookup(name).(*types.Var); ok {
			result = append(result, v)
		}
	}
	return result
}

func markerVariables(pkg *packages.Package, named *types.Named) []*types.Var {
	var result []*types.Var
	for _, v := range packageVariables(pkg) {
		vn, ok := namedType(v.Type())
		if ok && vn.Obj() == named.Obj() {
			result = append(result, v)
		}
	}
	return result
}
