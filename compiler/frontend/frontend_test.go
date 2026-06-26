package frontend

import (
	"strings"
	"testing"

	"github.com/WindowsSov8forUs/sonolus-core-go/core/resource"

	"github.com/WindowsSov8forUs/sonolus-go/compiler/ir"
	"github.com/WindowsSov8forUs/sonolus-go/compiler/snode"
)

func canon(n snode.SNode) string {
	switch t := n.(type) {
	case snode.Value:
		return "#" + snode.FormatNumber(float64(t))
	case snode.Func:
		ps := make([]string, len(t.Args))
		for i, a := range t.Args {
			ps[i] = canon(a)
		}
		return string(t.Func) + "(" + strings.Join(ps, ",") + ")"
	}
	return "?"
}

func playEnv() Env { return Env{Names: ModeAccessors(ir.ModePlay)} }

func compileToCanon(t *testing.T, src string) string {
	t.Helper()
	entry, err := Compile(src, playEnv())
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	// Resolve temp-backed locals to concrete cells before finalization.
	entry = ir.AllocateTempBlocks(entry, ir.DefaultTempMemoryBlock)
	return canon(ir.CFGToSNode(entry))
}

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
	if _, err := Compile(src, playEnv()); err != nil {
		t.Fatalf("compile: %v", err)
	}
}

func TestVec2Fields(t *testing.T) {
	// A Vec2 record local: 2 slots (x at 0, y at 1) in block 10000.
	src := `package p
func f() {
	v := vec2(3, 4)
	set(0, 0, v.x + v.y)
}`
	got := compileToCanon(t, src)
	want := "Block(JumpLoop(Execute(" +
		"Set(#10000,#0,#3),Set(#10000,#1,#4)," +
		"Set(#0,#0,Add(Get(#10000,#0),Get(#10000,#1))),#1),#0))"
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
		"Set(#10000,#0,#0),Set(#10000,#1,#0),Set(#10000,#0,#7)," +
		"Set(#0,#0,Get(#10000,#0)),#1),#0))"
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
	if _, err := Compile(src, playEnv()); err == nil {
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
	if _, err := Compile(`package p
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
		entry, err := Compile("package p\nfunc f() {\n"+body+"\n}\n", Env{Names: ModeAccessors(mode)})
		if err != nil {
			return "", err
		}
		entry = ir.AllocateTempBlocks(entry, ir.DefaultTempMemoryBlock)
		return canon(ir.CFGToSNode(entry)), nil
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
	if _, err := Compile(src, playEnv()); err != nil {
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
	entry, err := Compile(src, playEnv())
	if err != nil {
		t.Fatal(err)
	}
	var nodes []resource.EngineDataNode
	root, err := snode.NewAppender(&nodes).Append(ir.CFGToSNode(entry))
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

func TestUnsupportedReportsError(t *testing.T) {
	src := `package p
func f() {
	defer f()
}`
	if _, err := Compile(src, playEnv()); err == nil {
		t.Fatal("expected error for unsupported defer statement")
	}
}
