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
		key := "v:" + strconv.FormatUint(math.Float64bits(value), 16)
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

func execute(nodes ...snode) snode {
	if len(nodes) == 0 {
		return valueNode(0)
	}
	if len(nodes) == 1 {
		return nodes[0]
	}
	return call(resource.RuntimeFunctionExecute, nodes...)
}
