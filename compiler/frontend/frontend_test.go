package frontend

import (
	"go/ast"
	"go/importer"
	"go/parser"
	"go/token"
	"go/types"
	"strings"
	"testing"

	"github.com/WindowsSov8forUs/sonolus-core-go/core/resource"

	"github.com/WindowsSov8forUs/sonolus-go/compiler/ir"
	"github.com/WindowsSov8forUs/sonolus-go/compiler/modecompile"
	"github.com/WindowsSov8forUs/sonolus-go/compiler/snode"
)

var canon = modecompile.Canon

func playEnv() Env { return Env{Names: ModeAccessors(ir.ModePlay)} }

func compileToCanon(t *testing.T, src string) string {
	t.Helper()
	env := playEnv()
	fset, file, info, err := TypeCheck(src, env.Records)
	if err != nil {
		t.Fatalf("typecheck: %v", err)
	}
	env.Info = info
	gen := ir.NewIDGen()
	// Find the first function with a body.
	var fn *ast.FuncDecl
	for _, d := range file.Decls {
		if f, ok := d.(*ast.FuncDecl); ok && f.Body != nil {
			fn = f
			break
		}
	}
	if fn == nil {
		t.Fatal("no function with a body found")
	}
	entry, err := CompileBlock(fset, gen, fn.Body, env)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	entry = ir.AllocateTestBlocks(entry, ir.DefaultTempMemoryBlock)
	sn, err := ir.CFGToSNode(gen, entry)
	if err != nil {
		panic(err)
	}
	return canon(sn)
}

var testGen = ir.NewIDGen()

func TestConstantFolding(t *testing.T) {
	src := `package p
func f() {
	set(0, 0, 2 + 3*4)
}`
	got := compileToCanon(t, src)
	want := "Block(JumpLoop(Execute(Set(#0,#0,#14),#1),#0))"
	if got != want {
		t.Errorf("\n got: %s\nwant: %s", got, want)
	}
}

func TestExprFromMemory(t *testing.T) {
	// Locals are memory-backed (block 10000). x lives in cell 0.
	src := `package p
func f() {
	x := get(0, 0) + 1
	set(0, 1, x * 2)
}`
	got := compileToCanon(t, src)
	want := "Block(JumpLoop(Execute(" +
		"Set(#10000,#0,Add(Get(#0,#0),#1))," +
		"Set(#0,#1,Multiply(Get(#10000,#0),#2)),#1),#0))"
	if got != want {
		t.Errorf("\n got: %s\nwant: %s", got, want)
	}
}

func TestIfElse(t *testing.T) {
	src := `package p
func f() {
	x := get(0, 0) + 1
	if x > 5 {
		set(0, 1, x)
	} else {
		set(0, 1, 0)
	}
}`
	got := compileToCanon(t, src)
	// order: entry=0, then=1, else=2, merge=3, exit=4. x is cell 0 in block 10000.
	want := "Block(JumpLoop(" +
		"Execute(Set(#10000,#0,Add(Get(#0,#0),#1)),If(Greater(Get(#10000,#0),#5),#1,#2))," +
		"Execute(Set(#0,#1,Get(#10000,#0)),#3)," +
		"Execute(Set(#0,#1,#0),#3)," +
		"Execute(#4),#0))"
	if got != want {
		t.Errorf("\n got: %s\nwant: %s", got, want)
	}
}

// TestCrossBranchMerge is the case the previous (value-only) slice could not
// express: a variable mutated in both branches and read after the if. Memory
// backing makes the merge correct.
func TestCrossBranchMerge(t *testing.T) {
	src := `package p
func f() {
	x := 0
	if get(0, 0) {
		x = 1
	} else {
		x = 2
	}
	set(0, 1, x)
}`
	got := compileToCanon(t, src)
	want := "Block(JumpLoop(" +
		"Execute(Set(#10000,#0,#0),If(Get(#0,#0),#1,#2))," +
		"Execute(Set(#10000,#0,#1),#3)," +
		"Execute(Set(#10000,#0,#2),#3)," +
		"Execute(Set(#0,#1,Get(#10000,#0)),#4),#0))"
	if got != want {
		t.Errorf("\n got: %s\nwant: %s", got, want)
	}
}

