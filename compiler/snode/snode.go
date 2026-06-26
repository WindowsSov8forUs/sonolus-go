package snode

import "github.com/WindowsSov8forUs/sonolus-core-go/core/resource"

type SNode interface{ isSNode() }

type Value float64

func (v Value) isSNode() {}

type Func struct {
	Func resource.RuntimeFunction
	Args []SNode
}

func (f Func) isSNode() {}

func Val(v float64) Value { return Value(v) }
func Call(fn resource.RuntimeFunction, args ...SNode) Func {
	return Func{Func: fn, Args: args}
}
