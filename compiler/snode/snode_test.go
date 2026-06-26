package snode

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"strings"
	"testing"

	"github.com/WindowsSov8forUs/sonolus-core-go/core/resource"
)

// The golden fixture is produced by .goldtmp/harness.ts running the real
// sonolus.js-compiler optimizer + assembler. See compiler/snode/testdata.

type goldFile struct {
	NumberFormats []struct {
		Value float64 `json:"value"`
		Str   string  `json:"str"`
	} `json:"numberFormats"`
	Cases []struct {
		Name      string            `json:"name"`
		Input     json.RawMessage   `json:"input"`
		Optimized json.RawMessage   `json:"optimized"`
		Nodes     []json.RawMessage `json:"nodes"`
		Root      int               `json:"root"`
	} `json:"cases"`
}

func loadGold(t *testing.T) goldFile {
	t.Helper()
	data, err := os.ReadFile("testdata/snode_golden.json")
	if err != nil {
		t.Fatalf("read golden: %v", err)
	}
	var g goldFile
	if err := json.Unmarshal(data, &g); err != nil {
		t.Fatalf("parse golden: %v", err)
	}
	return g
}

// decodeSNode turns the golden JSON form (number | {func,args}) into an SNode.
func decodeSNode(raw json.RawMessage) SNode {
	var v any
	if err := json.Unmarshal(raw, &v); err != nil {
		panic(err)
	}
	return toSNode(v)
}

func toSNode(v any) SNode {
	switch t := v.(type) {
	case float64:
		return Value(t)
	case map[string]any:
		fn := t["func"].(string)
		argsRaw, _ := t["args"].([]any)
		args := make([]SNode, len(argsRaw))
		for i, a := range argsRaw {
			args[i] = toSNode(a)
		}
		return Func{Func: resource.RuntimeFunction(fn), Args: args}
	default:
		panic(fmt.Sprintf("bad snode json: %T", v))
	}
}

// canonSNode renders an SNode to a stable string using JS-faithful number
// formatting, so structurally identical trees compare equal regardless of
// language.
func canonSNode(n SNode) string {
	switch t := n.(type) {
	case Value:
		return "#" + FormatNumber(float64(t))
	case Func:
		parts := make([]string, len(t.Args))
		for i, a := range t.Args {
			parts[i] = canonSNode(a)
		}
		return string(t.Func) + "(" + strings.Join(parts, ",") + ")"
	default:
		return "?"
	}
}

func canonGoldNode(raw json.RawMessage) string {
	var m map[string]json.RawMessage
	if err := json.Unmarshal(raw, &m); err != nil {
		panic(err)
	}
	if v, ok := m["value"]; ok {
		var f float64
		_ = json.Unmarshal(v, &f)
		return "V" + FormatNumber(f)
	}
	var fn string
	_ = json.Unmarshal(m["func"], &fn)
	var args []int
	_ = json.Unmarshal(m["args"], &args)
	ss := make([]string, len(args))
	for i, a := range args {
		ss[i] = fmt.Sprintf("%d", a)
	}
	return "F" + fn + ":" + strings.Join(ss, ",")
}

func canonGoNode(n resource.EngineDataNode) string {
	switch t := n.(type) {
	case resource.EngineDataValueNode:
		return "V" + FormatNumber(t.Value)
	case resource.EngineDataFunctionNode:
		ss := make([]string, len(t.Args))
		for i, a := range t.Args {
			ss[i] = fmt.Sprintf("%d", a)
		}
		return "F" + string(t.Func) + ":" + strings.Join(ss, ",")
	default:
		return "?"
	}
}

func TestFormatNumber(t *testing.T) {
	g := loadGold(t)
	for _, nf := range g.NumberFormats {
		if got := FormatNumber(nf.Value); got != nf.Str {
			t.Errorf("FormatNumber(%v) = %q, want %q", nf.Value, got, nf.Str)
		}
	}
	// Negative zero cannot survive JSON; verify directly against JS semantics.
	if got := FormatNumber(math.Copysign(0, -1)); got != "0" {
		t.Errorf("FormatNumber(-0) = %q, want %q", got, "0")
	}
}

func TestOptimizeGolden(t *testing.T) {
	g := loadGold(t)
	for _, c := range g.Cases {
		t.Run(c.Name, func(t *testing.T) {
			got := Optimize(decodeSNode(c.Input))
			want := decodeSNode(c.Optimized)
			if canonSNode(got) != canonSNode(want) {
				t.Errorf("Optimize mismatch\n got: %s\nwant: %s", canonSNode(got), canonSNode(want))
			}
		})
	}
}

func TestAppendGolden(t *testing.T) {
	g := loadGold(t)
	for _, c := range g.Cases {
		t.Run(c.Name, func(t *testing.T) {
			var nodes []resource.EngineDataNode
			a := NewAppender(&nodes)
			root, err := a.Append(Optimize(decodeSNode(c.Input)))
			if err != nil {
				t.Fatalf("append: %v", err)
			}
			if root != c.Root {
				t.Errorf("root index = %d, want %d", root, c.Root)
			}
			if len(nodes) != len(c.Nodes) {
				t.Fatalf("node count = %d, want %d", len(nodes), len(c.Nodes))
			}
			for i := range nodes {
				if got, want := canonGoNode(nodes[i]), canonGoldNode(c.Nodes[i]); got != want {
					t.Errorf("node[%d] = %s, want %s", i, got, want)
				}
			}
		})
	}
}