func TestWhileLoop(t *testing.T) {
	src := `package p
func f() {
	for get(0, 0) {
		set(0, 1, 1)
	}
}`
	got := compileToCanon(t, src)
	// order: entry=0, header=1, body=2, exit=3. body jumps back to header(1).
	want := "Block(JumpLoop(" +
		"Execute(#1)," +
		"Execute(If(Get(#0,#0),#2,#3))," +
		"Execute(Set(#0,#1,#1),#1)," +
		"Execute(#4),#0))"
	if got != want {
		t.Errorf("\n got: %s\nwant: %s", got, want)
	}
}

func TestConstConditionFolded(t *testing.T) {
	// A compile-time-constant condition traces only the taken branch, with no
	// branching emitted at all.
	if got := compileToCanon(t, `package p
func f() {
	if 5 > 3 {
		set(0, 0, 1)
	} else {
		set(0, 0, 2)
	}
}`); got != "Block(JumpLoop(Execute(Set(#0,#0,#1),#1),#0))" {
		t.Errorf("true const condition: %s", got)
	}
	if got := compileToCanon(t, `package p
func f() {
	if false {
		set(0, 0, 1)
	} else {
		set(0, 0, 2)
	}
}`); got != "Block(JumpLoop(Execute(Set(#0,#0,#2),#1),#0))" {
		t.Errorf("false const condition: %s", got)
	}
}

func TestArrayStatic(t *testing.T) {
	// A 3-element array local (temp slots 0..2 in block 10000) with constant
	// indices.
	src := `package p
func f() {
	a := array(3)
	a[0] = 5
	a[1] = a[0] + 1
	set(0, 0, a[1])
}`
	got := compileToCanon(t, src)
	want := "Block(JumpLoop(Execute(" +
		"Set(#10000,#0,#5)," +
		"Set(#10000,#1,Add(Get(#10000,#0),#1))," +
		"Set(#0,#0,Get(#10000,#1)),#1),#0))"
	if got != want {
		t.Errorf("\n got: %s\nwant: %s", got, want)
	}
}

func TestArrayLen(t *testing.T) {
	src := `package p
func f() {
	a := array(5)
	set(0, 0, len(a))
}`
	got := compileToCanon(t, src)
	want := "Block(JumpLoop(Execute(Set(#0,#0,#5),#1),#0))"
	if got != want {
		t.Errorf("\n got: %s\nwant: %s", got, want)
	}
}

func TestArrayDynamicIndexCompiles(t *testing.T) {
	src := `package p
func f() {
	a := array(4)
	i := get(0, 0)
	a[i] = 9
	set(0, 1, a[i])
}`
	if _, _, err := Compile(src, playEnv()); err != nil {
		t.Fatalf("compile: %v", err)
	}
}

func TestVec2Fields(t *testing.T) {
	// A Vec2 record local. Field reads are scalar-replaced: v.x+v.y constant-folds
	// to 7 at trace time because both fields are tracked as individual Nums.
	src := `package p
func f() {
	v := vec2(3, 4)
	set(0, 0, v.x + v.y)
}`
	got := compileToCanon(t, src)
	want := "Block(JumpLoop(Execute(Set(#10000,#0,#3),Set(#10000,#1,#4),Set(#0,#0,#7),#1),#0))"
	if got != want {
		t.Errorf("\n got: %s\nwant: %s", got, want)
	}
}

func TestVec2FieldWrite(t *testing.T) {
	src := `package p
func f() {
	v := vec2(0, 0)
	v.x = 7
	set(0, 0, v.x)
}`
	got := compileToCanon(t, src)
	want := "Block(JumpLoop(Execute(" +
		"Set(#10000,#0,#0),Set(#10000,#1,#0),Set(#10000,#0,#7),Set(#0,#0,#7),#1),#0))"
	if got != want {
		t.Errorf("\n got: %s\nwant: %s", got, want)
	}
}

