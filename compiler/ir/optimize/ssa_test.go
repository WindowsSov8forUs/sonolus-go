package optimize

import (
	"encoding/json"
	"os"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/WindowsSov8forUs/sonolus-core-go/core/resource"

	"github.com/WindowsSov8forUs/sonolus-go/compiler/ir"
)

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
			thenB.Statements = []ir.Node{testGen.SetPlace(ir.TempCell(x), ir.Const(1))}
			elseB.Statements = []ir.Node{testGen.SetPlace(ir.TempCell(x), ir.Const(2))}
			merge.Statements = []ir.Node{testGen.SetPlace(ir.Cell(0, 1), ir.GetPlace(ir.TempCell(x)))}
			e.ConnectTo(elseB, ir.Cond(0))
			e.ConnectTo(thenB, nil)
			thenB.ConnectTo(merge, nil)
			elseB.ConnectTo(merge, nil)
			return e
		},
		"loop": func() *ir.BasicBlock {
			i := ir.NewTemp("i")
			e, header, body, exit := ir.NewBlock(), ir.NewBlock(), ir.NewBlock(), ir.NewBlock()
			e.Statements = []ir.Node{testGen.SetPlace(ir.TempCell(i), ir.Const(0))}
			header.Test = mget(0, 0)
			body.Statements = []ir.Node{testGen.SetPlace(ir.TempCell(i),
				testGen.PureInstr(resource.RuntimeFunctionAdd, ir.GetPlace(ir.TempCell(i)), ir.Const(1)))}
			exit.Statements = []ir.Node{testGen.SetPlace(ir.Cell(0, 1), ir.GetPlace(ir.TempCell(i)))}
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
			got := ssaCanon(ToSSA{}.Run(testGen, build()))
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
			entry := FromSSA{}.Run(testGen, ToSSA{}.Run(testGen, build()))
			if got := cfgCanon(entry); got != want {
				t.Errorf("\n got: %s\nwant: %s", got, want)
			}
		})
	}
}
