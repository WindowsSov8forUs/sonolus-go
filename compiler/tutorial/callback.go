// Package tutorial compiles and assembles Tutorial-mode engine data. Unlike
// play/watch/preview, tutorial has no archetypes — it has three global callbacks
// (Preprocess, Navigate, Update) that operate on global state.
package tutorial

import (
	"github.com/WindowsSov8forUs/sonolus-go/compiler/modecompile"
)

// Callback is a tutorial-mode callback name.
type Callback string

const (
	CallbackPreprocess Callback = "preprocess"
	CallbackNavigate   Callback = "navigate"
	CallbackUpdate     Callback = "update"
)

func init() {
	modecompile.RegisterModeOmit("tutorial", nil)
}
