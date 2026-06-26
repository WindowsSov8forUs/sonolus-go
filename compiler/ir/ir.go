// Package ir is the compiler's mid-level intermediate representation: a
// statement-level CFG of BasicBlocks holding IR nodes. It is a Go port of
// sonolus.py's sonolus/backend (ir.py, place.py, optimize/flow.py, finalize.py).
//
// The IR is CFG-shaped and mutable (suited to SSA + optimization passes). It is
// distinct from snode.SNode, which is the final immutable node tree. finalize.go
// bridges the two: CFGToSNode lowers an optimized CFG into an snode.SNode.
package ir

import "github.com/WindowsSov8forUs/sonolus-core-go/core/resource"

// Op is an IR/runtime operation. It shares the runtime function vocabulary with
// the final node list.
type Op = resource.RuntimeFunction

// Node is any IR node that can be lowered to an snode.SNode.
type Node interface{ irNode() }

// Const is a numeric literal (sonolus.py IRConst). Non-finite values lower to
// ROM reads during finalization.
type Const float64

func (Const) irNode() {}

// Instr is an operation applied to argument nodes. Pure marks side-effect-free
// operations (sonolus.py distinguishes IRPureInstr from IRInstr); the flag is
// for optimization passes and does not affect lowering. ID is a monotonic
// identifier for use as a comparable map key in liveness analysis.
type Instr struct {
	ID   int
	Op   Op
	Args []Node
	Pure bool
}

func (Instr) irNode() {}

// nextID is a package-level monotonic counter for Instr IDs.
var nextID = new(int)

// NodeKey is a comparable key for any IR node. For Instr nodes, it uses the
// monotonic ID; for other types it uses the pointer address.
type NodeKey int64

// Key returns a comparable key for this node suitable for use in maps.
func (n Instr) Key() NodeKey { return NodeKey(n.ID) }

// incrID returns the next monotonic ID.
func incrID() int { *nextID++; return *nextID }

// Get reads a memory place (sonolus.py IRGet).
type Get struct {
	Place Place
}

func (Get) irNode() {}

// Set writes value to a memory place (sonolus.py IRSet).
type Set struct {
	Place Place
	Value Node
}

func (Set) irNode() {}

// --- constructors ---

// PureInstr builds a side-effect-free operation node.
func PureInstr(op Op, args ...Node) Instr {
	return Instr{ID: incrID(), Op: op, Args: args, Pure: true}
}

// ImpureInstr builds an operation node that may have side effects.
func ImpureInstr(op Op, args ...Node) Instr {
	return Instr{ID: incrID(), Op: op, Args: args, Pure: false}
}

// GetPlace reads a memory place.
func GetPlace(p Place) Get { return Get{Place: p} }

// SetPlace writes a value to a memory place.
func SetPlace(p Place, value Node) Set { return Set{Place: p, Value: value} }
