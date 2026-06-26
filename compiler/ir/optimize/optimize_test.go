package optimize

import (
	"encoding/json"
	"os"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/WindowsSov8forUs/sonolus-core-go/core/resource"

	"github.com/WindowsSov8forUs/sonolus-go/compiler/frontend"
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

// --- case builders, mirroring testdata/harness.py exactly ---

func set(b, i, v int) ir.Node { return ir.SetPlace(ir.Cell(b, i), ir.Const(v)) }
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
			if got := canon(ir.CFGToSNode(build())); got != want.Before {
				t.Fatalf("before mismatch (CFG diverged)\n got: %s\nwant: %s", got, want.Before)
			}
			if got := canon(ir.CFGToSNode(UnreachableCodeElimination{}.Run(build()))); got != want.AfterUCE {
				t.Errorf("afterUCE mismatch\n got: %s\nwant: %s", got, want.AfterUCE)
			}
			if got := canon(ir.CFGToSNode(CoalesceFlow{}.Run(build()))); got != want.AfterCoalesce {
				t.Errorf("afterCoalesce mismatch\n got: %s\nwant: %s", got, want.AfterCoalesce)
			}
		})
	}
}

// --- DCE: IR-level canon, matching testdata/harness.py ---

func ircanonVal(n ir.Node) string {
	switch t := n.(type) {
	case ir.Const:
		return strconv.Itoa(int(t))
	case ir.Instr:
		ps := make([]string, len(t.Args))
		for i, a := range t.Args {
			ps[i] = ircanonVal(a)
		}
		return string(t.Op) + "[" + strings.Join(ps, ",") + "]"
	case ir.Get:
		return "G(" + ircanonPlace(t.Place) + ")"
	case ir.BlockPlace:
		return ircanonPlace(t)
	default:
		return "?"
	}
}

func ircanonPlace(p ir.Place) string {
	bp := p.(ir.BlockPlace)
	if tb, ok := bp.Block.(*ir.TempBlock); ok {
		return "T(" + tb.Name + ")"
	}
	return "M(" + ircanonVal(bp.Block) + "," + ircanonVal(bp.Index) + ")"
}

func ircanonStmt(s ir.Node) string {
	if set, ok := s.(ir.Set); ok {
		return ircanonPlace(set.Place) + "=" + ircanonVal(set.Value)
	}
	return ircanonVal(s)
}

func ircanonBlock(b *ir.BasicBlock) string {
	ps := make([]string, len(b.Statements))
	for i, s := range b.Statements {
		ps[i] = ircanonStmt(s)
	}
	return strings.Join(ps, ";")
}

func tcell(t *ir.TempBlock) ir.BlockPlace { return ir.TempCell(t) }
func mget(b, i int) ir.Get                { return ir.GetPlace(ir.Cell(b, i)) }

func dceBuilders() map[string]func() *ir.BasicBlock {
	return map[string]func() *ir.BasicBlock{
		"dead_store": func() *ir.BasicBlock {
			a, b := ir.NewTemp("a"), ir.NewTemp("b")
			e := ir.NewBlock()
			e.Statements = []ir.Node{
				ir.SetPlace(tcell(a), ir.Const(5)),
				ir.SetPlace(tcell(b), mget(0, 0)),
				ir.SetPlace(ir.Cell(0, 1), ir.GetPlace(tcell(b))),
			}
			return e
		},
		"self_copy": func() *ir.BasicBlock {
			a := ir.NewTemp("a")
			e := ir.NewBlock()
			e.Statements = []ir.Node{
				ir.SetPlace(tcell(a), mget(0, 0)),
				ir.SetPlace(tcell(a), ir.GetPlace(tcell(a))),
				ir.SetPlace(ir.Cell(0, 1), ir.GetPlace(tcell(a))),
			}
			return e
		},
		"side_effect": func() *ir.BasicBlock {
			a := ir.NewTemp("a")
			e := ir.NewBlock()
			e.Statements = []ir.Node{
				ir.SetPlace(tcell(a), ir.ImpureInstr(resource.RuntimeFunctionDraw, ir.Const(1), ir.Const(2))),
			}
			return e
		},
		"transitive": func() *ir.BasicBlock {
			a, b := ir.NewTemp("a"), ir.NewTemp("b")
			e := ir.NewBlock()
			e.Statements = []ir.Node{
				ir.SetPlace(tcell(a), mget(0, 0)),
				ir.SetPlace(tcell(b), ir.PureInstr(resource.RuntimeFunctionAdd, ir.GetPlace(tcell(a)), ir.Const(1))),
				ir.SetPlace(ir.Cell(0, 1), ir.GetPlace(tcell(b))),
			}
			return e
		},
	}
}

