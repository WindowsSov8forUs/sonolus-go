package backend

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/WindowsSov8forUs/sonolus-core-go/core/resource"
)

type jsSNodeGolden struct {
	NumberFormats []struct {
		Value float64 `json:"value"`
		Text  string  `json:"str"`
	} `json:"numberFormats"`
	Cases []struct {
		Name      string          `json:"name"`
		Input     json.RawMessage `json:"input"`
		Optimized json.RawMessage `json:"optimized"`
		Nodes     json.RawMessage `json:"nodes"`
		Root      int             `json:"root"`
	} `json:"cases"`
}

func TestSNodeMatchesPinnedJavaScriptGolden(t *testing.T) {
	path := filepath.Join("..", "testdata", "backend", "snode_golden.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var golden jsSNodeGolden
	if err := json.Unmarshal(data, &golden); err != nil {
		t.Fatal(err)
	}
	for _, format := range golden.NumberFormats {
		if got := jsNumberString(format.Value); got != format.Text {
			t.Errorf("format %v = %q, want %q", format.Value, got, format.Text)
		}
	}
	for _, test := range golden.Cases {
		t.Run(test.Name, func(t *testing.T) {
			input, err := decodeGoldenSNode(test.Input)
			if err != nil {
				t.Fatal(err)
			}
			optimized := simplify(input)
			actualTree, err := json.Marshal(encodeGoldenSNode(optimized))
			if err != nil {
				t.Fatal(err)
			}
			if !jsonEqual(actualTree, test.Optimized) {
				t.Fatalf("optimized tree\nactual: %s\nwant:   %s", actualTree, test.Optimized)
			}
			appender := newNodeAppender()
			root, err := appender.append(optimized)
			if err != nil {
				t.Fatal(err)
			}
			actualNodes, err := json.Marshal(appender.nodes)
			if err != nil {
				t.Fatal(err)
			}
			if root != test.Root || !jsonEqual(actualNodes, test.Nodes) {
				t.Fatalf("assembled nodes root=%d want=%d\nactual: %s\nwant:   %s", root, test.Root, actualNodes, test.Nodes)
			}
		})
	}
}

func decodeGoldenSNode(raw json.RawMessage) (snode, error) {
	var value any
	if err := json.Unmarshal(raw, &value); err != nil {
		return nil, err
	}
	return decodeGoldenValue(value), nil
}

func decodeGoldenValue(value any) snode {
	if number, ok := value.(float64); ok {
		return valueNode(number)
	}
	object := value.(map[string]any)
	rawArgs := object["args"].([]any)
	args := make([]snode, len(rawArgs))
	for i, arg := range rawArgs {
		args[i] = decodeGoldenValue(arg)
	}
	return functionNode{function: resource.RuntimeFunction(object["func"].(string)), args: args}
}

func encodeGoldenSNode(node snode) any {
	switch value := node.(type) {
	case valueNode:
		return float64(value)
	case functionNode:
		args := make([]any, len(value.args))
		for i, arg := range value.args {
			args[i] = encodeGoldenSNode(arg)
		}
		return map[string]any{"func": value.function, "args": args}
	default:
		return nil
	}
}

func jsonEqual(a, b []byte) bool {
	var av, bv any
	return json.Unmarshal(a, &av) == nil && json.Unmarshal(b, &bv) == nil && reflect.DeepEqual(av, bv)
}
