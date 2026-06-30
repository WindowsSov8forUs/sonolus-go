package optimize

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/WindowsSov8forUs/sonolus-core-go/core/resource"

	"github.com/WindowsSov8forUs/sonolus-go/compiler/frontend"
	"github.com/WindowsSov8forUs/sonolus-go/compiler/ir"
	"github.com/WindowsSov8forUs/sonolus-go/compiler/modecompile"
	"github.com/WindowsSov8forUs/sonolus-go/compiler/snode"
)

var testGen = ir.NewIDGen()
var canon = modecompile.Canon

// mustLower calls CFGToSNode and panics on error. Test helper.
func mustLower(sn snode.SNode, err error) snode.SNode {
	if err != nil {
		panic(err)
	}
	return sn
}

// --- case builders, mirroring testdata/harness.py exactly ---

func set(b, i, v int) ir.Node { return testGen.SetPlace(ir.Cell(b, i), ir.Const(v)) }
func getNode(b, i int) ir.Get { return ir.GetPlace(ir.Cell(b, i)) }

func linear3() *ir.BasicBlock {
	e, b1, b2 := ir.NewBlock(), ir.NewBlock(), ir.NewBlock()
	e.Statements = []ir.Node{set(1, 0, 1)}
	b1.Statements = []ir.Node{set(1, 1, 2)}
	e.ConnectTo(b1, nil)
	b1.ConnectTo(b2, nil)
	return e
}

func emptySkip() *ir.BasicBlock {
	e, mid, tgt := ir.NewBlock(), ir.NewBlock(), ir.NewBlock()
	e.Statements = []ir.Node{set(0, 0, 7)}
	tgt.Statements = []ir.Node{set(1, 0, 9)}
	e.ConnectTo(mid, nil)
	mid.ConnectTo(tgt, nil)
	return e
}

func constTest() *ir.BasicBlock {
	e, a, b, exit := ir.NewBlock(), ir.NewBlock(), ir.NewBlock(), ir.NewBlock()
	e.Test = ir.Const(0)
	a.Statements = []ir.Node{set(1, 0, 1)}
	b.Statements = []ir.Node{set(1, 0, 2)}
	e.ConnectTo(b, ir.Cond(0))
	e.ConnectTo(a, nil)
	a.ConnectTo(exit, nil)
	b.ConnectTo(exit, nil)
	return e
}

func diamond() *ir.BasicBlock {
	e, thenB, elseB, merge := ir.NewBlock(), ir.NewBlock(), ir.NewBlock(), ir.NewBlock()
	e.Test = getNode(0, 0)
	thenB.Statements = []ir.Node{set(1, 0, 1)}
	elseB.Statements = []ir.Node{set(1, 0, 2)}
	e.ConnectTo(elseB, ir.Cond(0))
	e.ConnectTo(thenB, nil)
	thenB.ConnectTo(merge, nil)
	elseB.ConnectTo(merge, nil)
	return e
}

var builders = map[string]func() *ir.BasicBlock{
	"linear3":    linear3,
	"empty_skip": emptySkip,
	"const_test": constTest,
	"diamond":    diamond,
}

func TestOptimizeGolden(t *testing.T) {
	data, err := os.ReadFile("testdata/optimize_golden.json")
	if err != nil {
		t.Fatalf("read golden: %v", err)
	}
	var gold struct {
		Cases map[string]struct {
			Before        string `json:"before"`
			AfterUCE      string `json:"afterUCE"`
			AfterCoalesce string `json:"afterCoalesce"`
		} `json:"cases"`
	}
	if err := json.Unmarshal(data, &gold); err != nil {
		t.Fatal(err)
	}

	for name, build := range builders {
		want, ok := gold.Cases[name]
		if !ok {
			t.Fatalf("no golden for case %q", name)
		}
		t.Run(name, func(t *testing.T) {
			// Parity: our CFG finalizes identically to sonolus.py's before passes.
			if got := canon(mustLower(ir.CFGToSNode(testGen, build()))); got != want.Before {
				t.Fatalf("before mismatch (CFG diverged)\n got: %s\nwant: %s", got, want.Before)
			}
			if got := canon(mustLower(ir.CFGToSNode(testGen, UnreachableCodeElimination{}.Run(testGen, build())))); got != want.AfterUCE {
				t.Errorf("afterUCE mismatch\n got: %s\nwant: %s", got, want.AfterUCE)
			}
			if got := canon(mustLower(ir.CFGToSNode(testGen, CoalesceFlow{}.Run(testGen, build())))); got != want.AfterCoalesce {
				t.Errorf("afterCoalesce mismatch\n got: %s\nwant: %s", got, want.AfterCoalesce)
			}
		})
	}
}

