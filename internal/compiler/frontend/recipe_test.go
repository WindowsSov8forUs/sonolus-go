package frontend

import (
	"go/token"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/WindowsSov8forUs/sonolus-go/v2/internal/compiler/catalog"
	"golang.org/x/tools/go/packages"
)

func TestEveryCatalogOperationHasFrontendLowering(t *testing.T) {
	var missing []string
	for i := range catalog.Symbols {
		symbol := &catalog.Symbols[i]
		recipe := catalog.LookupRecipe(symbol)
		if !supportsRecipe(recipe) {
			missing = append(missing, symbol.Key()+" -> "+string(recipe.Kind)+":"+recipe.Operation)
		}
	}
	if len(missing) != 0 {
		sort.Strings(missing)
		t.Fatalf("catalog operations without frontend lowering:\n%s", strings.Join(missing, "\n"))
	}
}

func TestFiniteVariantAlternativeLimit(t *testing.T) {
	variant := finiteVariant[int]{}
	for value := 0; value < 256; value++ {
		index, ok := variant.add(value, func(left, right int) bool { return left == right })
		if !ok || index != value {
			t.Fatalf("alternative %d: index=%d ok=%v", value, index, ok)
		}
	}
	if index, ok := variant.add(255, func(left, right int) bool { return left == right }); !ok || index != 255 {
		t.Fatalf("duplicate alternative: index=%d ok=%v", index, ok)
	}
	if index, ok := variant.add(256, func(left, right int) bool { return left == right }); ok || index != -1 {
		t.Fatalf("overflow alternative: index=%d ok=%v", index, ok)
	}
}

func TestDiagnosticSourcePathsAreCheckoutIndependent(t *testing.T) {
	position := func(root string) string {
		files := token.NewFileSet()
		file := files.AddFile(filepath.Join(root, "engine", "play.go"), -1, 16)
		file.AddLine(8)
		pkg := &packages.Package{Fset: files, Module: &packages.Module{Dir: root}}
		return sourcePos(pkg, file.Pos(9)).File
	}
	left := position(filepath.Join(`C:\`, "checkout-a"))
	right := position(filepath.Join(`D:\`, "nested", "checkout-b"))
	if left != "engine/play.go" || right != left {
		t.Fatalf("canonical source paths differ: left=%q right=%q", left, right)
	}
}
