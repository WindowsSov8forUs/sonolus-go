package frontend

import (
	"go/ast"
	"go/importer"
	"go/parser"
	"go/token"
	"go/types"
	"strings"
	"testing"
)

// TestTypeCheckValid verifies that well-typed engine source passes go/types.
func TestTypeCheckValid(t *testing.T) {
	src := "package p\n" +
		"func f() {\n" +
		"	x := vec2(1, 2)\n" +
		"	y := x.add(vec2(3, 4))\n" +
		"	set(0, 0, y.x)\n" +
		"}\n"
	_, _, info, err := TypeCheck(src, nil)
	if err != nil {
		t.Fatalf("type check: %v", err)
	}
	if info == nil {
		t.Fatal("expected non-nil types.Info")
	}
}

// TestTypeCheckWrongArgCount verifies that too-few arguments are caught.
func TestTypeCheckWrongArgCount(t *testing.T) {
	src := "package p\nfunc f() {\n	set(0, 0)\n}\n" // set needs 3 args
	_, _, _, err := TypeCheck(src, nil)
	if err == nil {
		t.Fatal("expected type error for wrong arg count")
	}
	t.Logf("type error (expected): %v", err)
}

// TestTypeCheckUndefinedIdent verifies that undefined names are caught.
func TestTypeCheckUndefinedIdent(t *testing.T) {
	src := "package p\nfunc f() {\n	set(0, 0, undefinedVar)\n}\n"
	_, _, _, err := TypeCheck(src, nil)
	if err == nil {
		t.Fatal("expected type error for undefined variable")
	}
	t.Logf("type error (expected): %v", err)
}

// TestPreludeSource verifies the prelude generates valid Go and includes
// user-record constructors with correct field signatures. User record types
// are defined in the engine source; the prelude only adds constructor stubs
// so go/types can resolve constructor calls.
func TestPreludeSource(t *testing.T) {
	src := PreludeSource("p", map[string][]string{"myRec": {"a", "b"}})
	if !strings.Contains(src, "type Vec2 struct") {
		t.Error("prelude missing Vec2")
	}
	if !strings.Contains(src, "func myRec(a float64, b float64) MyRec") {
		t.Errorf("prelude missing user record constructor myRec, got:\n%s", src)
	}
	// Verify prelude + user type declaration form a valid package.
	fset := token.NewFileSet()
	userSrc := "package p; type MyRec struct { a, b float64 }"
	preludeFile, _ := parser.ParseFile(fset, "prelude.go", src, 0)
	userFile, _ := parser.ParseFile(fset, "engine.go", userSrc, 0)
	conf := types.Config{Importer: importer.Default()}
	if _, err := conf.Check("p", fset, []*ast.File{preludeFile, userFile}, &types.Info{}); err != nil {
		t.Fatalf("prelude + user source check: %v", err)
	}
}

// TestPreludeSourcePair verifies that the prelude now includes Pair and VarArray
// type declarations and constructors.
func TestPreludeSourcePair(t *testing.T) {
	src := PreludeSource("p", nil)
	if !strings.Contains(src, "type Pair struct") {
		t.Error("prelude missing Pair type")
	}
	if !strings.Contains(src, "func pair(first, second float64) Pair") {
		t.Error("prelude missing pair() constructor")
	}
	if !strings.Contains(src, "type VarArray struct") {
		t.Error("prelude missing VarArray type")
	}
	if !strings.Contains(src, "func varArray(capacity float64) VarArray") {
		t.Error("prelude missing varArray() constructor")
	}
	// Verify prelude parses + type-checks.
	fset := token.NewFileSet()
	f, _ := parser.ParseFile(fset, "prelude.go", src, 0)
	conf := types.Config{Importer: importer.Default()}
	if _, err := conf.Check("p", fset, []*ast.File{f}, &types.Info{}); err != nil {
		t.Fatalf("prelude type check: %v", err)
	}
}