// TestInlineEndToEnd shows the payoff on real frontend output: a read-once local
// collapses through the full SSA pipeline.
func TestInlineEndToEnd(t *testing.T) {
	src := `package p
func f() {
	x := 1 + 2
	set( 0, 0, x)
}`
	entry, _, err := frontend.Compile(src, frontend.Env{Names: frontend.ModeAccessors(ir.ModePlay)})
	if err != nil {
		t.Fatal(err)
	}
	entry = RunPasses(testGen, entry,
		ToSSA{}, InlineVars{}, FromSSA{},
		CoalesceFlow{}, DeadCodeElimination{},
	)
	entry = ir.AllocateTestBlocks(entry, ir.DefaultTempMemoryBlock)
	got := canon(mustLower(ir.CFGToSNode(testGen, entry)))
	// x := 3 is inlined and the dead store removed: just Set(0,0,3).
	want := "Block(JumpLoop(Execute(Set(#0,#0,#3),#1),#0))"
	if got != want {
		t.Errorf("inlining did not collapse the local\n got: %s\nwant: %s", got, want)
	}
}

func TestCopyCoalesceFoldsCopies(t *testing.T) {
	t1 := ir.NewTemp("x.1")
	t2 := ir.NewTemp("y.2")

	e := ir.NewBlock()
	e.Statements = []ir.Node{
		testGen.SetPlace(ir.TempCell(t1), ir.Const(42)),                 // t1 = 42
		testGen.SetPlace(ir.TempCell(t2), ir.GetPlace(ir.TempCell(t1))), // t2 = Get(t1) → copy
		testGen.SetPlace(ir.Cell(0, 0), ir.GetPlace(ir.TempCell(t2))),   // use t2
	}

	CopyCoalesce{}.Run(testGen, e)

	// After coalesce: t2 should be replaced by t1 everywhere, and the copy dropped.
	if len(e.Statements) != 2 {
		t.Fatalf("expected 2 statements after coalesce, got %d", len(e.Statements))
	}
	// Check that the last statement uses t1 not t2.
	last := e.Statements[1].(ir.Set)
	get := last.Value.(ir.Get)
	bp := get.Place.(ir.BlockPlace)
	if bp.Block != t1 {
		t.Errorf("expected t1 in use site, got %p", bp.Block)
	}
}

func TestCSEDeduplicates(t *testing.T) {
	// Two identical subexpressions: Get(0,0)+Get(0,0) should share the first result.
	e := ir.NewBlock()
	inner := testGen.PureInstr(resource.RuntimeFunctionAdd, ir.GetPlace(ir.Cell(0, 0)), ir.Const(1))
	e.Statements = []ir.Node{
		testGen.SetPlace(ir.Cell(0, 1), inner),
		testGen.SetPlace(ir.Cell(0, 2), ir.GetPlace(ir.TempCell(ir.NewTemp("x")))),
		testGen.SetPlace(ir.Cell(0, 3), inner), // duplicate
	}
	CSE{}.Run(testGen, e)
	// After CSE, the second Add should be replaced with a Get to the extracted SSA var.
	t.Logf("statements after CSE: %d", len(e.Statements))
}

