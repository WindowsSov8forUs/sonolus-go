package snode

import (
	"strconv"

	"github.com/WindowsSov8forUs/sonolus-core-go/core/resource"
)

func intSliceKey(slice []int) string {
	if len(slice) == 0 {
		return ""
	}

	key := make([]byte, 0, len(slice)*4)
	for i, v := range slice {
		if i > 0 {
			key = append(key, ',')
		}
		key = strconv.AppendInt(key, int64(v), 10)
	}
	return string(key)
}

// Appender builds a flat EngineDataNode list from an SNode tree, deduplicating nodes.
type Appender struct {
	nodes *[]resource.EngineDataNode

	cache map[string]int
}

func NewAppender(nodes *[]resource.EngineDataNode) *Appender {
	return &Appender{
		nodes: nodes,
		cache: make(map[string]int),
	}
}

func (a *Appender) Append(snode SNode) (int, error) {
	switch v := snode.(type) {
	case Value:
		key := FormatNumber(float64(v))
		node := resource.EngineDataValueNode{
			Value: float64(v),
		}
		return a.appendNode(key, node), nil
	case Func:
		args := make([]int, len(v.Args))
		for i, arg := range v.Args {
			argIndex, err := a.Append(arg)
			if err != nil {
				return -1, err
			}
			args[i] = argIndex
		}

		key := a.funcKey(v.Op, args)
		node := resource.EngineDataFunctionNode{
			Func: v.Op,
			Args: args,
		}
		return a.appendNode(key, node), nil
	default:
		return -1, &ErrUnknownSNodeType{SNode: v}
	}
}

func (a *Appender) appendNode(key string, node resource.EngineDataNode) int {
	if index, ok := a.cache[key]; ok {
		return index
	}

	index := len(*a.nodes)
	*a.nodes = append(*a.nodes, node)
	a.cache[key] = index

	return index
}

func (a *Appender) funcKey(fn resource.RuntimeFunction, args []int) string {
	return string(fn) + ":" + intSliceKey(args)
}