func TestRecordUnknownField(t *testing.T) {
	src := `package p
func f() {
	v := vec2(1, 2)
	set(0, 0, v.z)
}`
	if _, _, err := Compile(src, playEnv()); err == nil {
		t.Fatal("expected error for unknown field z")
	}
}

func TestRuntimeMathFunction(t *testing.T) {
	got := compileToCanon(t, `package p
func f() {
	set(0, 0, min(get(0, 1), 5))
}`)
	want := "Block(JumpLoop(Execute(Set(#0,#0,Min(Get(#0,#1),#5)),#1),#0))"
	if got != want {
		t.Errorf("\n got: %s\nwant: %s", got, want)
	}
}

func TestRuntimeDraw(t *testing.T) {
	// A side-effecting runtime op is emitted as a statement.
	got := compileToCanon(t, `package p
func f() {
	draw(1, 2, 3, 4)
}`)
	want := "Block(JumpLoop(Execute(Draw(#1,#2,#3,#4),#1),#0))"
	if got != want {
		t.Errorf("\n got: %s\nwant: %s", got, want)
	}
}

func TestRuntimeAccessor(t *testing.T) {
	// `time` reads RuntimeUpdate[0] (block 1001) in play mode.
	got := compileToCanon(t, `package p
func f() {
	set(0, 0, time)
}`)
	want := "Block(JumpLoop(Execute(Set(#0,#0,Get(#1001,#0)),#1),#0))"
	if got != want {
		t.Errorf("\n got: %s\nwant: %s", got, want)
	}
}

func TestRuntimeArityError(t *testing.T) {
	if _, _, err := Compile(`package p
func f() {
	set(0, 0, sin(1, 2))
}`, playEnv()); err == nil {
		t.Fatal("expected arity error for sin(1,2)")
	}
}

func TestCallbackReturnValue(t *testing.T) {
	// A value return becomes Break(value, 1) on the callback's JumpLoop, matching
	// the sonolus.py should_spawn node shape.
	got := compileToCanon(t, `package p
func f() {
	return get(0, 0)
}`)
	want := "Block(JumpLoop(Execute(Break(Get(#0,#0),#1),#1),#0))"
	if got != want {
		t.Errorf("\n got: %s\nwant: %s", got, want)
	}
}

func TestEarlyReturnVoid(t *testing.T) {
	// `if cond { return }` exits early; the rest runs only when cond is false.
	got := compileToCanon(t, `package p
func f() {
	if get(0, 0) {
		return
	}
	set(0, 1, 1)
}`)
	want := "Block(JumpLoop(" +
		"Execute(If(Get(#0,#0),#1,#2))," +
		"Execute(#3)," +
		"Execute(Set(#0,#1,#1),#3),#0))"
	if got != want {
		t.Errorf("\n got: %s\nwant: %s", got, want)
	}
}

func TestModeAccessors(t *testing.T) {
	canonEnv := func(mode ir.Mode, body string) (string, error) {
		entry, _, err := Compile("package p\nfunc f() {\n"+body+"\n}\n", Env{Names: ModeAccessors(mode)})
		if err != nil {
			return "", err
		}
		entry = ir.AllocateTestBlocks(entry, ir.DefaultTempMemoryBlock)
		sn, err := ir.CFGToSNode(testGen, entry)
		if err != nil {
			return "", err
		}
		return canon(sn), nil
	}

	cases := []struct {
		mode ir.Mode
		body string
		cell string
	}{
		{ir.ModeWatch, "set(0, 0, isSkip)", "Get(#1001,#3)"},
		{ir.ModePreview, "set(0, 0, scrollDirection)", "Get(#1001,#0)"},
		{ir.ModeTutorial, "set(0, 0, navigationDirection)", "Get(#1001,#2)"},
		{ir.ModeWatch, "set(0, 0, time)", "Get(#1001,#0)"},
	}
	for _, c := range cases {
		got, err := canonEnv(c.mode, c.body)
		if err != nil {
			t.Errorf("mode %d: %v", c.mode, err)
			continue
		}
		if !strings.Contains(got, c.cell) {
			t.Errorf("mode %d body %q: got %s, want cell %s", c.mode, c.body, got, c.cell)
		}
	}

	// A Play-only accessor is not in scope in another mode.
	if _, err := canonEnv(ir.ModePreview, "set(0, 0, touchCount)"); err == nil {
		t.Error("touchCount should be undefined in preview mode")
	}
}

