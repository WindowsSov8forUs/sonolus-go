package modecompile

import (
	"strings"

	"github.com/WindowsSov8forUs/sonolus-core-go/core/resource"

	"github.com/WindowsSov8forUs/sonolus-go/internal/compiler/snode"
)

// Val is a test helper that creates a numeric SNode value.
func Val(v float64) snode.Value { return snode.Val(v) }

// Get is a test helper that creates an opaque Get Func node the optimizer
// leaves untouched.
func Get(i float64) snode.Func {
	return snode.Call(resource.RuntimeFunctionGet, Val(1000), Val(i))
}

// Exec is a test helper that creates an Execute Func node.
func Exec(args ...snode.SNode) snode.Func {
	return snode.Call(resource.RuntimeFunctionExecute, args...)
}

// Canon returns a canonical string representation of an SNode tree for test
// assertions. Values are printed as "#<num>" and Funcs as "Name(args...)".
func Canon(n snode.SNode) string {
	switch t := n.(type) {
	case snode.Value:
		return "#" + snode.FormatNumber(float64(t))
	case snode.Func:
		ps := make([]string, len(t.Args))
		for i, a := range t.Args {
			ps[i] = Canon(a)
		}
		return string(t.Op) + "(" + strings.Join(ps, ",") + ")"
	}
	return "?" // unreachable: snode.SNode is sealed to Value | Func
}
