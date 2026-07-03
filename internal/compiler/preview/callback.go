// Package preview compiles and assembles Preview-mode engine data.
package preview

import (
	"github.com/WindowsSov8forUs/sonolus-core-go/core/resource"
)

// Callback is a preview-mode archetype callback name.
type Callback string

const (
	CallbackPreprocess Callback = "preprocess"
	CallbackRender     Callback = "render"
)

// Setters maps each Preview callback name to its archetype field setter.
var Setters = map[string]func(*resource.EnginePreviewDataArchetype, int, int){
	"preprocess": func(a *resource.EnginePreviewDataArchetype, i, o int) {
		a.Preprocess = &resource.EnginePreviewDataArchetypeCallback{Index: i, Order: o}
	},
	"render": func(a *resource.EnginePreviewDataArchetype, i, o int) {
		a.Render = &resource.EnginePreviewDataArchetypeCallback{Index: i, Order: o}
	},
}