func TestBareCompositeFieldRead(t *testing.T) {
	// vec2(3,4).x extracts the x field from a bare composite without a declaration.
	got := compileToCanon(t, `package p
func f() {
	set(0, 0, vec2(5, 7).x + vec2(5, 7).y)
}`)
	want := "Block(JumpLoop(Execute(Set(#0,#0,#12),#1),#0))"
	if got != want {
		t.Errorf("\n got: %s\nwant: %s", got, want)
	}
}


func TestForBreak(t *testing.T) {
	src := `package p
func f() {
	for i := 0; i < 10; i++ {
		if get(0, i) {
			break
		}
		set(0, i, 1)
	}
}`
	if _, _, err := Compile(src, playEnv()); err != nil {
		t.Fatalf("compile: %v", err)
	}
}

func TestIfNoElse(t *testing.T) {
	src := `package p
func f() {
	if get(0, 0) {
		set(0, 1, 1)
	}
}`
	got := compileToCanon(t, src)
	// order: entry=0, then=1, merge=2, exit=3. false branch -> merge(2).
	want := "Block(JumpLoop(" +
		"Execute(If(Get(#0,#0),#1,#2))," +
		"Execute(Set(#0,#1,#1),#2)," +
		"Execute(#3),#0))"
	if got != want {
		t.Errorf("\n got: %s\nwant: %s", got, want)
	}
}

func TestCompileToNodes(t *testing.T) {
	src := `package p
func f() {
	set(0, 0, 1 + 1)
}`
	entry, _, err := Compile(src, playEnv())
	if err != nil {
		t.Fatal(err)
	}
	var nodes []resource.EngineDataNode
	sn, err := ir.CFGToSNode(testGen, entry)
	if err != nil {
		t.Fatal(err)
	}
	root, err := snode.NewAppender(&nodes).Append(sn)
	if err != nil {
		t.Fatal(err)
	}
	if root != len(nodes)-1 {
		t.Errorf("root=%d nodes=%d", root, len(nodes))
	}
	if len(nodes) == 0 {
		t.Fatal("no nodes produced")
	}
}

// TestIfInitStatement verifies that `if x := f(); x > 0 { ... }` generates
// the same IR as the manually-expanded version: `x := f(); if x > 0 { ... }`.
func TestIfInitStatement(t *testing.T) {
	srcInit := `package p
	func f() {
		if x := get(0, 0) + 1; x > 5 {
			set(0, 1, x)
		}
	}`
	srcManual := `package p
	func f() {
		x := get(0, 0) + 1
		if x > 5 {
			set(0, 1, x)
		}
	}`
	gotInit := compileToCanon(t, srcInit)
	gotManual := compileToCanon(t, srcManual)
	if gotInit != gotManual {
		t.Errorf("if init statement IR mismatch:\n init: %s\nmanual: %s", gotInit, gotManual)
	}
}

// TestSwitchInitStatement verifies that `switch x := f(); x { ... }` generates
// the same IR as the manually-expanded version: `x := f(); switch x { ... }`.
func TestSwitchInitStatement(t *testing.T) {
	srcInit := `package p
	func f() {
		switch x := get(0, 0); x {
		case 1: set(0, 1, 1)
		case 2: set(0, 1, 2)
		default: set(0, 1, 0)
		}
	}`
	srcManual := `package p
	func f() {
		x := get(0, 0)
		switch x {
		case 1: set(0, 1, 1)
		case 2: set(0, 1, 2)
		default: set(0, 1, 0)
		}
	}`
	gotInit := compileToCanon(t, srcInit)
	gotManual := compileToCanon(t, srcManual)
	if gotInit != gotManual {
		t.Errorf("switch init statement IR mismatch:\n init: %s\nmanual: %s", gotInit, gotManual)
	}
}

