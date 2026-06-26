package play

import (
	"fmt"

	"github.com/WindowsSov8forUs/sonolus-core-go/core/resource"

	"github.com/WindowsSov8forUs/sonolus-go/compiler/snode"
)

// Assemble folds compiled callbacks into the play-data skeleton: each callback's
// SNode tree is appended (with shared dedup) to data.Nodes, and the resulting
// root index + order is recorded on the owning archetype. Mirrors the assemble
// step in sonolus.js-compiler (build/play/assemble.ts).
//
// nil results are skipped. The order of results determines node indices.
func Assemble(data *resource.EnginePlayData, results []*CompileResult) error {
	app := snode.NewAppender(&data.Nodes)

	for _, c := range results {
		if c == nil {
			continue
		}
		if c.ArchetypeIndex < 0 || c.ArchetypeIndex >= len(data.Archetypes) {
			return fmt.Errorf("assemble: archetype index %d out of range", c.ArchetypeIndex)
		}

		index, err := app.Append(c.Node)
		if err != nil {
			return fmt.Errorf("assemble: archetype %d callback %s: %w", c.ArchetypeIndex, c.Callback, err)
		}

		cb := resource.EnginePlayDataArchetypeCallback{Index: index, Order: c.Order}
		if err := setPlayCallback(&data.Archetypes[c.ArchetypeIndex], c.Callback, cb); err != nil {
			return err
		}
	}

	return nil
}

// setPlayCallback assigns a compiled callback to the matching typed field of an
// archetype.
func setPlayCallback(arch *resource.EnginePlayDataArchetype, cb Callback, value resource.EnginePlayDataArchetypeCallback) error {
	v := value
	switch cb {
	case CallbackPreprocess:
		arch.Preprocess = &v
	case CallbackSpawnOrder:
		arch.SpawnOrder = &v
	case CallbackShouldSpawn:
		arch.ShouldSpawn = &v
	case CallbackInitialize:
		arch.Initialize = &v
	case CallbackUpdateSequential:
		arch.UpdateSequential = &v
	case CallbackTouch:
		arch.Touch = &v
	case CallbackUpdateParallel:
		arch.UpdateParallel = &v
	case CallbackTerminate:
		arch.Terminate = &v
	default:
		return fmt.Errorf("assemble: unknown play callback %q", cb)
	}
	return nil
}
