package frontend

import (
	"sort"
	"strings"
	"testing"

	"github.com/WindowsSov8forUs/sonolus-go/v2/internal/compiler/catalog"
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
