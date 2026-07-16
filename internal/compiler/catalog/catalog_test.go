package catalog_test

import (
	"go/types"
	"path/filepath"
	"runtime"
	"testing"

	"golang.org/x/tools/go/packages"

	"github.com/WindowsSov8forUs/sonolus-go/internal/compiler/catalog"
)

func repositoryRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("caller path unavailable")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", ".."))
}

func TestCatalogKeysAndRuntimeCoverage(t *testing.T) {
	keys := map[string]bool{}
	runtimes := map[string]bool{}
	for i := range catalog.Symbols {
		symbol := &catalog.Symbols[i]
		if symbol.Source == "" || symbol.Kind == "" {
			t.Fatalf("incomplete symbol: %#v", symbol)
		}
		if keys[symbol.Key()] {
			t.Fatalf("duplicate catalog key %q", symbol.Key())
		}
		keys[symbol.Key()] = true
		if symbol.Runtime != "" {
			if runtimes[string(symbol.Runtime)] {
				t.Fatalf("duplicate RuntimeFunction %q", symbol.Runtime)
			}
			runtimes[string(symbol.Runtime)] = true
			if !symbol.Internal && symbol.Signature == "" {
				t.Fatalf("public native %q has no signature", symbol.Name)
			}
		}
	}
}

func TestEveryPublicDeclarationIsCataloged(t *testing.T) {
	config := &packages.Config{
		Dir:  repositoryRoot(t),
		Mode: packages.NeedName | packages.NeedTypes | packages.NeedImports | packages.NeedDeps,
	}
	pkgs, err := packages.Load(config, "./sonolus/...")
	if err != nil {
		t.Fatal(err)
	}
	if packages.PrintErrors(pkgs) != 0 {
		t.Fatal("load public API packages")
	}
	checkMethodSet := func(set *types.MethodSet) {
		for i := 0; i < set.Len(); i++ {
			object := set.At(i).Obj()
			if !object.Exported() {
				continue
			}
			if _, ok := catalog.LookupObject(object); !ok {
				t.Errorf("method missing from catalog: %s.%s", object.Pkg().Path(), object.Name())
			}
		}
	}
	for _, pkg := range pkgs {
		for _, name := range pkg.Types.Scope().Names() {
			object := pkg.Types.Scope().Lookup(name)
			if object.Exported() {
				if _, ok := catalog.LookupObject(object); !ok {
					t.Errorf("object missing from catalog: %s.%s", pkg.PkgPath, name)
				}
			}
			switch object := object.(type) {
			case *types.TypeName:
				checkMethodSet(types.NewMethodSet(object.Type()))
				checkMethodSet(types.NewMethodSet(types.NewPointer(object.Type())))
			case *types.Var:
				checkMethodSet(types.NewMethodSet(object.Type()))
			}
		}
	}
}