func TestDCEGolden(t *testing.T) {
	data, err := os.ReadFile("testdata/optimize_golden.json")
	if err != nil {
		t.Fatal(err)
	}
	var gold struct {
		DCECases map[string]string `json:"dceCases"`
	}
	if err := json.Unmarshal(data, &gold); err != nil {
		t.Fatal(err)
	}

	for name, build := range dceBuilders() {
		want, ok := gold.DCECases[name]
		if !ok {
			t.Fatalf("no DCE golden for %q", name)
		}
		t.Run(name, func(t *testing.T) {
			got := ircanonBlock(DeadCodeElimination{}.Run(build()))
			if got != want {
				t.Errorf("\n got: %s\nwant: %s", got, want)
			}
		})
	}
}

// --- SSA: IR canon of the SSA form, matching harness.py ssaCases ---

func ssaPlaceStr(p ir.Place) string {
	if s, ok := p.(ir.SSAPlace); ok {
		return s.Name + "." + strconv.Itoa(s.Num)
	}
	bp := p.(ir.BlockPlace)
	if tb, ok := bp.Block.(*ir.TempBlock); ok {
		return "T(" + tb.Name + ")"
	}
	return "M(" + ssaNoHash(bp.Block) + "," + ssaNoHash(bp.Index) + ")"
}

// ssaNoHash renders place operands (no '#' on constants), mirroring vcanon.
func ssaNoHash(n ir.Node) string {
	switch t := n.(type) {
	case ir.Const:
		return strconv.Itoa(int(t))
	case ir.SSAPlace:
		return ssaPlaceStr(t)
	case ir.Get:
		return "G(" + ssaPlaceStr(t.Place) + ")"
	case ir.Instr:
		ps := make([]string, len(t.Args))
		for i, a := range t.Args {
			ps[i] = ssaNoHash(a)
		}
		return string(t.Op) + "[" + strings.Join(ps, ",") + "]"
	case ir.BlockPlace:
		return ssaPlaceStr(t)
	default:
		return "?"
	}
}

// ssaVal renders values ('#' on constants), mirroring vcanon2.
func ssaVal(n ir.Node) string {
	switch t := n.(type) {
	case ir.Const:
		return "#" + strconv.Itoa(int(t))
	case ir.SSAPlace:
		return ssaPlaceStr(t)
	case ir.Get:
		return "G(" + ssaPlaceStr(t.Place) + ")"
	case ir.Instr:
		ps := make([]string, len(t.Args))
		for i, a := range t.Args {
			ps[i] = ssaVal(a)
		}
		return string(t.Op) + "[" + strings.Join(ps, ",") + "]"
	case ir.BlockPlace:
		return ssaPlaceStr(t)
	default:
		return "?"
	}
}

func ssaStmt(s ir.Node) string {
	if set, ok := s.(ir.Set); ok {
		return ssaPlaceStr(set.Place) + "=" + ssaVal(set.Value)
	}
	return ssaVal(s)
}

func ssaCanon(entry *ir.BasicBlock) string {
	blocks := ir.ReversePostorder(entry)
	idx := map[*ir.BasicBlock]int{}
	for i, b := range blocks {
		idx[b] = i
	}
	var sb strings.Builder
	for _, b := range blocks {
		var parts []string
		for _, phi := range b.Phis {
			type pa struct {
				i int
				s string
			}
			var pas []pa
			for pred, arg := range phi.Args {
				pas = append(pas, pa{idx[pred], "P" + strconv.Itoa(idx[pred]) + ":" + ssaPlaceStr(arg)})
			}
			sort.Slice(pas, func(i, j int) bool { return pas[i].i < pas[j].i })
			ss := make([]string, len(pas))
			for i, p := range pas {
				ss[i] = p.s
			}
			parts = append(parts, ssaPlaceStr(phi.Target)+"=phi("+strings.Join(ss, ",")+")")
		}
		for _, s := range b.Statements {
			parts = append(parts, ssaStmt(s))
		}
		if c, ok := b.Test.(ir.Const); !ok || float64(c) != 0 {
			parts = append(parts, "?"+ssaVal(b.Test))
		}
		sb.WriteString("B" + strconv.Itoa(idx[b]) + "{" + strings.Join(parts, ";") + "}")
	}
	return sb.String()
}

