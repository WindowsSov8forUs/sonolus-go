// Package preview compiles and assembles Preview-mode engine data.
package preview

import (
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

// previewSetters maps each Preview callback name to its archetype field setter.
var previewSetters = map[string]func(*resource.EnginePreviewDataArchetype, int, int){
	"preprocess": func(a *resource.EnginePreviewDataArchetype, i, o int) { a.Preprocess = &resource.EnginePreviewDataArchetypeCallback{Index: i, Order: o} },
	"render":     func(a *resource.EnginePreviewDataArchetype, i, o int) { a.Render = &resource.EnginePreviewDataArchetypeCallback{Index: i, Order: o} },
}

// SetPreviewCallback is the modecompile.SetCallback for Preview mode, created from
// the previewSetters dispatch table.
var SetPreviewCallback = modecompile.NewCallbackSetter(previewSetters)