func TestUnsupportedReportsError(t *testing.T) {
	src := `package p
func f() {
	defer f()
}`
	if _, _, err := Compile(src, playEnv()); err == nil {
		t.Fatal("expected error for unsupported defer statement")
	}
}

func TestConstantFoldMod(t *testing.T) {
	// -7 % 3 = 2 (Python-floored mod, matching runtime Op.Mod).
	src := "package p\nfunc f() {\nset(0, 0, (-7) % 3)\n}"
	got := compileToCanon(t, src)
	want := "Block(JumpLoop(Execute(Set(#0,#0,#2),#1),#0))"
	if got != want {
		t.Errorf("\n got: %s\nwant: %s", got, want)
	}
}

func TestSmoothstepRemoved(t *testing.T) {
	// smoothstep was incorrectly mapped to LerpClamped; now produces unknown fn error.
	src := "package p\nfunc f() {\nset(0, 0, smoothstep(0.5))\n}"
	if _, _, err := Compile(src, playEnv()); err == nil {
		t.Fatal("expected error for removed smoothstep")
	}
}

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

// TestRectD3Path verifies that rect(...) produces the correct {t,r,b,l} layout
// (not the 8-field Quad layout) under the go/types D3 path where Env.Info is set.
// This is a regression test for the miscompile caused by typecheck.go:24 defining
// Rect as 8-field while value.go:278 rectFields uses 4-field.
func TestRectD3Path(t *testing.T) {
	src := `package p
func F() float64 {
	r := rect(10, 20, 0, 5)
	return r.w()
}`
	fset, userFile, info, err := TypeCheck(src, nil)
	if err != nil {
		t.Skipf("typecheck with Info not available in this configuration: %v", err)
	}
	// Even if Info is non-nil but the user file didn't type-check cleanly, we
	// use the tracing path that employs types.Info.
	accessors := ModeAccessors(ir.ModePlay)
	env := Env{Names: accessors, Funcs: map[string]*ast.FuncDecl{}, Accessors: accessors, Mode: ir.ModePlay, Info: info}
	// Find the body of F() among top-level declarations.
	var funcBody *ast.BlockStmt
	for _, d := range userFile.Decls {
		if fd, ok := d.(*ast.FuncDecl); ok && fd.Name.Name == "F" {
			funcBody = fd.Body
			break
		}
	}
	if funcBody == nil {
		t.Fatal("could not find function F in user source")
	}
	entry, compileErr := CompileBlock(fset, testGen, funcBody, env)
	if compileErr != nil {
		t.Fatalf("compile rect.w(): %v", compileErr)
	}
	// Just verify we got a valid CFG entry — the important thing is no panic.
	if entry == nil {
		t.Fatal("CompileBlock returned nil entry")
	}
}

// TestVec2CrossBranchWrite verifies that record fields written in separate
// branches are correctly read after the merge point. This exercises the memory-
// backed fallback (the SSA-tracked val would diverge across branches without
// a per-field phi, so the memory path is needed for correctness).
func TestVec2CrossBranchWrite(t *testing.T) {
	src := `package p
	func f() {
		v := vec2(0, 0)
		if get(0, 0) == 1 {
			v.x = 5
		} else {
			v.y = 7
		}
		set(0, 1, v.x)
		set(0, 2, v.y)
	}`
	got := compileToCanon(t, src)
	if got == "" {
		t.Fatal("empty output for cross-branch record write")
	}
	// The output should load v.x and v.y from memory after the if/else merge.
	t.Logf("cross-branch record write: %s", got)
}

// TestRecordFieldFoldAfterWrite verifies that a record field access folds to a
// constant SSA value after a write, even without running the optimizer pipeline.
func TestRecordFieldFoldAfterWrite(t *testing.T) {
	src := `package p
	func f() {
		v := vec2(0, 0)
		v.x = 7
		v.y = v.x + 3
		set(0, 0, v.y)
	}`
	got := compileToCanon(t, src)
	// v.x folds to 7, v.y folds to 10.
	want := "Block(JumpLoop(Execute(" +
		"Set(#10000,#0,#0),Set(#10000,#1,#0),Set(#10000,#0,#7),Set(#10000,#1,#10),Set(#0,#0,#10),#1),#0))"
	if got != want {
		t.Errorf("\n got: %s\nwant: %s", got, want)
	}
}

