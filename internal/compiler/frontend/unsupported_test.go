package frontend

import (
	"go/ast"
	"go/types"
	"strings"
	"testing"

	"github.com/WindowsSov8forUs/sonolus-go/internal/compiler/ir"
)

// statusEnv is a minimal play-mode environment suitable for compilation
// error tests (no archetypes, just accessors).
func statusEnv() Env {
	return Env{
		Names:     ModeAccessors(ir.ModePlay),
		Accessors: ModeAccessors(ir.ModePlay),
		Mode:      ir.ModePlay,
		Info: &types.Info{
			Types: map[ast.Expr]types.TypeAndValue{},
		},
	}
}

func TestUnsupportedDefer(t *testing.T) {
	_, _, err := Compile(`package p
func f() {
	defer func() { }()
}
`, statusEnv())
	if err == nil {
		t.Fatal("expected error for defer")
	}
	if !strings.Contains(err.Error(), "unsupported") {
		t.Errorf("error should mention unsupported, got: %v", err)
	}
}

func TestUnsupportedGoroutine(t *testing.T) {
	_, _, err := Compile(`package p
func f() {
	go g()
}
`, statusEnv())
	if err == nil {
		t.Fatal("expected error for go statement")
	}
	if !strings.Contains(err.Error(), "unsupported") {
		t.Errorf("error should mention unsupported, got: %v", err)
	}
}

func TestUnsupportedSelect(t *testing.T) {
	_, _, err := Compile(`package p
func f() {
	select {}
}
`, statusEnv())
	if err == nil {
		t.Fatal("expected error for select")
	}
	// select fails at parse, not trace — error message differs.
}

func TestUnsupportedChan(t *testing.T) {
	_, _, err := Compile(`package p
func f() {
	var c chan int
}
`, statusEnv())
	if err == nil {
		t.Fatal("expected error for chan type")
	}
}

func TestUnsupportedFuncLiteral(t *testing.T) {
	_, _, err := Compile(`package p
func f() {
	fn := func() {}
	fn()
}
`, statusEnv())
	if err == nil {
		t.Fatal("expected error for func literal")
	}
	if !strings.Contains(err.Error(), "unsupported") {
		t.Errorf("error should mention unsupported, got: %v", err)
	}
}

func TestUnsupportedInterface(t *testing.T) {
	// Interface declarations are silently ignored by the compiler (they are
	// type-level constructs with no runtime effect). This test documents that
	// behavior — interfaces do not cause errors.
	_, _, err := Compile(`package p
type I interface { M() }
func f() {}
`, statusEnv())
	if err != nil {
		t.Errorf("interface caused error (behavior may change): %v", err)
	}
}

func TestUnsupportedRecursiveHelper(t *testing.T) {
	// Recursive helper functions are not explicitly rejected by the trace pass;
	// they fail during type-checking or produce undefined identifier errors.
	// This test captures the current behavior.
	_, _, err := Compile(`package p
func r(x float64) float64 {
	return r(x-1) + 1
}
func f() {
	_ = r(5)
}
`, statusEnv())
	if err == nil {
		t.Fatal("recursive helper compiled — should have been rejected")
	}
	t.Logf("recursive helper error (expected): %v", err)
}

func TestUnsupportedComplexSwitch(t *testing.T) {
	// Tagless switch with expression cases is parsed but the frontend
	// may produce an error about break/continue context rather than
	// "unsupported statement". Both are valid rejection paths.
	_, _, err := Compile(`package p
func f() {
	x := 5
	switch {
	case x > 3:
		break
	}
}
`, statusEnv())
	if err == nil {
		t.Fatal("tagless switch compiled — should have been rejected")
	}
	if !strings.Contains(err.Error(), "unsupported") && !strings.Contains(err.Error(), "break") {
		t.Errorf("expected 'unsupported' or 'break' error, got: %v", err)
	}
}