func ssaBuilders() map[string]func() *ir.BasicBlock {
	return map[string]func() *ir.BasicBlock{
		"diamond": func() *ir.BasicBlock {
			x := ir.NewTemp("x")
			e, thenB, elseB, merge := ir.NewBlock(), ir.NewBlock(), ir.NewBlock(), ir.NewBlock()
			e.Test = mget(0, 0)
			thenB.Statements = []ir.Node{ir.SetPlace(ir.TempCell(x), ir.Const(1))}
			elseB.Statements = []ir.Node{ir.SetPlace(ir.TempCell(x), ir.Const(2))}
			merge.Statements = []ir.Node{ir.SetPlace(ir.Cell(0, 1), ir.GetPlace(ir.TempCell(x)))}
			e.ConnectTo(elseB, ir.Cond(0))
			e.ConnectTo(thenB, nil)
			thenB.ConnectTo(merge, nil)
			elseB.ConnectTo(merge, nil)
			return e
		},
		"loop": func() *ir.BasicBlock {
			i := ir.NewTemp("i")
			e, header, body, exit := ir.NewBlock(), ir.NewBlock(), ir.NewBlock(), ir.NewBlock()
			e.Statements = []ir.Node{ir.SetPlace(ir.TempCell(i), ir.Const(0))}
			header.Test = mget(0, 0)
			body.Statements = []ir.Node{ir.SetPlace(ir.TempCell(i),
				ir.PureInstr(resource.RuntimeFunctionAdd, ir.GetPlace(ir.TempCell(i)), ir.Const(1)))}
			exit.Statements = []ir.Node{ir.SetPlace(ir.Cell(0, 1), ir.GetPlace(ir.TempCell(i)))}
			e.ConnectTo(header, nil)
			header.ConnectTo(exit, ir.Cond(0))
			header.ConnectTo(body, nil)
			body.ConnectTo(header, nil)
			return e
		},
	}
}

func TestToSSAGolden(t *testing.T) {
	data, err := os.ReadFile("testdata/optimize_golden.json")
	if err != nil {
		t.Fatal(err)
	}
	var gold struct {
		SSACases map[string]string `json:"ssaCases"`
	}
	if err := json.Unmarshal(data, &gold); err != nil {
		t.Fatal(err)
	}
	for name, build := range ssaBuilders() {
		want, ok := gold.SSACases[name]
		if !ok {
			t.Fatalf("no SSA golden for %q", name)
		}
		t.Run(name, func(t *testing.T) {
			got := ssaCanon(ToSSA{}.Run(build()))
			if got != want {
				t.Errorf("\n got: %s\nwant: %s", got, want)
			}
		})
	}
}

// --- FromSSA: full-CFG IR canon, matching harness.py fromSSACases ---

func cfgCanon(entry *ir.BasicBlock) string {
	blocks := ir.ReversePostorder(entry)
	idx := map[*ir.BasicBlock]int{}
	for i, b := range blocks {
		idx[b] = i
	}
	var sb strings.Builder
	for _, b := range blocks {
		var body []string
		for _, s := range b.Statements {
			body = append(body, ircanonStmt(s))
		}
		if c, ok := b.Test.(ir.Const); !ok || float64(c) != 0 {
			body = append(body, "?"+ircanonVal(b.Test))
		}
		edges := append([]*ir.FlowEdge(nil), b.Outgoing...)
		sort.SliceStable(edges, func(i, j int) bool {
			ei, ej := edges[i], edges[j]
			if (ei.Cond == nil) != (ej.Cond == nil) {
				return ei.Cond != nil
			}
			if ei.Cond == nil {
				return false
			}
			return *ei.Cond < *ej.Cond
		})
		var es []string
		for _, e := range edges {
			lab := "->"
			if e.Cond != nil {
				lab = strconv.Itoa(int(*e.Cond)) + ":"
			}
			es = append(es, lab+"B"+strconv.Itoa(idx[e.Dst]))
		}
		sb.WriteString("B" + strconv.Itoa(idx[b]) + "{" + strings.Join(body, ";") + "}(" + strings.Join(es, ",") + ")")
	}
	return sb.String()
}

func TestFromSSAGolden(t *testing.T) {
	data, err := os.ReadFile("testdata/optimize_golden.json")
	if err != nil {
		t.Fatal(err)
	}
	var gold struct {
		FromSSACases map[string]string `json:"fromSSACases"`
	}
	if err := json.Unmarshal(data, &gold); err != nil {
		t.Fatal(err)
	}
	for name, build := range ssaBuilders() {
		want, ok := gold.FromSSACases[name]
		if !ok {
			t.Fatalf("no FromSSA golden for %q", name)
		}
		t.Run(name, func(t *testing.T) {
			entry := FromSSA{}.Run(ToSSA{}.Run(build()))
			if got := cfgCanon(entry); got != want {
				t.Errorf("\n got: %s\nwant: %s", got, want)
			}
		})
	}
}

