// Package tutorial compiles and assembles Tutorial-mode engine data. Unlike
// play/watch/preview, tutorial has no archetypes — it has three global callbacks
// (Preprocess, Navigate, Update) that operate on global state.
package tutorial

import (
	"github.com/WindowsSov8forUs/sonolus-go/compiler/modecompile"
	"github.com/WindowsSov8forUs/sonolus-go/compiler/snode"
)

// Callback is a tutorial-mode callback name.
type Callback string

const (
	CallbackPreprocess Callback = "preprocess"
	CallbackNavigate   Callback = "navigate"
	CallbackUpdate     Callback = "update"
)

// CompileCallback optimizes one tutorial callback's SNode tree. Tutorial has no
// value-callback omission rules, but CompileCallback still runs peephole
// optimization and the general pure-constant/trailing-zero stripping rules.
func CompileCallback(archetypeIndex int, cb Callback, node snode.SNode) *modecompile.Result {
	return modecompile.CompileCallback(archetypeIndex, string(cb), node, nil)
}
