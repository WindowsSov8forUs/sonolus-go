// Operation classification tables for the ir package: which RuntimeFunction
// operations are pure (side-effect-free and deterministic in their arguments)
// and which have side effects. Generated from sonolus.py backend/ops.py.
//
//go:generate go run ./generate/main.go

package ir

// SideEffects reports whether an operation has observable side effects.
// The classification is generated from sonolus.py's Op.side_effects.
func SideEffects(op Op) bool { return sideEffectOps[op] }

// Pure reports whether an operation is pure (side-effect-free and deterministic
// in its arguments). The classification is generated from sonolus.py's Op.pure.
func Pure(op Op) bool { return pureOps[op] }
