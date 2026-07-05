package frontend

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// TestStubDispatchAudit verifies that every function and method declared in
// sonolus/sonolus.go has a corresponding dispatch entry in at least one of the
// compiler's dispatch tables (runtimeFns, resolveBuiltinCall, callWithArgs,
// recordMethods, recordStatics, or builtinRecords). Any stub without a mapping
// will fail at compile time with "unknown function" — this test catches such
// gaps before they reach users.
func TestStubDispatchAudit(t *testing.T) {
	stubPath := filepath.Join("..", "..", "..", "sonolus", "sonolus.go")
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, stubPath, nil, 0)
	if err != nil {
		if os.IsNotExist(err) {
			t.Skipf("stub file not found (expected at %s relative to test wd): %v", stubPath, err)
		}
		t.Fatalf("parse stub: %v", err)
	}

	// Collect every function and method name from the stub package.
	stubFuncs := make(map[string]bool) // lowerFirst name → true
	stubMethods := make(map[string][]string) // type name → method names
	ast.Inspect(f, func(n ast.Node) bool {
		fd, ok := n.(*ast.FuncDecl)
		if !ok {
			return true
		}
		name := fd.Name.Name
		if fd.Recv != nil && len(fd.Recv.List) > 0 {
			// Method — record under its receiver type.
			recvType := receiverTypeName(fd.Recv.List[0].Type)
			if recvType != "" {
				stubMethods[recvType] = append(stubMethods[recvType], name)
			}
		} else {
			stubFuncs[lowerFirst(name)] = true
		}
		return true
	})

	// Build the set of all valid function dispatch targets.
	valid := make(map[string]string) // lowerFirst name → source of dispatch

	// 1. runtimeFns entries (builtins.go).
	for k := range runtimeFns {
		valid[k] = "runtimeFns"
	}

	// 2. resolveBuiltinCall cases (trace_call.go). These are function names
	// dispatched without pre-evaluating arguments.
	resolveBuiltinCases := []string{
		"sprite",
		"len",
		"array",
		"screen",
		"safeArea",
		"offsetAdjustedTime",
		"prevTime",
		"isPlay", "isWatch", "isPreview", "isTutorial",
		"varArray", "arrayMap", "arraySet", "frozenNumSet",
		"debugTerminate",
		"entityInfoIndex", "entityInfoArchetype", "entityInfoState",
		"entityInfoAt", "selfInfo", "life", "archetypeLife", "canvas", "skin", "skinSprite",
		"skinTransform", "setSkinTransform",
		"particleTransform", "setParticleTransform",
		"background", "setBackground",
		"levelScore", "setLevelScore",
		"levelLife", "setLevelLife",
	}
	for _, c := range resolveBuiltinCases {
		valid[c] = "resolveBuiltinCall"
	}

	// 3. callWithArgs cases (trace_call.go). These need pre-evaluated arguments.
	callWithArgsCases := []string{
		"touchId", "touchID", "touchStarted", "touchEnded", "touchX", "touchY",
		"get", "set",
		"pnpoly", "perspectiveApproach",
		"sortLinkedEntities",
		"debugError", "debugRequire", "debugAssertTrue", "debugAssertFalse",
	}
	for _, c := range callWithArgsCases {
		valid[c] = "callWithArgs"
	}

	// 4. Composite constructors — any name in builtinRecords is a valid function
	// target (handled by builtinRecordFields → inlineComposite).
	for _, rd := range builtinRecords {
		valid[rd.name] = "builtinRecords"
		// Also the underscored form: vec2_ → looks up "vec2" in builtinRecords
		valid[rd.name+"_"] = "builtinRecords"
	}

	// 5. Static constructors — handled by recordStatics (vec2Zero etc.).
	for _, methods := range recordStatics {
		for m := range methods {
			valid[m] = "recordStatics"
		}
	}

	// 6. Vec2Up etc. are also valid as bare constructors (lowerFirst matches).
	for _, n := range []string{"vec2Zero", "vec2One", "vec2Up", "vec2Down", "vec2Left", "vec2Right"} {
		valid[n] = "recordStatics"
	}

	// Build the set of valid method dispatch targets per receiver type.
	validMethods := make(map[string]map[string]bool)
	for rtype, methods := range recordMethods {
		set := make(map[string]bool)
		for m := range methods {
			set[m] = true
			set[lowerFirst(m)] = true // also PascalCase variant
		}
		validMethods[rtype] = set
	}

	// --- Audit functions ---
	var funcGaps []string
	for stubName := range stubFuncs {
		if _, ok := valid[stubName]; ok {
			continue
		}
		// Strip trailing underscore and check again (Vec2_ → vec2).
		trimmed := strings.TrimSuffix(stubName, "_")
		if _, ok := valid[trimmed]; ok {
			continue
		}
		// Handled by builtinRecordFields fallback in resolveBuiltinCall default case.
		if _, ok := builtinRecordFields(trimmed); ok {
			continue
		}
		// EntityInfoIndex etc. are already in resolveBuiltinCases above.
		funcGaps = append(funcGaps, stubName)
	}

	if len(funcGaps) > 0 {
		sort.Strings(funcGaps)
		t.Errorf("UNMAPPED stub functions (no dispatch in runtimeFns, resolveBuiltinCall, callWithArgs, builtinRecords, or recordStatics):\n")
		for _, g := range funcGaps {
			t.Errorf("  %s  → call to sonolus.%s would fail at compile time", g, pascalify(g))
		}
	}

	// --- Audit methods ---
	var methodGaps []string
	for rtype, methods := range stubMethods {
		rtLower := lowerFirst(rtype)
		validSet := validMethods[rtLower]
		if validSet == nil {
			// Check if the entire type is handled elsewhere
			methodGaps = append(methodGaps, rtype+": all methods (no recordMethods entry for type "+rtLower+")")
			continue
		}
		for _, m := range methods {
			lf := lowerFirst(m)
			if validSet[lf] || validSet[m] {
				continue
			}
			methodGaps = append(methodGaps, rtype+"."+m+" (lowerFirst="+lf+")")
		}
	}

	if len(methodGaps) > 0 {
		sort.Strings(methodGaps)
		t.Errorf("UNMAPPED stub methods (no dispatch in recordMethods):\n")
		for _, g := range methodGaps {
			t.Errorf("  %s", g)
		}
	}

	if len(funcGaps) == 0 && len(methodGaps) == 0 {
		t.Logf("All %d stub functions and %d types of methods are properly mapped.", len(stubFuncs), len(stubMethods))
	}
}

func receiverTypeName(expr ast.Expr) string {
	switch t := expr.(type) {
	case *ast.Ident:
		return t.Name
	case *ast.StarExpr:
		if id, ok := t.X.(*ast.Ident); ok {
			return id.Name
		}
	}
	return ""
}

func pascalify(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}