// --- InlineVars: SSA-form canon, matching harness.py inlineCases ---

func inlineBuilders() map[string]func() *ir.BasicBlock {
	bs := map[string]func() *ir.BasicBlock{
		"const_chain": func() *ir.BasicBlock {
			x, y := ir.NewTemp("x"), ir.NewTemp("y")
			e := ir.NewBlock()
			e.Statements = []ir.Node{
				ir.SetPlace(ir.TempCell(x), ir.Const(5)),
				ir.SetPlace(ir.TempCell(y), ir.PureInstr(resource.RuntimeFunctionAdd, ir.GetPlace(ir.TempCell(x)), ir.Const(3))),
				ir.SetPlace(ir.Cell(0, 0), ir.GetPlace(ir.TempCell(y))),
			}
			return e
		},
		"shared_const": func() *ir.BasicBlock {
			x := ir.NewTemp("x")
			e := ir.NewBlock()
			e.Statements = []ir.Node{
				ir.SetPlace(ir.TempCell(x), ir.Const(5)),
				ir.SetPlace(ir.Cell(0, 0), ir.GetPlace(ir.TempCell(x))),
				ir.SetPlace(ir.Cell(0, 1), ir.GetPlace(ir.TempCell(x))),
			}
			return e
		},
	}
	bs["diamond"] = ssaBuilders()["diamond"]
	// A memory read from EngineRom (3000) inlines into a LevelMemory (2000) write
	// once the oracle knows updateParallel can't write EngineRom.
	bs["memory"] = func() *ir.BasicBlock {
		tb := ir.NewTemp("t")
		e := ir.NewBlock()
		e.Statements = []ir.Node{
			ir.SetPlace(ir.TempCell(tb), ir.GetPlace(ir.Cell(3000, 0))),
			ir.SetPlace(ir.Cell(2000, 1), ir.GetPlace(ir.TempCell(tb))),
		}
		return e
	}
	return bs
}

func TestInlineVarsGolden(t *testing.T) {
	data, err := os.ReadFile("testdata/optimize_golden.json")
	if err != nil {
		t.Fatal(err)
	}
	var gold struct {
		InlineCases map[string]string `json:"inlineCases"`
	}
	if err := json.Unmarshal(data, &gold); err != nil {
		t.Fatal(err)
	}
	// The memory case needs a real block oracle + callback; the rest are
	// metadata-independent.
	configs := map[string]InlineVars{
		"memory": {Callback: "updateParallel", Oracle: ir.Blocks(ir.ModePlay)},
	}
	for name, build := range inlineBuilders() {
		want, ok := gold.InlineCases[name]
		if !ok {
			t.Fatalf("no InlineVars golden for %q", name)
		}
		t.Run(name, func(t *testing.T) {
			got := ssaCanon(configs[name].Run(ToSSA{}.Run(build())))
			if got != want {
				t.Errorf("\n got: %s\nwant: %s", got, want)
			}
		})
	}
}

// --- SCCP: SSA-form canon, matching harness.py sccpCases ---

func sccpBuilders() map[string]func() *ir.BasicBlock {
	return map[string]func() *ir.BasicBlock{
		"const_fold": func() *ir.BasicBlock {
			x, y := ir.NewTemp("x"), ir.NewTemp("y")
			e := ir.NewBlock()
			e.Statements = []ir.Node{
				ir.SetPlace(ir.TempCell(x), ir.Const(5)),
				ir.SetPlace(ir.TempCell(y), ir.PureInstr(resource.RuntimeFunctionAdd, ir.GetPlace(ir.TempCell(x)), ir.Const(3))),
				ir.SetPlace(ir.Cell(0, 0), ir.GetPlace(ir.TempCell(y))),
			}
			return e
		},
		"phi_const": func() *ir.BasicBlock {
			x := ir.NewTemp("x")
			e, thenB, elseB, merge := ir.NewBlock(), ir.NewBlock(), ir.NewBlock(), ir.NewBlock()
			e.Test = mget(0, 0)
			thenB.Statements = []ir.Node{ir.SetPlace(ir.TempCell(x), ir.Const(7))}
			elseB.Statements = []ir.Node{ir.SetPlace(ir.TempCell(x), ir.Const(7))}
			merge.Statements = []ir.Node{ir.SetPlace(ir.Cell(0, 1), ir.GetPlace(ir.TempCell(x)))}
			e.ConnectTo(elseB, ir.Cond(0))
			e.ConnectTo(thenB, nil)
			thenB.ConnectTo(merge, nil)
			elseB.ConnectTo(merge, nil)
			return e
		},
	}
}

