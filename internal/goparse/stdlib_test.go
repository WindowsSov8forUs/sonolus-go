package goparse

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/tools/go/packages"
)

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
