package engine

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"

	"github.com/WindowsSov8forUs/sonolus-go/internal/compiler/frontend"
	"github.com/WindowsSov8forUs/sonolus-go/internal/compiler/utils"
	"github.com/WindowsSov8forUs/sonolus-go/internal/goparse"
	"golang.org/x/tools/go/packages"
)

func packageFilterNotThirdParty(p *goparse.Parser) *goparse.PackageFilter {
	filterFunc := func(pkg *packages.Package) bool {
		path := pkg.PkgPath
		if strings.HasPrefix(path, p.Module()) {
			return true
		}
		if path == utils.SonolusPkgPath() {
			return true
		}
		if !strings.Contains(path, ".") {
			return true
		}
		return false
	}
	return &goparse.PackageFilter{
		Func:     filterFunc,
		ErrorMsg: "invalid third party lib",
	}
}

func LoadPackage(pattern ...string) (*packages.Package, error) {
	parser := goparse.NewParser()
	parser.SetFilters(goparse.PackageFilterNotStandard(), packageFilterNotThirdParty(parser))
	return parser.Load(pattern...)
}

// EngineSources bundles the parsed engine project for compilation.
type EngineSources struct {
	Pkg     *packages.Package
	sources map[string]map[string]string

	// Backward-compatible fields for tests.
	RootDir string
	Main    map[string]string
	Imports map[string]map[string]string
}

func (ess *EngineSources) MainPkg() *packages.Package { return ess.Pkg }
func (ess *EngineSources) ImportPkg(path string) *packages.Package {
	if ess.Pkg == nil {
		return nil
	}
	return ess.Pkg.Imports[path]
}
func (ess *EngineSources) ImportedFileCount() int {
	if ess.Pkg == nil {
		return 0
	}
	n := 0
	for _, pkg := range ess.Pkg.Imports {
		n += len(pkg.Syntax)
	}
	return n
}

func NewSingleFileSources(src string) *EngineSources {
	return &EngineSources{
		Main:    map[string]string{"engine.go": src},
		Imports: map[string]map[string]string{},
	}
}

func NewEngineSources(main map[string]string, imports map[string]map[string]string) (*EngineSources, error) {
	return &EngineSources{Main: main, Imports: imports}, nil
}

func LoadEngineSources(patterns ...string) (*EngineSources, error) {
	if len(patterns) == 0 {
		return nil, fmt.Errorf("source: no path provided")
	}

	pkg, err := LoadPackage(patterns...)
	if err != nil {
		return nil, fmt.Errorf("source: %w", err)
	}
	return &EngineSources{Pkg: pkg, sources: readAllSources(pkg)}, nil
}

func (ess *EngineSources) ResolveImports() error {
	if ess.Pkg != nil {
		return nil
	}
	if ess.Main == nil && ess.Imports == nil {
		return fmt.Errorf("no source provided")
	}

	// Legacy path: parse inline source maps directly via go/parser.
	// This avoids module-resolution complexities with packages.Load.
	_, mainASTs, err := parseSourceMap(ess.Main)
	if err != nil {
		return err
	}

	importASTs := make(map[string][]*ast.File)
	for impPath, files := range ess.Imports {
		_, asts, err := parseSourceMap(files)
		if err != nil {
			return fmt.Errorf("import %q: %w", impPath, err)
		}
		importASTs[impPath] = asts
	}

	// Construct a minimal *packages.Package for the main package.
	mainPkgName := extractPkgName(ess.Main)
	mainPkg := &packages.Package{
		Name:    mainPkgName,
		PkgPath: "main",
		Syntax:  mainASTs,
		Imports: make(map[string]*packages.Package),
	}

	sources := map[string]map[string]string{"": ess.Main}

	for impPath, asts := range importASTs {
		impPkgName := extractPkgName(ess.Imports[impPath])
		impPkg := &packages.Package{
			Name:    impPkgName,
			PkgPath: impPath,
			Syntax:  asts,
		}
		mainPkg.Imports[impPath] = impPkg
		sources[impPath] = ess.Imports[impPath]
	}

	ess.Pkg = mainPkg
	ess.sources = sources
	return nil
}

// parseSourceMap parses a set of source strings into *ast.File nodes.
func parseSourceMap(files map[string]string) (*token.FileSet, []*ast.File, error) {
	fset := token.NewFileSet()
	var asts []*ast.File
	for name, src := range files {
		f, err := parser.ParseFile(fset, name, src, 0)
		if err != nil {
			return nil, nil, err
		}
		asts = append(asts, f)
	}
	return fset, asts, nil
}

// extractPkgName extracts the package name from a set of source files.
func extractPkgName(files map[string]string) string {
	for _, src := range files {
		fset := token.NewFileSet()
		f, err := parser.ParseFile(fset, "", src, parser.PackageClauseOnly)
		if err == nil {
			return f.Name.Name
		}
	}
	return ""
}

func (ess *EngineSources) Access() *frontend.EngineSourcesAccess {
	return &frontend.EngineSourcesAccess{
		RootDir:   ".",
		MainFiles: ess.sources[""],
		Imports:   ess.sources,
	}
}

func readAllSources(main *packages.Package) map[string]map[string]string {
	out := map[string]map[string]string{"": readSourceFiles(main)}
	for path, pkg := range main.Imports {
		out[path] = readSourceFiles(pkg)
	}
	return out
}

func readSourceFiles(pkg *packages.Package) map[string]string {
	files := make(map[string]string, len(pkg.GoFiles))
	for _, f := range pkg.GoFiles {
		if data, err := os.ReadFile(f); err == nil {
			files[filepath.Base(f)] = string(data)
		}
	}
	return files
}

