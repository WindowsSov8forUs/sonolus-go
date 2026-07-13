package goparse

import (
	"go/token"
	"testing"

	"golang.org/x/tools/go/packages"
	"os"
	"path/filepath"
	"strings"
)

func TestLoad_Packages(t *testing.T) {
	cfg := &packages.Config{
		Dir: "./test/project_mainfile",
		Mode: packages.NeedName |
			packages.NeedFiles |
			packages.NeedSyntax |
			packages.NeedTypes |
			packages.NeedTypesInfo |
			packages.NeedImports |
			packages.NeedModule,
		Fset: token.NewFileSet(),
	}

	pkgs, err := packages.Load(cfg, "main.go")
	if err != nil {
		t.Fatalf("LoadProject: %v", err)
	}
	t.Logf("Load Results: %v\n", pkgs)
	for _, pkg := range pkgs {
		t.Logf("Load Result: %#v\n", pkg)
		t.Logf("Module: %#v\n", pkg.Module)
		for impPath, dep := range pkg.Imports {
			t.Logf("%s -> Module: %#v\n", impPath, dep.Module)
			for impPath2, dep2 := range dep.Imports {
				t.Logf("%s -> Module: %#v\n", impPath2, dep2.Module)
			}
		}
		if pkg.Module != nil {
			t.Logf("Main Module: %s\n", pkg.Module.Path)
		} else {
			for _, dep := range pkg.Imports {
				if dep.Module != nil && dep.Module.Main {
					t.Logf("Main Module: %s\n", dep.Module.Path)
				}
			}
		}
	}
}
func loadTestModule(t *testing.T, source string, configure func(*Parser)) error {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.com/test\n\ngo 1.25.4\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte(source), 0o644); err != nil {
		t.Fatal(err)
	}
	parser := NewParser().SetConfig(&packages.Config{Dir: dir})
	if configure != nil {
		configure(parser)
	}
	_, err := parser.Load(".")
	return err
}

func TestStandardPackagesDeniedByDefault(t *testing.T) {
	err := loadTestModule(t, "package main\nimport \"math\"\nvar _ = math.Pi\nfunc main() {}\n", func(parser *Parser) {
		parser.SetPackageFilters(PackageFilterNotStandard())
	})
	if err == nil || !strings.Contains(err.Error(), "standard lib is not allowed") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAllowedStandardPackagesAreIntrinsicLeaves(t *testing.T) {
	src := "package main\nimport (\n_ \"embed\"\n\"math\"\n\"math/rand\"\n)\nvar _, _ = math.Pi, rand.Float64()\nfunc main() {}\n"
	err := loadTestModule(t, src, func(parser *Parser) {
		parser.SetPackageFilters(&PackageFilter{Func: func(pkg *packages.Package) bool {
			return pkg.Module != nil || !strings.Contains(pkg.PkgPath, ".")
		}})
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestAllowedStandardPackagesStillRejectOthersAndDotImports(t *testing.T) {
	t.Run("other package", func(t *testing.T) {
		err := loadTestModule(t, "package main\nimport \"fmt\"\nvar _ = fmt.Sprint(1)\nfunc main() {}\n", func(parser *Parser) {
			parser.SetPackageFilters(PackageFilterNotStandard())
		})
		if err == nil || !strings.Contains(err.Error(), "standard lib is not allowed") {
			t.Fatalf("unexpected error: %v", err)
		}
	})
	t.Run("dot import", func(t *testing.T) {
		err := loadTestModule(t, "package main\nimport . \"math\"\nvar _ = Pi\nfunc main() {}\n", func(parser *Parser) {
			parser.SetPackageFilters(&PackageFilter{Func: func(pkg *packages.Package) bool {
				return pkg.Module != nil || !strings.Contains(pkg.PkgPath, ".")
			}})
			parser.SetImportFilters(ImportFilterNoDotImport())
		})
		if err == nil || !strings.Contains(err.Error(), "do not use dot import") {
			t.Fatalf("unexpected error: %v", err)
		}
	})
}