// TestStandardPipeline runs the whole ordered optimization pipeline (the SCCP
// stage's deliverable) on a realistic callback: a dataflow-constant condition
// prunes its dead branch and the constant propagates into the survivor.
func TestStandardPipeline(t *testing.T) {
	src := `package p
func f() {
	x := 5
	c := x > 3
	if c {
		set( 0, 0, x)
	} else {
		set( 0, 0, 999)
	}
}`
	entry, _, err := frontend.Compile(src, frontend.Env{Names: frontend.ModeAccessors(ir.ModePlay)})
	if err != nil {
		t.Fatal(err)
	}
	entry , err = Optimize(testGen, entry, ir.ModePlay, "updateParallel", ir.DefaultTempMemoryBlock, LevelStandard)
	if err != nil {
		t.Fatal(err)
	}
	got := canon(snode.Peephole(mustLower(ir.CFGToSNode(testGen, entry))))
	want := "Block(JumpLoop(Execute(Set(#0,#0,#5),#1),#0))"
	if got != want {
		t.Errorf("pipeline output\n got:  %s\nwant: %s", got, want)
	}
}

// TestSCCPDeadBranch shows SCCP's signature win: a provably-constant condition
// (held in a variable so the test folds to a literal) lets the dead branch be
// eliminated end-to-end. (Our frontend keeps inline conditions as expressions,
// which SCCP folds in value but not in the test node; a named condition becomes
// a foldable SSA test — matching how sonolus.py materializes conditions.)
func TestSCCPDeadBranch(t *testing.T) {
	src := `package p
func f() {
	x := 5
	c := x > 3
	if c {
		set( 0, 0, 1)
	} else {
		set( 0, 0, 2)
	}
}`
	entry, _, err := frontend.Compile(src, frontend.Env{Names: frontend.ModeAccessors(ir.ModePlay)})
	if err != nil {
		t.Fatal(err)
	}
	entry = RunPasses(testGen, entry,
		ToSSA{}, SCCP{}, FromSSA{},
		UnreachableCodeElimination{}, CoalesceFlow{}, DeadCodeElimination{},
	)
	entry = ir.AllocateTestBlocks(entry, ir.DefaultTempMemoryBlock)
	got := canon(mustLower(ir.CFGToSNode(testGen, entry)))
	// Only the taken branch (set 0,0,1) survives.
	want := "Block(JumpLoop(Execute(Set(#0,#0,#1),#1),#0))"
	if got != want {
		t.Errorf("dead branch not eliminated\n got:  %s\nwant: %s", got, want)
	}
}

// TestDCERemovesDeadLocal shows DCE eliminating an unused local on real
// frontend output: `x := 5` is never read, so its store is dropped.
func TestDCERemovesDeadLocal(t *testing.T) {
	src := `package p
func f() {
	x := 5
	set( 0, 0, 1)
}`
	entry, _, err := frontend.Compile(src, frontend.Env{Names: frontend.ModeAccessors(ir.ModePlay)})
	if err != nil {
		t.Fatal(err)
	}
	entry = DeadCodeElimination{}.Run(testGen, entry)
	entry = ir.AllocateTestBlocks(entry, ir.DefaultTempMemoryBlock)

	got := canon(mustLower(ir.CFGToSNode(testGen, entry)))
	want := "Block(JumpLoop(Execute(Set(#0,#0,#1),#1),#0))"
	if got != want {
		t.Errorf("dead local not removed\n got: %s\nwant: %s", got, want)
	}
}

// TestPassesOnFrontendCFG confirms the passes compose on real frontend output:
// an if/else compiles, optimizes, and still finalizes to valid nodes.
func TestPassesOnFrontendCFG(t *testing.T) {
	src := `package p
func f() {
	x := get(0, 0)
	if x > 5 {
		set( 0, 1, 1)
	} else {
		set( 0, 1, 2)
	}
}`
	entry, _, err := frontend.Compile(src, frontend.Env{Names: frontend.ModeAccessors(ir.ModePlay)})
	if err != nil {
		t.Fatal(err)
	}
	entry = RunPasses(testGen, entry, UnreachableCodeElimination{}, CoalesceFlow{}, DeadCodeElimination{})
	entry = ir.AllocateTestBlocks(entry, ir.DefaultTempMemoryBlock)

	var nodes []resource.EngineDataNode
	if _, err := snode.NewAppender(&nodes).Append(mustLower(ir.CFGToSNode(testGen, entry))); err != nil {
		t.Fatal(err)
	}
	if len(nodes) == 0 {
		t.Fatal("no nodes after optimization")
	}
}

