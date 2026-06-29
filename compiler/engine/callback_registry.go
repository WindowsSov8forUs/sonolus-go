// Package engine — callback registry shared across all four engine modes.
package engine

import (
	"github.com/WindowsSov8forUs/sonolus-go/compiler/play"
	"github.com/WindowsSov8forUs/sonolus-go/compiler/preview"
	"github.com/WindowsSov8forUs/sonolus-go/compiler/watch"
)

// CallbackRegistry maps Go method names to mode-specific callback enum strings.
// Each mode defines one package-level registry; the generic [modeAssembler]
// references the registry to classify archetype methods into callbacks vs helpers.
type CallbackRegistry struct {
	Names map[string]string // Go method name → callback enum string
}

// playCallbacks maps Play-mode archetype method names to their callback enum values.
var playCallbacks = &CallbackRegistry{Names: map[string]string{
	"Preprocess":       string(play.CallbackPreprocess),
	"SpawnOrder":       string(play.CallbackSpawnOrder),
	"ShouldSpawn":      string(play.CallbackShouldSpawn),
	"Initialize":       string(play.CallbackInitialize),
	"UpdateSequential": string(play.CallbackUpdateSequential),
	"Touch":            string(play.CallbackTouch),
	"UpdateParallel":   string(play.CallbackUpdateParallel),
	"Terminate":        string(play.CallbackTerminate),
}}

// watchCallbacks maps Watch-mode archetype method names to their callback enum values.
var watchCallbacks = &CallbackRegistry{Names: map[string]string{
	"Preprocess":       string(watch.CallbackPreprocess),
	"SpawnTime":        string(watch.CallbackSpawnTime),
	"DespawnTime":      string(watch.CallbackDespawnTime),
	"Initialize":       string(watch.CallbackInitialize),
	"UpdateSequential": string(watch.CallbackUpdateSequential),
	"UpdateParallel":   string(watch.CallbackUpdateParallel),
	"Terminate":        string(watch.CallbackTerminate),
}}

// previewCallbacks maps Preview-mode archetype method names to their callback enum values.
var previewCallbacks = &CallbackRegistry{Names: map[string]string{
	"Preprocess": string(preview.CallbackPreprocess),
	"Render":     string(preview.CallbackRender),
}}
