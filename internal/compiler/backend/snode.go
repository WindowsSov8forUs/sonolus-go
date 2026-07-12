package backend

import (
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/WindowsSov8forUs/sonolus-core-go/core/resource"
)

type snode interface{ isSNode() }

type valueNode float64

func (valueNode) isSNode() {}

type functionNode struct {
	function resource.RuntimeFunction
	args     []snode
}

func (functionNode) isSNode() {}

func call(function resource.RuntimeFunction, args ...snode) snode {
	return functionNode{function: function, args: args}
}

type nodeAppender struct {
	nodes []resource.EngineDataNode
	cache map[string]int
}

func newNodeAppender() *nodeAppender {
	return &nodeAppender{cache: map[string]int{}}
}

func (a *nodeAppender) append(node snode) (int, error) {
	switch node := node.(type) {
	case valueNode:
		value := float64(node)
		key := "v:" + jsNumberString(value)
		if index, ok := a.cache[key]; ok {
			return index, nil
		}
		index := len(a.nodes)
		a.nodes = append(a.nodes, resource.EngineDataValueNode{Value: value})
		a.cache[key] = index
		return index, nil
	case functionNode:
		indexes := make([]int, len(node.args))
		for i, argument := range node.args {
			index, err := a.append(argument)
			if err != nil {
				return -1, err
			}
			indexes[i] = index
		}
		var key strings.Builder
		key.WriteString("f:")
		key.WriteString(string(node.function))
		for _, index := range indexes {
			key.WriteByte(':')
			key.WriteString(strconv.Itoa(index))
		}
		if index, ok := a.cache[key.String()]; ok {
			return index, nil
		}
		index := len(a.nodes)
		a.nodes = append(a.nodes, resource.EngineDataFunctionNode{Func: node.function, Args: indexes})
		a.cache[key.String()] = index
		return index, nil
	default:
		return -1, fmt.Errorf("backend: unknown SNode %T", node)
	}
}

func jsNumberString(value float64) string {
	if value == 0 {
		return "0"
	}
	if math.IsNaN(value) {
		return "NaN"
	}
	if math.IsInf(value, 1) {
		return "Infinity"
	}
	if math.IsInf(value, -1) {
		return "-Infinity"
	}
	abs := math.Abs(value)
	if abs >= 1e21 || abs < 1e-6 {
		formatted := strconv.FormatFloat(value, 'e', -1, 64)
		parts := strings.SplitN(formatted, "e", 2)
		exponent := parts[1]
		sign := "+"
		if strings.HasPrefix(exponent, "-") {
			sign = "-"
		}
		exponent = strings.TrimLeft(exponent, "+-0")
		if exponent == "" {
			exponent = "0"
		}
		return parts[0] + "e" + sign + exponent
	}
	return strconv.FormatFloat(value, 'f', -1, 64)
}

func execute(nodes ...snode) snode {
	if len(nodes) == 0 {
		return valueNode(0)
	}
	if len(nodes) == 1 {
		return nodes[0]
	}
	return call(resource.RuntimeFunctionExecute, nodes...)
}