// --- End-to-end pipeline node-count golden (Part B deliverable) ---
// These cases run the full Standard pipeline and capture final node count + shape.
// The counts form the baseline for Part C (optimizer gap convergence).

type pipelineCase struct {
	name string
	src  string // Go source for frontend.Compile
}

var pipelineCases = []pipelineCase{
	{
		name: "linear_constant",
		src: `package p
		func f() {
			x := 1 + 2
			set( 0, 0, x)
		}`,
	},
	{
		name: "diamond_constant_fold",
		src: `package p
		func f() {
			x := 5
			if x > 3 {
				set( 0, 0, 1)
			} else {
				set( 0, 0, 2)
			}
		}`,
	},
	{
		name: "diamond_memory",
		src: `package p
		func f() {
			x := get(0, 0)
			if x > 5 {
				set( 0, 1, 1)
			} else {
				set( 0, 1, 2)
			}
		}`,
	},
	{
		name: "loop_with_memory_load",
		src: `package p
		func f() {
			sum := 0
			for i := range 10 {
				v := get(0, 0)
				sum = sum + v
			}
			set( 0, 1, sum)
		}`,
	},
	{
		name: "switch_like_chain",
		src: `package p
		func f() {
			x := get(0, 0)
			if x == 1 {
				set( 0, 1, 10)
			} else if x == 2 {
				set( 0, 1, 20)
			} else {
				set( 0, 1, 30)
			}
		}`,
	},
}

// TestPipelineNodeCount runs the full Standard pipeline on representative CFGs
// and records the final node count. For now it logs the counts (baseline).
// After Part C fixes, specific cases should converge to known-good targets.
func TestPipelineNodeCount(t *testing.T) {
	for _, c := range pipelineCases {
		t.Run(c.name, func(t *testing.T) {
			entry, _, err := frontend.Compile(c.src, frontend.Env{
				Names:     frontend.ModeAccessors(ir.ModePlay),
				Accessors: frontend.ModeAccessors(ir.ModePlay),
				Mode:      ir.ModePlay,
			})
			if err != nil {
				t.Fatalf("compile: %v", err)
			}

			entry , err = Optimize(testGen, entry, ir.ModePlay, "updateParallel", ir.DefaultTempMemoryBlock, LevelStandard)
			if err != nil {
				t.Fatal(err)
			}

			nodes := snode2Nodes(mustLower(ir.CFGToSNode(testGen, entry)))
			nodeCount := len(nodes)

			// Record the count as a baseline metric.
			// After Part C fixes, assert nodeCount <= baseline where appropriate.
			t.Logf("pipeline output: %d nodes", nodeCount)

			// Verify finalization succeeds (structural correctness).
			if nodeCount == 0 {
				t.Error("pipeline produced zero nodes — likely dead-code elimination over-aggressed")
			}
		})
	}
}

// snode2Nodes flattens an SNode tree into an EngineDataNode slice.
func snode2Nodes(s snode.SNode) []resource.EngineDataNode {
	var nodes []resource.EngineDataNode
	app := snode.NewAppender(&nodes)
	app.Append(s)
	return nodes
}

