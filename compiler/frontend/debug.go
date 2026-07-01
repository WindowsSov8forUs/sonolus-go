package frontend

import (
	"github.com/WindowsSov8forUs/sonolus-core-go/core/resource"
	"github.com/WindowsSov8forUs/sonolus-go/compiler/ir"
)

// debugError compiles a runtime error: emits DebugLog for the message, then
// DebugPause, then terminates the current block (no further statements will be
// emitted). Corresponds to sonolus.py error() in debug.py.
func (t *tracer) debugError(msg ir.Node) {
	t.emit(t.gen.ImpureInstr(resource.RuntimeFunctionDebugLog, msg))
	t.emit(t.gen.ImpureInstr(resource.RuntimeFunctionDebugPause))
	t.terminated = true
}

// debugRequire compiles a runtime assertion: if cond is false (0), calls
// debugError with the given message. If cond is a compile-time constant that
// is truthy, emits nothing. A dynamic condition produces a two-way CFG branch.
// Corresponds to sonolus.py require() in debug.py.
func (t *tracer) debugRequire(cond, msg Num) {
	if cond.isConst {
		if cond.c != 0 {
			return // condition is truthy → no-op
		}
		// Condition is statically false → unconditional error.
		msgNode, err := msg.Node()
		if err != nil {
			// msg should always be a string literal; fall back to a generic message.
			msgNode = ir.Const(0)
		}
		t.debugError(msgNode)
		return
	}

	// Dynamic condition → CFG branch.
	condBlock := t.current
	condBlock.Test = cond.mustNode()

	errorBlock := ir.NewBlock()
	merge := ir.NewBlock()

	condBlock.ConnectTo(merge, nil)             // true → continue normally
	condBlock.ConnectTo(errorBlock, ir.Cond(0)) // false → error

	t.enter(errorBlock)
	msgNode, err := msg.Node()
	if err != nil {
		msgNode = ir.Const(0)
	}
	t.debugError(msgNode)
	// errorBlock is now terminated; no fallthrough edge added.

	t.enter(merge)
	// merge may have only the true-branch edge if errorBlock is dead-ended.
	t.terminated = len(merge.Incoming) == 0
}
