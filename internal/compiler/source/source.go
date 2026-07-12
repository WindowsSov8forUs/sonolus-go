package source

import (
	"fmt"
	"runtime/debug"
	"strings"
	"sync"

	"golang.org/x/tools/go/packages"

	"github.com/WindowsSov8forUs/sonolus-go/v2/internal/compiler/intrinsic"
	"github.com/WindowsSov8forUs/sonolus-go/v2/internal/compiler/mode"
	"github.com/WindowsSov8forUs/sonolus-go/v2/internal/goparse"
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

func newParser() *goparse.Parser {
	parser := goparse.NewParser()
	parser.SetImportFilters(goparse.ImportFilterNoDotImport())
	parser.SetPackageFilters(packageFilterAllowedStandard(), packageFilterNotThirdParty())
	return parser
}

func load(parser *goparse.Parser, pattern ...string) ([]*packages.Package, error) {
	pkgs, err := parser.Load(pattern...)
	if err != nil {
		return nil, err
	}
	if err := intrinsic.ValidateStandardImports(pkgs...); err != nil {
		return nil, err
	}
	return pkgs, nil
}

func Load(pattern ...string) ([]*packages.Package, error) {
	return load(newParser(), pattern...)
}

func LoadMode(m mode.Mode, pattern ...string) ([]*packages.Package, error) {
	if !m.Valid() {
		return nil, fmt.Errorf("invalid Sonolus mode %q; expected play, watch, preview, or tutorial", m)
	}
	parser := newParser().SetConfig(&packages.Config{BuildFlags: []string{"-tags=" + string(m)}})
	return load(parser, pattern...)
}
