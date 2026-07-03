// Package snode defines the final SNode representation — the output layer
// of the compiler pipeline. SNode trees are serialized directly into
// EngineDataNodes for the Sonolus runtime. This package also provides
// tree optimization (shared-node dedup via Appender) before emission.
package snode

import "github.com/WindowsSov8forUs/sonolus-core-go/core/resource"

// SNode is the final immutable node tree emitted by the compiler. It is either a
// literal Value or a Func (runtime-function call with SNode arguments).
type SNode interface{ isSNode() }

// Value is a numeric literal in the final node tree.
type Value float64

func (v Value) isSNode() {}

// Func is a runtime-function call with SNode arguments in the final node tree.
type Func struct {
	Op   resource.RuntimeFunction
	Args []SNode
}

func (f Func) isSNode() {}

// Val creates a constant number Value node.
func Val(v float64) Value { return Value(v) }

// Call creates a function call Func node with the given runtime function and arguments.
func Call(op resource.RuntimeFunction, args ...SNode) Func {
	return Func{Op: op, Args: args}
}