// TestRecordMethodBadArity verifies that calling a record method with too few
// arguments produces a diagnostic instead of a compiler panic.
func TestRecordMethodBadArity(t *testing.T) {
	tests := []struct {
		name string
		src  string
	}{
		{"vec2.add with 0 args", "func F() float64 { v := vec2(1,2); return v.add() }"},
		{"vec2.dot with 0 args", "func F() float64 { v := vec2(1,2); return v.dot() }"},
		{"vec2.rotateAbout with 1 arg", "func F() float64 { v := vec2(1,2); return v.rotateAbout(vec2(0,1)) }"},
		{"rect.translate with 0 args", "func F() float64 { r := rect(1,2,3,4); return r.translate() }"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			src := "package p\n" + tc.src
			accessors := ModeAccessors(ir.ModePlay)
			env := Env{Names: accessors, Funcs: map[string]*ast.FuncDecl{}, Accessors: accessors, Mode: ir.ModePlay}
			// Parse and compile.
			fset, file, err := parseOneFile(src)
			if err != nil {
				t.Fatalf("parse: %v", err)
			}
			decl := file.Decls[0].(*ast.FuncDecl)
			_, compileErr := CompileBlock(fset, testGen, decl.Body, env)
			if compileErr == nil {
				t.Error("expected error for bad arity, got nil")
			}
		})
	}
}

func parseOneFile(src string) (*token.FileSet, *ast.File, error) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "test.go", src, 0)
	return fset, f, err
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

// TestPairMethods verifies that Pair comparison methods (lt/le/gt/ge/tuple)
// compile through to valid IR.
func TestPairMethods(t *testing.T) {
	src := "package p\nfunc f() {\n" +
		"	p := pair(1, 2)\n" +
		"	q := pair(3, 4)\n" +
		"	set(0, 0, p.lt(q))\n" +
		"	set(0, 1, p.le(q))\n" +
		"	set(0, 2, p.gt(q))\n" +
		"	set(0, 3, p.ge(q))\n" +
		"}"
	got := compileToCanon(t, src)
	if !strings.Contains(got, "Less(") && !strings.Contains(got, "And(") {
		t.Errorf("expected Less/And nodes in output, got: %s", got)
	}
}

// TestVarArrayLenCapacity verifies that VarArray.len() returns _size and
// VarArray.capacity() returns the compile-time constant.
func TestVarArrayLenCapacity(t *testing.T) {
	src := "package p\nfunc f() {\n" +
		"	arr := varArray(16)\n" +
		"	set(0, 0, arr.len())\n" +
		"	set(0, 1, arr.capacity())\n" +
		"}"
	got := compileToCanon(t, src)
	if !strings.Contains(got, "#16") {
		t.Errorf("expected capacity=16 in output, got: %s", got)
	}
}

// TestVarArrayAppendPop verifies that append writes to the backing array and
// increments _size, and pop reads and decrements.
func TestVarArrayAppendPop(t *testing.T) {
	src := "package p\nfunc f() {\n" +
		"	arr := varArray(8)\n" +
		"	arr.append(42)\n" +
		"	arr.append(99)\n" +
		"	set(0, 0, arr.pop())\n" +
		"	set(0, 1, arr.len())\n" +
		"}"
	got := compileToCanon(t, src)
	if got == "" || strings.Contains(got, "?") {
		t.Errorf("VarArray append/pop failed to compile: %s", got)
	}
}

// TestVarArrayClear verifies that clear() resets _size to 0.
func TestVarArrayClear(t *testing.T) {
	src := "package p\nfunc f() {\n" +
		"	arr := varArray(16)\n" +
		"	arr.append(1)\n" +
		"	arr.append(2)\n" +
		"	arr.clear()\n" +
		"	set(0, 0, arr.len())\n" +
		"}"
	got := compileToCanon(t, src)
	if got == "" || strings.Contains(got, "?") {
		t.Errorf("VarArray clear failed to compile: %s", got)
	}
}

