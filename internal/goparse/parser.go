package goparse

import (
	"fmt"
	"go/ast"
	"go/token"
	"sort"
	"strings"

	"golang.org/x/tools/go/packages"
)

const defaultModuleName = "command-line-arguments"

const Modes = packages.NeedName | packages.NeedFiles |
	packages.NeedSyntax | packages.NeedTypes | packages.NeedTypesInfo |
	packages.NeedDeps | packages.NeedImports | packages.NeedModule |
	packages.NeedEmbedFiles | packages.NeedEmbedPatterns | packages.NeedCompiledGoFiles |
	packages.NeedTypesSizes

type PackageFilter struct {
	Func     func(pkg *packages.Package) bool
	ErrorMsg string
}

type ImportFilter struct {
	Func     func(imp *ast.ImportSpec) bool
	ErrorMsg string
}

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

// ensureModule 确保 module 名称
func ensureModule(pkg *packages.Package) error {
	if pkg.Name != "main" {
		return fmt.Errorf("package is not main package")
	}

	if pkg.PkgPath != defaultModuleName {
		return nil
	}

	module := getModule(pkg)
	if module == "" {
		return fmt.Errorf("could not get module name")
	}

	pkg.ID = module
	pkg.PkgPath = module

	return nil
}

func getErrors(pkg *packages.Package, visited map[string]bool, errs *[]error) {
	if isVisited, ok := visited[pkg.PkgPath]; ok {
		if isVisited {
			return
		}
	}
	visited[pkg.PkgPath] = true

	for _, pkgErr := range pkg.Errors {
		err := fmt.Errorf("%s: %s", pkgErr.Pos, pkgErr.Msg)
		*errs = append(*errs, err)
	}

	paths := make([]string, 0, len(pkg.Imports))
	for path := range pkg.Imports {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	for _, path := range paths {
		getErrors(pkg.Imports[path], visited, errs)
	}
}

type Parser struct {
	cfg        *packages.Config
	pkgFilters []*PackageFilter
	impFilters []*ImportFilter
}

func NewParser() *Parser {
	cfg := &packages.Config{
		Mode: Modes,
		Fset: token.NewFileSet(),
	}
	return &Parser{cfg: cfg}
}

func (p *Parser) refreshFset() {
	p.cfg.Fset = token.NewFileSet()
}

func (p *Parser) Fset() *token.FileSet {
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

	paths := make([]string, 0, len(pkg.Imports))
	for impPath := range pkg.Imports {
		paths = append(paths, impPath)
	}
	sort.Strings(paths)
	for _, impPath := range paths {
		dep := pkg.Imports[impPath]
		for _, filter := range p.pkgFilters {
			if !filter.Func(dep) {
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
				pos := p.Fset().Position(imp.Pos())
				err := fmt.Errorf(
					`%s:%d:%d: could not import %q (%s)`, pos.Filename, pos.Line, pos.Column, path, errorMsg,
				)
				*errs = append(*errs, err)
			}
			for _, filter := range p.impFilters {
				if !filter.Func(imp) {
					pos := p.Fset().Position(imp.Pos())
					err := fmt.Errorf(
						`%s:%d:%d: %s`, pos.Filename, pos.Line, pos.Column, filter.ErrorMsg,
					)
					*errs = append(*errs, err)
				}
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

func (p *Parser) load(patterns ...string) ([]*packages.Package, []error) {
	pkgs, err := packages.Load(p.cfg, patterns...)
	if err != nil {
		return nil, []error{err}
	}
	if len(pkgs) < 1 {
		return nil, []error{fmt.Errorf("no golang source file found")}
	}

	var errs []error
	for _, pkg := range pkgs {
		visited := make(map[string]bool)
		getErrors(pkg, visited, &errs)
	}

	for _, pkg := range pkgs {
		if err := ensureModule(pkg); err != nil {
			errs = append(errs, err)
		}
	}

	return pkgs, errs
}

// SetConfig 自定义 packages.Config 选项
func (p *Parser) SetConfig(cfg *packages.Config) *Parser {
	if cfg.Dir != "" {
		p.cfg.Dir = cfg.Dir
	}
	if len(cfg.Env) > 0 {
		p.cfg.Env = cfg.Env
	}
	if cfg.Tests {
		p.cfg.Tests = true
	}
	if len(cfg.BuildFlags) > 0 {
		p.cfg.BuildFlags = cfg.BuildFlags
	}

	return p
}

// SetPackageFilters 包过滤，若遇到被过滤的包则报错
func (p *Parser) SetPackageFilters(filters ...*PackageFilter) *Parser {
	p.pkgFilters = filters
	return p
}

// SetImportFilters 导入过滤，若遇到被过滤的导入则报错
func (p *Parser) SetImportFilters(filters ...*ImportFilter) *Parser {
	p.impFilters = filters
	return p
}

func (p *Parser) Load(patterns ...string) ([]*packages.Package, error) {
	p.refreshFset()

	pkgs, errs := p.load(patterns...)
	if len(errs) > 0 {
		var msgs []string
		for _, e := range errs {
			msgs = append(msgs, e.Error())
		}
		return nil, fmt.Errorf("%s", strings.Join(msgs, "\n"))
	}

	var msgs []string
	for _, pkg := range pkgs {
		if errs := p.validateImportsAll(pkg); len(errs) > 0 {
			for _, e := range errs {
				msgs = append(msgs, e.Error())
			}
		}
	}
	if len(msgs) > 0 {
		return nil, fmt.Errorf("%s", strings.Join(msgs, "\n"))
	}

	return pkgs, nil
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

// ImportFilterNoDotImport ImportFilter 过滤点导入
func ImportFilterNoDotImport() *ImportFilter {
	filterFunc := func(imp *ast.ImportSpec) bool {
		if imp.Name != nil && imp.Name.Name == "." {
			return false
		} else {
			return true
		}
	}
	return &ImportFilter{
		Func:     filterFunc,
		ErrorMsg: "do not use dot import",
	}
}
