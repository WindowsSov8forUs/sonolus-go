package engine

import (
	"github.com/WindowsSov8forUs/sonolus-go/compiler/frontend"
	"github.com/WindowsSov8forUs/sonolus-go/compiler/ir"
)

// scoreLifeBindings returns per-entity Score and Life field bindings for an
// archetype. EntityScore is block 4006 (Play), EntityLife is block 4007 (Play).
// These are writable by updateSequential/touch callbacks (score adds, life
// deductions happen), read-only for others.
func scoreLifeBindings(hasScore, hasLife bool) map[string]frontend.Binding {
	b := map[string]frontend.Binding{}
	if hasScore {
		b["entityPerfect"] = frontend.Binding{Block: ir.BlockEntityScore, Index: 0, Writable: true}
		b["entityGreat"] = frontend.Binding{Block: ir.BlockEntityScore, Index: 1, Writable: true}
		b["entityGood"] = frontend.Binding{Block: ir.BlockEntityScore, Index: 2, Writable: true}
		b["entityMiss"] = frontend.Binding{Block: ir.BlockEntityScore, Index: 3, Writable: true}
	}
	if hasLife {
		b["entityLifePerfect"] = frontend.Binding{Block: ir.BlockEntityLife, Index: 0, Writable: true}
		b["entityLifeGreat"] = frontend.Binding{Block: ir.BlockEntityLife, Index: 1, Writable: true}
		b["entityLifeGood"] = frontend.Binding{Block: ir.BlockEntityLife, Index: 2, Writable: true}
		b["entityLifeMiss"] = frontend.Binding{Block: ir.BlockEntityLife, Index: 3, Writable: true}
	}
	return b
}

// ArchetypeScoreLife merges ScoreLife bindings into an existing binding map.
func ArchetypeScoreLife(b map[string]frontend.Binding, hasScore, hasLife bool) {
	for k, v := range scoreLifeBindings(hasScore, hasLife) {
		b[k] = v
	}
}