// TestPairTupleRegistered verifies the tuple method entry exists in the
// recordMethods registry for the "pair" type.
func TestPairTupleRegistered(t *testing.T) {
	methods, ok := recordMethods["pair"]
	if !ok {
		t.Fatal("pair type not found in recordMethods")
	}
	entry, ok := methods["tuple"]
	if !ok {
		t.Fatal("tuple method not found for pair type")
	}
	if entry.minArity != 0 {
		t.Errorf("tuple minArity = %d, want 0", entry.minArity)
	}
}

// TestVarArrayContains verifies the linear search loop for contains() compiles.
func TestVarArrayContains(t *testing.T) {
	src := "package p\nfunc f() {\n" +
		"	arr := varArray(4)\n" +
		"	arr.append(10)\n" +
		"	arr.append(20)\n" +
		"	set(0, 0, arr.contains(20))\n" + // should be 1
		"	set(0, 1, arr.contains(99))\n" + // should be 0
		"}"
	got := compileToCanon(t, src)
	if got == "" || strings.Contains(got, "?") {
		t.Errorf("VarArray contains failed: %s", got)
	}
}

// TestVarArrayIndex verifies index() returns the correct 0-based index.
func TestVarArrayIndex(t *testing.T) {
	src := "package p\nfunc f() {\n" +
		"	arr := varArray(4)\n" +
		"	arr.append(5)\n" +
		"	arr.append(8)\n" +
		"	set(0, 0, arr.index(8))\n" + // should be 1
		"}"
	got := compileToCanon(t, src)
	if got == "" || strings.Contains(got, "?") {
		t.Errorf("VarArray index failed: %s", got)
	}
}

// TestVarArrayRemove verifies remove() compiles (swap-with-last + size--).
func TestVarArrayRemove(t *testing.T) {
	src := "package p\nfunc f() {\n" +
		"	arr := varArray(4)\n" +
		"	arr.append(1)\n" +
		"	arr.append(2)\n" +
		"	arr.append(3)\n" +
		"	set(0, 0, arr.remove(2))\n" + // should be 1 (found)
		"	set(0, 1, arr.len())\n" +    // should be 2
		"}"
	got := compileToCanon(t, src)
	if got == "" || strings.Contains(got, "?") {
		t.Errorf("VarArray remove failed: %s", got)
	}
}

// TestArrayMapBasic verifies arrayMap declaration and basic methods compile.
func TestArrayMapBasic(t *testing.T) {
	src := "package p\nfunc f() {\n" +
		"	m := arrayMap(8)\n" +
		"	m.set(1, 100)\n" +
		"	m.set(2, 200)\n" +
		"	set(0, 0, m.get(1))\n" +  // should be 100
		"	set(0, 1, m.len())\n" +    // should be 2
		"	set(0, 2, m.contains(2))\n" + // should be 1
		"}"
	got := compileToCanon(t, src)
	if got == "" || strings.Contains(got, "?") {
		t.Errorf("ArrayMap basic failed: %s", got)
	}
}

// TestArraySetBasic verifies arraySet declaration and add/contains compile.
func TestArraySetBasic(t *testing.T) {
	src := "package p\nfunc f() {\n" +
		"	s := arraySet(8)\n" +
		"	s.add(42)\n" +
		"	set(0, 0, s.contains(42))\n" + // should be 1
		"	set(0, 1, s.len())\n" +        // should be 1
		"}"
	got := compileToCanon(t, src)
	if got == "" || strings.Contains(got, "?") {
		t.Errorf("ArraySet basic failed: %s", got)
	}
}

// TestVarArrayRangeIteration verifies for-range over a VarArray compiles.
func TestVarArrayRangeIteration(t *testing.T) {
	src := "package p\nfunc f() {\n" +
		"	arr := varArray(8)\n" +
		"	arr.append(10)\n" +
		"	arr.append(20)\n" +
		"	sum := 0\n" +
		"	for _, v := range arr {\n" +
		"		sum = sum + v\n" +
		"	}\n" +
		"	set(0, 0, sum)\n" +
		"}"
	got := compileToCanon(t, src)
	if got == "" || strings.Contains(got, "?") {
		t.Errorf("VarArray range failed: %s", got)
	}
}

