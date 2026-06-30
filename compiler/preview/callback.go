// Package preview compiles and assembles Preview-mode engine data.
package preview

import (
	"fmt"

	"github.com/WindowsSov8forUs/sonolus-core-go/core/resource"

	"github.com/WindowsSov8forUs/sonolus-go/compiler/modecompile"
	"github.com/WindowsSov8forUs/sonolus-go/compiler/snode"
)

// Callback is a preview-mode archetype callback name.
type Callback string

const (
	CallbackPreprocess Callback = "preprocess"
	CallbackRender     Callback = "render"
)

// CompileCallback optimizes one archetype callback's SNode tree with
// preview omission rules (no value-callback checks).
func CompileCallback(archetypeIndex int, cb Callback, node snode.SNode) *modecompile.Result {
	// preview has no value-callback omission rules.
	return modecompile.CompileCallback(archetypeIndex, string(cb), node, nil)
}

// SetPreviewCallback assigns a compiled callback index to the matching typed field
// on a Preview-mode archetype. It satisfies modecompile.SetCallback[*EnginePreviewDataArchetype].
func SetPreviewCallback(arch *resource.EnginePreviewDataArchetype, cb string, index int) error {
	value := resource.EnginePreviewDataArchetypeCallback{Index: index}
	switch Callback(cb) {
	case CallbackPreprocess:
		arch.Preprocess = &value
	case CallbackRender:
		arch.Render = &value
	default:
		return fmt.Errorf("assemble: unknown preview callback %q", cb)
	}
	return nil
}