func TestSCCPGolden(t *testing.T) {
	data, err := os.ReadFile("testdata/optimize_golden.json")
	if err != nil {
		t.Fatal(err)
	}
	var gold struct {
		SCCPCases map[string]string `json:"sccpCases"`
	}
	if err := json.Unmarshal(data, &gold); err != nil {
		t.Fatal(err)
	}
	for name, build := range sccpBuilders() {
		want, ok := gold.SCCPCases[name]
		if !ok {
			t.Fatalf("no SCCP golden for %q", name)
		}
		t.Run(name, func(t *testing.T) {
			got := ssaCanon(SCCP{}.Run(ToSSA{}.Run(build())))
			if got != want {
				t.Errorf("\n got: %s\nwant: %s", got, want)
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
	set(0, 0, x)
}`
	entry, err := frontend.Compile(src, frontend.Env{Names: frontend.ModeAccessors(ir.ModePlay)})
	if err != nil {
		t.Fatal(err)
	}
	entry = RunPasses(entry,
		ToSSA{}, InlineVars{}, FromSSA{},
		CoalesceFlow{}, DeadCodeElimination{},
	)
	entry = ir.AllocateTempBlocks(entry, ir.DefaultTempMemoryBlock)
	got := canon(ir.CFGToSNode(entry))
	// x := 3 is inlined and the dead store removed: just Set(0,0,3).
	want := "Block(JumpLoop(Execute(Set(#0,#0,#3),#1),#0))"
	if got != want {
		t.Errorf("inlining did not collapse the local\n got: %s\nwant: %s", got, want)
	}
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
		set(0, 0, x)
	} else {
		set(0, 0, 999)
	}
}`
	entry, err := frontend.Compile(src, frontend.Env{Names: frontend.ModeAccessors(ir.ModePlay)})
	if err != nil {
		t.Fatal(err)
	}
	entry = Optimize(entry, ir.ModePlay, "updateParallel", ir.DefaultTempMemoryBlock)
	got := canon(snode.Optimize(ir.CFGToSNode(entry)))
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
		set(0, 0, 1)
	} else {
		set(0, 0, 2)
	}
}`
	entry, err := frontend.Compile(src, frontend.Env{Names: frontend.ModeAccessors(ir.ModePlay)})
	if err != nil {
		t.Fatal(err)
	}
	entry = RunPasses(entry,
		ToSSA{}, SCCP{}, FromSSA{},
		UnreachableCodeElimination{}, CoalesceFlow{}, DeadCodeElimination{},
	)
	entry = ir.AllocateTempBlocks(entry, ir.DefaultTempMemoryBlock)
	got := canon(ir.CFGToSNode(entry))
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
	set(0, 0, 1)
}`
	entry, err := frontend.Compile(src, frontend.Env{Names: frontend.ModeAccessors(ir.ModePlay)})
	if err != nil {
		t.Fatal(err)
	}
	entry = DeadCodeElimination{}.Run(entry)
	entry = ir.AllocateTempBlocks(entry, ir.DefaultTempMemoryBlock)

	got := canon(ir.CFGToSNode(entry))
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
		set(0, 1, 1)
	} else {
		set(0, 1, 2)
	}
}`
	entry, err := frontend.Compile(src, frontend.Env{Names: frontend.ModeAccessors(ir.ModePlay)})
	if err != nil {
		t.Fatal(err)
	}
	entry = RunPasses(entry, UnreachableCodeElimination{}, CoalesceFlow{}, DeadCodeElimination{})
	entry = ir.AllocateTempBlocks(entry, ir.DefaultTempMemoryBlock)

	var nodes []resource.EngineDataNode
	if _, err := snode.NewAppender(&nodes).Append(ir.CFGToSNode(entry)); err != nil {
		t.Fatal(err)
	}
	if len(nodes) == 0 {
		t.Fatal("no nodes after optimization")
	}
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
	set(0, 1, x)
}`
	entry, err := frontend.Compile(src, frontend.Env{Names: frontend.ModeAccessors(ir.ModePlay)})
	if err != nil {
		t.Fatal(err)
	}
	entry = RunPasses(entry,
		ToSSA{}, FromSSA{},
		CoalesceFlow{}, DeadCodeElimination{},
	)
	entry = ir.AllocateTempBlocks(entry, ir.DefaultTempMemoryBlock)

	var nodes []resource.EngineDataNode
	if _, err := snode.NewAppender(&nodes).Append(ir.CFGToSNode(entry)); err != nil {
		t.Fatalf("finalize after SSA round trip: %v", err)
	}
	if len(nodes) == 0 {
		t.Fatal("no nodes after SSA round trip")
	}
}