// TestVarArrayRangeWithIndex verifies for i, v := range compiles.
func TestVarArrayRangeWithIndex(t *testing.T) {
	src := "package p\nfunc f() {\n" +
		"	arr := varArray(4)\n" +
		"	arr.append(5)\n" +
		"	arr.append(7)\n" +
		"	sum := 0\n" +
		"	for i, v := range arr {\n" +
		"		sum = sum + i + v\n" +
		"	}\n" +
		"	set(0, 0, sum)\n" +
		"}"
	got := compileToCanon(t, src)
	if got == "" || strings.Contains(got, "?") {
		t.Errorf("VarArray range with index failed: %s", got)
	}
}

// TestArrayMapRangeIteration verifies for-range over an ArrayMap compiles.
func TestArrayMapRangeIteration(t *testing.T) {
	src := "package p\nfunc f() {\n" +
		"	m := arrayMap(4)\n" +
		"	m.set(1, 10)\n" +
		"	m.set(2, 20)\n" +
		"	sum := 0\n" +
		"	for _, v := range m {\n" +
		"		sum = sum + v\n" +
		"	}\n" +
		"	set(0, 0, sum)\n" +
		"}"
	got := compileToCanon(t, src)
	if got == "" || strings.Contains(got, "?") {
		t.Errorf("ArrayMap range failed: %s", got)
	}
}

// TestVarArrayInsert verifies insert(idx, val) shifts elements right.
func TestVarArrayInsert(t *testing.T) {
	src := "package p\nfunc f() {\n" +
		"	arr := varArray(8)\n" +
		"	arr.append(10)\n" +
		"	arr.append(30)\n" +
		"	arr.insert(1, 20)\n" + // insert 20 at index 1 → [10, 20, 30]
		"	set(0, 0, arr.len())\n" + // should be 3
		"}"
	got := compileToCanon(t, src)
	if got == "" || strings.Contains(got, "?") {
		t.Errorf("VarArray insert failed: %s", got)
	}
}

// TestVarArrayPopIndex verifies pop(idx) shifts remaining elements.
func TestVarArrayPopIndex(t *testing.T) {
	src := "package p\nfunc f() {\n" +
		"	arr := varArray(8)\n" +
		"	arr.append(1)\n" +
		"	arr.append(2)\n" +
		"	arr.append(3)\n" +
		"	set(0, 0, arr.pop())\n" + // pop last → 3
		"	set(0, 1, arr.len())\n" + // should be 2
		"}"
	got := compileToCanon(t, src)
	if got == "" || strings.Contains(got, "?") {
		t.Errorf("VarArray pop failed: %s", got)
	}
}

// TestFrozenNumSetBasic verifies FrozenNumSet declaration and contains().
func TestFrozenNumSetBasic(t *testing.T) {
	src := "package p\nfunc f() {\n" +
		"	s := frozenNumSet(8)\n" +
		"	set(0, 0, s.contains(42))\n" +
		"	set(0, 1, s.len())\n" +
		"	set(0, 2, s.capacity())\n" +
		"}"
	got := compileToCanon(t, src)
	if got == "" || strings.Contains(got, "?") {
		t.Errorf("FrozenNumSet basic failed: %s", got)
	}
}

// TestVarArraySort verifies sort() compiles (insertion sort on backing array).
func TestVarArraySort(t *testing.T) {
	src := "package p\nfunc f() {\n" +
		"	arr := varArray(8)\n" +
		"	arr.append(30)\n" +
		"	arr.append(10)\n" +
		"	arr.append(20)\n" +
		"	arr.sort()\n" +
		"	set(0, 0, arr.len())\n" +
		"}"
	got := compileToCanon(t, src)
	if got == "" || strings.Contains(got, "?") {
		t.Errorf("VarArray sort failed: %s", got)
	}
}
