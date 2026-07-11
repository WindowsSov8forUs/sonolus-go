package source

import (
	"runtime/debug"
	"strings"
	"sync"

	"golang.org/x/tools/go/packages"

	"github.com/WindowsSov8forUs/sonolus-go/internal/goparse"
	"github.com/WindowsSov8forUs/sonolus-go/internal/newcompiler/intrinsic"
)

var (
	sonolusPkgPath     string
	sonolusPkgPathOnce sync.Once
)

func SonolusPkgPath() string {
	sonolusPkgPathOnce.Do(func() {
		info, _ := debug.ReadBuildInfo()
		module := info.Main.Path
		if module == "" {
			module = info.Path
		}
		sonolusPkgPath = module + "/sonolus"
	})

	return sonolusPkgPath
}

func IsSonolusPkg(pkg *packages.Package) bool {
	return IsSonolusPkgPath(pkg.PkgPath)
}

func IsSonolusPkgPath(path string) bool {
	root := SonolusPkgPath()
	return path == root || strings.HasPrefix(path, root+"/")
}

func packageFilterAllowedStandard() *goparse.PackageFilter {
	return &goparse.PackageFilter{
		Func: func(pkg *packages.Package) bool {
			return pkg.Module != nil || intrinsic.IsAllowedStandardDependency(pkg.PkgPath)
		},
		ErrorMsg: intrinsic.StandardLibraryFilterError,
	}
}

func packageFilterNotThirdParty() *goparse.PackageFilter {
	return &goparse.PackageFilter{
		Func: func(pkg *packages.Package) bool {
			if pkg.Module == nil {
				return true
			}
			if pkg.Module.Main {
				return true
			}
			return IsSonolusPkg(pkg) || !strings.Contains(pkg.PkgPath, ".")
		},
		ErrorMsg: "invalid third party lib",
	}
}

func Load(pattern ...string) ([]*packages.Package, error) {
	parser := goparse.NewParser()
	parser.SetImportFilters(goparse.ImportFilterNoDotImport())
	parser.SetPackageFilters(packageFilterAllowedStandard(), packageFilterNotThirdParty())
	pkgs, err := parser.Load(pattern...)
	if err != nil {
		return nil, err
	}
	if err := intrinsic.ValidateStandardImports(pkgs...); err != nil {
		return nil, err
	}
	return pkgs, nil
}
