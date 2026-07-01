package frontend

import (
	"go/ast"
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
		"	set(0, 1, arr.len())\n" + // should be 2
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
		"	set(0, 0, m.get(1))\n" + // should be 100
		"	set(0, 1, m.len())\n" + // should be 2
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
		"	set(0, 1, s.len())\n" + // should be 1
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

// --- Statement type tests (trace_stmt.go coverage) ---

func TestSwitchStatement(t *testing.T) {
	// switch with multiple cases lowers to if-else chain.
	src := `package p
	func f() {
		x := get(0, 0)
		switch x {
		case 1:
			set(0, 1, 2)
		case 2:
			set(0, 1, 3)
		default:
			set(0, 1, 0)
		}
	}`
	got := compileToCanon(t, src)
	if got == "" {
		t.Fatal("switch statement failed to compile")
	}
	if !strings.Contains(got, "If") {
		t.Errorf("expected If in switch output: %s", got)
	}
}

func TestForLoopWithCondition(t *testing.T) {
	src := `package p
	func f() {
		x := 0
		for x < 10 {
			set(0, 0, x)
			x = x + 1
		}
	}`
	got := compileToCanon(t, src)
	if got == "" {
		t.Fatal("for loop with condition failed to compile")
	}
	// for x < 10 { ... } lowers to a header block with If test and jumpback.
}

func TestForLoopInfinite(t *testing.T) {
	src := `package p
	func f() {
		for {
			set(0, 0, 1)
			break
		}
	}`
	got := compileToCanon(t, src)
	if got == "" {
		t.Fatal("infinite for loop failed to compile")
	}
}

func TestReturnStatement(t *testing.T) {
	src := `package p
	func f() {
		if get(0, 0) {
			return
		}
		set(0, 0, 1)
	}`
	got := compileToCanon(t, src)
	if got == "" {
		t.Fatal("return statement failed to compile")
	}
}

func TestShortVarDecl(t *testing.T) {
	src := `package p
	func f() {
		x := get(0, 0)
		y := x + 1
		set(0, 0, y)
	}`
	got := compileToCanon(t, src)
	if got == "" {
		t.Fatal("short var decl failed to compile")
	}
}

// --- Expression type tests (trace_control.go coverage) ---

func TestMinMaxFunctions(t *testing.T) {
	src := `package p
	func f() {
		a := get(0, 0)
		b := get(0, 1)
		set(0, 2, max(a, b))
		set(0, 3, min(a, b))
	}`
	got := compileToCanon(t, src)
	if got == "" || strings.Contains(got, "?") {
		t.Errorf("min/max failed: %s", got)
	}
}

func TestClampLerp(t *testing.T) {
	src := `package p
	func f() {
		x := get(0, 0)
		v := clamp(x, 0, 1)
		w := lerp(0, 100, v)
		set(0, 1, w)
	}`
	got := compileToCanon(t, src)
	if got == "" || strings.Contains(got, "?") {
		t.Errorf("clamp/lerp failed: %s", got)
	}
}

func TestLogicalAndShortCircuit(t *testing.T) {
	src := `package p
	func f() {
		a := get(0, 0)
		b := get(0, 1)
		if a > 0 && b > 0 {
			set(0, 2, 1)
		}
	}`
	got := compileToCanon(t, src)
	if got == "" {
		t.Fatal("logical AND short-circuit failed to compile")
	}
}

func TestLogicalOrShortCircuit(t *testing.T) {
	src := `package p
	func f() {
		a := get(0, 0)
		b := get(0, 1)
		if a > 0 || b > 0 {
			set(0, 2, 1)
		}
	}`
	got := compileToCanon(t, src)
	if got == "" {
		t.Fatal("logical OR short-circuit failed to compile")
	}
}

func TestUnaryNegation(t *testing.T) {
	src := `package p
	func f() {
		x := get(0, 0)
		set(0, 1, -x)
	}`
	got := compileToCanon(t, src)
	if got == "" || strings.Contains(got, "?") {
		t.Errorf("unary negation failed: %s", got)
	}
}

func TestVec2Add(t *testing.T) {
	src := `package p
	func f() {
		a := vec2(get(0, 0), get(0, 1))
		b := vec2(get(0, 2), get(0, 3))
		c := a.add(b)
		set(0, 4, c.x)
	}`
	got := compileToCanon(t, src)
	if got == "" || strings.Contains(got, "?") {
		t.Errorf("Vec2.add failed: %s", got)
	}
}

func TestVec2Sub(t *testing.T) {
	src := `package p
	func f() {
		a := vec2(get(0, 0), get(0, 1))
		b := vec2(get(0, 2), get(0, 3))
		c := a.sub(b)
		set(0, 4, c.x)
	}`
	got := compileToCanon(t, src)
	if got == "" || strings.Contains(got, "?") {
		t.Errorf("Vec2.sub failed: %s", got)
	}
}

func TestVec2Mul(t *testing.T) {
	src := `package p
	func f() {
		v := vec2(3.0, 4.0)
		s := v.mul(2.0)
		set(0, 0, s.x)
	}`
	got := compileToCanon(t, src)
	if got == "" || strings.Contains(got, "?") {
		t.Errorf("Vec2.mul failed: %s", got)
	}
}

func TestRectFields(t *testing.T) {
	src := `package p
	func f() {
		r := rect(get(0, 0), get(0, 1), get(0, 2), get(0, 3))
		set(0, 8, r.l)
		set(0, 9, r.r)
		set(0, 10, r.b)
		set(0, 11, r.t)
	}`
	got := compileToCanon(t, src)
	if got == "" || strings.Contains(got, "?") {
		t.Errorf("Rect field access failed: %s", got)
	}
}

func TestVec2Div(t *testing.T) {
	src := `package p
	func f() {
		v := vec2(get(0, 0), get(0, 1))
		d := v.div(2.0)
		set(0, 2, d.x)
	}`
	got := compileToCanon(t, src)
	if got == "" || strings.Contains(got, "?") {
		t.Errorf("Vec2.div failed: %s", got)
	}
}

// --- Control flow combination tests ---

func TestNestedIfInFor(t *testing.T) {
	src := `package p
	func f() {
		x := 0
		for x < 5 {
			if x > 2 {
				set(0, x, 1)
			}
			x = x + 1
		}
	}`
	got := compileToCanon(t, src)
	if got == "" {
		t.Fatal("nested if in for failed to compile")
	}
}

func TestIfElseIfChain(t *testing.T) {
	src := `package p
	func f() {
		x := get(0, 0)
		if x < 0 {
			set(0, 1, -1)
		} else if x == 0 {
			set(0, 1, 0)
		} else {
			set(0, 1, 1)
		}
	}`
	got := compileToCanon(t, src)
	if got == "" {
		t.Fatal("if-else-if chain failed to compile")
	}
}

// --- Error path tests ---

func TestUnusedVariableCompiles(t *testing.T) {
	// An unused variable should still compile (it's a warning, not an error).
	src := `package p
	func f() {
		x := 5
		set(0, 0, 1)
	}`
	got := compileToCanon(t, src)
	if got == "" {
		t.Fatal("unused variable should compile")
	}
}

func TestDeeplyNestedExpr(t *testing.T) {
	src := `package p
	func f() {
		x := get(0, 0)
		y := (x + 1) * (x - 2) + (x * 3) - (x / 2)
		set(0, 1, y)
	}`
	got := compileToCanon(t, src)
	if got == "" || strings.Contains(got, "?") {
		t.Errorf("deeply nested expr failed: %s", got)
	}
}

// --- Runtime function tests (trace_call.go / builtins_fn.go coverage) ---

func TestRuntimeFloorCeil(t *testing.T) {
	src := `package p
	func f() {
		a := floor(3.7)
		b := ceil(2.1)
		set(0, 0, a + b)
	}`
	got := compileToCanon(t, src)
	if got == "" || strings.Contains(got, "?") {
		t.Errorf("floor/ceil failed: %s", got)
	}
}

func TestRuntimeRoundFrac(t *testing.T) {
	src := `package p
	func f() {
		a := round(3.5)
		b := frac(3.7)
		set(0, 0, a + b)
	}`
	got := compileToCanon(t, src)
	if got == "" || strings.Contains(got, "?") {
		t.Errorf("round/frac failed: %s", got)
	}
}

func TestRuntimeSignAbs(t *testing.T) {
	src := `package p
	func f() {
		a := sign(-5)
		b := abs(-5)
		set(0, 0, a + b)
	}`
	got := compileToCanon(t, src)
	if got == "" || strings.Contains(got, "?") {
		t.Errorf("sign/abs failed: %s", got)
	}
}

func TestRuntimeTrig(t *testing.T) {
	src := `package p
	func f() {
		a := sin(0.5)
		b := cos(0.5)
		c := tan(0.3)
		set(0, 0, a + b + c)
	}`
	got := compileToCanon(t, src)
	if got == "" || strings.Contains(got, "?") {
		t.Errorf("sin/cos/tan failed: %s", got)
	}
}

func TestRuntimeHyperbolic(t *testing.T) {
	src := `package p
	func f() {
		a := sinh(1.0)
		b := cosh(1.0)
		c := tanh(0.5)
		set(0, 0, a + b + c)
	}`
	got := compileToCanon(t, src)
	if got == "" || strings.Contains(got, "?") {
		t.Errorf("sinh/cosh/tanh failed: %s", got)
	}
}

func TestRuntimeArcTrig(t *testing.T) {
	src := `package p
	func f() {
		a := asin(0.5)
		b := acos(0.5)
		c := atan(1.0)
		d := atan2(1.0, 1.0)
		set(0, 0, a + b + c + d)
	}`
	got := compileToCanon(t, src)
	if got == "" || strings.Contains(got, "?") {
		t.Errorf("asin/acos/atan/atan2 failed: %s", got)
	}
}

func TestRuntimeRadianDegree(t *testing.T) {
	src := `package p
	func f() {
		r := radian(180)
		d := degree(3.14159)
		set(0, 0, r + d)
	}`
	got := compileToCanon(t, src)
	if got == "" || strings.Contains(got, "?") {
		t.Errorf("radian/degree failed: %s", got)
	}
}

func TestRuntimeLogPow(t *testing.T) {
	src := `package p
	func f() {
		a := log(10)
		b := power(2, 8)
		set(0, 0, a + b)
	}`
	got := compileToCanon(t, src)
	if got == "" || strings.Contains(got, "?") {
		t.Errorf("log/power failed: %s", got)
	}
}

func TestRuntimeRemap(t *testing.T) {
	src := `package p
	func f() {
		v := remap(0.5, 0, 1, 0, 100)
		w := remapClamped(0.5, 0, 1, 0, 100)
		set(0, 0, v + w)
	}`
	got := compileToCanon(t, src)
	if got == "" || strings.Contains(got, "?") {
		t.Errorf("remap/remapClamped failed: %s", got)
	}
}

func TestRuntimeLerp(t *testing.T) {
	src := `package p
	func f() {
		v := lerp(0, 100, 0.5)
		w := lerpClamped(0, 100, 0.5)
		set(0, 0, v + w)
	}`
	got := compileToCanon(t, src)
	if got == "" || strings.Contains(got, "?") {
		t.Errorf("lerp/lerpClamped failed: %s", got)
	}
}

func TestRuntimePowerMod(t *testing.T) {
	src := `package p
	func f() {
		a := power(2, 3)
		b := mod(10, 3)
		c := rem(10, 3)
		set(0, 0, a + b + c)
	}`
	got := compileToCanon(t, src)
	if got == "" || strings.Contains(got, "?") {
		t.Errorf("power/mod/rem failed: %s", got)
	}
}

func TestRuntimeComparisonOps(t *testing.T) {
	src := `package p
	func f() {
		a := get(0, 0)
		b := get(0, 1)
		if a < b {
			set(0, 2, 1)
		}
		if a >= b {
			set(0, 2, 2)
		}
	}`
	got := compileToCanon(t, src)
	if got == "" || strings.Contains(got, "?") {
		t.Errorf("comparison ops failed: %s", got)
	}
}

// --- VarArray method tests (containers.go coverage) ---

func TestVarArrayLenAndCapacity(t *testing.T) {
	src := `package p
	func f() {
		arr := varArray(8)
		arr.append(10)
		l := arr.len()
		c := arr.capacity()
		set(0, 0, l)
		set(0, 1, c)
	}`
	got := compileToCanon(t, src)
	if got == "" || strings.Contains(got, "?") {
		t.Errorf("VarArray len/capacity failed: %s", got)
	}
}

func TestVarArrayIsFull(t *testing.T) {
	src := `package p
	func f() {
		arr := varArray(1)
		arr.append(42)
		if arr.isFull() {
			set(0, 0, 1)
		}
	}`
	got := compileToCanon(t, src)
	if got == "" || strings.Contains(got, "?") {
		t.Errorf("VarArray isFull failed: %s", got)
	}
}

func TestVarArrayPopAndClear(t *testing.T) {
	src := `package p
	func f() {
		arr := varArray(8)
		arr.append(10)
		arr.append(20)
		v := arr.pop()
		arr.clear()
		set(0, 0, v)
	}`
	got := compileToCanon(t, src)
	if got == "" || strings.Contains(got, "?") {
		t.Errorf("VarArray pop/clear failed: %s", got)
	}
}

func TestVarArrayRemoveAt(t *testing.T) {
	src := `package p
	func f() {
		arr := varArray(8)
		arr.append(10)
		arr.append(20)
		arr.append(30)
		arr.remove(1)
		set(0, 0, arr.len())
	}`
	got := compileToCanon(t, src)
	if got == "" || strings.Contains(got, "?") {
		t.Errorf("VarArray remove failed: %s", got)
	}
}

// --- ArrayMap method tests ---

func TestArrayMapGetSet(t *testing.T) {
	src := `package p
	func f() {
		m := arrayMap(4)
		m.set(2, 99)
		v := m.get(2)
		set(0, 0, v)
	}`
	got := compileToCanon(t, src)
	if got == "" || strings.Contains(got, "?") {
		t.Errorf("ArrayMap get/set failed: %s", got)
	}
}

func TestArrayMapContains(t *testing.T) {
	src := `package p
	func f() {
		m := arrayMap(4)
		m.set(2, 99)
		if m.contains(2) {
			set(0, 0, 1)
		}
	}`
	got := compileToCanon(t, src)
	if got == "" || strings.Contains(got, "?") {
		t.Errorf("ArrayMap contains failed: %s", got)
	}
}

// --- ArraySet method test ---

func TestArraySetAddContains(t *testing.T) {
	src := `package p
	func f() {
		s := arraySet(4)
		s.add(10)
		if s.contains(10) {
			set(0, 0, 1)
		}
	}`
	got := compileToCanon(t, src)
	if got == "" || strings.Contains(got, "?") {
		t.Errorf("ArraySet add/contains failed: %s", got)
	}
}

// --- FrozenNumSet test ---

func TestFrozenNumSetContainsMultiple(t *testing.T) {
	src := `package p
	func f() {
		fs := frozenNumSet(8)
		if fs.contains(8) {
			set(0, 0, 1)
		}
	}`
	got := compileToCanon(t, src)
	if got == "" || strings.Contains(got, "?") {
		t.Errorf("FrozenNumSet contains failed: %s", got)
	}
}

// --- Pair tuple test ---

func TestPairValues(t *testing.T) {
	src := `package p
	func f() {
		p := pair(42, 99)
		a := p.first
		b := p.second
		set(0, 0, a + b)
	}`
	got := compileToCanon(t, src)
	if got == "" || strings.Contains(got, "?") {
		t.Errorf("Pair first/second failed: %s", got)
	}
}

// --- For-range iteration tests ---

func TestVarArrayRangeBreak(t *testing.T) {
	src := `package p
	func f() {
		arr := varArray(4)
		arr.append(1)
		arr.append(2)
		arr.append(3)
		for i, v := range arr {
			set(0, 0, v)
		}
	}`
	got := compileToCanon(t, src)
	if got == "" || strings.Contains(got, "?") {
		t.Errorf("VarArray range failed: %s", got)
	}
}

func TestArrayMapRange(t *testing.T) {
	src := `package p
	func f() {
		m := arrayMap(4)
		m.set(0, 10)
		m.set(1, 20)
		for k, v := range m {
			set(0, k, v)
		}
	}`
	got := compileToCanon(t, src)
	if got == "" || strings.Contains(got, "?") {
		t.Errorf("ArrayMap range failed: %s", got)
	}
}

// --- Record field write tests ---

func TestRecordFieldWrite(t *testing.T) {
	src := `package p
	func f() {
		r := rect(get(0, 0), get(0, 1), get(0, 2), get(0, 3))
		r.l = 10.0
		set(0, 4, r.l)
	}`
	got := compileToCanon(t, src)
	if got == "" || strings.Contains(got, "?") {
		t.Errorf("Record field write failed: %s", got)
	}
}

// --- Return with value test ---

func TestReturnWithValue(t *testing.T) {
	src := `package p
	func f() {
		if get(0, 0) > 5 {
			set(0, 0, 1)
			return
		}
		set(0, 0, 0)
	}`
	got := compileToCanon(t, src)
	if got == "" || strings.Contains(got, "?") {
		t.Errorf("return with value failed: %s", got)
	}
}

// --- Arithmetic assignment tests ---

func TestAddAssign(t *testing.T) {
	src := `package p
	func f() {
		x := get(0, 0)
		x += 5
		set(0, 1, x)
	}`
	got := compileToCanon(t, src)
	if got == "" || strings.Contains(got, "?") {
		t.Errorf("add assign failed: %s", got)
	}
}

func TestMulAssign(t *testing.T) {
	src := `package p
	func f() {
		x := get(0, 0)
		x *= 2
		set(0, 1, x)
	}`
	got := compileToCanon(t, src)
	if got == "" || strings.Contains(got, "?") {
		t.Errorf("mul assign failed: %s", got)
	}
}

// --- Increment/decrement tests ---

func TestIncrement(t *testing.T) {
	src := `package p
	func f() {
		x := get(0, 0)
		x++
		set(0, 1, x)
	}`
	got := compileToCanon(t, src)
	if got == "" || strings.Contains(got, "?") {
		t.Errorf("increment failed: %s", got)
	}
}

func TestDecrement(t *testing.T) {
	src := `package p
	func f() {
		x := get(0, 0)
		x--
		set(0, 1, x)
	}`
	got := compileToCanon(t, src)
	if got == "" || strings.Contains(got, "?") {
		t.Errorf("decrement failed: %s", got)
	}
}

// --- Integer literal test ---

func TestIntegerLiteral(t *testing.T) {
	src := `package p
	func f() {
		set(0, 0, 42)
	}`
	got := compileToCanon(t, src)
	if got == "" || strings.Contains(got, "?") {
		t.Errorf("integer literal failed: %s", got)
	}
}

// --- Boolean literal test ---

func TestBooleanLiteral(t *testing.T) {
	src := `package p
	func f() {
		if true {
			set(0, 0, 1)
		}
	}`
	got := compileToCanon(t, src)
	if got == "" || strings.Contains(got, "?") {
		t.Errorf("boolean literal failed: %s", got)
	}
}

// --- Float literal test ---

func TestFloatLiteral(t *testing.T) {
	src := `package p
	func f() {
		set(0, 0, 3.14159)
	}`
	got := compileToCanon(t, src)
	if got == "" || strings.Contains(got, "?") {
		t.Errorf("float literal failed: %s", got)
	}
}

// --- Vec2 composite literal test ---

func TestVec2CompositeLiteral(t *testing.T) {
	src := `package p
	func f() {
		v := vec2{3, 4}
		set(0, 0, v.x)
	}`
	got := compileToCanon(t, src)
	if got == "" || strings.Contains(got, "?") {
		t.Errorf("Vec2 composite literal failed: %s", got)
	}
}

// --- Additional coverage tests for statement/expression paths ---

func TestWhileLoopWithBreak(t *testing.T) {
	src := `package p
	func f() {
		x := 0
		for {
			if x > 5 {
				break
			}
			x = x + 1
		}
		set(0, 0, x)
	}`
	got := compileToCanon(t, src)
	if got == "" || strings.Contains(got, "?") {
		t.Errorf("while loop with break failed: %s", got)
	}
}

func TestIfElseNestedAssign(t *testing.T) {
	src := `package p
	func f() {
		x := get(0, 0)
		y := 0
		if x > 10 {
			y = 100
		} else if x > 5 {
			y = 50
		}
		set(0, 1, y)
	}`
	got := compileToCanon(t, src)
	if got == "" || strings.Contains(got, "?") {
		t.Errorf("nested if-else assign failed: %s", got)
	}
}

func TestSubAssign(t *testing.T) {
	src := `package p
	func f() {
		x := get(0, 0)
		x -= 3
		set(0, 1, x)
	}`
	got := compileToCanon(t, src)
	if got == "" || strings.Contains(got, "?") {
		t.Errorf("sub assign failed: %s", got)
	}
}

func TestDivAssign(t *testing.T) {
	src := `package p
	func f() {
		x := get(0, 0)
		if x == 0 {
			x = 1
		}
		x /= 2
		set(0, 1, x)
	}`
	got := compileToCanon(t, src)
	if got == "" || strings.Contains(got, "?") {
		t.Errorf("div assign failed: %s", got)
	}
}

func TestLogicalNotExpr(t *testing.T) {
	src := `package p
	func f() {
		a := get(0, 0)
		if !(a > 0) {
			set(0, 1, 1)
		}
	}`
	got := compileToCanon(t, src)
	if got == "" || strings.Contains(got, "?") {
		t.Errorf("logical not failed: %s", got)
	}
}

func TestRecordZeroValue(t *testing.T) {
	src := `package p
	func f() {
		v := vec2(0, 0)
		set(0, 0, v.x)
	}`
	got := compileToCanon(t, src)
	if got == "" || strings.Contains(got, "?") {
		t.Errorf("vec2 zero value failed: %s", got)
	}
}

func TestNestedRecordAccess(t *testing.T) {
	src := `package p
	func f() {
		r := rect(get(0, 0), get(0, 1), get(0, 2), get(0, 3))
		set(0, 4, r.l)
		set(0, 5, r.b)
	}`
	got := compileToCanon(t, src)
	if got == "" || strings.Contains(got, "?") {
		t.Errorf("nested record access failed: %s", got)
	}
}

func TestComplexArithExpr(t *testing.T) {
	src := `package p
	func f() {
		a := get(0, 0)
		b := get(0, 1)
		c := ((a + b) * (a - b)) / (a + 1)
		set(0, 2, c)
	}`
	got := compileToCanon(t, src)
	if got == "" || strings.Contains(got, "?") {
		t.Errorf("complex arith expr failed: %s", got)
	}
}

func TestMultipleReturns(t *testing.T) {
	src := `package p
	func f() {
		x := get(0, 0)
		if x < 0 {
			set(0, 1, -1)
			return
		}
		if x == 0 {
			set(0, 1, 0)
			return
		}
		set(0, 1, 1)
	}`
	got := compileToCanon(t, src)
	if got == "" || strings.Contains(got, "?") {
		t.Errorf("multiple returns failed: %s", got)
	}
}

// --- Runtime dispatch path tests (builtins_fn.go / trace_call.go coverage) ---

func TestRuntimeOffsetAdjustedTime(t *testing.T) {
	src := `package p
	func f() {
		t := offsetAdjustedTime()
		set(0, 0, t)
	}`
	got := compileToCanon(t, src)
	if got == "" || strings.Contains(got, "?") {
		t.Errorf("offsetAdjustedTime() failed: %s", got)
	}
}

func TestRuntimePrevTime(t *testing.T) {
	src := `package p
	func f() {
		t := prevTime()
		set(0, 0, t)
	}`
	got := compileToCanon(t, src)
	if got == "" || strings.Contains(got, "?") {
		t.Errorf("prevTime() failed: %s", got)
	}
}

func TestRuntimeTouchId(t *testing.T) {
	src := `package p
	func f() {
		v := touchId(0)
		set(0, 0, v)
	}`
	got := compileToCanon(t, src)
	if got == "" || strings.Contains(got, "?") {
		t.Errorf("touchId() failed: %s", got)
	}
}

func TestRuntimeTouchStarted(t *testing.T) {
	src := `package p
	func f() {
		v := touchStarted(0)
		set(0, 0, v)
	}`
	got := compileToCanon(t, src)
	if got == "" || strings.Contains(got, "?") {
		t.Errorf("touchStarted() failed: %s", got)
	}
}

func TestRuntimeTouchEnded(t *testing.T) {
	src := `package p
	func f() {
		v := touchEnded(0)
		set(0, 0, v)
	}`
	got := compileToCanon(t, src)
	if got == "" || strings.Contains(got, "?") {
		t.Errorf("touchEnded() failed: %s", got)
	}
}

func TestRuntimeTouchXY(t *testing.T) {
	src := `package p
	func f() {
		x := touchX(0)
		y := touchY(0)
		set(0, 0, x + y)
	}`
	got := compileToCanon(t, src)
	if got == "" || strings.Contains(got, "?") {
		t.Errorf("touchX/touchY failed: %s", got)
	}
}

func TestRuntimePnPoly(t *testing.T) {
	src := `package p
	func f() {
		v := vec2(get(0, 0), get(0, 1))
		q := quad(get(0, 2), get(0, 3), get(0, 4), get(0, 5),
			get(0, 6), get(0, 7), get(0, 8), get(0, 9))
		r := pnpoly(v, q)
		set(0, 10, r)
	}`
	got := compileToCanon(t, src)
	if got == "" || strings.Contains(got, "?") {
		t.Errorf("pnpoly() failed: %s", got)
	}
}

func TestRuntimePerspectiveApproach(t *testing.T) {
	src := `package p
	func f() {
		x := get(0, 0)
		y := get(0, 1)
		v := perspectiveApproach(x, y)
		set(0, 2, v)
	}`
	got := compileToCanon(t, src)
	if got == "" || strings.Contains(got, "?") {
		t.Errorf("perspectiveApproach() failed: %s", got)
	}
}

func TestRuntimeEasing(t *testing.T) {
	src := `package p
	func f() {
		t := get(0, 0)
		v := easeInCubic(t)
		set(0, 1, v)
	}`
	got := compileToCanon(t, src)
	if got == "" || strings.Contains(got, "?") {
		t.Errorf("easeInCubic failed: %s", got)
	}
}

func TestRuntimePlayScheduled(t *testing.T) {
	src := `package p
	func f() {
		playScheduled(0, 0, 0)
		stopLooped(0)
	}`
	got := compileToCanon(t, src)
	if got == "" || strings.Contains(got, "?") {
		t.Errorf("playScheduled/stopLooped failed: %s", got)
	}
}

func TestRuntimeTruncUnlerp(t *testing.T) {
	src := `package p
	func f() {
		a := trunc(3.7)
		b := unlerp(25, 0, 100)
		c := unlerpClamped(25, 0, 100)
		set(0, 0, a + b + c)
	}`
	got := compileToCanon(t, src)
	if got == "" || strings.Contains(got, "?") {
		t.Errorf("trunc/unlerp failed: %s", got)
	}
}

func TestRuntimeInterpAliases(t *testing.T) {
	src := `package p
	func f() {
		v := interp(0, 100, 0.5)
		w := interpClamped(0, 100, 0.5)
		set(0, 0, v + w)
	}`
	got := compileToCanon(t, src)
	if got == "" || strings.Contains(got, "?") {
		t.Errorf("interp/interpClamped failed: %s", got)
	}
}

// --- Record method tests (value_vec2.go / value_rect.go / value_quad.go coverage) ---

func TestVec2Magnitude(t *testing.T) {
	src := `package p
	func f() {
		v := vec2(3, 4)
		m := v.magnitude()
		set(0, 0, m)
	}`
	got := compileToCanon(t, src)
	if got == "" || strings.Contains(got, "?") {
		t.Errorf("Vec2.magnitude failed: %s", got)
	}
}

func TestVec2Dot(t *testing.T) {
	src := `package p
	func f() {
		a := vec2(1, 2)
		b := vec2(3, 4)
		d := a.dot(b)
		set(0, 0, d)
	}`
	got := compileToCanon(t, src)
	if got == "" || strings.Contains(got, "?") {
		t.Errorf("Vec2.dot failed: %s", got)
	}
}

func TestVec2Normalize(t *testing.T) {
	src := `package p
	func f() {
		v := vec2(3, 4)
		n := v.normalize()
		set(0, 0, n.x)
	}`
	got := compileToCanon(t, src)
	if got == "" || strings.Contains(got, "?") {
		t.Errorf("Vec2.normalize failed: %s", got)
	}
}

func TestVec2Angle(t *testing.T) {
	src := `package p
	func f() {
		v := vec2(1, 1)
		a := v.angle()
		set(0, 0, a)
	}`
	got := compileToCanon(t, src)
	if got == "" || strings.Contains(got, "?") {
		t.Errorf("Vec2.angle failed: %s", got)
	}
}

func TestVec2Rotate(t *testing.T) {
	src := `package p
	func f() {
		v := vec2(1, 0)
		r := v.rotate(3.14159)
		set(0, 0, r.x)
	}`
	got := compileToCanon(t, src)
	if got == "" || strings.Contains(got, "?") {
		t.Errorf("Vec2.rotate failed: %s", got)
	}
}

func TestVec2Orthogonal(t *testing.T) {
	src := `package p
	func f() {
		v := vec2(1, 2)
		o := v.orthogonal()
		set(0, 0, o.x)
	}`
	got := compileToCanon(t, src)
	if got == "" || strings.Contains(got, "?") {
		t.Errorf("Vec2.orthogonal failed: %s", got)
	}
}

func TestRectWAndH(t *testing.T) {
	src := `package p
	func f() {
		r := rect(10, 20, 30, 40)
		w := r.w()
		h := r.h()
		set(0, 0, w + h)
	}`
	got := compileToCanon(t, src)
	if got == "" || strings.Contains(got, "?") {
		t.Errorf("Rect.w/h failed: %s", got)
	}
}

func TestRectCenter(t *testing.T) {
	src := `package p
	func f() {
		r := rect(0, 10, 0, 10)
		c := r.center()
		set(0, 0, c.x)
	}`
	got := compileToCanon(t, src)
	if got == "" || strings.Contains(got, "?") {
		t.Errorf("Rect.center failed: %s", got)
	}
}

func TestVec2NormalizeOrZero(t *testing.T) {
	src := `package p
	func f() {
		v := vec2(3, 4)
		n := v.normalizeOrZero()
		set(0, 0, n.x)
	}`
	got := compileToCanon(t, src)
	if got == "" || strings.Contains(got, "?") {
		t.Errorf("Vec2.normalizeOrZero failed: %s", got)
	}
}

func TestVec2AngleDiff(t *testing.T) {
	src := `package p
	func f() {
		a := vec2(1, 0)
		b := vec2(0, 1)
		d := a.angleDiff(b)
		set(0, 0, d)
	}`
	got := compileToCanon(t, src)
	if got == "" || strings.Contains(got, "?") {
		t.Errorf("Vec2.angleDiff failed: %s", got)
	}
}

func TestVec2SignedAngleDiff(t *testing.T) {
	src := `package p
	func f() {
		a := vec2(1, 0)
		b := vec2(0, 1)
		d := a.signedAngleDiff(b)
		set(0, 0, d)
	}`
	got := compileToCanon(t, src)
	if got == "" || strings.Contains(got, "?") {
		t.Errorf("Vec2.signedAngleDiff failed: %s", got)
	}
}

func TestVec2RotateAbout(t *testing.T) {
	src := `package p
	func f() {
		v := vec2(1, 0)
		c := vec2(0, 0)
		r := v.rotateAbout(c, 1.57)
		set(0, 0, r.x)
	}`
	got := compileToCanon(t, src)
	if got == "" || strings.Contains(got, "?") {
		t.Errorf("Vec2.rotateAbout failed: %s", got)
	}
}

// --- Debug / judge / sort / stack runtime function tests ---

func TestDebugTerminate(t *testing.T) {
	src := `package p
	func f() {
		debugTerminate()
		set(0, 0, 1)
	}`
	got := compileToCanon(t, src)
	if got == "" || strings.Contains(got, "?") {
		t.Errorf("debugTerminate failed: %s", got)
	}
}

func TestDebugLog(t *testing.T) {
	src := `package p
	func f() {
		debugLog(1)
		set(0, 0, 1)
	}`
	got := compileToCanon(t, src)
	if got == "" || strings.Contains(got, "?") {
		t.Errorf("debugLog failed: %s", got)
	}
}

func TestDebugPause(t *testing.T) {
	src := `package p
	func f() {
		debugPause()
		set(0, 0, 1)
	}`
	got := compileToCanon(t, src)
	if got == "" || strings.Contains(got, "?") {
		t.Errorf("debugPause failed: %s", got)
	}
}

func TestDebugError(t *testing.T) {
	src := `package p
	func f() {
		debugError(1)
		set(0, 0, 1)
	}`
	got := compileToCanon(t, src)
	if got == "" || strings.Contains(got, "?") {
		t.Errorf("debugError failed: %s", got)
	}
}

func TestDebugRequireCall(t *testing.T) {
	src := `package p
	func f() {
		debugRequire(1, 1)
		set(0, 0, 1)
	}`
	got := compileToCanon(t, src)
	if got == "" || strings.Contains(got, "?") {
		t.Errorf("debugRequire failed: %s", got)
	}
}

func TestDebugAssertTrueCall(t *testing.T) {
	src := `package p
	func f() {
		debugAssertTrue(1, 1)
		set(0, 0, 1)
	}`
	got := compileToCanon(t, src)
	if got == "" || strings.Contains(got, "?") {
		t.Errorf("debugAssertTrue failed: %s", got)
	}
}

func TestDebugAssertFalseCall(t *testing.T) {
	src := `package p
	func f() {
		debugAssertFalse(1, 1)
		set(0, 0, 1)
	}`
	got := compileToCanon(t, src)
	if got == "" || strings.Contains(got, "?") {
		t.Errorf("debugAssertFalse failed: %s", got)
	}
}

func TestJudgeSimple(t *testing.T) {
	src := `package p
	func f() {
		judgeSimple(0, 0, 0)
		set(0, 0, 1)
	}`
	got := compileToCanon(t, src)
	if got == "" || strings.Contains(got, "?") {
		t.Errorf("judgeSimple failed: %s", got)
	}
}

func TestJudge(t *testing.T) {
	src := `package p
	func f() {
		judge(0, 0, 0)
		set(0, 0, 1)
	}`
	got := compileToCanon(t, src)
	if got == "" || strings.Contains(got, "?") {
		t.Errorf("judge failed: %s", got)
	}
}

func TestSortLinkedEntities(t *testing.T) {
	src := `package p
	func f() {
		sortLinkedEntities(0, 0, 0)
		set(0, 0, 1)
	}`
	got := compileToCanon(t, src)
	if got == "" || strings.Contains(got, "?") {
		t.Errorf("sortLinkedEntities failed: %s", got)
	}
}

func TestStackEnterLeave(t *testing.T) {
	src := `package p
	func f() {
		stackEnter(0, 0, 0)
		stackLeave()
		set(0, 0, 1)
	}`
	got := compileToCanon(t, src)
	if got == "" || strings.Contains(got, "?") {
		t.Errorf("stackEnter/stackLeave failed: %s", got)
	}
}

func TestEasingInOut(t *testing.T) {
	src := `package p
	func f() {
		t := get(0, 0)
		a := easeInOutCubic(t)
		b := easeInOutSine(t)
		set(0, 1, a + b)
	}`
	got := compileToCanon(t, src)
	if got == "" || strings.Contains(got, "?") {
		t.Errorf("easeInOutCubic/easeInOutSine failed: %s", got)
	}
}

func TestBeatToTime(t *testing.T) {
	src := `package p
	func f() {
		v := beatToTime(0)
		set(0, 0, v)
	}`
	got := compileToCanon(t, src)
	if got == "" || strings.Contains(got, "?") {
		t.Errorf("beatToTime failed: %s", got)
	}
}

func TestTimeToScaledTime(t *testing.T) {
	src := `package p
	func f() {
		v := timeToScaledTime(1)
		set(0, 0, v)
	}`
	got := compileToCanon(t, src)
	if got == "" || strings.Contains(got, "?") {
		t.Errorf("timeToScaledTime failed: %s", got)
	}
}

func TestPrint(t *testing.T) {
	src := `package p
	func f() {
		print(42)
		set(0, 0, 1)
	}`
	got := compileToCanon(t, src)
	if got == "" || strings.Contains(got, "?") {
		t.Errorf("print failed: %s", got)
	}
}

func TestHasSkinSprite(t *testing.T) {
	src := `package p
	func f() {
		b := hasSkinSprite(0)
		set(0, 0, b)
	}`
	got := compileToCanon(t, src)
	if got == "" || strings.Contains(got, "?") {
		t.Errorf("hasSkinSprite failed: %s", got)
	}
}

func TestHasEffectClip(t *testing.T) {
	src := `package p
	func f() {
		b := hasEffectClip(0)
		set(0, 0, b)
	}`
	got := compileToCanon(t, src)
	if got == "" || strings.Contains(got, "?") {
		t.Errorf("hasEffectClip failed: %s", got)
	}
}

func TestDrawCurved(t *testing.T) {
	src := `package p
	func f() {
		drawCurvedB(0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0)
	}`
	got := compileToCanon(t, src)
	if got == "" || strings.Contains(got, "?") {
		t.Errorf("drawCurvedB failed: %s", got)
	}
}

func TestPlayLoopedScheduled(t *testing.T) {
	src := `package p
	func f() {
		playLooped(0, 0)
		playLoopedScheduled(0, 0, 0)
		stopLoopedScheduled(0, 0)
	}`
	got := compileToCanon(t, src)
	if got == "" || strings.Contains(got, "?") {
		t.Errorf("playLooped/stopLoopedScheduled failed: %s", got)
	}
}

func TestSpawnParticle(t *testing.T) {
	src := `package p
	func f() {
		spawn(0, 0, 0, 0, 0, 0, 0, 0)
		spawnParticle(0, 0, 0, 0, 0)
		moveParticle(0, 0, 0, 0, 0)
		destroyParticle(0)
	}`
	got := compileToCanon(t, src)
	if got == "" || strings.Contains(got, "?") {
		t.Errorf("spawn/spawnParticle/moveParticle/destroyParticle failed: %s", got)
	}
}