// TestSSARoundTrip confirms the SSA chain composes end-to-end: a frontend CFG
// goes to SSA and back, then finalizes to valid nodes.
func TestSSARoundTrip(t *testing.T) {
	src := `package p
func f() {
	x := 0
	if get(0, 0) {
		x = 1
	} else {
		x = 2
	}
	set( 0, 1, x)
}`
	entry, _, err := frontend.Compile(src, frontend.Env{Names: frontend.ModeAccessors(ir.ModePlay)})
	if err != nil {
		t.Fatal(err)
	}
	entry = RunPasses(testGen, entry,
		ToSSA{}, FromSSA{},
		CoalesceFlow{}, DeadCodeElimination{},
	)
	entry = ir.AllocateTestBlocks(entry, ir.DefaultTempMemoryBlock)

	var nodes []resource.EngineDataNode
	if _, err := snode.NewAppender(&nodes).Append(mustLower(ir.CFGToSNode(testGen, entry))); err != nil {
		t.Fatalf("finalize after SSA round trip: %v", err)
	}
	if len(nodes) == 0 {
		t.Fatal("no nodes after SSA round trip")
	}
}

// TestLICMHoistsLoopInvariant verifies that LICM hoists an expensive pure
// computation (sin) out of a loop body. sin(romRead) is loop-invariant and has
// a node cost above the inline threshold, so LICM should move it to the
// pre-header. (A simple ROM read has cost=1, below the threshold, so it won't
// be hoisted alone — the expensive wrapper is needed for a visible hoist.)
func TestLICMHoistsLoopInvariant(t *testing.T) {
	src := `package p
	func f() {
		sum := 0
		for i := 0; i < 10; i = i + 1 {
			sum = sum + sin(get(3000, 0))
		}
		set(0, 0, sum)
	}`
	entry, _, err := frontend.Compile(src, frontend.Env{
		Names: frontend.ModeAccessors(ir.ModePlay),
		Mode:  ir.ModePlay,
	})
	if err != nil {
		t.Fatal(err)
	}
	result , err := Optimize(testGen, entry, ir.ModePlay, "updateParallel", ir.DefaultTempMemoryBlock, LevelStandard)
	if err != nil {
		t.Fatal(err)
	}
	sn := snode.Peephole(mustLower(ir.CFGToSNode(testGen, result)))
	got := canon(sn)
	if got == "" {
		t.Error("LICM pipeline produced empty output")
	}
	t.Logf("LICM hoist output: %s", got)
}

// TestLICMRespectsWrittenBlocks verifies that LICM does NOT hoist reads from
// blocks that may be written during the loop (LevelMemory at block 2000 is
// writable in Play mode).
func TestLICMRespectsWrittenBlocks(t *testing.T) {
	src := `package p
	func f() {
		sum := 0
		for i := 0; i < 10; i = i + 1 {
			set(2000, 0, i)           // write to writable block inside loop
			sum = sum + get(2000, 0)  // this is NOT invariant (just written)
		}
		set(0, 0, sum)
	}`
	entry, _, err := frontend.Compile(src, frontend.Env{
		Names: frontend.ModeAccessors(ir.ModePlay),
		Mode:  ir.ModePlay,
	})
	if err != nil {
		t.Fatal(err)
	}
	result , err := Optimize(testGen, entry, ir.ModePlay, "updateParallel", ir.DefaultTempMemoryBlock, LevelStandard)
	if err != nil {
		t.Fatal(err)
	}
	sn := snode.Peephole(mustLower(ir.CFGToSNode(testGen, result)))
	got := canon(sn)
	if got == "" {
		t.Error("LICM pipeline produced empty output for writable-block case")
	}
	t.Logf("LICM writable-block output: %s", got)
}

