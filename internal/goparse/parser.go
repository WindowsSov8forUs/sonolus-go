package goparse

import (
	"fmt"
	"go/token"
	"strings"

	"golang.org/x/tools/go/packages"
)

const Modes = packages.NeedName | packages.NeedFiles | packages.NeedSyntax |
	packages.NeedDeps | packages.NeedImports | packages.NeedModule |
	packages.NeedEmbedFiles | packages.NeedEmbedPatterns

var PackageErrorKind map[packages.ErrorKind]string = map[packages.ErrorKind]string{
	packages.UnknownError: "unknown error",
	packages.ListError:    "list error",
	packages.ParseError:   "parse error",
	packages.TypeError:    "type error",
}

type PackageFilter struct {
	Func     func(pkg *packages.Package) bool
	ErrorMsg string
}

// getModule 获取 module 名称
func getModule(pkg *packages.Package) string {
	if pkg.Module != nil {
		return pkg.Module.Path
	}
	// 单文件模式
	for _, dep := range pkg.Imports {
		if dep.Module != nil && dep.Module.Main {
			return dep.Module.Path
		}
	}
	return ""
}

func getErrors(pkg *packages.Package, visited map[string]bool, errs *[]error) {
	if isVisited, ok := visited[pkg.PkgPath]; ok {
		if isVisited {
			return
		}
	}
	visited[pkg.PkgPath] = true

	for _, pkgErr := range pkg.Errors {
		err := fmt.Errorf("%s (%s): %s", PackageErrorKind[pkgErr.Kind], pkgErr.Pos, pkgErr.Msg)
		*errs = append(*errs, err)
	}

	for _, dep := range pkg.Imports {
		getErrors(dep, visited, errs)
	}
}

type Parser struct {
	cfg     *packages.Config
	filters []*PackageFilter
	module  string
}

func NewParser() *Parser {
	cfg := &packages.Config{
		Mode: Modes,
		Fset: token.NewFileSet(),
	}
	return &Parser{
		cfg: cfg,
	}
}

func (p *Parser) Module() string {
	return p.module
}

func (p *Parser) fset() *token.FileSet {
	return p.cfg.Fset
}

func (p *Parser) validateImports(pkg *packages.Package, visited map[string]bool, errs *[]error) {
	if isVisited, ok := visited[pkg.PkgPath]; ok {
		if isVisited {
			return
		}
	}
	visited[pkg.PkgPath] = true
	invalidPkgs := make(map[string]string)

	for impPath, dep := range pkg.Imports {
		for _, filter := range p.filters {
			if !filter.Func(dep) {
				err := fmt.Errorf(`import error in "%s": failed to import "%q" (%s).`, pkg.PkgPath, impPath, filter.ErrorMsg)
				*errs = append(*errs, err)
				invalidPkgs[impPath] = filter.ErrorMsg
				break
			}
		}
		if _, ok := invalidPkgs[impPath]; ok {
			continue
		}
		p.validateImports(dep, visited, errs)
	}

	for _, file := range pkg.Syntax {
		for _, imp := range file.Imports {
			path := strings.Trim(imp.Path.Value, `"`)
			if errorMsg, ok := invalidPkgs[path]; ok {
				pos := p.fset().Position(imp.Pos())
				err := fmt.Errorf(
					`failed to import %q (%s:%d): %s`, path, pos.Filename, pos.Line, errorMsg,
				)
				*errs = append(*errs, err)
			}
		}
	}
}

// validateImportsAll 检查所有包的 import 是否合法
func (p *Parser) validateImportsAll(pkg *packages.Package) []error {
	var errs []error
	visited := make(map[string]bool)

	p.validateImports(pkg, visited, &errs)
	return errs
}

func (p *Parser) load(patterns ...string) (*packages.Package, []error) {
	pkgs, err := packages.Load(p.cfg, patterns...)
	if err != nil {
		return nil, []error{err}
	}
	if len(pkgs) < 1 {
		return nil, []error{fmt.Errorf("no golang source file found.")}
	}
	if len(pkgs) > 1 {
		return nil, []error{fmt.Errorf("multiple top golang package found.")}
	}
	pkg := pkgs[0]
	var errs []error
	visited := make(map[string]bool)
	getErrors(pkg, visited, &errs)

	p.module = getModule(pkg)

	return pkg, errs
}

// SetFilters 导入过滤，若遇到被过滤的导入则报错
func (p *Parser) SetFilters(filters ...*PackageFilter) *Parser {
	p.filters = filters
	return p
}

// LoadWithConfig loads packages using a caller-supplied packages.Config.
// This allows callers to inject overlay, custom Dir, etc.
func (p *Parser) LoadWithConfig(cfg packages.Config, patterns ...string) (*packages.Package, error) {
	p.cfg = &cfg
	if p.cfg.Fset == nil {
		p.cfg.Fset = token.NewFileSet()
	}
	if p.cfg.Mode == 0 {
		p.cfg.Mode = Modes
	}
	return p.Load(patterns...)
}

func (p *Parser) Load(patterns ...string) (*packages.Package, error) {
	pkg, errs := p.load(patterns...)
	if len(errs) > 0 {
		var msgs []string
		for _, e := range errs {
			msgs = append(msgs, e.Error())
		}
		return nil, fmt.Errorf("load: %s", strings.Join(msgs, "; "))
	}

	if errs := p.validateImportsAll(pkg); len(errs) > 0 {
		var msgs []string
		for _, e := range errs {
			msgs = append(msgs, e.Error())
		}
		return nil, fmt.Errorf("import validation: %s", strings.Join(msgs, "; "))
	}

	return pkg, nil
}

// PackageFilterNotStandard PackageFilter 过滤所有标准库
func PackageFilterNotStandard() *PackageFilter {
	filterFunc := func(pkg *packages.Package) bool {
		return pkg.Module != nil
	}
	return &PackageFilter{
		Func:     filterFunc,
		ErrorMsg: "standard lib is not allowed",
	}
}
