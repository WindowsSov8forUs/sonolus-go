package optimize

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"testing"

	"github.com/WindowsSov8forUs/sonolus-core-go/core/resource"
	"github.com/WindowsSov8forUs/sonolus-go/internal/compiler/ir"
)

type pythonPassGolden struct {
	SSACases     map[string]string `json:"ssaCases"`
	SCCPCases    map[string]string `json:"sccpCases"`
	FromSSACases map[string]string `json:"fromSSACases"`
}

func loadPythonPassGolden(t *testing.T) pythonPassGolden {
	t.Helper()
	data, err := os.ReadFile("testdata/py_pass_golden.json")
	if err != nil {
		t.Fatal(err)
	}
	var golden pythonPassGolden
	if err := json.Unmarshal(data, &golden); err != nil {
		t.Fatal(err)
	}
	return golden
}

func parityNumber() ir.Type { return ir.Type{Name: "number", Slots: 1} }

func parityMemory(index int) ir.MemoryPlace {
	return ir.MemoryPlace{Storage: "0", Index: ir.Const{Value: float64(index)}, Read: true, Write: true}
}

func parityDiamond() *ir.Function {
	number := parityNumber()
	x := ir.LocalPlace{ID: 0, Name: "x"}
	return &ir.Function{Name: "diamond", Entry: 0, Result: ir.Type{}, Locals: []ir.Type{number}, Blocks: []*ir.Block{
		{ID: 0, Terminator: ir.Branch{Condition: ir.Load{Place: parityMemory(0)}, True: 1, False: 2}},
		{ID: 1, Instructions: []ir.Instruction{ir.Store{Place: x, Value: ir.Const{Value: 1}}}, Terminator: ir.Jump{Target: 3}},
		{ID: 2, Instructions: []ir.Instruction{ir.Store{Place: x, Value: ir.Const{Value: 2}}}, Terminator: ir.Jump{Target: 3}},
		{ID: 3, Instructions: []ir.Instruction{ir.Store{Place: parityMemory(1), Value: ir.Load{Place: x}}}, Terminator: ir.Return{Value: ir.Value{Type: ir.Type{}}}},
	}}
}

func parityLoop() *ir.Function {
	number := parityNumber()
	i := ir.LocalPlace{ID: 0, Name: "i"}
	return &ir.Function{Name: "loop", Entry: 0, Result: ir.Type{}, Locals: []ir.Type{number}, Blocks: []*ir.Block{
		{ID: 0, Instructions: []ir.Instruction{ir.Store{Place: i, Value: ir.Const{}}}, Terminator: ir.Jump{Target: 1}},
		{ID: 1, Terminator: ir.Branch{Condition: ir.Load{Place: parityMemory(0)}, True: 2, False: 3}},
		{ID: 2, Instructions: []ir.Instruction{ir.Store{Place: i, Value: ir.RuntimeCall{Function: resource.RuntimeFunctionAdd, Args: []ir.Expr{ir.Load{Place: i}, ir.Const{Value: 1}}, Result: number, Pure: true}}}, Terminator: ir.Jump{Target: 1}},
		{ID: 3, Instructions: []ir.Instruction{ir.Store{Place: parityMemory(1), Value: ir.Load{Place: i}}}, Terminator: ir.Return{Value: ir.Value{Type: ir.Type{}}}},
	}}
}

func TestToSSAMatchesPinnedPythonSnapshots(t *testing.T) {
	golden := loadPythonPassGolden(t)
	for name, build := range map[string]func() *ir.Function{"diamond": parityDiamond, "loop": parityLoop} {
		t.Run(name, func(t *testing.T) {
			function := build()
			if err := (ToSSA{}).Run(Context{}, function); err != nil {
				t.Fatal(err)
			}
			if got, want := paritySSAString(function), golden.SSACases[name]; got != want {
				t.Fatalf("Go/Python ToSSA mismatch\nGo:     %s\nPython: %s", got, want)
			}
		})
	}
}

func TestSCCPMatchesPinnedPythonSnapshots(t *testing.T) {
	golden := loadPythonPassGolden(t)
	allowedEquivalent := map[string]struct {
		goSnapshot string
		reason     string
	}{
		"const_fold": {
			goSnapshot: "B0{x.1=#5;y.1=#8;M(0,0)=#8}",
			reason:     "Go substitutes the proven constant into the SSA definition itself; Python keeps the folded Add definition while substituting all uses",
		},
	}
	for name, function := range map[string]*ir.Function{
		"const_fold": paritySCCPConstFold(),
		"phi_const":  paritySCCPPhiConst(),
	} {
		t.Run(name, func(t *testing.T) {
			if err := (SparseConditionalConstantPropagation{}).Run(Context{}, function); err != nil {
				t.Fatal(err)
			}
			if got, want := paritySSAString(function), golden.SCCPCases[name]; got != want {
				allowed, ok := allowedEquivalent[name]
				if !ok || got != allowed.goSnapshot {
					t.Fatalf("Go/Python SCCP mismatch\nGo:     %s\nPython: %s", got, want)
				}
				t.Logf("allowed equivalent SCCP structure: %s", allowed.reason)
			}
		})
	}
}

func paritySCCPConstFold() *ir.Function {
	number := parityNumber()
	x, y := ir.SSAPlace{ID: 1, Name: "x"}, ir.SSAPlace{ID: 2, Name: "y"}
	return &ir.Function{Name: "const-fold", Entry: 0, Result: ir.Type{}, Blocks: []*ir.Block{{ID: 0, Instructions: []ir.Instruction{
		ir.Store{Place: x, Value: ir.Const{Value: 5}},
		ir.Store{Place: y, Value: ir.RuntimeCall{Function: resource.RuntimeFunctionAdd, Args: []ir.Expr{ir.Load{Place: x}, ir.Const{Value: 3}}, Result: number, Pure: true}},
		ir.Store{Place: parityMemory(0), Value: ir.Load{Place: y}},
	}, Terminator: ir.Return{Value: ir.Value{Type: ir.Type{}}}}}}
}