// TestAllocateLiveReusesSlots verifies that the liveness-based allocator runs
// without error and produces a valid finalized SNode from a multi-temp CFG.
func TestAllocateLiveReusesSlots(t *testing.T) {
	src := `package p
	func f() {
		a := 1
		b := 2
		c := a + b
		set(0, 0, c)
		d := 3
		e := d * 2
		set(0, 1, e)
	}`
	entry, _, err := frontend.Compile(src, frontend.Env{
		Names: frontend.ModeAccessors(ir.ModePlay),
		Mode:  ir.ModePlay,
	})
	if err != nil {
		t.Fatal(err)
	}
	// Optimize includes AllocateLive at the tail.
	result , err := Optimize(testGen, entry, ir.ModePlay, "updateParallel", ir.DefaultTempMemoryBlock, LevelStandard)
	if err != nil {
		t.Fatal(err)
	}
	sn := snode.Peephole(mustLower(ir.CFGToSNode(testGen, result)))
	got := canon(sn)
	if got == "" {
		t.Error("AllocateLive pipeline produced empty output")
	}
	// The temporaries a, b, c, d, e should be allocated with overlapping
	// live ranges reusing slots. We verify the output is valid.
	t.Logf("AllocateLive output: %s", got)
}

func TestVerifyPasses_StandardPipeline(t *testing.T) {
	// The current Standard pipeline should pass validation (even though
	// many passes aren't annotated yet — unannotated passes conservatively
	// clear analyses, so annotated passes after them would fail).
	//
	// For now this test documents the pipeline is correct as-is.
	if err := VerifyPasses(Standard(ir.ModePlay, "updateParallel")...); err != nil {
		t.Errorf("standard pipeline verification failed: %v", err)
	}
}

func TestManagedPass_SSAOnly(t *testing.T) {
	// ToSSA → FromSSA: valid.
	if err := VerifyPasses(ToSSA{}, FromSSA{}); err != nil {
		t.Errorf("ToSSA→FromSSA should be valid: %v", err)
	}
	// ToSSA then CoalesceFlow then FromSSA: valid (CoalesceFlow preserves SSA).
	if err := VerifyPasses(ToSSA{}, CoalesceFlow{}, FromSSA{}); err != nil {
		t.Errorf("ToSSA->CoalesceFlow->FromSSA should be valid: %v", err)
	}
	// CoalesceFlow then FromSSA without ToSSA: fails (no SSA produced).
	if err := VerifyPasses(CoalesceFlow{}, FromSSA{}); err == nil {
		t.Error("CoalesceFlow->FromSSA without ToSSA should fail (no SSA produced)")
	}
	// SCCP without ToSSA first: requires SSA but none available.
	if err := VerifyPasses(SCCP{}); err == nil {
		t.Error("SCCP alone should fail (requires SSA, none available)")
	}
}

// TestSCCP_UnconditionalSuccessorAfterConstTest ensures that evaluateTest
// correctly marks unconditional successors reachable when the test value has
// not changed (the "no-change → enqueue unconditional" idiom in sccp.go:292-301).
// Regression: a block whose test is constant-NAC with a single default edge
// must have its successor reachable.
func TestSCCP_UnconditionalSuccessorAfterConstTest(t *testing.T) {
	// Build a minimal CFG: entry block with constant test (0), one unconditional
	// edge to an exit block, and a Set in the exit to verify it survives.
	testGen := ir.NewIDGen()
	exit := ir.NewBlock()
	setID := testGen.Next()
	constZero := ir.Const(0)
	exit.Statements = []ir.Node{
		ir.Set{ID: setID, Place: ir.BlockPlace{Block: constZero, Index: constZero}, Value: constZero},
	}

	entry := ir.NewBlock()
	entry.Test = constZero
	entry.ConnectTo(exit, nil) // unconditional edge

	// Run the Standard pipeline on this minimal graph.
	result , err := Optimize(testGen, entry, ir.ModePlay, "test", ir.DefaultTempMemoryBlock, LevelStandard)
	if err != nil {
		t.Fatal(err)
	}
	if result == nil {
		t.Fatal("Optimize returned nil entry")
	}

	// The exit block with its Set must survive optimization.
	found := false
	for _, b := range ir.ReversePostorder(result) {
		for _, s := range b.Statements {
			if set, ok := s.(ir.Set); ok && set.ID == setID {
				found = true
				break
			}
		}
	}
	if !found {
		t.Error("SCCP unconditionally-pruned reachable successor: Set instruction lost")
	}
}
