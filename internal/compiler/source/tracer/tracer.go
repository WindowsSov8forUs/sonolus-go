package tracer

import (
	"go/token"
	"go/types"
	"sync"

	"golang.org/x/tools/go/packages"
)

// ASTTracer owns the package graph and all compile-time source caches for one
// compilation. Public entry points serialize access so the returned graphs are
// complete before they become visible to callers.
type ASTTracer struct {
	pkg *packages.Package
	mu  sync.Mutex

	packagesByTypes map[*types.Package]*packages.Package
	types           *typeResolver
	evaluator       *staticEvaluator
}

func (t *ASTTracer) Package() *packages.Package {
	return t.pkg
}

func (t *ASTTracer) Fset() *token.FileSet {
	if t == nil || t.pkg == nil {
		return nil
	}
	return t.pkg.Fset
}

func (t *ASTTracer) registerPackageGraph(pkg *packages.Package, visited map[*packages.Package]bool) {
	if pkg == nil || visited[pkg] {
		return
	}
	visited[pkg] = true
	if pkg.Types != nil {
		t.packagesByTypes[pkg.Types] = pkg
	}
	for _, dependency := range pkg.Imports {
		t.registerPackageGraph(dependency, visited)
	}
}

func (t *ASTTracer) packageForTypes(pkg *types.Package) (*packages.Package, bool) {
	if pkg == nil {
		return nil, false
	}
	result, ok := t.packagesByTypes[pkg]
	return result, ok
}

func (t *ASTTracer) packageForObject(object types.Object) (*packages.Package, bool) {
	if object == nil || object.Pkg() == nil {
		return nil, false
	}
	return t.packageForTypes(object.Pkg())
}

func NewASTTracer(pkg *packages.Package) *ASTTracer {
	tracer := &ASTTracer{
		pkg:             pkg,
		packagesByTypes: make(map[*types.Package]*packages.Package),
	}
	tracer.registerPackageGraph(pkg, make(map[*packages.Package]bool))
	tracer.types = newTypeResolver(tracer)
	tracer.evaluator = newStaticEvaluator(tracer)
	return tracer
}