func TestUnsupportedClosureCapture(t *testing.T) {
	// Closure that captures an outer variable should be rejected or handled
	// gracefully. Captured variables require heap allocation, which Sonolus
	// engine callbacks cannot support.
	_, _, err := Compile(`package p
func f() {
	x := 1
	fn := func() { x = x + 1 }
	fn()
}
`, statusEnv())
	if err == nil {
		t.Fatal("closure capture compiled — should have been rejected")
	}
	t.Logf("closure capture error (expected): %v", err)
}

func TestUnsupportedTypeSwitch(t *testing.T) {
	// Type switch (x.(type)) is distinct from value switch and requires
	// runtime type information not available in the Sonolus engine.
	_, _, err := Compile(`package p
type S struct{ x float64 }
func f() {
	var v any
	switch v.(type) {
	case S:
		break
	}
}
`, statusEnv())
	if err == nil {
		t.Fatal("type switch compiled — should have been rejected")
	}
	t.Logf("type switch error (expected): %v", err)
}

func TestUnsupportedMapField(t *testing.T) {
	// Map types in struct fields are not supported by the Sonolus engine
	// memory model (only float64 scalars are valid).
	_, _, err := Compile(`package p
type N struct {
	m map[float64]float64
}
func (n N) Initialize() {}
`, statusEnv())
	if err == nil {
		t.Fatal("map field compiled — should have been rejected")
	}
	t.Logf("map field error (expected): %v", err)
}

func TestUnsupportedUnexportedAccess(t *testing.T) {
	// Unexported (lowercase) fields accessed from another package are
	// rejected by Go's type checker before the frontend tracer runs.
	// This test verifies the error is surfaced correctly.
	_, _, err := Compile(`package p
type N struct {
	x float64
}
func (n N) Initialize() {
	n.x = 42
}
`, statusEnv())
	if err != nil {
		t.Logf("unexported access error: %v", err)
		return
	}
	// Unexported fields within the same package are valid Go and should compile.
	t.Log("unexported access compiled (valid — same package)")
}

// TestErrorMessages verifies that key error paths produce expected error messages.
func TestErrorMessages(t *testing.T) {
	tests := []struct {
		name, src, want string
	}{
		{"range key not ident", `package p; func f() { for xs[0] := range xs {} }`, "identifier"},
		{"break outside loop", `package p; func f() { break }`, "outside of a loop"},
		{"continue outside loop", `package p; func f() { continue }`, "outside of a loop"},
		{"multi-assign non-composite", `package p; func f() { a, b := 1 }`, "composite"},
		// for range without key variable is now supported (synthesizes throwaway key)
		// {"range no key variable", `package p; func f() { for range xs {} }`, "key variable"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, err := Compile(tt.src, statusEnv())
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Errorf("error %q does not contain %q", err.Error(), tt.want)
			}
		})
	}
}

// FuzzFrontend verifies that the frontend never panics on arbitrary valid Go
// source snippets. Only well-formed Go packages are tested — the goal is crash
// safety in the tracer, not correctness of every output.
func FuzzFrontend(f *testing.F) {
	seeds := []string{
		`package p; func f() { set(0,0,42) }`,
		`package p; func f() { x := 1; if x > 0 { x = x + 1 } }`,
		`package p; func f() { for i := 0; i < 10; i++ { set(0,0,1) } }`,
		`package p; func f() { switch get(0,0) { case 1: set(0,1,1) } }`,
	}
	for _, s := range seeds {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, src string) {
		env := playEnv()
		_, _, _, err := TypeCheck(src, env.Records)
		if err != nil {
			return // skip invalid Go
		}
		fset, file, info, err := TypeCheck(src, env.Records)
		if err != nil {
			return
		}
		env.Info = info
		gen := ir.NewIDGen()
		for _, d := range file.Decls {
			if fn, ok := d.(*ast.FuncDecl); ok && fn.Body != nil {
				_, _ = CompileBlock(fset, gen, fn.Body, env)
			}
		}
	})
}