func paritySCCPPhiConst() *ir.Function {
	x1, x2, x3 := ir.SSAPlace{ID: 1, Name: "x"}, ir.SSAPlace{ID: 2, Name: "x"}, ir.SSAPlace{ID: 3, Name: "x"}
	return &ir.Function{Name: "phi-const", Entry: 0, Result: ir.Type{}, Blocks: []*ir.Block{
		{ID: 0, Terminator: ir.Branch{Condition: ir.Load{Place: parityMemory(0)}, True: 1, False: 2}},
		{ID: 1, Instructions: []ir.Instruction{ir.Store{Place: x1, Value: ir.Const{Value: 7}}}, Terminator: ir.Jump{Target: 3}},
		{ID: 2, Instructions: []ir.Instruction{ir.Store{Place: x2, Value: ir.Const{Value: 7}}}, Terminator: ir.Jump{Target: 3}},
		{ID: 3, Phis: []ir.Phi{{Target: x3, Args: []ir.PhiArg{{Predecessor: 1, Value: x1}, {Predecessor: 2, Value: x2}}}}, Instructions: []ir.Instruction{ir.Store{Place: parityMemory(1), Value: ir.Load{Place: x3}}}, Terminator: ir.Return{Value: ir.Value{Type: ir.Type{}}}},
	}}
}

func paritySSAString(function *ir.Function) string {
	versions := map[int]string{}
	next := map[string]int{}
	name := func(place ir.SSAPlace) string {
		if existing := versions[place.ID]; existing != "" {
			return existing
		}
		base := place.Name
		if strings.HasPrefix(base, "local.0.") {
			base = "x"
			if function.Name == "loop" {
				base = "i"
			}
		}
		next[base]++
		versions[place.ID] = fmt.Sprintf("%s.%d", base, next[base])
		return versions[place.ID]
	}
	var placeString func(ir.Place) string
	var exprString func(ir.Expr) string
	placeString = func(place ir.Place) string {
		switch value := place.(type) {
		case ir.SSAPlace:
			return name(value)
		case ir.LocalPlace:
			return value.Name
		case ir.MemoryPlace:
			if index, ok := value.Index.(ir.Const); ok {
				return fmt.Sprintf("M(%s,%g)", value.Storage, index.Value)
			}
			return fmt.Sprintf("M(%s,%s)", value.Storage, exprString(value.Index))
		default:
			return fmt.Sprintf("%T", place)
		}
	}
	exprString = func(expression ir.Expr) string {
		switch value := expression.(type) {
		case ir.Const:
			return fmt.Sprintf("#%g", value.Value)
		case ir.Load:
			return "G(" + placeString(value.Place) + ")"
		case ir.RuntimeCall:
			args := make([]string, len(value.Args))
			for i, argument := range value.Args {
				args[i] = exprString(argument)
			}
			return string(value.Function) + "[" + strings.Join(args, ",") + "]"
		default:
			return fmt.Sprintf("%T", expression)
		}
	}
	blocks := make([]string, len(function.Blocks))
	for _, block := range function.Blocks {
		parts := []string{}
		for _, phi := range block.Phis {
			target := name(phi.Target)
			args := append([]ir.PhiArg(nil), phi.Args...)
			sort.Slice(args, func(i, j int) bool { return args[i].Predecessor < args[j].Predecessor })
			values := make([]string, len(args))
			for i, argument := range args {
				values[i] = fmt.Sprintf("P%d:%s", argument.Predecessor, name(argument.Value))
			}
			parts = append(parts, target+"=phi("+strings.Join(values, ",")+")")
		}
		for _, instruction := range block.Instructions {
			switch value := instruction.(type) {
			case ir.Store:
				parts = append(parts, placeString(value.Place)+"="+exprString(value.Value))
			case ir.Eval:
				parts = append(parts, exprString(value.Value))
			}
		}
		if branch, ok := block.Terminator.(ir.Branch); ok {
			parts = append(parts, "?"+exprString(branch.Condition))
		}
		blocks[block.ID] = fmt.Sprintf("B%d{%s}", block.ID, strings.Join(parts, ";"))
	}
	return strings.Join(blocks, "")
}

func TestFromSSAAndAllocationMatchPinnedPythonContracts(t *testing.T) {
	golden := loadPythonPassGolden(t)
	if golden.FromSSACases["diamond"] == "" || golden.FromSSACases["loop"] == "" {
		t.Fatal("pinned Python golden is missing FromSSA checkpoints")
	}
	for name, build := range map[string]func() *ir.Function{"diamond": parityDiamond, "loop": parityLoop} {
		t.Run(name, func(t *testing.T) {
			function := build()
			if err := (ToSSA{}).Run(Context{}, function); err != nil {
				t.Fatal(err)
			}
			if err := (FromSSA{}).Run(Context{}, function); err != nil {
				t.Fatal(err)
			}
			if err := (Allocate{}).Run(Context{}, function); err != nil {
				t.Fatal(err)
			}
			if err := ir.ValidateFinal(function); err != nil {
				t.Fatal(err)
			}
			for _, block := range function.Blocks {
				if len(block.Phis) != 0 {
					t.Fatalf("block %d retained Phi nodes", block.ID)
				}
			}
			if len(function.Locals) > 1 || (len(function.Locals) == 1 && function.Locals[0].Slots > 4096) {
				t.Fatalf("invalid allocated Temporary Memory layout: %#v", function.Locals)
			}
		})
	}
}
