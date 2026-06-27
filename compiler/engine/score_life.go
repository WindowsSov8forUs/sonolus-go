package engine

import (
	"github.com/WindowsSov8forUs/sonolus-core-go/core/resource"

	"github.com/WindowsSov8forUs/sonolus-go/compiler/frontend"
)

// ScoreLifeBindings returns per-entity Score and Life field bindings for an
// archetype. EntityScore is block 4006 (Play), EntityLife is block 4007 (Play).
// These are writable by updateSequential/touch callbacks (score adds, life
// deductions happen), read-only for others.
func ScoreLifeBindings(hasScore, hasLife bool) map[string]frontend.Binding {
	b := map[string]frontend.Binding{}
	if hasScore {
		b["entityPerfect"] = frontend.Binding{Block: 4006, Index: 0, Writable: true}
		b["entityGreat"] = frontend.Binding{Block: 4006, Index: 1, Writable: true}
		b["entityGood"] = frontend.Binding{Block: 4006, Index: 2, Writable: true}
		b["entityMiss"] = frontend.Binding{Block: 4006, Index: 3, Writable: true}
	}
	if hasLife {
		b["entityLifePerfect"] = frontend.Binding{Block: 4007, Index: 0, Writable: true}
		b["entityLifeGreat"] = frontend.Binding{Block: 4007, Index: 1, Writable: true}
		b["entityLifeGood"] = frontend.Binding{Block: 4007, Index: 2, Writable: true}
		b["entityLifeMiss"] = frontend.Binding{Block: 4007, Index: 3, Writable: true}
	}
	return b
}

// ArchetypeScoreLife merges ScoreLife bindings into an existing binding map.
func ArchetypeScoreLife(b map[string]frontend.Binding, hasScore, hasLife bool) {
	for k, v := range ScoreLifeBindings(hasScore, hasLife) {
		b[k] = v
	}
}

var _ = resource.EngineTutorialData{}
