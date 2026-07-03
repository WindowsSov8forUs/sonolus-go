// Package ir is the compiler's mid-level intermediate representation: a
// statement-level CFG of BasicBlocks holding IR nodes. It is a Go port of
// sonolus.py's sonolus/backend (ir.py, place.py, optimize/flow.py, finalize.py).
//
// The IR is CFG-shaped and mutable (suited to SSA + optimization passes). It is
// distinct from snode.SNode, which is the final immutable node tree. finalize.go
// bridges the two: CFGToSNode lowers an optimized CFG into an snode.SNode.
package ir

import (
	"math"

	"github.com/WindowsSov8forUs/sonolus-core-go/core/resource"
)

// Runtime memory block IDs used by the Sonolus engine across all modes.
const (
	BlockRuntimeEnvironment = 1000
	BlockRuntimeUpdate      = 1001 // RuntimeCanvas in Preview mode
	BlockRuntimeTouch       = 1002
	BlockEngineRom          = 3000
	TouchFieldStride        = 9
)

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
// identifier used by liveness analysis.
type Instr struct {
	ID   int
	Op   Op
	Args []Node
	Pure bool
}

func (Instr) irNode() {}

// IDGen generates monotonic node identifiers for a single compilation. Each
// compilation entry point creates one IDGen and threads it through the frontend
// tracer, optimizer pipeline, and finalizer. This eliminates shared mutable state,
// making concurrent compilations safe.
type IDGen struct {
	n int // next ID to assign; monotonically incremented by Next()
}

// NewIDGen returns a fresh ID generator starting at 1.
func NewIDGen() *IDGen { return &IDGen{} }

// Next returns the next monotonic ID from this generator.
func (g *IDGen) Next() int { g.n++; return g.n }

// --- constructors ---

// PureInstr builds a side-effect-free operation node.
func (g *IDGen) PureInstr(op Op, args ...Node) Instr {
	return Instr{ID: g.Next(), Op: op, Args: args, Pure: true}
}

// ImpureInstr builds an operation node that may have side effects.
func (g *IDGen) ImpureInstr(op Op, args ...Node) Instr {
	return Instr{ID: g.Next(), Op: op, Args: args, Pure: false}
}

// Get reads a memory place (sonolus.py IRGet).
type Get struct {
	Place Place
}

func (Get) irNode() {}

// Set writes value to a memory place (sonolus.py IRSet). ID is a monotonic
// identifier used by liveness analysis.
type Set struct {
	ID    int
	Place Place
	Value Node
}

func (Set) irNode() {}

// SetPlace writes a value to a memory place.
func (g *IDGen) SetPlace(p Place, value Node) Set {
	return Set{ID: g.Next(), Place: p, Value: value}
}

// GetPlace reads a memory place.
func GetPlace(p Place) Get { return Get{Place: p} }

// FloorMod computes a floored modulo (result has the sign of the divisor),
// matching Python's % and the runtime RuntimeFunctionMod semantics.
// Used by frontend constant folding (value.go) and SCCP (sccp.go).
func FloorMod(a, b float64) float64 {
	r := math.Mod(a, b)
	if r != 0 && (r < 0) != (b < 0) {
		r += b
	}
	return r
}

// IEEERem computes IEEE 754 remainder, matching the runtime RuntimeFunctionRem semantics.
// Used by frontend constant folding and SCCP for compile-time evaluation.
func IEEERem(a, b float64) float64 {
	return math.Remainder(a, b)
}
